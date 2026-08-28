package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

	"nvs-server/security"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== API 聚合中间层（Batch） ====================
//
// 设计目标：
//   - 浏览器通过一次 POST /api/batch 批量获取多个 API 的数据，减少请求数量与网络开销。
//   - 服务器并发（goroutine）执行各子请求，一次性返回全部结果。
//   - 完全复用现有路由与中间件（认证/限流），内存内转发，无真实网络开销。
//   - 向后兼容：原有各 API 完全不变，仍可独立直接调用。
//
// 请求格式：
//
//	POST /api/batch
//	{
//	  "requests": [
//	    { "id": "novels",  "method": "GET",  "path": "/api/novels", "query": {"page":"1"} },
//	    { "id": "cats",    "method": "GET",  "path": "/api/categories" },
//	    { "id": "search",  "method": "POST", "path": "/api/search/authors", "body": {"q":"星"} }
//	  ],
//	  "timeout_ms": 10000
//	}
//
// 响应格式：
//
//	{
//	  "code": 0,
//	  "message": "success",
//	  "data": {
//	    "results": [
//	      { "id": "novels", "status": 200, "code": 0, "message": "success", "data": {...}, "success": true, "error": "" },
//	      ...
//	    ],
//	    "total": 3, "succeeded": 2, "failed": 1
//	  }
//	}
//
// 错误处理策略：
//   - 每个子请求独立返回结果（success 标记），互不影响。
//   - 子请求失败（4xx/5xx、超时、非法参数）→ success=false + error 说明原因，不影响其它子请求。
//   - 聚合接口本身始终返回 200，便于浏览器统一处理部分失败。
//
// 并发与超时：
//   - 所有子请求用 goroutine 并发执行，sync.WaitGroup 等待，结果按请求顺序返回。
//   - 整体超时由 timeout_ms 控制（默认 10s，上限 30s），超时的子请求标记为失败。

const (
	batchMaxRequests = 20    // 单次聚合最多子请求数
	batchDefaultMS   = 10000 // 默认整体超时
	batchMaxMS       = 30000 // 整体超时上限
)

// BatchEngine 由 main.go 注入的 gin 引擎，供聚合接口内存内转发复用路由/中间件
var BatchEngine *gin.Engine

// BatchRequest 聚合子请求
type BatchRequest struct {
	ID     string            `json:"id"`     // 可选，用于关联响应，缺省用索引
	Method string            `json:"method"` // 可选，默认 GET
	Path   string            `json:"path"`   // 必填，如 /api/novels
	Query  map[string]string `json:"query"`  // 可选，URL query 参数
	Body   json.RawMessage   `json:"body"`   // 可选，POST 的 JSON body
}

// BatchResult 聚合子响应
type BatchResult struct {
	ID      string          `json:"id"`
	Status  int             `json:"status"`  // 子请求 HTTP 状态码
	Code    int             `json:"code"`    // 业务 code（与独立调用一致）
	Message string          `json:"message"` // 业务 message
	Data    json.RawMessage `json:"data"`    // 业务 data（原始 JSON）
	Success bool            `json:"success"` // 是否成功
	Error   string          `json:"error"`   // 失败原因（成功时为空）
}

// BatchHandler POST /api/batch — API 聚合中间层
func BatchHandler(c *gin.Context) {
	var req struct {
		Requests  []BatchRequest `json:"requests"`
		TimeoutMs int            `json:"timeout_ms"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误：缺少 requests 数组")
		return
	}
	if len(req.Requests) == 0 {
		utils.BadRequest(c, "requests 不能为空")
		return
	}
	if len(req.Requests) > batchMaxRequests {
		utils.BadRequest(c, fmt.Sprintf("单次最多聚合 %d 个请求", batchMaxRequests))
		return
	}

	timeoutMs := req.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = batchDefaultMS
	}
	if timeoutMs > batchMaxMS {
		timeoutMs = batchMaxMS
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	engine := BatchEngine
	if engine == nil {
		utils.InternalError(c, "聚合服务未初始化")
		return
	}

	results := make([]BatchResult, len(req.Requests))
	var wg sync.WaitGroup

	// 并发执行所有子请求
	for i, br := range req.Requests {
		wg.Add(1)
		go func(idx int, br BatchRequest) {
			defer wg.Done()
			results[idx] = executeSubRequest(ctx, engine, c, br, idx)
		}(i, br)
	}
	wg.Wait()

	succeeded, failed := 0, 0
	for _, r := range results {
		if r.Success {
			succeeded++
		} else {
			failed++
		}
	}

	utils.Success(c, gin.H{
		"results":   results,
		"total":     len(results),
		"succeeded": succeeded,
		"failed":    failed,
	})
}

// executeSubRequest 在内存中执行单个子请求（复用 gin 路由与中间件）
func executeSubRequest(ctx context.Context, engine *gin.Engine, parent *gin.Context, br BatchRequest, idx int) BatchResult {
	result := BatchResult{ID: br.ID, Status: 0, Code: -1}
	if result.ID == "" {
		result.ID = fmt.Sprintf("%d", idx)
	}

	// 校验 method（白名单 GET/POST）
	method := strings.ToUpper(strings.TrimSpace(br.Method))
	if method == "" {
		method = http.MethodGet
	}
	if method != http.MethodGet && method != http.MethodPost {
		result.Error = "不支持的 method: " + method
		return result
	}

	// 校验 path：仅允许 /api/ 前缀，防任意路径访问与 SSRF
	path := strings.TrimSpace(br.Path)
	if path == "" {
		result.Error = "path 不能为空"
		return result
	}
	if !strings.HasPrefix(path, "/api/") {
		result.Error = "path 必须以 /api/ 开头"
		return result
	}
	if strings.Contains(path, "..") || strings.Contains(path, "\\") {
		result.Error = "非法路径"
		return result
	}
	// 禁止递归调用聚合接口
	if path == "/api/batch" || strings.HasPrefix(path, "/api/batch/") {
		result.Error = "聚合接口不支持嵌套调用"
		return result
	}

	// 构造 body
	var bodyReader io.Reader
	if method == http.MethodPost {
		if len(br.Body) > 0 {
			bodyReader = bytes.NewReader(br.Body)
		} else {
			bodyReader = bytes.NewReader([]byte("{}"))
		}
	}

	// 构造子请求
	subReq, err := http.NewRequestWithContext(ctx, method, path, bodyReader)
	if err != nil {
		result.Error = "构造请求失败: " + err.Error()
		return result
	}

	// 透传认证信息（Cookie/Authorization）与客户端 IP（供限流使用）
	subReq.Header = parent.Request.Header.Clone()
	// 内存内转发无需内容压缩：去掉 Accept-Encoding，避免 gzip 中间件压缩子响应，
	// 导致 recorder 拿到压缩字节、json.Unmarshal 解析失败（data 退化为乱码字符串，
	// 前端聚合数据全空 —— 首页分类/小说消失的根因；curl 不带 gzip 所以此前未暴露）。
	subReq.Header.Del("Accept-Encoding")
	subReq.RemoteAddr = parent.Request.RemoteAddr
	// 标记为内部聚合子请求：外层 /api/batch 已统一限流，子请求跳过重复计数（防伪造密钥）
	subReq.Header.Set(security.HeaderInternalBatch, security.InternalBatchSecret)
	if method == http.MethodPost {
		subReq.Header.Set("Content-Type", "application/json")
	}

	// 拼接 query 参数
	if len(br.Query) > 0 {
		q := subReq.URL.Query()
		for k, v := range br.Query {
			q.Set(k, v)
		}
		subReq.URL.RawQuery = q.Encode()
	}

	// 内存内转发执行
	recorder := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		engine.ServeHTTP(recorder, subReq)
		close(done)
	}()

	select {
	case <-done:
		// 正常完成，继续解析
	case <-ctx.Done():
		result.Error = "请求超时"
		return result
	}

	result.Status = recorder.Code
	body := recorder.Body.Bytes()

	// 解析统一响应结构 {code, message, data}
	var jsonResp struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &jsonResp); err != nil {
		// 非标准 JSON 响应（如文件下载、HTML）：原样作为 data 字符串返回
		if len(body) == 0 {
			result.Data = json.RawMessage("null")
		} else if b, e := json.Marshal(string(body)); e == nil {
			result.Data = b
		} else {
			result.Data = json.RawMessage("null")
		}
		result.Message = http.StatusText(result.Status)
		result.Success = result.Status >= 200 && result.Status < 300
		if !result.Success {
			result.Error = fmt.Sprintf("HTTP %d %s", result.Status, http.StatusText(result.Status))
		}
		return result
	}

	result.Code = jsonResp.Code
	result.Message = jsonResp.Message
	if jsonResp.Data != nil && len(jsonResp.Data) > 0 {
		result.Data = jsonResp.Data
	} else {
		result.Data = json.RawMessage("null")
	}
	// 成功判定：HTTP 2xx 且业务 code == 0
	result.Success = result.Status >= 200 && result.Status < 300 && jsonResp.Code == utils.CodeSuccess
	if !result.Success {
		result.Error = jsonResp.Message
		if result.Error == "" {
			result.Error = fmt.Sprintf("HTTP %d", result.Status)
		}
	}
	return result
}
