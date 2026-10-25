package vendor

import "testing"

func TestParseModelID(t *testing.T) {
	cases := []struct {
		raw, prefix, version, sub string
	}{
		{"deepseek-v3.1-turbo", "deepseek", "v3.1", "turbo"},
		{"DeepSeek_V3-1/Turbo", "DeepSeek", "V3", "1-Turbo"}, // _ 与 / 归一为 -，保留原始大小写
		{"gpt-4o", "gpt", "4o", ""},
		{"claude", "claude", "", ""}, // 单段整串作前缀
		{"qwen2.5-7b-instruct", "qwen2.5", "7b", "instruct"},
	}
	for _, c := range cases {
		p := ParseModelID(c.raw)
		if p.Prefix != c.prefix || p.Version != c.version || p.SubVersion != c.sub {
			t.Fatalf("ParseModelID(%q) = %+v, want prefix=%q version=%q sub=%q",
				c.raw, p, c.prefix, c.version, c.sub)
		}
	}
}

func TestNewestSubVersion(t *testing.T) {
	cases := []struct {
		subs   []string
		newest string
	}{
		{[]string{"base", "turbo", "pro"}, "pro"},            // 词表档位：pro(55)>turbo(50)>base(20)
		{[]string{"0125", "0706", "1106"}, "1106"},           // 纯数字小版本：字典序数值最大
		{[]string{"0801", "20240101", "v1"}, "20240101"},     // 日期型压倒词表/数字
		{[]string{"base", ""}, "base"},                       // 空亚版本忽略
		{[]string{"mini", "pro", "max"}, "max"},              // max 最高档
		{[]string{"turbo-0125", "turbo-0706"}, "turbo-0706"}, // 含数字后缀
	}
	for _, c := range cases {
		if got := NewestSubVersion(c.subs); got != c.newest {
			t.Fatalf("NewestSubVersion(%v) = %q, want %q", c.subs, got, c.newest)
		}
	}
}
