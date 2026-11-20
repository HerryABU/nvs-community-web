package dbadmin

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"nvs-server/config"
	"nvs-server/contentstore"
	"nvs-server/models"
)

func setupStorageTestDB(t *testing.T) {
	t.Helper()
	name := fmt.Sprintf("file:storagemode_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(name), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := models.AutoMigrateAll(db); err != nil {
		t.Fatal(err)
	}
	models.DB = db
	config.UploadDir = filepath.Join(t.TempDir(), "uploads")
	t.Cleanup(func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
}

// 老数据里 chapter.author_id 常为 0（作者只在 novel 上）。
// 迁移按用户分批时若把 0 过滤掉，这类章节会全部漏迁，切纯数据库模式后正文读不出来。
func TestMigrateChaptersWithZeroAuthorID(t *testing.T) {
	setupStorageTestDB(t)
	dir := t.TempDir()
	p := filepath.Join(dir, "1.html")
	if err := os.WriteFile(p, []byte("老章节正文"), 0o644); err != nil {
		t.Fatal(err)
	}
	ch := models.Chapter{
		NovelID: 1, ChapterNumber: 1, Title: "第一章", AuthorID: 0,
		ContentPath: filepath.ToSlash(p), ContentHash: "x", WordCount: 5,
	}
	if err := models.DB.Create(&ch).Error; err != nil {
		t.Fatal(err)
	}

	users := collectContentUsers()
	found := false
	for _, u := range users {
		if u == 0 {
			found = true
		}
	}
	if !found {
		t.Fatalf("author_id=0 必须也在分批用户列表里: %#v", users)
	}
	if n := countMissingContent(); n != 1 {
		t.Fatalf("迁移前应有 1 条缺失: %d", n)
	}

	if n := migrateUserContent(0, contentstore.ModeDatabase, 20); n != 1 {
		t.Fatalf("应处理 1 条章节，实际 %d", n)
	}
	text, ok := contentstore.LoadDB(contentstore.RefChapter, ch.ID)
	if !ok || text != "老章节正文" {
		t.Fatalf("章节正文未入库: %q %v", text, ok)
	}
	if n := countMissingContent(); n != 0 {
		t.Fatalf("迁移后不应还有缺失: %d", n)
	}
}
