package aiclient

import (
	"testing"
)

// ===== (原 claude_test.go) =====

// TestBuildClaudeRequest 校验 OpenAI → Claude 请求转换的语义差异
func TestBuildClaudeRequest(t *testing.T) {
	req := ChatRequest{
		Model:     "claude-sonnet-4-5",
		MaxTokens: 1024,
		Messages: []Message{
			{Role: "system", Content: "你是小说助手"},
			{Role: "system", Content: "保持文风一致"},
			{Role: "user", Content: "续写第一章"},
			{Role: "assistant", Content: "好的"},
			{Role: "tool", Content: "工具输出"}, // 非标准角色应降级为 user
		},
	}
	cr := buildClaudeRequest(req)

	if cr.System != "你是小说助手\n\n保持文风一致" {
		t.Fatalf("system 抽取错误：%q", cr.System)
	}
	if len(cr.Messages) != 3 {
		t.Fatalf("system 消息应从 messages 中移除，实际 %d 条", len(cr.Messages))
	}
	for _, m := range cr.Messages {
		if m.Role != "user" && m.Role != "assistant" {
			t.Fatalf("Claude 仅接受 user/assistant，得到 %q", m.Role)
		}
	}
	if cr.MaxTokens != 1024 || cr.Model != "claude-sonnet-4-5" {
		t.Fatalf("模型/上限透传错误：%+v", cr)
	}

	// 空 messages 兜底：Claude 要求至少一条 user
	empty := buildClaudeRequest(ChatRequest{Model: "m"})
	if len(empty.Messages) != 1 || empty.Messages[0].Role != "user" {
		t.Fatalf("空消息兜底失败：%+v", empty.Messages)
	}
	if empty.MaxTokens != 2048 {
		t.Fatalf("默认 max_tokens 应为 2048，得到 %d", empty.MaxTokens)
	}
}

// TestClaudeURL 校验 base 带/不带 /v1 都能拼出 /v1/messages
func TestClaudeURL(t *testing.T) {
	cases := map[string]string{
		"https://api.anthropic.com":     "https://api.anthropic.com/v1/messages",
		"https://api.anthropic.com/":    "https://api.anthropic.com/v1/messages",
		"https://api.anthropic.com/v1":  "https://api.anthropic.com/v1/messages",
		"https://proxy.example.com/v1/": "https://proxy.example.com/v1/messages",
	}
	for base, want := range cases {
		c := NewClient(base, "k", 0)
		if got := c.claudeURL(); got != want {
			t.Fatalf("base=%s 得到 %s，期望 %s", base, got, want)
		}
	}
}

// ===== 合并自 deepseek_test.go =====

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

// ===== 合并自 image_refs_test.go =====

// TestParseDataURL 参考图只接受 data URL（拒绝外链，防 SSRF）且限图片类型
func TestParseDataURL(t *testing.T) {
	if _, err := ParseDataURL("https://evil.example.com/a.png"); err == nil {
		t.Fatalf("外链参考图应被拒绝")
	}
	ok := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="
	ref, err := ParseDataURL(ok)
	if err != nil {
		t.Fatalf("合法 data URL 解析失败：%v", err)
	}
	if ref.Mime != "image/png" || len(ref.Data) == 0 {
		t.Fatalf("解析结果异常：%+v", ref)
	}
	if _, err := ParseDataURL("data:text/html;base64,PHNjcmlwdD4="); err == nil {
		t.Fatalf("非图片类型应被拒绝")
	}
	if _, err := ParseDataURL("data:image/png;base64,!!!notbase64"); err == nil {
		t.Fatalf("非法 base64 应被拒绝")
	}
	// 数量上限
	refs := []string{ok, ok, ok, ok}
	if _, err := ImageRefsFromDataURLs(refs); err == nil {
		t.Fatalf("超过 %d 张应被拒绝", MaxRefImages)
	}
}

// ===== 合并自 normalize_test.go =====

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

// ===== 合并自 parts_test.go =====

func TestSplitDataURL(t *testing.T) {
	mt, data := SplitDataURL("data:application/pdf;base64,QUJD")
	if mt != "application/pdf" || data != "QUJD" {
		t.Fatalf("got %q %q", mt, data)
	}
	if mt, _ := SplitDataURL("https://x/a.png"); mt != "" {
		t.Fatalf("非 data URL 应返回空，得到 %q", mt)
	}
	if mt, _ := SplitDataURL("data:image/png,raw"); mt != "" {
		t.Fatalf("缺 ;base64 应返回空，得到 %q", mt)
	}
}

func TestMultimodalContentDegrades(t *testing.T) {
	// 无附件 → 纯字符串（兼容不接受数组的上游）
	if v := MultimodalContent("你好", nil, nil); v != "你好" {
		t.Fatalf("无附件应退化为字符串，得到 %#v", v)
	}
	// 只有图片 → parts 数组
	v := MultimodalContent("看图", []ImageURLPart{{URL: "data:image/png;base64,AA"}}, nil)
	parts, ok := v.([]ContentPart)
	if !ok || len(parts) != 2 || parts[1].Type != "image_url" {
		t.Fatalf("图片未生成 parts：%#v", v)
	}
	// 文件 → type=file
	v = MultimodalContent("读文档", nil, []FilePart{{Filename: "a.pdf", FileData: "data:application/pdf;base64,AA"}})
	parts, ok = v.([]ContentPart)
	if !ok || len(parts) != 2 || parts[1].Type != "file" || parts[1].File == nil {
		t.Fatalf("文件未生成 parts：%#v", v)
	}
}

func TestClaudeBlocksFromParts(t *testing.T) {
	blocks := ClaudeBlocksFromParts([]ContentPart{
		{Type: "text", Text: "看这个"},
		{Type: "image_url", ImageURL: &ImageURLPart{URL: "data:image/png;base64,AA"}},
		{Type: "image_url", ImageURL: &ImageURLPart{URL: "https://x/a.png"}},
		{Type: "file", File: &FilePart{Filename: "a.pdf", FileData: "data:application/pdf;base64,BB"}},
	})
	if len(blocks) != 4 {
		t.Fatalf("期望 4 个 block，得到 %d：%#v", len(blocks), blocks)
	}
	// 内联图片 → image/base64
	src := blocks[1]["source"].(map[string]any)
	if blocks[1]["type"] != "image" || src["type"] != "base64" || src["media_type"] != "image/png" {
		t.Fatalf("内联图片转换错误：%#v", blocks[1])
	}
	// 远程图片 → image/url
	src = blocks[2]["source"].(map[string]any)
	if src["type"] != "url" {
		t.Fatalf("远程图片应走 url：%#v", blocks[2])
	}
	// 文件 → document/base64
	src = blocks[3]["source"].(map[string]any)
	if blocks[3]["type"] != "document" || src["media_type"] != "application/pdf" || src["data"] != "BB" {
		t.Fatalf("文件转换错误：%#v", blocks[3])
	}
}
