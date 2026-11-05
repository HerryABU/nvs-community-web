package wallet

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"nvs-server/models"
)

// setupAiTestDB 独立唯一内存库。原根包同名 helper 未导出、跨包不可复用，
// 此处以 models.AutoMigrateAll 迁移全部已注册模型，行为一致。
func setupAiTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:wallet_test_" + time.Now().Format("20060102150405.000000000") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if err := models.AutoMigrateAll(db); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	models.DB = db
	t.Cleanup(func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}
