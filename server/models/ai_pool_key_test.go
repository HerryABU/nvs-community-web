package models

import "testing"

// TestModelKeyPool 密钥池：取用、失败计数、达阈值自动停用、重置、回退
func TestModelKeyPool(t *testing.T) {
	db := setupAiTestDB(t)
	old := DB
	DB = db
	defer func() { DB = old }()

	m := &AiModel{Name: "pool-test", Kind: AiKindChat, ModelID: "x", BaseURL: "https://x", Enabled: true}
	if err := DB.Create(m).Error; err != nil {
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
	DB.Where("id = ?", k1.ID).First(&after)
	if after.Enabled {
		t.Fatalf("达失败阈值应自动停用")
	}

	// 重置并重新启用后可用，成功上报清零失败计数
	if err := UpdateAiModelKey(k1.ID, m.ID, map[string]interface{}{"enabled": true, "fail_count": 0}); err != nil {
		t.Fatalf("重置失败：%v", err)
	}
	ReportAiModelKey(k1.ID, false)
	ReportAiModelKey(k1.ID, true)
	DB.Where("id = ?", k1.ID).First(&after)
	if after.FailCount != 0 {
		t.Fatalf("成功上报应清零失败计数，得到 %d", after.FailCount)
	}
}

// TestAiStyleCRUD 风格字典：重名拒绝、改名级联作品、删除后历史风格保留
func TestAiStyleCRUD(t *testing.T) {
	db := setupAiTestDB(t)
	old := DB
	DB = db
	defer func() { DB = old }()

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

	task := &AiMediaTask{UserID: 1, Kind: AiKindImage, Prompt: "p", Status: AiMediaSucceeded, Style: "写实摄影"}
	DB.Create(task)

	if err := UpdateAiStyle(st.ID, "写实摄影PRO", 3, true); err != nil {
		t.Fatalf("改名失败：%v", err)
	}
	var after AiMediaTask
	DB.Where("id = ?", task.ID).First(&after)
	if after.Style != "写实摄影PRO" {
		t.Fatalf("改名未级联作品：%q", after.Style)
	}

	if err := DeleteAiStyle(st.ID); err != nil {
		t.Fatalf("删除失败：%v", err)
	}
	DB.Where("id = ?", task.ID).First(&after)
	if after.Style != "写实摄影PRO" {
		t.Fatalf("删除风格不应清空历史作品风格")
	}
}
