package models

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupBranchDB 独立命名内存库（同名共享库会在多测试/重跑间复用数据）。
func setupBranchDB(t *testing.T) {
	t.Helper()
	prev := DB
	dsn := fmt.Sprintf("file:branch_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("打开内存库失败：%v", err)
	}
	if err := db.AutoMigrate(&AiChatSession{}, &AiChatMessage{}); err != nil {
		t.Fatalf("建表失败：%v", err)
	}
	DB = db
	t.Cleanup(func() { DB = prev })
}

func mustSession(t *testing.T) *AiChatSession {
	t.Helper()
	s, err := CreateAiChatSession(1, 0, "", 1)
	if err != nil {
		t.Fatalf("建会话失败：%v", err)
	}
	return s
}

func mustMsg(t *testing.T, sid uint, role, content string, parent uint) *AiChatMessage {
	t.Helper()
	m, err := AppendAiChatMessageWithParent(sid, role, content, parent, 0, 0, 0, 0)
	if err != nil {
		t.Fatalf("落消息失败：%v", err)
	}
	return m
}

// TestThread_Linear 线性对话：回溯应按顺序返回全部。
func TestThread_Linear(t *testing.T) {
	setupBranchDB(t)
	s := mustSession(t)
	u1 := mustMsg(t, s.ID, "user", "问1", 0)
	a1 := mustMsg(t, s.ID, "assistant", "答1", u1.ID)
	u2 := mustMsg(t, s.ID, "user", "问2", a1.ID)
	a2 := mustMsg(t, s.ID, "assistant", "答2", u2.ID)

	list, err := ListAiChatThread(s.ID, a2.ID)
	if err != nil {
		t.Fatalf("回溯失败：%v", err)
	}
	if len(list) != 4 {
		t.Fatalf("应回溯 4 条，实际 %d", len(list))
	}
	if list[0].Content != "问1" || list[3].Content != "答2" {
		t.Fatalf("顺序错误：%v", []string{list[0].Content, list[3].Content})
	}
}

// TestThread_Branch 分支：两条 assistant 共享同一个 parent，各自成为兄弟。
func TestThread_Branch(t *testing.T) {
	setupBranchDB(t)
	s := mustSession(t)
	u1 := mustMsg(t, s.ID, "user", "问1", 0)
	a1 := mustMsg(t, s.ID, "assistant", "答A", u1.ID)
	a2 := mustMsg(t, s.ID, "assistant", "答B", u1.ID) // 对 a1「重新生成」

	// 切到 a1 分支：只有 问1 + 答A
	list, _ := ListAiChatThread(s.ID, a1.ID)
	if len(list) != 2 || list[1].Content != "答A" {
		t.Fatalf("a1 分支错误：%+v", list)
	}
	// 切到 a2 分支：只有 问1 + 答B
	list, _ = ListAiChatThread(s.ID, a2.ID)
	if len(list) != 2 || list[1].Content != "答B" {
		t.Fatalf("a2 分支错误：%+v", list)
	}

	// 兄弟信息：2 条，索引分别为 1 和 2
	idx, cnt := SiblingIndex(a1)
	if cnt != 2 || idx != 1 {
		t.Fatalf("a1 兄弟信息错误：idx=%d cnt=%d", idx, cnt)
	}
	idx, cnt = SiblingIndex(a2)
	if cnt != 2 || idx != 2 {
		t.Fatalf("a2 兄弟信息错误：idx=%d cnt=%d", idx, cnt)
	}
	// user 消息没有兄弟
	if idx, cnt := SiblingIndex(u1); cnt != 1 || idx != 1 {
		t.Fatalf("user 兄弟信息错误：idx=%d cnt=%d", idx, cnt)
	}
	if ids := SiblingIDs(a1); len(ids) != 2 || ids[0] != a1.ID || ids[1] != a2.ID {
		t.Fatalf("兄弟 ID 列表错误：%v", ids)
	}
}

// TestBranch_KeepsFollowUp 切回旧分支时，那条分支后续的追问不能被吞掉。
//
// 这是 DeepSeek 式分支最容易做错的地方：只切显示位置而不跟到该分支末端，
// 切回去只看得到「问1 + 答A」，后面的「追问」全消失。
func TestBranch_KeepsFollowUp(t *testing.T) {
	setupBranchDB(t)
	s := mustSession(t)
	u1 := mustMsg(t, s.ID, "user", "问1", 0)
	a1 := mustMsg(t, s.ID, "assistant", "答A", u1.ID)
	u2 := mustMsg(t, s.ID, "user", "追问A", a1.ID)
	a2 := mustMsg(t, s.ID, "assistant", "续答A", u2.ID)
	// 另一条分支
	b1 := mustMsg(t, s.ID, "assistant", "答B", u1.ID)

	if leaf := DeepestLeafFrom(s.ID, a1.ID); leaf != a2.ID {
		t.Fatalf("从 a1 应跟到 a2（%d），实际 %d", a2.ID, leaf)
	}
	list, _ := ListAiChatThread(s.ID, DeepestLeafFrom(s.ID, a1.ID))
	if len(list) != 4 {
		t.Fatalf("a 分支应含 4 条（含追问），实际 %d", len(list))
	}
	// b 分支只有 2 条
	if list, _ := ListAiChatThread(s.ID, DeepestLeafFrom(s.ID, b1.ID)); len(list) != 2 {
		t.Fatalf("b 分支应含 2 条，实际 %d", len(list))
	}
}

// TestThread_LegacyNoParent 老数据（parent_id 全 0）回填后必须能完整回溯。
func TestThread_LegacyNoParent(t *testing.T) {
	setupBranchDB(t)
	s := mustSession(t)
	// 模拟升级前写入的数据：parent_id 全 0
	for _, r := range []struct{ role, content string }{
		{"user", "老问1"}, {"assistant", "老答1"}, {"user", "老问2"}, {"assistant", "老答2"},
	} {
		m := &AiChatMessage{
			SessionID: s.ID, Role: r.role, Content: r.content,
			CreatedAt: time.Now(),
		}
		if err := DB.Create(m).Error; err != nil {
			t.Fatalf("写老数据失败：%v", err)
		}
	}

	// 未回填：active_leaf=0 → 只拿到最后一条（这就是"看起来数据丢了"的症状）
	list, _ := ListAiChatThread(s.ID, 0)
	if len(list) != 1 {
		t.Fatalf("回填前应只能拿到最新 1 条，实际 %d", len(list))
	}

	BackfillMessageParents()

	list, _ = ListAiChatThread(s.ID, 0)
	if len(list) != 4 {
		t.Fatalf("回填后应能回溯 4 条，实际 %d", len(list))
	}
	if list[0].Content != "老问1" || list[3].Content != "老答2" {
		t.Fatalf("回填后顺序错误：%s / %s", list[0].Content, list[3].Content)
	}

	// 幂等：再跑一次不能把链串坏
	BackfillMessageParents()
	if list, _ := ListAiChatThread(s.ID, 0); len(list) != 4 {
		t.Fatalf("重复回填破坏了链路，剩 %d 条", len(list))
	}
}

// TestLockSession 终止后不可用、解锁后恢复。
func TestLockSession(t *testing.T) {
	setupBranchDB(t)
	s := mustSession(t)
	if err := LockAiChatSession(s.ID, "测试终止", 0); err != nil {
		t.Fatalf("终止失败：%v", err)
	}
	got, _ := GetAiChatSession(s.ID, 1)
	if got == nil || !got.Locked {
		t.Fatalf("会话应处于终止状态")
	}
	if got.LockReason != "测试终止" {
		t.Fatalf("终止原因未落库：%q", got.LockReason)
	}
	if err := UnlockAiChatSession(s.ID); err != nil {
		t.Fatalf("解锁失败：%v", err)
	}
	got, _ = GetAiChatSession(s.ID, 1)
	if got.Locked {
		t.Fatalf("解锁后不应仍处于终止状态")
	}
}
