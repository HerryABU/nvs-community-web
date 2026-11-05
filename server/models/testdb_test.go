package models

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupTestDB 创建内存 sqlite 测试库并迁移全部表，替换全局 DB。
// 每个测试独立调用，避免相互污染。博客域拆分后，原 models 包测试仍依赖此 helper。
func setupTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
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
