package handlers

import (
	"strings"
	"testing"
)

// 用户根提示词必须「追加」而不是「替换」：替换等于把安全准则删了
func TestComposeSystemPromptAppends(t *testing.T) {
	base := "【站点安全准则】不得提供违法内容。\n\n你是一位乐于助人的中文写作助手。"
	got := composeSystemPrompt(base, "你说话要像鲁迅。")
	if !strings.HasPrefix(got, base) {
		t.Fatal("站点 system prompt 必须保持在前、不被替换")
	}
	if !strings.Contains(got, "你说话要像鲁迅。") {
		t.Fatal("用户根提示词没进去")
	}
	if !strings.Contains(got, "低于以上规则") {
		t.Fatal("必须声明用户设定优先级更低（防提示注入）")
	}
	// 顺序：站点在前，用户在后
	if strings.Index(got, "站点安全准则") > strings.Index(got, "鲁迅") {
		t.Fatal("用户设定不能跑到站点规则前面")
	}
}

// 空值不改动；超长按 1500 字截断（既不烧 token，也避免把规则稀释掉）
func TestComposeSystemPromptBounds(t *testing.T) {
	if got := composeSystemPrompt("base", "   "); got != "base" {
		t.Fatalf("空值应原样返回：%q", got)
	}

	long := strings.Repeat("长", 2000)
	got := composeSystemPrompt("base", long)
	// 只保留 1500 个「长」
	if n := strings.Count(got, "长"); n != maxUserSystemRunes {
		t.Fatalf("截断后字数 = %d，期望 %d", n, maxUserSystemRunes)
	}
}
