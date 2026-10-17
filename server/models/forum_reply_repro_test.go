package models

import (
	"testing"

	"gorm.io/gorm"
)

// TestForumReplyVisibility 复现论坛「看不到回复」：建帖子→建楼→建楼内回复，
// 验证 GetPostsByThread 是否把回复随楼层一并返回，且回复的 root_id 指向楼层。
func TestForumReplyVisibility(t *testing.T) {
	setupTestDB(t)

	// 造两个用户（楼主 + 回复者）
	owner := &User{Username: "fowner", Nickname: "楼主", Email: "fowner@example.com"}
	replier := &User{Username: "freplier", Nickname: "回复者", Email: "freplier@example.com"}
	if err := DB.Create(owner).Error; err != nil {
		t.Fatalf("seed owner: %v", err)
	}
	if err := DB.Create(replier).Error; err != nil {
		t.Fatalf("seed replier: %v", err)
	}

	// 造论坛 + 帖子（线程）
	forum := &Forum{Name: "测试论坛", Description: "d", Type: "general"}
	if err := DB.Create(forum).Error; err != nil {
		t.Fatalf("seed forum: %v", err)
	}
	thread := &Thread{ForumID: forum.ID, UserID: owner.ID, Title: "测试帖", Content: "主楼内容"}
	if err := CreateThread(thread); err != nil {
		t.Fatalf("seed thread: %v", err)
	}

	// 1) 楼主发一条楼层（parent_id 为空）
	floor := &Post{ThreadID: thread.ID, UserID: owner.ID, Content: "我是1楼", ParentID: nil}
	if err := CreatePost(floor); err != nil {
		t.Fatalf("create floor: %v", err)
	}
	t.Logf("floor id=%d root=%v floor_no=%d", floor.ID, floor.RootID, floor.FloorNo)

	// 2) 回复者回复该楼层（parent_id = floor.ID）
	replyParent := floor.ID
	reply := &Post{ThreadID: thread.ID, UserID: replier.ID, Content: "回复1楼", ParentID: &replyParent}
	if err := CreatePost(reply); err != nil {
		t.Fatalf("create reply: %v", err)
	}
	t.Logf("reply id=%d parent=%v root=%v floor_no=%d", reply.ID, reply.ParentID, reply.RootID, reply.FloorNo)

	// 3) 取回帖子回复
	pg, err := GetPostsByThread(thread.ID, 1, 20)
	if err != nil {
		t.Fatalf("GetPostsByThread: %v", err)
	}
	t.Logf("returned list len=%d total=%d floorTotal=%d", len(pg.List), pg.Total, pg.FloorTotal)

	foundFloor, foundReply := false, false
	for _, p := range pg.List {
		if p.ID == floor.ID {
			foundFloor = true
		}
		if p.ID == reply.ID {
			foundReply = true
		}
	}
	if !foundFloor {
		t.Errorf("楼层未返回")
	}
	if !foundReply {
		t.Errorf("楼内回复未返回（Bug 复现：论坛中看不到回复）")
	}

	// 4) 前端分组校验：reply.root_id 必须等于 floor.ID
	if reply.RootID == nil || *reply.RootID != floor.ID {
		t.Errorf("回复 root_id=%v，期望指向楼层 %d（前端按 root_id 分组将失败）", reply.RootID, floor.ID)
	}
}

// 确保 gorm 引用被使用（避免某些构建裁剪）
var _ = gorm.ErrRecordNotFound
