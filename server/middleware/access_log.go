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

		// 读取请求体（带上限保护，避免大文件如上传占用内存），并还原以便后续 handler 复用。
		// ⚠️ multipart/form-data（文件上传）请求绝不读取 body——读取再还原会截断大文件，
		// 导致后续 gin 解析 multipart 报「boundary 缺失」（历史 bug：上传 >64KB 必失败）。
		var bodySnippet string
		ct := c.Request.Header.Get("Content-Type")
		if strings.HasPrefix(strings.ToLower(ct), "multipart/form-data") {
			bodySnippet = "[multipart/form-data 文件上传]"
		} else if c.Request.Body != nil {
			raw, err := io.ReadAll(io.LimitReader(c.Request.Body, 64*1024))
			if err == nil {
				c.Request.Body = io.NopCloser(bytes.NewReader(raw))
				bodySnippet = redactSensitive(string(raw))
			}
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
