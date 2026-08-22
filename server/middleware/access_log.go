package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"time"

	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// AccessLog 请求日志中间件（176号令合规：上网日志关联用户标识）。
// 全量记录每一条 Web 请求（包括静态资源、404、OPTIONS 预检、鉴权失败等）：
// 时间戳、user_id、method、path、query、请求头、请求体（脱敏+64KB截断）、
// 响应状态码、响应大小、耗时、客户端 IP、User-Agent。
// 按小时写入 log/requests/<YYYY-MM-DD>/<HH>.log（request 一个文件夹，
// 每天内按小时切分；追加写，重启不覆盖）。
//
// 注意：本中间件为全局中间件（在 r.Use 中注册，先于所有路由），
// 在 AuthRequired 之前执行；通过 c.Next() 后再取 user_id，
// 因此能拿到后续鉴权中间件写入的登录态。
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 读取请求体（带上限保护，避免大文件如上传占用内存），并完整还原以便后续 handler 复用。
		// ⚠️ 历史 bug：曾用 64KB LimitReader 读取后再整体还原 → 超过 64KB 的 JSON 正文
		// （长章节/长博客）被截断成非法 JSON，导致 handler 的 ShouldBindJSON 报
		// 「请填写章节标题和内容 / 参数错误」（用户明明填了标题却保存失败）。
		// 修复：只缓冲前 maxBodyReadCap 字节用于日志取样，未读完部分保留在原 Body 流中，
		// 用 io.MultiReader 将「已读部分 + 剩余流」拼接还原 —— 任意大小的正文都能完整传给 handler。
		// ⚠️ multipart/form-data（文件上传）请求绝不读取 body——读取再还原会截断大文件，
		// 导致后续 gin 解析 multipart 报「boundary 缺失」（历史 bug：上传 >64KB 必失败）。
		const maxBodyReadCap = 4 * 1024 * 1024 // 4MB：日志取样上限（仅缓冲此大小）
		var bodySnippet string
		ct := c.Request.Header.Get("Content-Type")
		if strings.HasPrefix(strings.ToLower(ct), "multipart/form-data") {
			bodySnippet = "[multipart/form-data 文件上传]"
		} else if c.Request.Body != nil {
			raw, err := io.ReadAll(io.LimitReader(c.Request.Body, maxBodyReadCap))
			if err == nil {
				// 完整还原：已读部分 + 未读剩余部分（MultiReader 先读完 raw 再续读原 Body）
				c.Request.Body = io.NopCloser(io.MultiReader(bytes.NewReader(raw), c.Request.Body))
				bodySnippet = redactSensitive(string(raw))
			}
		}
		// 日志只保留前 64KB，防止超长正文撑爆日志（脱敏已在 redactSensitive 内完成）
		if len(bodySnippet) > 64*1024 {
			bodySnippet = bodySnippet[:64*1024] + "...(truncated)"
		}

		c.Next()

		userID := UserID(c)
		status := c.Writer.Status()
		size := c.Writer.Size()
		if size < 0 {
			size = 0
		}

		headers := map[string]string{}
		for k, v := range c.Request.Header {
			// 脱敏 Authorization，避免令牌落盘
			if strings.EqualFold(k, "Authorization") {
				headers[k] = "<redacted>"
				continue
			}
			headers[k] = strings.Join(v, "; ")
		}

		entry := map[string]interface{}{
			"timestamp": time.Now().Format("2006-01-02 15:04:05"),
			"user_id":   userID,
			"method":    c.Request.Method,
			"path":      c.Request.URL.Path,
			"query":     c.Request.URL.RawQuery,
			"ip":        c.ClientIP(),
			"ua":        c.Request.UserAgent(),
			"status":    status,
			"size":      size,
			"latency":   time.Since(start).String(),
			"headers":   headers,
			"body":      bodySnippet,
		}

		if raw, err := json.Marshal(entry); err == nil {
			_ = utils.WriteDailyLog("requests", string(raw))
		}
	}
}

// redactSensitive 对请求体做最小必要脱敏：屏蔽密码、令牌等敏感字段，避免明文落盘。
func redactSensitive(body string) string {
	if body == "" {
		return ""
	}
	// 仅对 JSON 结构做键值脱敏，非 JSON 直接截断返回（防止把正文全文记下）
	if !strings.HasPrefix(strings.TrimSpace(body), "{") {
		if len(body) > 512 {
			return body[:512] + "...(truncated)"
		}
		return body
	}
	dec := json.NewDecoder(strings.NewReader(body))
	dec.UseNumber()
	var data map[string]interface{}
	if err := dec.Decode(&data); err != nil {
		if len(body) > 512 {
			return body[:512] + "...(truncated)"
		}
		return body
	}
	sensitive := map[string]bool{
		"password": true, "password_confirm": true, "old_password": true,
		"new_password": true, "token": true, "access_token": true,
		"refresh_token": true, "secret": true, "auth_code": true,
	}
	for k, v := range data {
		if sensitive[k] {
			switch val := v.(type) {
			case string:
				if val != "" {
					data[k] = "******"
				}
			default:
				data[k] = "******"
			}
		}
	}
	out, err := json.Marshal(data)
	if err != nil {
		return body
	}
	return string(out)
}
