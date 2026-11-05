package models

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupAiTestDB 供仍留在根包的支付等测试复用。原实现位于 models/ai/ai_billing_test.go
// （未导出，跨包不可复用），此处以 AutoMigrateAll 迁移全部已注册模型，行为一致。
func setupAiTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:root_ai_test_" + time.Now().Format("20060102150405.000000000") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if err := AutoMigrateAll(db); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	DB = db
	t.Cleanup(func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}
