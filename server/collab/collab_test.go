package collab

import (
	"strings"
	"testing"

	"nvs-server/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupCollabDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:collab_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if err := models.AutoMigrateAll(db); err != nil {
		t.Fatalf("主迁移失败: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("协作表迁移失败: %v", err)
	}
	old := models.DB
	models.DB = db
	t.Cleanup(func() {
		models.DB = old
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func mkUser(t *testing.T, username, role string) *models.User {
	t.Helper()
	u := &models.User{Username: username, Email: username + "@example.com", PasswordHash: "x", Role: role}
	if err := models.DB.Create(u).Error; err != nil {
		t.Fatalf("建用户失败: %v", err)
	}
	return u
}

func mkNovel(t *testing.T, ownerID uint, title string) *models.Novel {
	t.Helper()
	n := &models.Novel{AuthorID: ownerID, Title: title, Category: "科幻", Status: "published"}
	if err := models.DB.Create(n).Error; err != nil {
		t.Fatalf("建作品失败: %v", err)
	}
	return n
}

// TestInviteAndActivate 邀请 → 待接受（无权限）→ 接受（生效）
func TestInviteAndActivate(t *testing.T) {
	setupCollabDB(t)
	owner := mkUser(t, "c_owner", models.RoleAuthor)
	co := mkUser(t, "c_co", models.RoleReader)
	novel := mkNovel(t, owner.ID, "协作测试")

	c, err := Invite(novel.ID, owner.ID, co.Username, RoleCoAuthor)
	if err != nil {
		t.Fatalf("邀请失败: %v", err)
	}
	if c.Status != StatusPending {
		t.Fatalf("邀请后状态应为 pending，得到 %s", c.Status)
	}
	// 未接受不授予任何权限
	actor := ActorOf(novel.ID, co.ID, "")
	if actor.Can(CapWriteChapter) {
		t.Fatalf("pending 状态不应有创作权限")
	}

	if err := Respond(novel.ID, co.ID, true); err != nil {
		t.Fatalf("接受失败: %v", err)
	}
	actor = ActorOf(novel.ID, co.ID, "")
	if !actor.Can(CapWriteChapter) {
		t.Fatalf("协作作者应可新建章节")
	}
	if actor.Can(CapPublishChapter) {
		t.Fatalf("协作作者默认不应有直接发布权限")
	}
	if actor.Can(CapManageCollab) {
		t.Fatalf("协作作者不应有协作者管理权限")
	}

	// 主笔拥有全部能力
	ow := ActorOf(novel.ID, owner.ID, "")
	for _, cap := range []Capability{CapEditNovel, CapManageCollab, CapWriteChapter, CapPublishChapter, CapReviewChapter} {
		if !ow.Can(cap) {
			t.Fatalf("主笔应具备能力 %s", cap)
		}
	}
	// 路人无权限
	stranger := mkUser(t, "c_stranger", models.RoleReader)
	if ActorOf(novel.ID, stranger.ID, "").Can(CapWriteChapter) {
		t.Fatalf("非协作成员不应有创作权限")
	}
	// 邀请自己应失败
	if _, err := Invite(novel.ID, owner.ID, owner.Username, RoleCoAuthor); err == nil {
		t.Fatalf("不应允许邀请自己")
	}
}

// TestRoleDefaultsAndCaps 角色默认能力：编辑可改他人、审稿可审核、协作作者不可
func TestRoleDefaultsAndCaps(t *testing.T) {
	setupCollabDB(t)
	owner := mkUser(t, "d_owner", models.RoleAuthor)
	ed := mkUser(t, "d_editor", models.RoleReader)
	rv := mkUser(t, "d_reviewer", models.RoleReader)
	novel := mkNovel(t, owner.ID, "角色默认值")

	if _, err := Invite(novel.ID, owner.ID, ed.Username, RoleEditor); err != nil {
		t.Fatalf("邀请编辑失败: %v", err)
	}
	if _, err := Invite(novel.ID, owner.ID, rv.Username, RoleReviewer); err != nil {
		t.Fatalf("邀请审稿失败: %v", err)
	}
	_ = Respond(novel.ID, ed.ID, true)
	_ = Respond(novel.ID, rv.ID, true)

	ea := ActorOf(novel.ID, ed.ID, "")
	if !ea.Can(CapEditOthers) {
		t.Fatalf("编辑应可编辑他人章节")
	}
	if ea.Can(CapWriteChapter) {
		t.Fatalf("编辑不应新建章节（文档 §2.2）")
	}
	if ea.Can(CapPublishChapter) {
		t.Fatalf("编辑不应有发布权")
	}

	ra := ActorOf(novel.ID, rv.ID, "")
	if !ra.Can(CapReviewChapter) {
		t.Fatalf("审稿应可审核")
	}
	if ra.Can(CapWriteChapter) || ra.Can(CapEditOthers) {
		t.Fatalf("审稿不应写正文")
	}
}

// TestShareAndByline 分成总和校验 + 署名顺序
func TestShareAndByline(t *testing.T) {
	setupCollabDB(t)
	owner := mkUser(t, "s_owner", models.RoleAuthor)
	a := mkUser(t, "s_a", models.RoleReader)
	b := mkUser(t, "s_b", models.RoleReader)
	novel := mkNovel(t, owner.ID, "分成测试")

	_, _ = Invite(novel.ID, owner.ID, a.Username, RoleCoAuthor)
	_, _ = Invite(novel.ID, owner.ID, b.Username, RoleCoAuthor)
	_ = Respond(novel.ID, a.ID, true)
	_ = Respond(novel.ID, b.ID, true)

	share := 6000
	if err := Update(novel.ID, owner.ID, a.ID, UpdateInput{RevenueShareBP: &share}); err != nil {
		t.Fatalf("设置分成失败: %v", err)
	}
	share2 := 6000
	if err := Update(novel.ID, owner.ID, b.ID, UpdateInput{RevenueShareBP: &share2}); err == nil {
		t.Fatalf("分成总和超过 100%% 应被拒绝")
	}
	okShare := 4000
	if err := Update(novel.ID, owner.ID, b.ID, UpdateInput{RevenueShareBP: &okShare}); err != nil {
		t.Fatalf("合计 100%% 应允许: %v", err)
	}

	// 署名顺序：a 在前
	orderA, orderB := 10, 20
	_ = Update(novel.ID, owner.ID, a.ID, UpdateInput{BylineOrder: &orderA})
	_ = Update(novel.ID, owner.ID, b.ID, UpdateInput{BylineOrder: &orderB})
	byline := Byline(novel.ID)
	if !strings.Contains(byline, "主笔：s_owner") || !strings.Contains(byline, "协作：s_a、s_b") {
		t.Fatalf("署名格式异常：%s", byline)
	}
	if strings.Index(byline, "s_a") > strings.Index(byline, "s_b") {
		t.Fatalf("署名顺序应按 byline_order：%s", byline)
	}

	// 非主笔不能改设置
	if err := Update(novel.ID, a.ID, b.ID, UpdateInput{RevenueShareBP: &okShare}); err == nil {
		t.Fatalf("协作者不应修改分成")
	}
}

// TestChapterPermission 章节级权限：本人可改、他人需 edit_others、已发布不可直接删
func TestChapterPermission(t *testing.T) {
	setupCollabDB(t)
	owner := mkUser(t, "p_owner", models.RoleAuthor)
	co := mkUser(t, "p_co", models.RoleReader)
	ed := mkUser(t, "p_ed", models.RoleReader)
	novel := mkNovel(t, owner.ID, "章节权限")

	_, _ = Invite(novel.ID, owner.ID, co.Username, RoleCoAuthor)
	_, _ = Invite(novel.ID, owner.ID, ed.Username, RoleEditor)
	_ = Respond(novel.ID, co.ID, true)
	_ = Respond(novel.ID, ed.ID, true)

	coChapter := &models.Chapter{NovelID: novel.ID, ChapterNumber: 1, Title: "协作者章节",
		ContentPath: "x", Status: models.ChapterStatusDraft, AuthorID: co.ID}
	ownerChapter := &models.Chapter{NovelID: novel.ID, ChapterNumber: 2, Title: "主笔章节",
		ContentPath: "y", Status: models.ChapterStatusPublished, AuthorID: owner.ID}
	models.DB.Create(coChapter)
	models.DB.Create(ownerChapter)

	coActor := ActorOf(novel.ID, co.ID, "")
	if !coActor.CanEditChapter(coChapter) {
		t.Fatalf("协作者应可编辑自己的章节")
	}
	if coActor.CanEditChapter(ownerChapter) {
		t.Fatalf("协作者不应编辑主笔章节（无 edit_others）")
	}
	if !coActor.CanDeleteChapter(coChapter) {
		t.Fatalf("草稿应可删除")
	}
	if coActor.CanDeleteChapter(ownerChapter) {
		t.Fatalf("不应删除他人已发布章节")
	}

	edActor := ActorOf(novel.ID, ed.ID, "")
	if !edActor.CanEditChapter(ownerChapter) {
		t.Fatalf("编辑应可改主笔章节")
	}
	if edActor.CanDeleteChapter(coChapter) {
		t.Fatalf("编辑不应删除他人章节")
	}
}
