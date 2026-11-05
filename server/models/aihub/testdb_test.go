package aihub

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"nvs-server/models"
)

// setupAigcTestDB 独立唯一内存库。原实现位于 aiops/aigc_test.go（未导出，
// 跨包不可复用），此处以 models.AutoMigrateAll 迁移全部已注册模型，行为等价。
func setupAigcTestDB(t *testing.T) {
	t.Helper()
	prev := models.DB
	dsn := "file:aihub_test_" + time.Now().Format("20060102150405.000000000") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存库失败: %v", err)
	}
	if err := models.AutoMigrateAll(db); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	models.DB = db
	t.Cleanup(func() {
		models.DB = prev
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
}

// setupAiTestDB 供密钥池等测试复用：独立唯一内存库，迁移全部已注册模型。
func setupAiTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:aihub_pool_test_" + time.Now().Format("20060102150405.000000000") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if err := models.AutoMigrateAll(db); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	// 与根包原 helper 一致：替换全局 DB，供只调用不接收返回值的测试使用。
	models.DB = db
	return db
}

// seedAiModel 与根包 ai_billing_test.go 中的同名 helper 等价（未导出跨包不可复用）。
func seedAiModel(t *testing.T, priceIn, priceOut int64, maxTokens int) *models.AiModel {
	t.Helper()
	m := &models.AiModel{
		Name: "测试模型", ModelID: "test-model", BaseURL: "http://127.0.0.1:1/v1",
		Enabled: true, PriceIn: priceIn, PriceOut: priceOut, MaxTokens: maxTokens,
	}
	if err := models.DB.Create(m).Error; err != nil {
		t.Fatalf("创建模型失败: %v", err)
	}
	return m
}
