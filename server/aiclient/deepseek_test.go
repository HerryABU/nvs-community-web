package aiclient

import "testing"

// DeepSeek 官方域名识别：只有官方才走 /user/balance，中转站走通用分支
func TestIsDeepSeekOfficial(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"https://api.deepseek.com/v1", true},
		{"https://api.deepseek.com", true},
		{"https://api.hcnsec.cn/v1", false}, // 中转站：不能误判
		{"https://api.openai.com/v1", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isDeepSeekOfficial(c.url); got != c.want {
			t.Errorf("isDeepSeekOfficial(%q) = %v，期望 %v", c.url, got, c.want)
		}
	}
}

// 余额字段可能是字符串也可能是数字（DeepSeek 返回 string），两种都要能取到
func TestNumOf(t *testing.T) {
	if v := numOf("110.35"); v != 110.35 {
		t.Errorf("字符串金额解析错误: %v", v)
	}
	if v := numOf(float64(5.5)); v != 5.5 {
		t.Errorf("数字金额解析错误: %v", v)
	}
	if v := numOf(nil); v != 0 {
		t.Errorf("nil 应为 0，实际 %v", v)
	}
}
