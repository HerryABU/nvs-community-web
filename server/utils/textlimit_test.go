package utils

import "testing"

func TestClampString(t *testing.T) {
	cases := []struct {
		in   string
		max  int
		want string
	}{
		{"", 10, ""},
		{"abc", 10, "abc"},          // 未超限原样
		{"abcde", 3, "abc"},         // ASCII 截断
		{"你好世界", 2, "你好"},        // 中文按字符截断
		{"你好世界", 0, "你好世界"},     // max<=0 不限制
		{"ab你好", 3, "ab你"},        // 混合：按 rune 边界截断
		{"你好", 5, "你好"},          // 未超限
	}
	for _, c := range cases {
		if got := ClampString(c.in, c.max); got != c.want {
			t.Errorf("ClampString(%q, %d) = %q, want %q", c.in, c.max, got, c.want)
		}
	}
}

func TestRuneLenAndExceeds(t *testing.T) {
	if RuneLen("你好") != 2 {
		t.Error("RuneLen 应按字符计")
	}
	if !ExceedsLimit("你好世界", 3) {
		t.Error("应检测超限")
	}
	if ExceedsLimit("你好", 3) {
		t.Error("未超限不应报 true")
	}
	// 截断结果必须是合法 UTF-8（不出现半个汉字）
	s := ClampString("中文测试内容", 3)
	for _, r := range s {
		if r == '\uFFFD' {
			t.Error("截断产生了非法 UTF-8")
		}
	}
}

func TestClampStrings(t *testing.T) {
	m := map[string]string{"title": "很长的标题内容", "desc": "短"}
	ClampStrings(m, map[string]int{"title": 4, "desc": 100})
	if m["title"] != "很长的标" {
		t.Errorf("title 应被截断: %s", m["title"])
	}
	if m["desc"] != "短" {
		t.Errorf("desc 不应被截断: %s", m["desc"])
	}
}
