package models

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// 楼加密策略测试。
//
// 背景：「楼加密」有两种语义 —— 只锁楼正文（楼内回复照常可见），
// 或连楼内回复一起藏（整块对话私密）。管理员可选，默认前者。
// 这里锁住「默认行为」与「切换后行为」两套语义，防止以后改代码时
// 悄悄把「评论区不加密」又改回去。

// setupPolicyTestDB 独立内存库（唯一名，避免与其它测试共享污染）
func setupPolicyTestDB(t *testing.T) {
	t.Helper()
	name := fmt.Sprintf("file:comment_policy_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(name), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存测试库失败: %v", err)
	}
	if err := AutoMigrateAll(db); err != nil {
		t.Fatalf("迁移测试库失败: %v", err)
	}
	DB = db
	t.Cleanup(func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
}

// seedFloorWithReply 造一楼 + 一条楼内回复
func seedFloorWithReply(t *testing.T) (floor *Comment, reply *Comment) {
	t.Helper()
	floor = &Comment{UserID: 11, Content: "楼主的私密内容", HasPassword: true, PasswordHash: "x"}
	if err := DB.Create(floor).Error; err != nil {
		t.Fatalf("造楼失败: %v", err)
	}
	reply = &Comment{UserID: 22, Content: "楼内回复", ParentID: &floor.ID, RootID: &floor.ID}
	if err := DB.Create(reply).Error; err != nil {
		t.Fatalf("造楼内回复失败: %v", err)
	}
	return floor, reply
}

// TestRedactLockedComments_默认不藏楼内回复
// 默认语义：楼加密只锁楼正文，楼内回复（评论区）照常返回。
func TestRedactLockedComments_默认不藏楼内回复(t *testing.T) {
	setupPolicyTestDB(t)
	floor, reply := seedFloorWithReply(t)

	list := []Comment{*floor, *reply}
	out := RedactLockedComments(list, 999 /* 非楼作者 */, false, nil)

	if len(out) != 2 {
		t.Fatalf("默认策略下应返回 2 条（楼 + 楼内回复），实际 %d 条", len(out))
	}
	// 楼正文被脱敏
	if !out[0].Locked || out[0].Content != "" {
		t.Fatalf("未解锁的楼应脱敏（Locked=true, Content 为空），实际 Locked=%v Content=%q",
			out[0].Locked, out[0].Content)
	}
	// 楼内回复照常返回且内容完整
	found := false
	for _, c := range out {
		if c.ID == reply.ID {
			found = true
			if c.Content != "楼内回复" {
				t.Fatalf("楼内回复内容应完整保留，实际 %q", c.Content)
			}
			if c.Locked {
				t.Fatalf("楼内回复不应被标记为 Locked")
			}
		}
	}
	if !found {
		t.Fatal("楼内回复被丢掉了 —— 楼加密不应连带加密评论区")
	}
}

// TestRedactLockedComments_开启后藏楼内回复
// 管理员选择「连带隐藏」时，保留旧行为：楼内回复整批不返回。
func TestRedactLockedComments_开启后藏楼内回复(t *testing.T) {
	setupPolicyTestDB(t)
	if err := SaveCommentPolicy(CommentPolicy{LockHidesReplies: true}); err != nil {
		t.Fatalf("保存策略失败: %v", err)
	}
	floor, reply := seedFloorWithReply(t)

	list := []Comment{*floor, *reply}
	out := RedactLockedComments(list, 999, false, nil)

	if len(out) != 1 {
		t.Fatalf("开启「连带隐藏」后应只剩 1 条（楼），实际 %d 条", len(out))
	}
	if out[0].ID != floor.ID {
		t.Fatalf("留下的应是楼本身，实际 id=%d", out[0].ID)
	}
}

// TestRedactLockedComments_解锁后一切正常
// 带对密码（或楼作者本人）时，楼正文与楼内回复都应完整返回。
func TestRedactLockedComments_解锁后一切正常(t *testing.T) {
	setupPolicyTestDB(t)
	floor, reply := seedFloorWithReply(t)

	// 楼作者本人：无需密码
	list := []Comment{*floor, *reply}
	out := RedactLockedComments(list, floor.UserID, false, nil)
	if len(out) != 2 {
		t.Fatalf("楼作者本人应看到全部 2 条，实际 %d 条", len(out))
	}
	for _, c := range out {
		if c.Locked {
			t.Fatalf("楼作者本人不应看到 Locked 标记")
		}
	}
	if out[0].Content != "楼主的私密内容" {
		t.Fatalf("解锁后楼正文应完整，实际 %q", out[0].Content)
	}
	_ = reply
}

// TestCommentPolicy_默认值
// 默认必须是「评论区不加密」—— 这是用户明确表达的产品意图。
func TestCommentPolicy_默认值(t *testing.T) {
	setupPolicyTestDB(t)
	p := LoadCommentPolicy()
	if p.LockHidesReplies {
		t.Fatal("默认不应连带隐藏楼内回复（楼加密 ≠ 评论区加密）")
	}
	if p.LockedFloorCanReply {
		t.Fatal("默认不应允许对未解锁的楼盲回")
	}
}

// TestCommentPolicy_持久化
func TestCommentPolicy_持久化(t *testing.T) {
	setupPolicyTestDB(t)
	if err := SaveCommentPolicy(CommentPolicy{LockHidesReplies: true, LockedFloorCanReply: true}); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	p := LoadCommentPolicy()
	if !p.LockHidesReplies || !p.LockedFloorCanReply {
		t.Fatalf("策略未正确持久化: %+v", p)
	}
}
