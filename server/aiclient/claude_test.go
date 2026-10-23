package aiclient

import "testing"

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
