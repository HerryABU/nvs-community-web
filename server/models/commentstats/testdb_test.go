package commentstats

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"nvs-server/models"
)

// setupTestDB 独立唯一内存库。原根包同名 helper 未导出、跨包不可复用，
// 此处以 models.AutoMigrateAll 迁移全部已注册模型，行为一致。
func setupTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:commentstats_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("创建内存库失败: %v", err)
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
}
