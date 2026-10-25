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

// TestSeedAiVendorsBackfillsPrefix 老库升级：ai_vendors 没有 prefix 列 →
// AutoMigrate 只加列、存量全是空串，SeedAiVendors 必须把前缀补回来，
// 否则「按模型 ID 前缀匹配厂商」在老站上永远匹配不上（且界面看不出异常）。
func TestSeedAiVendorsBackfillsPrefix(t *testing.T) {
	db := setupAiTestDB(t)
	old := DB
	DB = db
	defer func() { DB = old }()

	// 模拟老数据：预置厂商 + 自建厂商，前缀都是空
	DB.Create(&AiVendor{Name: "OpenAI", Prefix: "", SortOrder: 1, Enabled: true})
	DB.Create(&AiVendor{Name: "DeepSeek", Prefix: "", SortOrder: 2, Enabled: true})
	DB.Create(&AiVendor{Name: "My Vendor", Prefix: "", SortOrder: 3, Enabled: true})
	// 管理员手工改过的前缀不能被覆盖
	DB.Create(&AiVendor{Name: "Google", Prefix: "g-custom", SortOrder: 4, Enabled: true})

	SeedAiVendors()

	cases := map[string]string{
		"OpenAI":    "openai",
		"DeepSeek":  "deepseek",
		"My Vendor": "myvendor",
		"Google":    "g-custom", // 已有值保持不动
	}
	for name, want := range cases {
		var v AiVendor
		if err := DB.Where("name = ?", name).First(&v).Error; err != nil {
			t.Fatalf("厂商 %s 不存在", name)
		}
		if v.Prefix != want {
			t.Fatalf("厂商 %s 前缀回填错误：得到 %q，期望 %q", name, v.Prefix, want)
		}
	}
	// 回填后应能按前缀匹配
	if got := MatchVendorByPrefix("deepseek"); got == nil || got.Name != "DeepSeek" {
		t.Fatalf("按前缀匹配失败：%+v", got)
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
