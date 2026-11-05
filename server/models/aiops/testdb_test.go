package aiops

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"nvs-server/models"
)

// setupAiTestDB 独立唯一内存库。原根包 ai_billing_test.go 里的同名 helper 未导出，
// 子包无法跨包复用，故在本子包内自建一份（行为一致）。
func setupAiTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:aiops_test_" + time.Now().Format("20060102150405.000000000") + "?mode=memory&cache=shared"
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
