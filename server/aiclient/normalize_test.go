package aiclient

import "testing"

// TestNormalizeBaseURL 站长把「具体接口地址」粘进渠道地址时要能还原成站点根
func TestNormalizeBaseURL(t *testing.T) {
	cases := []struct{ in, want string }{
		// 实测踩过的坑：整个 /api/user/self 粘进来 → 拼成 .../api/user/self/api/user/self → 404
		{"https://api.hcnsec.cn/api/user/self", "https://api.hcnsec.cn"},
		{"https://api.hcnsec.cn/api/user/self/", "https://api.hcnsec.cn"},
		{"https://api.hcnsec.cn/api/log/self", "https://api.hcnsec.cn"},
		{"https://api.openai.com/v1/models", "https://api.openai.com"},
		{"https://api.openai.com/v1/chat/completions", "https://api.openai.com"},
		{"https://api.deepseek.com/v1", "https://api.deepseek.com"},
		{"https://api.example.com/", "https://api.example.com"},
		// 本来就填对的不许改坏（含自建站点的自定义路径前缀）
		{"https://api.example.com", "https://api.example.com"},
		{"https://api.example.com/openai", "https://api.example.com/openai"},
		{"https://api.example.com/sub/v1", "https://api.example.com/sub"},
		{"  https://api.example.com/v1  ", "https://api.example.com"},
		{"", ""},
		// 只有域名 + 协议，别把协议头剥掉
		{"https://x.com/v1", "https://x.com"},
	}
	for _, c := range cases {
		if got := NormalizeBaseURL(c.in); got != c.want {
			t.Errorf("NormalizeBaseURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
