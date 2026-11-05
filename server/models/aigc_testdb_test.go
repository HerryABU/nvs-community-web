package models

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupAigcTestDB 为仍留在根包的 AIGC / 渠道账户相关测试提供独立内存库。
// 原实现位于 aiops/aigc_test.go（未导出，跨包不可复用），此处改以 AutoMigrateAll
// 迁移全部已注册模型，行为等价。
func setupAigcTestDB(t *testing.T) {
	t.Helper()
	prev := DB
	dsn := fmt.Sprintf("file:aigc_root_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存库失败: %v", err)
	}
	if err := AutoMigrateAll(db); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	DB = db
	t.Cleanup(func() {
		DB = prev
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
}
