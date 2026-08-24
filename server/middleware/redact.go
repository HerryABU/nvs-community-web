package middleware

import (
	"encoding/json"
	"net/url"
	"strings"

	"nvs-server/config"
)

// ==================== 环境感知敏感信息脱敏（公安部令第176号合规） ====================
//
// 需求（NVS日志系统——环境敏感信息脱敏功能需求文档 v1.0）：
//   - 开发环境（LOG_REDACT_SENSITIVE=false）：保留完整日志数据，方便调试（F1）
//   - 生产环境（LOG_REDACT_SENSITIVE=true）：自动脱敏 Token/密码/敏感字段（F2）
//   - 密码类字段固定掩码 ******（3.2.1），其余敏感字段使用可配置掩码（默认 ***REDACTED***）
//   - 敏感字段列表可由管理后台动态增删（3.2.2），无需重启
//
// 性能考量（4.4）：仅作用于日志写入路径、不阻塞业务；字段匹配用小写 key 集合
// （strings.Contains 语义），避免正则开销；JSON 解析仅对 JSON 结构 body 执行（F6 非 JSON 保持不变）。

// Redactor 脱敏处理接口（需求 4.2）
type Redactor interface {
	// RedactHeaders 脱敏 headers 中的敏感字段（Authorization / Cookie / 自定义敏感头）
	RedactHeaders(headers map[string]string) map[string]string
	// RedactBody 脱敏 body 中的敏感字段（仅 JSON 结构，非 JSON 保持不变）
	RedactBody(body string) string
	// RedactQuery 脱敏 query 参数中的敏感字段（保持原参数顺序与格式）
	RedactQuery(query string) string
	// IsEnabled 判断是否启用脱敏
	IsEnabled() bool
}

// DefaultRedactor 基于 config.RedactConfig 的默认脱敏实现。
// 规则：
//   - password 及任意含 "password" 的字段（password_confirm / old_password / new_password…）
//     → 固定掩码 ******（内置，始终生效，3.2.1）
//   - 其余命中敏感列表的字段（token / secret / apikey / 后台自定义）→ 可配置掩码
//   - Authorization 头 → 保留 Bearer 前缀，令牌体替换为掩码
//   - Cookie 头 → 逐键值对脱敏（token=xxx → token=***REDACTED***，其余未命中键保持原样）
type DefaultRedactor struct {
	cfg  config.RedactConfig
	keys map[string]bool // 小写敏感字段集合（预编译，避免每次匹配遍历）
}

// NewRedactor 基于配置构造脱敏器（配置会自动规范化）
func NewRedactor(cfg config.RedactConfig) *DefaultRedactor {
	cfg.Normalize()
	r := &DefaultRedactor{cfg: cfg, keys: make(map[string]bool, len(cfg.SensitiveKeys))}
	for _, k := range cfg.SensitiveKeys {
		r.keys[strings.ToLower(k)] = true
	}
	return r
}

// IsEnabled 是否启用脱敏
func (r *DefaultRedactor) IsEnabled() bool { return r.cfg.Enabled }

// isSensitiveKey 判断字段名是否敏感：密码类字段内置固定敏感，其余查可配置列表。
func (r *DefaultRedactor) isSensitiveKey(key string) bool {
	lk := strings.ToLower(strings.TrimSpace(key))
	if strings.Contains(lk, "password") {
		return true // 密码类字段内置固定脱敏（3.2.1 固定脱敏字段），不允许通过配置关闭
	}
	return r.keys[lk]
}

// maskFor 返回字段对应的掩码：密码类 → ******，其余 → 可配置掩码
func (r *DefaultRedactor) maskFor(key string) string {
	if strings.Contains(strings.ToLower(key), "password") {
		return config.PasswordRedactMask
	}
	return r.cfg.Mask
}

// RedactHeaders 脱敏请求头（需求 3.3.1）
func (r *DefaultRedactor) RedactHeaders(headers map[string]string) map[string]string {
	if !r.cfg.Enabled || len(headers) == 0 {
		return headers
	}
	result := make(map[string]string, len(headers))
	for k, v := range headers {
		lower := strings.ToLower(k)
		switch {
		case lower == "authorization":
			// 保留 Bearer 前缀，令牌体替换为掩码；无前缀的裸令牌整体替换
			if idx := strings.Index(strings.ToLower(v), "bearer "); idx >= 0 {
				result[k] = v[:idx] + "Bearer " + r.cfg.Mask
			} else {
				result[k] = r.cfg.Mask
			}
		case lower == "cookie":
			result[k] = r.redactCookie(v)
		case r.isSensitiveKey(lower):
			// 自定义敏感头（如 apikey）整体替换
			result[k] = r.cfg.Mask
		default:
			result[k] = v
		}
	}
	return result
}

// redactCookie 脱敏 Cookie 中的敏感键值对（保留键名大小写与其他键值对）。
// 输入: "token=eyJ...; other=value" → 输出: "token=***REDACTED***; other=value"
func (r *DefaultRedactor) redactCookie(cookie string) string {
	parts := strings.Split(cookie, ";")
	for i, p := range parts {
		p = strings.TrimSpace(p)
		eq := strings.IndexByte(p, '=')
		if eq <= 0 {
			continue // 无键值对结构（如 HttpOnly 等标记）跳过
		}
		name := strings.TrimSpace(p[:eq])
		if r.isSensitiveKey(name) {
			parts[i] = p[:eq] + "=" + r.maskFor(name)
		}
	}
	return strings.Join(parts, ";")
}

// RedactBody 脱敏 JSON 请求体（需求 3.3.2）：递归遍历整棵 JSON 树，
// 命中敏感 key 的值替换为掩码；非 JSON / 非法 JSON 原样返回（F6，不报错）。
func (r *DefaultRedactor) RedactBody(body string) string {
	if !r.cfg.Enabled || body == "" {
		return body
	}
	trimmed := strings.TrimSpace(body)
	if trimmed == "" || (trimmed[0] != '{' && trimmed[0] != '[') {
		return body // 非 JSON 结构保持不变
	}
	dec := json.NewDecoder(strings.NewReader(body))
	dec.UseNumber()
	var data interface{}
	if err := dec.Decode(&data); err != nil {
		return body // 非法 JSON 原样返回，不 panic
	}
	r.redactJSONValue(&data)
	out, err := json.Marshal(data)
	if err != nil {
		return body
	}
	return string(out)
}

// redactJSONValue 递归遍历 JSON 树，敏感 key 替换值。
func (r *DefaultRedactor) redactJSONValue(val *interface{}) {
	switch v := (*val).(type) {
	case map[string]interface{}:
		for k, item := range v {
			if r.isSensitiveKey(k) {
				v[k] = r.maskFor(k)
				continue
			}
			ptr := item
			r.redactJSONValue(&ptr)
			v[k] = ptr
		}
	case []interface{}:
		for i := range v {
			r.redactJSONValue(&v[i])
		}
	}
}

// RedactQuery 脱敏 query 参数（需求 3.3.3）：按 & 拆分逐对处理，保持原顺序与格式，
// 仅替换命中敏感 key 的值；URL 解码后的 key 参与匹配。
func (r *DefaultRedactor) RedactQuery(query string) string {
	if !r.cfg.Enabled || query == "" {
		return query
	}
	parts := strings.Split(query, "&")
	for i, p := range parts {
		eq := strings.IndexByte(p, '=')
		name := p
		if eq >= 0 {
			name = p[:eq]
		}
		if unescaped, err := url.QueryUnescape(name); err == nil {
			name = unescaped
		}
		if !r.isSensitiveKey(name) {
			continue
		}
		// 掩码为平台自己的标记（非用户输入），原样拼接，不做 URL 转义（保证日志可读性）
		if eq >= 0 {
			parts[i] = p[:eq] + "=" + r.maskFor(name)
		} else {
			parts[i] = p + "=" + r.maskFor(name)
		}
	}
	return strings.Join(parts, "&")
}
