package models

import "testing"

func TestParseAiBotInvocation(t *testing.T) {
	setupAiTestDB(t)
	// @ai 词首
	if _, prompt, ok := ParseAiBotInvocation("@ai 帮我想个结局"); !ok || prompt != "帮我想个结局" {
		t.Fatalf("@ai 词首应命中，实际 ok=%v prompt=%q", ok, prompt)
	}
	// 句中
	if _, _, ok := ParseAiBotInvocation("各位觉得呢 @ai 点评一下"); !ok {
		t.Fatal("句中 @ai 应命中")
	}
	// 大写
	if _, _, ok := ParseAiBotInvocation("@AI 你好"); !ok {
		t.Fatal("大写 @AI 应命中")
	}
	// @ai助手
	if _, _, ok := ParseAiBotInvocation("@ai助手 在吗"); !ok {
		t.Fatal("@ai助手 应命中")
	}
	// 更长用户名 @aiko 不误触发
	if _, _, ok := ParseAiBotInvocation("@aiko 看看这个"); ok {
		t.Fatal("@aiko 不应误触发")
	}
	// 无标记
	if _, _, ok := ParseAiBotInvocation("普通评论一条"); ok {
		t.Fatal("无标记不应命中")
	}
	// @智能体名（需先建启用智能体）
	if err := CreateAiAgent(&AiAgent{Name: "毒舌评论家", Enabled: true, SystemPrompt: "毒舌人设"}); err != nil {
		t.Fatalf("建智能体失败: %v", err)
	}
	name, prompt, ok := ParseAiBotInvocation("@毒舌评论家 点评下这段")
	if !ok || name != "毒舌评论家" || prompt != "点评下这段" {
		t.Fatalf("@智能体名 应命中，实际 name=%q prompt=%q ok=%v", name, prompt, ok)
	}
}

func TestAiBotQuota(t *testing.T) {
	setupAiTestDB(t)
	// 0 = 禁止
	if CheckAiBotQuota(1, 0) {
		t.Fatal("limit=0 应禁止")
	}
	// -1 = 不限
	if !CheckAiBotQuota(1, -1) {
		t.Fatal("limit=-1 应不限")
	}
	// 2 次用尽
	if !CheckAiBotQuota(1, 2) {
		t.Fatal("首次应放行")
	}
	RecordAiBotUsage(1)
	if !CheckAiBotQuota(1, 2) {
		t.Fatal("第二次应放行")
	}
	RecordAiBotUsage(1)
	if CheckAiBotQuota(1, 2) {
		t.Fatal("第三次应禁止")
	}
	if CountTodayAiBotUsage(1) != 2 {
		t.Fatalf("用量应为 2，实际 %d", CountTodayAiBotUsage(1))
	}
	// 换用户不受影响
	if !CheckAiBotQuota(2, 2) {
		t.Fatal("其他用户应放行")
	}
}

func TestEnsureAiBotUser(t *testing.T) {
	setupAiTestDB(t)
	id1, err := EnsureAiBotUser()
	if err != nil || id1 == 0 {
		t.Fatalf("创建机器人用户失败: id=%d err=%v", id1, err)
	}
	// 幂等
	id2, err := EnsureAiBotUser()
	if err != nil || id2 != id1 {
		t.Fatalf("应幂等返回同一 ID：%d vs %d", id2, id1)
	}
	u, uerr := GetUserByID(id1)
	if uerr != nil || u.Username != AiBotUsername || u.Nickname != AiBotNickname {
		t.Fatalf("机器人用户属性异常: %+v err=%v", u, uerr)
	}
	if u.Role != "reader" {
		t.Fatalf("机器人角色应为 reader，实际 %s", u.Role)
	}
}
