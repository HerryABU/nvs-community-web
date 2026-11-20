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

	if n, _ := migrateUserContent(0, contentstore.ModeDatabase, 20, nil); n != 1 {
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

// 迁移结果必须分类计数：源为空 / 源缺失跳过 / 正常入库 三种要能区分开，
// 否则管理员看到「库内条数对不上」时无从判断是漏迁还是原本就没有正文。
func TestMigrateTallyAndStorageHealth(t *testing.T) {
	setupStorageTestDB(t)
	dir := t.TempDir()
	okPath := filepath.Join(dir, "ok.html")
	if err := os.WriteFile(okPath, []byte("<p>正文</p>"), 0o644); err != nil {
		t.Fatal(err)
	}
	emptyPath := filepath.Join(dir, "empty.html")
	if err := os.WriteFile(emptyPath, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	missingPath := filepath.Join(dir, "gone.html") // 故意不创建

	mk := func(num int, p string) models.Chapter {
		ch := models.Chapter{
			NovelID: 9, ChapterNumber: num, Title: fmt.Sprintf("第%d章", num), AuthorID: 5,
			ContentPath: filepath.ToSlash(p), ContentHash: "h", WordCount: 1,
		}
		if err := models.DB.Create(&ch).Error; err != nil {
			t.Fatal(err)
		}
		return ch
	}
	mk(1, okPath)
	emp := mk(2, emptyPath)
	gone := mk(3, missingPath)

	n, tally := migrateUserContent(5, contentstore.ModeDatabase, 20, nil)
	if n != 3 {
		t.Fatalf("应处理 3 条，实际 %d", n)
	}
	if tally.OK != 1 || tally.Empty != 1 || tally.Skipped != 1 {
		t.Fatalf("分类计数不对: %+v", tally)
	}
	// 源为空 → 忠实写空副本；源缺失 → 绝不能写空副本冒充迁移成功
	if txt, ok := contentstore.LoadDB(contentstore.RefChapter, emp.ID); !ok || txt != "" {
		t.Fatalf("空源章节应写入空副本: %q %v", txt, ok)
	}
	if _, ok := contentstore.LoadDB(contentstore.RefChapter, gone.ID); ok {
		t.Fatal("源缺失的章节不应产生库内副本")
	}

	rep := storageHealth()
	if rep.Chapter.Total != 3 || rep.Chapter.Blob != 2 || rep.Chapter.Empty != 1 || rep.Chapter.Missed != 1 {
		t.Fatalf("健康报告计数不对: %+v", rep.Chapter)
	}
	if rep.Chapter.Bytes != int64(len("<p>正文</p>")) {
		t.Fatalf("正文字节数应按真实字节统计（中文不折半）: %d", rep.Chapter.Bytes)
	}
	if len(rep.Missing) != 1 || rep.Missing[0]["id"] != gone.ID {
		t.Fatalf("缺失样本应列出该章节: %+v", rep.Missing)
	}
	if rep.Missing[0]["file_exists"] != false {
		t.Fatalf("file_exists 应为 false: %+v", rep.Missing[0])
	}
}
