package models

import (
	"strings"
	"testing"
)

func TestNovelMemoryUpsert(t *testing.T) {
	setupAiTestDB(t)
	// 大纲 upsert
	if err := UpsertNovelOutline(1, "少年持剑入京，欲报杀父之仇。"); err != nil {
		t.Fatalf("保存大纲失败: %v", err)
	}
	if got := GetNovelOutline(1); got != "少年持剑入京，欲报杀父之仇。" {
		t.Fatalf("大纲读取异常: %q", got)
	}
	if err := UpsertNovelOutline(1, "修订版大纲。"); err != nil {
		t.Fatalf("更新大纲失败: %v", err)
	}
	if got := GetNovelOutline(1); got != "修订版大纲。" {
		t.Fatalf("大纲应被更新，实际 %q", got)
	}
	// 空内容 = 删除
	if err := UpsertNovelOutline(1, "  "); err != nil {
		t.Fatalf("清空大纲失败: %v", err)
	}
	if got := GetNovelOutline(1); got != "" {
		t.Fatalf("大纲应被清空，实际 %q", got)
	}
	// 章节摘要
	if err := UpsertChapterSummary(1, 10, "第十章：少年遇师，习得剑法。"); err != nil {
		t.Fatalf("保存摘要失败: %v", err)
	}
	if err := UpsertChapterSummary(1, 11, "第十一章：夜闯王府，负伤遁走。"); err != nil {
		t.Fatalf("保存摘要失败: %v", err)
	}
	if got := GetChapterSummary(1, 10); got != "第十章：少年遇师，习得剑法。" {
		t.Fatalf("摘要读取异常: %q", got)
	}
	// 列表倒序（新→旧）+ 排除当前章
	list, err := ListRecentChapterSummaries(1, 0, 5)
	if err != nil || len(list) != 2 || list[0].ChapterID != 11 {
		t.Fatalf("摘要列表应倒序且含 2 条，实际 %+v err=%v", list, err)
	}
	list2, _ := ListRecentChapterSummaries(1, 11, 5)
	if len(list2) != 1 || list2[0].ChapterID != 10 {
		t.Fatalf("排除当前章后应只剩 1 条，实际 %+v", list2)
	}
	// 删除
	if err := DeleteChapterMemory(1, 10); err != nil {
		t.Fatalf("删除摘要失败: %v", err)
	}
	if got := GetChapterSummary(1, 10); got != "" {
		t.Fatalf("摘要应被删除，实际 %q", got)
	}
}

func TestBuildNovelContext(t *testing.T) {
	setupAiTestDB(t)
	// 空记忆：仅尾部窗口
	long := strings.Repeat("正文内容。", 300) // 远超 1200 runes
	ctx, tokens := BuildNovelContext(1, 5, long, AiContextBudget)
	if ctx == "" || tokens <= 0 {
		t.Fatalf("无记忆时至少应有尾部窗口，实际 ctx=%q tokens=%d", ctx, tokens)
	}
	if !strings.Contains(ctx, "【当前章节结尾】") {
		t.Fatal("上下文应包含当前章结尾标记")
	}
	// 加记忆后：大纲 + 前文脉络均注入
	UpsertNovelOutline(1, "大纲：复仇主线。")
	UpsertChapterSummary(1, 3, "第三章：拜师。")
	UpsertChapterSummary(1, 4, "第四章：初战。")
	ctx2, tokens2 := BuildNovelContext(1, 5, long, AiContextBudget)
	if !strings.Contains(ctx2, "【作品大纲】") || !strings.Contains(ctx2, "【前文脉络】") {
		t.Fatalf("应注入大纲与前文脉络，实际:\n%s", ctx2)
	}
	if !strings.Contains(ctx2, "拜师") || !strings.Contains(ctx2, "初战") {
		t.Fatal("前文脉络应含章节摘要内容")
	}
	// 旧→新顺序：拜师在初战之前
	if strings.Index(ctx2, "拜师") > strings.Index(ctx2, "初战") {
		t.Fatal("摘要应按旧→新排列")
	}
	if tokens2 <= tokens {
		t.Fatalf("有记忆后 tokens 应增加：%d → %d", tokens, tokens2)
	}
	// 极小预算：只保留最必要部分（尾部窗口优先）
	ctx3, _ := BuildNovelContext(1, 5, long, 100)
	if strings.Contains(ctx3, "【作品大纲】") {
		t.Fatal("极小预算不应注入大纲")
	}
}
