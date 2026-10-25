package moderation

import "testing"

// TestFailHook 违规自动处置回调：注册后能收到 scope/target/reason；
// 未注册时静默跳过、不 panic；回调内部 panic 也不能把审核主链路带崩。
//
// 这个 hook 是 moderation 包与 handlers 之间唯一的逆向依赖点
// （包不能 import handlers，否则循环依赖），所以必须锁住行为。
func TestFailHook(t *testing.T) {
	defer SetOnFailHook(nil)

	var gotScope, gotReason string
	var gotID uint
	SetOnFailHook(func(scope string, id uint, reason string) {
		gotScope, gotID, gotReason = scope, id, reason
	})
	runFailHook(ScopeChapter, 42, "涉暴力描写")
	if gotScope != ScopeChapter || gotID != 42 || gotReason != "涉暴力描写" {
		t.Fatalf("回调参数错误：scope=%q id=%d reason=%q", gotScope, gotID, gotReason)
	}

	// 回调内部 panic 不应冒泡（自动处置失败不能影响审核记录落库）
	SetOnFailHook(func(string, uint, string) { panic("boom") })
	runFailHook(ScopeBlog, 1, "x")

	// 未注册时安全跳过
	SetOnFailHook(nil)
	runFailHook(ScopeBlog, 1, "x")
}
