package handlers

import (
	"testing"

	"nvs-server/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ─────────────────────────────────────────────────────────────
// 黑名单 hide 过滤（F13 补全：论坛帖子/回帖列表）测试
// 独立命名内存库，避免与其它测试共享内存库互相污染
// ─────────────────────────────────────────────────────────────

func setupBlacklistFilterTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:blacklist_filter_test?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("打开内存测试库失败: %v", err)
	}
	if err := models.AutoMigrateAll(db); err != nil {
		t.Fatalf("迁移测试库失败: %v", err)
	}
	models.DB = db
}

func TestFilterHiddenItems(t *testing.T) {
	setupBlacklistFilterTestDB(t)

	const viewer, hiddenAuthor, normalAuthor uint = 1, 2, 3
	// viewer 以 hide 模式拉黑 hiddenAuthor，以 block 模式拉黑 normalAuthor
	if err := models.AddBlacklist(&models.Blacklist{UserID: viewer, BlockedUserID: hiddenAuthor, Mode: models.BlacklistModeHide}); err != nil {
		t.Fatalf("建 hide 拉黑失败: %v", err)
	}
	if err := models.AddBlacklist(&models.Blacklist{UserID: viewer, BlockedUserID: normalAuthor, Mode: models.BlacklistModeBlock}); err != nil {
		t.Fatalf("建 block 拉黑失败: %v", err)
	}

	threads := []models.Thread{
		{UserID: hiddenAuthor}, // 应被滤除
		{UserID: normalAuthor}, // block 模式不在列表过滤范围，保留
		{UserID: 999},          // 无关作者，保留
	}
	kept := FilterHiddenItems(viewer, threads, func(th models.Thread) uint { return th.UserID })
	if len(kept) != 2 || kept[0].UserID != normalAuthor || kept[1].UserID != 999 {
		t.Fatalf("hide 过滤结果不符合预期: %+v", kept)
	}

	// 未登录（viewerID=0）原样返回
	if got := FilterHiddenItems(0, threads, func(th models.Thread) uint { return th.UserID }); len(got) != 3 {
		t.Fatalf("未登录应原样返回 3 条，实际 %d", len(got))
	}

	// Post 列表同样生效
	posts := []models.Post{{UserID: hiddenAuthor}, {UserID: 999}}
	keptPosts := FilterHiddenItems(viewer, posts, func(p models.Post) uint { return p.UserID })
	if len(keptPosts) != 1 || keptPosts[0].UserID != 999 {
		t.Fatalf("Post hide 过滤不符合预期: %+v", keptPosts)
	}

	// 无隐藏记录的用户不过滤任何人
	if got := FilterHiddenItems(777, threads, func(th models.Thread) uint { return th.UserID }); len(got) != 3 {
		t.Fatalf("无拉黑记录的用户应原样返回，实际 %d", len(got))
	}
}
