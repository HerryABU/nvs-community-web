package aiworkbench

import (
	"testing"
)

// 提示词优化：模型输出的清理。
//
// 模型很爱加壳（"优化后的提示词："、代码围栏、成对引号、加粗星号），
// 不清理的话用户会拿到带壳文本，粘回输入框会污染下一次生成。

func TestCleanOptimizedPrompt(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"优化后的提示词：一只猫在窗边", "一只猫在窗边"},
		{"优化后的提示词:一只猫在窗边", "一只猫在窗边"},
		{"优化结果：一只猫在窗边", "一只猫在窗边"},
		{"Optimized prompt: a cat by the window", "a cat by the window"},
		{"Optimized Prompt: a cat by the window", "a cat by the window"},
		{"Prompt: a cat by the window", "a cat by the window"},
		{"```\na cat by the window\n```", "a cat by the window"},
		{"```text\na cat by the window\n```", "a cat by the window"},
		{"\"a cat by the window\"", "a cat by the window"},
		{"「a cat by the window」", "a cat by the window"},
		{"**a cat by the window**", "a cat by the window"},
		{"  a cat by the window  ", "a cat by the window"},
	}
	for _, c := range cases {
		if got := cleanOptimizedPrompt(c.in); got != c.want {
			t.Errorf("cleanOptimizedPrompt(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if got := cleanOptimizedPrompt("   "); got != "" {
		t.Errorf("空白输入应返回空串，实际 %q", got)
	}
	if got := cleanOptimizedPrompt(""); got != "" {
		t.Errorf("空输入应返回空串，实际 %q", got)
	}
}
