package collab

import (
	"testing"

	"nvs-server/models"
)

// TestTransferOwner 主笔转让：换主笔 + 受让人退出成员表 + 原主笔降为编辑 + 审计
func TestTransferOwner(t *testing.T) {
	setupCollabDB(t)
	owner := mkUser(t, "t_owner", models.RoleAuthor)
	peer := mkUser(t, "t_peer", models.RoleReader)
	novel := mkNovel(t, owner.ID, "转让测试")

	// 邀请并接受
	if _, err := Invite(novel.ID, owner.ID, peer.Username, RoleEditor); err != nil {
		t.Fatalf("邀请失败: %v", err)
	}
	if err := Respond(novel.ID, peer.ID, true); err != nil {
		t.Fatalf("接受失败: %v", err)
	}

	if err := Transfer(novel.ID, owner.ID, peer.ID, false); err != nil {
		t.Fatalf("转让失败: %v", err)
	}
	// 1) 主笔变更
	var n models.Novel
	_ = models.DB.Where("id = ?", novel.ID).First(&n).Error
	if n.AuthorID != peer.ID {
		t.Fatalf("主笔未变更: %d", n.AuthorID)
	}
	// 2) 受让人不再有协作者记录（主笔身份由 Novel.AuthorID 体现）
	if m, err := Get(novel.ID, peer.ID); err == nil && m != nil {
		t.Fatalf("受让人应退出成员表: %+v", m)
	}
	// 3) 原主笔降为编辑且保留改他人章节
	old, err := Get(novel.ID, owner.ID)
	if err != nil || old == nil {
		t.Fatalf("原主笔应保留成员记录: %v", err)
	}
	if old.Role != RoleEditor || !old.CanEditOthers {
		t.Fatalf("原主笔应为 editor 且有改他人权限: %+v", old)
	}
	// 4) 审计
	var cnt int64
	models.DB.Model(&Audit{}).Where("novel_id = ? AND action = ?", novel.ID, "transfer_owner").Count(&cnt)
	if cnt != 1 {
		t.Fatalf("审计缺失: %d", cnt)
	}
}

// TestTransferRejections 非主笔 / 转自己 / 非协作者 三种拒绝
func TestTransferRejections(t *testing.T) {
	setupCollabDB(t)
	owner := mkUser(t, "t2_owner", models.RoleAuthor)
	other := mkUser(t, "t2_other", models.RoleReader)
	novel := mkNovel(t, owner.ID, "拒绝测试")

	// 非主笔发起
	if err := Transfer(novel.ID, other.ID, owner.ID, false); err == nil {
		t.Fatal("非主笔不应能转让")
	}
	// 转给自己
	if err := Transfer(novel.ID, owner.ID, owner.ID, false); err == nil {
		t.Fatal("转给自己应被拒绝")
	}
	// 非协作者
	if err := Transfer(novel.ID, owner.ID, other.ID, false); err == nil {
		t.Fatal("非协作者不应成为受让人")
	}
	// 平台可以（受让人仍是协作者时）
	if _, err := Invite(novel.ID, owner.ID, other.Username, RoleCoAuthor); err != nil {
		t.Fatal(err)
	}
	if err := Respond(novel.ID, other.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := Transfer(novel.ID, 9999, other.ID, true); err != nil {
		t.Fatalf("平台应可转让: %v", err)
	}
}
