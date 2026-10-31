package aiclient

import "strings"

// NormalizeBaseURL 把站长粘贴进来的「具体接口地址」还原成站点根地址。
//
// 为什么需要：渠道监控里最容易出错的一步就是地址。站长往往直接从文档/浏览器
// 复制完整的接口地址，例如：
//
//	https://api.hcnsec.cn/api/user/self
//	https://api.hcnsec.cn/v1/models
//	https://api.openai.com/v1
//
// 而本程序内部是按「站点根 + 固定路径」拼的（root + /api/user/self、
// root + /v1/models）。不归一化就会拼成
// https://api.hcnsec.cn/api/user/self/api/user/self → 404，
// 面板还会误导成「没有 quota 字段，可能不是 New API 站点」。
//
// 只剥已知的接口后缀，不动路径中间的段，避免把自建站点的自定义前缀改坏。
func NormalizeBaseURL(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	s = strings.TrimRight(s, "/")

	// 从最长到最短：先剥完整接口路径，再剥 /v1 这类版本前缀
	suffixes := []string{
		"/api/user/self",
		"/api/log/self",
		"/api/user/self/",
		"/v1/chat/completions",
		"/v1/models",
		"/api/user",
		"/api/log",
		"/v1",
		"/models",
	}
	for {
		changed := false
		low := strings.ToLower(s)
		for _, suf := range suffixes {
			if strings.HasSuffix(low, suf) {
				s = strings.TrimRight(s[:len(s)-len(suf)], "/")
				changed = true
				break
			}
		}
		if !changed {
			break
		}
		// 剥完可能只剩协议头（如用户只填了 https://x.com/v1），避免继续剥
		if s == "" || strings.HasSuffix(s, "://") {
			break
		}
	}
	return s
}
