package vendor

import "testing"

// 四段式解析：模型大名 / 版本号 / 类型名称 / 子号
func TestParseModelID_FourSegments(t *testing.T) {
	cases := []struct {
		raw     string
		family  string
		version string
		variant string
		build   string
	}{
		// 用户给的例子：deepseek V4.1-Flash-1017
		{"deepseek-v4.1-flash-1017", "deepseek", "v4.1", "flash", "1017"},
		{"deepseek-chat", "deepseek", "", "chat", ""},
		{"deepseek-reasoner", "deepseek", "", "reasoner", ""},
		{"gpt-4o-mini", "gpt", "4o", "mini", ""},
		{"gpt-4o-2024-08-06", "gpt", "4o", "", "2024-08-06"},
		{"claude-3-5-sonnet-20241022", "claude", "3-5", "sonnet", "20241022"},
		{"claude-3-opus-latest", "claude", "3", "opus-latest", ""},
		{"qwen-max-latest", "qwen", "", "max-latest", ""},
		{"qwen2.5-72b-instruct", "qwen", "2.5", "instruct", "72b"},
		{"gemini-2.5-pro-preview-05-06", "gemini", "2.5", "pro-preview", "05-06"},
		{"glm-4-plus", "glm", "4", "plus", ""},
		{"moonshot-v1-128k", "moonshot", "v1", "", "128k"},
		{"text-embedding-3-large", "text-embedding", "3", "large", ""},
		{"llama-3.1-70b-instruct", "llama", "3.1", "instruct", "70b"},
		{"o1-mini", "o1", "", "mini", ""},
	}
	for _, c := range cases {
		got := ParseModelID(c.raw)
		if got.Prefix != c.family || got.Version != c.version || got.Variant != c.variant || got.Build != c.build {
			t.Errorf("%s => family=%q version=%q variant=%q build=%q；期望 %q/%q/%q/%q",
				c.raw, got.Prefix, got.Version, got.Variant, got.Build,
				c.family, c.version, c.variant, c.build)
		}
	}
}

// 旧字段 sub_version 必须是 variant+build 的组合（兼容历史调用方）
func TestParseModelID_SubVersionCompat(t *testing.T) {
	got := ParseModelID("deepseek-v4.1-flash-1017")
	if got.SubVersion != "flash-1017" {
		t.Errorf("sub_version = %q，期望 flash-1017", got.SubVersion)
	}
	// 不能丢信息：四段拼回去应能还原原始串（分隔符归一后）
	if got.Prefix+"-"+got.Version+"-"+got.Variant+"-"+got.Build != "deepseek-v4.1-flash-1017" {
		t.Errorf("四段无法拼回原串: %+v", got)
	}
}

// 冒号后缀（OpenRouter :free）应归入 variant，不污染家族名
func TestParseModelID_ColonSuffix(t *testing.T) {
	got := ParseModelID("deepseek/deepseek-chat-v3:free")
	if got.Variant == "" || got.Variant[len(got.Variant)-4:] != "free" {
		t.Errorf("冒号后缀未归入 variant: %+v", got)
	}
	if got.Prefix == "" || got.Prefix[0:4] != "deep" {
		t.Errorf("家族名被污染: %q", got.Prefix)
	}
}

// 空串不应 panic
func TestParseModelID_Empty(t *testing.T) {
	got := ParseModelID("   ")
	if got.Prefix != "" {
		t.Errorf("空串应返回空结构，实际 %+v", got)
	}
}
