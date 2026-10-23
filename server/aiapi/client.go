// Package aiapi AI 端点调用封装（需求文档 §6.2 / §7）。
//
// 设计要点：
//   - 所有 AI 请求经后端发起，前端不持有任何凭据；
//   - 凭据经 crypto.SecretSource 注入（环境变量或 0600 权限文件）；
//   - 按 ProviderHint 输出一次性合规提示（不阻断调用）；
//   - 记录调用元数据（时间戳/归属/状态码/错误类型），不记录用户输入正文。
package aiapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"nvs-server/crypto"
	"nvs-server/utils"
)

// ProviderHint AI 端点归属标注（需求文档 §7.1）。
type ProviderHint string

const (
	// HintDomestic 境内模型端点。
	HintDomestic ProviderHint = "domestic"
	// HintOverseas 境外模型端点。
	HintOverseas ProviderHint = "overseas"
	// HintEmpty 未标注（空串）。
	HintEmpty ProviderHint = ""
)

// 错误类型（需求文档 §7.3）。
const (
	ErrTypeAuth         = "auth"
	ErrTypeRateLimit    = "rate_limit"
	ErrTypeTimeout      = "timeout"
	ErrTypeContentFilter = "content_filter"
	ErrTypeNetwork      = "network"
	ErrTypeUnknown      = "unknown"
)

// Client 框架级 AI 端点客户端（需求文档 §6.2）。
type Client struct {
	// Endpoint 上游端点（必填，无默认值）。
	Endpoint string
	// Secret 凭据来源（密钥隔离，不硬编码）。
	Secret crypto.SecretSource
	// KeyEnvName 凭据名称（从 Secret 读取）。
	KeyEnvName string
	// ProviderHint 端点归属标注（domestic/overseas/空）。
	ProviderHint ProviderHint
	// LogRetentionDays 元数据日志保留天数（默认 30）。
	LogRetentionDays int
	// HTTPClient 可注入的 HTTP 客户端（超时/连接池由调用方控制）。
	HTTPClient *http.Client
}

// Request AI 调用请求。
type Request struct {
	// Body 请求体（已序列化或由 Payload 提供）。
	Body []byte
	// Payload 结构化负载（Body 为空时序列化）。
	Payload any
	// Path 端点路径（空 = Endpoint 本身）。
	Path string
}

// Response AI 调用响应。
type Response struct {
	StatusCode int
	Body       []byte
}

// Do 发起一次 AI 调用（需求文档 §6.2）。
// 首次调用前按 ProviderHint 输出一次性合规提示；记录元数据日志（不含正文）。
func (c *Client) Do(ctx context.Context, req Request) (*Response, error) {
	if c.Endpoint == "" {
		return nil, fmt.Errorf("AI 端点未配置（Endpoint 必填）")
	}
	noticeOnce(string(c.ProviderHint))

	body := req.Body
	if len(body) == 0 && req.Payload != nil {
		enc, err := json.Marshal(req.Payload)
		if err != nil {
			return nil, fmt.Errorf("序列化请求负载失败: %w", err)
		}
		body = enc
	}

	url := c.Endpoint
	if req.Path != "" {
		url = strings.TrimRight(c.Endpoint, "/") + "/" + strings.TrimLeft(req.Path, "/")
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		logCallMeta(c.ProviderHint, 0, ErrTypeNetwork)
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.Secret != nil && c.KeyEnvName != "" {
		key, err := c.Secret.Get(ctx, c.KeyEnvName)
		if err != nil {
			return nil, fmt.Errorf("获取 API Key 失败: %w", err)
		}
		httpReq.Header.Set("Authorization", "Bearer "+key)
	}

	hc := c.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 120 * time.Second}
	}
	resp, err := hc.Do(httpReq)
	if err != nil {
		logCallMeta(c.ProviderHint, 0, classifyHTTPErr(err))
		return nil, fmt.Errorf("AI 调用失败: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		logCallMeta(c.ProviderHint, resp.StatusCode, ErrTypeNetwork)
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	if resp.StatusCode >= 400 {
		logCallMeta(c.ProviderHint, resp.StatusCode, classifyStatus(resp.StatusCode))
		return nil, fmt.Errorf("AI 端点返回 %d", resp.StatusCode)
	}
	logCallMeta(c.ProviderHint, resp.StatusCode, "")
	return &Response{StatusCode: resp.StatusCode, Body: respBody}, nil
}

// ClassifyError 将调用错误归类（需求文档 §7.3）：auth/rate_limit/timeout/content_filter/network。
func ClassifyError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "401") || strings.Contains(msg, "403") || strings.Contains(msg, "鉴权") || strings.Contains(msg, "API Key"):
		return ErrTypeAuth
	case strings.Contains(msg, "429") || strings.Contains(msg, "rate"):
		return ErrTypeRateLimit
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "超时") || strings.Contains(msg, "deadline"):
		return ErrTypeTimeout
	case strings.Contains(msg, "content_filter") || strings.Contains(msg, "敏感") || strings.Contains(msg, "risk"):
		return ErrTypeContentFilter
	case strings.Contains(msg, "connection") || strings.Contains(msg, "dial") || strings.Contains(msg, "network"):
		return ErrTypeNetwork
	}
	return ErrTypeUnknown
}

// classifyStatus HTTP 状态码 → 错误类型。
func classifyStatus(code int) string {
	switch {
	case code == 401 || code == 403:
		return ErrTypeAuth
	case code == 429:
		return ErrTypeRateLimit
	case code >= 500:
		return ErrTypeNetwork
	}
	return ErrTypeUnknown
}

func classifyHTTPErr(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline") {
		return ErrTypeTimeout
	}
	return ErrTypeNetwork
}

// ==================== 合规提示（一次性） ====================

var (
	noticeMu   sync.Mutex
	noticedSet = map[ProviderHint]bool{}
)

// NoticeForHint 返回指定归属的合规提示（级别, 文案）——需求文档 §7.2 对照表。
func NoticeForHint(hint ProviderHint) (level, message string) {
	switch hint {
	case HintOverseas:
		return "WARN", "请求数据将传输至境外，可能触发数据出境合规义务；建议优先境内已备案模型。"
	case HintDomestic:
		return "INFO", "若平台具有舆论属性，站长需评估生成式 AI 算法备案/大模型登记义务。"
	default:
		return "WARN", "未标注端点归属，无法提供合规提示，请站长自行判断数据流向。"
	}
}

// NoticeOnce 首次使用某归属端点时输出合规提示（每种归属一次，不阻断调用）。
// 供模型池等既有 AI 路径复用同一套提示逻辑（需求文档 §7.2）。
func NoticeOnce(hint ProviderHint) {
	noticeOnce(string(hint))
}

// noticeOnce 每种归属仅提示一次（不阻断调用）。
func noticeOnce(hint string) {
	noticeMu.Lock()
	defer noticeMu.Unlock()
	if noticedSet[ProviderHint(hint)] {
		return
	}
	noticedSet[ProviderHint(hint)] = true
	level, msg := NoticeForHint(ProviderHint(hint))
	line := fmt.Sprintf("[ai-hint] provider_hint=%q level=%s %s", hint, level, msg)
	_ = utils.WriteDailyLog("ai_meta", line)
}

// logCallMeta 记录调用元数据（不含用户输入正文，需求文档 §7.3）。
func logCallMeta(hint ProviderHint, status int, errType string) {
	line := fmt.Sprintf(`{"ts":%d,"provider_hint":%q,"status":%d,"error_type":%q}`,
		time.Now().Unix(), string(hint), status, errType)
	_ = utils.WriteDailyLog("ai_meta", line)
}
