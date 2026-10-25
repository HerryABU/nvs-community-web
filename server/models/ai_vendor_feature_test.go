package models

import "testing"

// TestAiVendorCRUD 厂商字典：重名拒绝、改名级联、被引用时禁止删除
func TestAiVendorCRUD(t *testing.T) {
	db := setupAiTestDB(t)
	old := DB
	DB = db
	defer func() { DB = old }()

	v, err := CreateAiVendor("OpenAI", 1)
	if err != nil {
		t.Fatalf("新增厂商失败：%v", err)
	}
	if _, err := CreateAiVendor("OpenAI", 2); err == nil {
		t.Fatalf("重名厂商应被拒绝")
	}

	// 关联一个模型
	m := &AiModel{Name: "gpt-test", Kind: AiKindChat, ModelID: "gpt-test", BaseURL: "https://x", Vendor: "OpenAI", Enabled: true}
	if err := DB.Create(m).Error; err != nil {
		t.Fatalf("建模型失败：%v", err)
	}
	if err := DeleteAiVendor(v.ID); err == nil {
		t.Fatalf("仍有模型引用时应拒绝删除")
	}

	// 改名级联（prefix 不传则保留原值）
	if err := UpdateAiVendor(v.ID, "OpenAI Inc", 3, true); err != nil {
		t.Fatalf("改名失败：%v", err)
	}
	var after AiModel
	DB.Where("id = ?", m.ID).First(&after)
	if after.Vendor != "OpenAI Inc" {
		t.Fatalf("改名未级联到模型：%q", after.Vendor)
	}

	// 模型改走后即可删除
	DB.Model(&AiModel{}).Where("id = ?", m.ID).UpdateColumn("vendor", "其他")
	if err := DeleteAiVendor(v.ID); err != nil {
		t.Fatalf("无引用应可删除：%v", err)
	}
}

// TestAiFeatureFlags AI 功能开关：默认值、范围校正、持久化
func TestAiFeatureFlags(t *testing.T) {
	db := setupAiTestDB(t)
	old := DB
	DB = db
	defer func() { DB = old }()

	def := GetAiFeatureFlags()
	if !def.AllowByok || def.ContextMsgs != 20 || def.DailyChatLimit != -1 {
		t.Fatalf("默认值异常：%+v", def)
	}

	if err := SetAiFeatureFlags(AiFeatureFlags{
		AllowByok: false, AllowI2I: false, AllowFaceSwap: true,
		ContextMsgs: 999, DailyChatLimit: -9, PublicGallery: false,
	}); err != nil {
		t.Fatalf("保存失败：%v", err)
	}
	got := GetAiFeatureFlags()
	if got.ContextMsgs != 60 {
		t.Fatalf("上下文条数应被钳到 60，得到 %d", got.ContextMsgs)
	}
	if got.DailyChatLimit != -1 {
		t.Fatalf("非法限额应回归 -1，得到 %d", got.DailyChatLimit)
	}
	if got.AllowByok || got.PublicGallery != false {
		t.Fatalf("开关未持久化：%+v", got)
	}
}
