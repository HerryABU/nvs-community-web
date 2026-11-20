package contentstore

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"nvs-server/models"
)

func setupStoreTest(t *testing.T) string {
	t.Helper()
	name := fmt.Sprintf("file:contentstore_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(name), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := models.AutoMigrateAll(db); err != nil {
		t.Fatal(err)
	}
	models.DB = db
	dir := t.TempDir()
	t.Cleanup(func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
	return dir
}

func TestHybridWritesFileAndDB(t *testing.T) {
	dir := setupStoreTest(t)
	_ = models.SetPlatformConfig(configKey, ModeHybrid)
	path := filepath.Join(dir, "1.html")
	if err := Put(RefChapter, 1, path, "正文一"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("混合模式应落文件: %v", err)
	}
	if text, ok := LoadDB(RefChapter, 1); !ok || text != "正文一" {
		t.Fatalf("混合模式也应有库内镜像: %q %v", text, ok)
	}
}

func TestDatabaseModeNoFile(t *testing.T) {
	dir := setupStoreTest(t)
	_ = models.SetPlatformConfig(configKey, ModeDatabase)
	path := filepath.Join(dir, "2.html")
	if err := Put(RefChapter, 2, path, "正文二"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("纯数据库模式不应落文件: %v", err)
	}
	got, err := Get(RefChapter, 2, path)
	if err != nil || got != "正文二" {
		t.Fatalf("纯数据库模式应能从库读取: %q %v", got, err)
	}
}

func TestHybridFallsBackToFileLoss(t *testing.T) {
	dir := setupStoreTest(t)
	_ = models.SetPlatformConfig(configKey, ModeHybrid)
	path := filepath.Join(dir, "3.html")
	if err := Put(RefChapter, 3, path, "正文三"); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(path)
	got, err := Get(RefChapter, 3, path)
	if err != nil || got != "正文三" {
		t.Fatalf("文件丢失应回退库内副本: %q %v", got, err)
	}
}
