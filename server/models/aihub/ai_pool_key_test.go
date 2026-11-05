package aihub

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"nvs-server/models"
	ai "nvs-server/models/ai"
	"testing"
	"time"
)

// ===== (原 ai_pool_key_test.go) =====

// TestModelKeyPool 密钥池：取用、失败计数、达阈值自动停用、重置、回退
func TestModelKeyPool(t *testing.T) {
	db := setupAiTestDB(t)
	old := models.DB
	models.DB = db
	defer func() { models.DB = old }()

	m := &ai.AiModel{Name: "pool-test", Kind: ai.AiKindChat, ModelID: "x", BaseURL: "https://x", Enabled: true}
	if err := models.DB.Create(m).Error; err != nil {
		t.Fatalf("建模型失败：%v", err)
	}
	// 无池时回退到模型自带密钥（未设置则报错，属预期）
	if _, err := AcquireModelKey(m); err == nil {
		t.Fatalf("既无池也无自带密钥时应失败")
	}

	k1, err := AddAiModelKey(m.ID, "主", "sk-aaaaaaaaaaaa1111", 1)
	if err != nil {
		t.Fatalf("添加密钥失败：%v", err)
	}
	if _, err := AddAiModelKey(m.ID, "空", "   ", 1); err == nil {
		t.Fatalf("空密钥应被拒绝")
	}
	if _, err := AddAiModelKey(m.ID, "短", "abc", 1); err == nil {
		t.Fatalf("过短密钥应被拒绝")
	}

	lease, err := AcquireModelKey(m)
	if err != nil || lease == nil || lease.Key == "" {
		t.Fatalf("取密钥失败：%v", err)
	}
	if lease.Source != "pool" || lease.KeyID != k1.ID {
		t.Fatalf("应来自密钥池：%+v", lease)
	}

	// 连续失败达阈值 → 自动停用 → 池内无可用
	for i := 0; i < MaxFailBeforeDisable; i++ {
		ReportAiModelKey(k1.ID, false)
	}
	if k := PickAiModelKey(m.ID); k != nil {
		t.Fatalf("达失败阈值后不应再被选中：%+v", k)
	}
	var after AiModelKey
	models.DB.Where("id = ?", k1.ID).First(&after)
	if after.Enabled {
		t.Fatalf("达失败阈值应自动停用")
	}

	// 重置并重新启用后可用，成功上报清零失败计数
	if err := UpdateAiModelKey(k1.ID, m.ID, map[string]interface{}{"enabled": true, "fail_count": 0}); err != nil {
		t.Fatalf("重置失败：%v", err)
	}
	ReportAiModelKey(k1.ID, false)
	ReportAiModelKey(k1.ID, true)
	models.DB.Where("id = ?", k1.ID).First(&after)
	if after.FailCount != 0 {
		t.Fatalf("成功上报应清零失败计数，得到 %d", after.FailCount)
	}
}

// TestAiStyleCRUD 风格字典：重名拒绝、改名级联作品、删除后历史风格保留
func TestAiStyleCRUD(t *testing.T) {
	db := setupAiTestDB(t)
	old := models.DB
	models.DB = db
	defer func() { models.DB = old }()

	st, err := CreateAiStyle("image", "写实摄影", 1)
	if err != nil {
		t.Fatalf("新增风格失败：%v", err)
	}
	if _, err := CreateAiStyle("image", "写实摄影", 2); err == nil {
		t.Fatalf("同类型重名应被拒绝")
	}
	if _, err := CreateAiStyle("video", "写实摄影", 1); err != nil {
		t.Fatalf("不同类型允许同名：%v", err)
	}

	task := &ai.AiMediaTask{UserID: 1, Kind: ai.AiKindImage, Prompt: "p", Status: ai.AiMediaSucceeded, Style: "写实摄影"}
	models.DB.Create(task)

	if err := UpdateAiStyle(st.ID, "写实摄影PRO", 3, true); err != nil {
		t.Fatalf("改名失败：%v", err)
	}
	var after ai.AiMediaTask
	models.DB.Where("id = ?", task.ID).First(&after)
	if after.Style != "写实摄影PRO" {
		t.Fatalf("改名未级联作品：%q", after.Style)
	}

	if err := DeleteAiStyle(st.ID); err != nil {
		t.Fatalf("删除失败：%v", err)
	}
	models.DB.Where("id = ?", task.ID).First(&after)
	if after.Style != "写实摄影PRO" {
		t.Fatalf("删除风格不应清空历史作品风格")
	}
}

// ===== 合并自 testdb_test.go =====

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
func seedAiModel(t *testing.T, priceIn, priceOut int64, maxTokens int) *ai.AiModel {
	t.Helper()
	m := &ai.AiModel{
		Name: "测试模型", ModelID: "test-model", BaseURL: "http://127.0.0.1:1/v1",
		Enabled: true, PriceIn: priceIn, PriceOut: priceOut, MaxTokens: maxTokens,
	}
	if err := models.DB.Create(m).Error; err != nil {
		t.Fatalf("创建模型失败: %v", err)
	}
	return m
}
