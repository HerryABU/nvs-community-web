package aiops

import (
	"fmt"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"nvs-server/models"
	ai "nvs-server/models/ai"
	aihub "nvs-server/models/aihub"
	"testing"
	"time"
)

// ===== (原 aigc_test.go) =====

// setupAigcTestDB 独立内存库（不用 setupTestDB 的共享库：同名 cache=shared 会串数据）
func setupAigcTestDB(t *testing.T) {
	t.Helper()
	prev := models.DB
	dsn := fmt.Sprintf("file:aigc_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存库失败: %v", err)
	}
	if err := db.AutoMigrate(&AigcAsset{}, &AigcExport{}, &AigcWaiverLog{}, &aihub.AiChannelAccount{}, &models.PlatformConfig{}); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	models.DB = db
	t.Cleanup(func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
		models.DB = prev
	})
}

// 关键回归：labeled / waived / prompt_attached 的「正确值」经常是 false，
// 而 GORM 的 Create 会跳过零值、让列默认值生效 —— 一旦默认值写错，
// 审计里就会看到「用户上传图也标了角标」这种假记录。
func TestCreateAigcExportKeepsFalseFlags(t *testing.T) {
	setupAigcTestDB(t)

	unlabeled := &AigcExport{
		ProduceID: "pid-unlabeled",
		ModelName: "user-upload",
		Labeled:   false,
		Waived:    false,
	}
	if err := CreateAigcExport(unlabeled); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := GetAigcExportByProduceID("pid-unlabeled")
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if got.Labeled {
		t.Fatal("labeled 应保持 false（Create 跳零值 + 列默认 true 会把它写成 true）")
	}
	if got.Waived || got.PromptAttached {
		t.Fatalf("waived/prompt 应为 false：%+v", got)
	}

	labeled := &AigcExport{ProduceID: "pid-labeled", ModelName: "m", Labeled: true, Waived: true, PromptAttached: true}
	if err := CreateAigcExport(labeled); err != nil {
		t.Fatalf("create labeled: %v", err)
	}
	got2, err := GetAigcExportByProduceID("pid-labeled")
	if err != nil {
		t.Fatalf("read back labeled: %v", err)
	}
	if !got2.Labeled || !got2.Waived || !got2.PromptAttached {
		t.Fatalf("true 值被吞了：%+v", got2)
	}
}

// ProduceID 必须唯一（同一个内容编号不能对应两条记录，否则回查会歧义）
func TestAigcExportProduceIDUnique(t *testing.T) {
	setupAigcTestDB(t)
	if err := CreateAigcExport(&AigcExport{ProduceID: "dup", ModelName: "a"}); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := CreateAigcExport(&AigcExport{ProduceID: "dup", ModelName: "b"}); err == nil {
		t.Fatal("重复 ProduceID 应当报错")
	}
}

// 豁免日志独立表：字段与隔离墙无关，且能按 ProduceID 回查
func TestAigcWaiverLogFields(t *testing.T) {
	setupAigcTestDB(t)
	log := &AigcWaiverLog{
		UserID:           7,
		UserName:         "测试用户",
		ProduceID:        "pid-w",
		AgreementVersion: "sha256:abcd1234",
		InputText:        "我确认申请不含显式标识版本",
		InputHash:        "deadbeef",
		ResponseResult:   "granted",
	}
	if err := CreateAigcWaiverLog(log); err != nil {
		t.Fatalf("create: %v", err)
	}
	list, total := ListAigcWaiverLogs(1, 10)
	if total != 1 || len(list) != 1 {
		t.Fatalf("列表 = %d/%d", len(list), total)
	}
	if list[0].InputText != log.InputText || list[0].InputHash != log.InputHash {
		t.Fatalf("原文/哈希没存下来：%+v", list[0])
	}
}

// 配置保存与读取：角标字号低于国标 5% 必须被抬上来
func TestAigcConfigNormalize(t *testing.T) {
	setupAigcTestDB(t)
	c := DefaultAigcConfig()
	c.FontSizePct = 0.01  // 故意低于国标
	c.Position = "middle" // 非法位置
	c.TextColor = "red"   // 非法颜色
	c.ContentProducer = "  某站  "
	if err := SaveAigcConfig(c); err != nil {
		t.Fatalf("save: %v", err)
	}
	got := LoadAigcConfig()
	if got.FontSizePct < 0.05 {
		t.Fatalf("字号比例 %v 低于 5%%", got.FontSizePct)
	}
	if got.Position != "bottom-right" {
		t.Fatalf("非法位置未回退：%q", got.Position)
	}
	if got.TextColor != "#FFFFFF" {
		t.Fatalf("非法颜色未回退：%q", got.TextColor)
	}
	if got.ContentProducer != "某站" {
		t.Fatalf("ContentProducer 未去空格：%q", got.ContentProducer)
	}
}

// ===== 合并自 ai_vendor_feature_test.go =====

// TestAiVendorCRUD 厂商字典：重名拒绝、改名级联、被引用时禁止删除
func TestAiVendorCRUD(t *testing.T) {
	db := setupAiTestDB(t)
	old := models.DB
	models.DB = db
	defer func() { models.DB = old }()

	v, err := CreateAiVendor("OpenAI", 1)
	if err != nil {
		t.Fatalf("新增厂商失败：%v", err)
	}
	if _, err := CreateAiVendor("OpenAI", 2); err == nil {
		t.Fatalf("重名厂商应被拒绝")
	}

	// 关联一个模型
	m := &ai.AiModel{Name: "gpt-test", Kind: ai.AiKindChat, ModelID: "gpt-test", BaseURL: "https://x", Vendor: "OpenAI", Enabled: true}
	if err := models.DB.Create(m).Error; err != nil {
		t.Fatalf("建模型失败：%v", err)
	}
	if err := DeleteAiVendor(v.ID); err == nil {
		t.Fatalf("仍有模型引用时应拒绝删除")
	}

	// 改名级联（prefix 不传则保留原值）
	if err := UpdateAiVendor(v.ID, "OpenAI Inc", 3, true); err != nil {
		t.Fatalf("改名失败：%v", err)
	}
	var after ai.AiModel
	models.DB.Where("id = ?", m.ID).First(&after)
	if after.Vendor != "OpenAI Inc" {
		t.Fatalf("改名未级联到模型：%q", after.Vendor)
	}

	// 模型改走后即可删除
	models.DB.Model(&ai.AiModel{}).Where("id = ?", m.ID).UpdateColumn("vendor", "其他")
	if err := DeleteAiVendor(v.ID); err != nil {
		t.Fatalf("无引用应可删除：%v", err)
	}
}

// TestSeedAiVendorsBackfillsPrefix 老库升级：ai_vendors 没有 prefix 列 →
// AutoMigrate 只加列、存量全是空串，SeedAiVendors 必须把前缀补回来，
// 否则「按模型 ID 前缀匹配厂商」在老站上永远匹配不上（且界面看不出异常）。
func TestSeedAiVendorsBackfillsPrefix(t *testing.T) {
	db := setupAiTestDB(t)
	old := models.DB
	models.DB = db
	defer func() { models.DB = old }()

	// 模拟老数据：预置厂商 + 自建厂商，前缀都是空
	models.DB.Create(&AiVendor{Name: "OpenAI", Prefix: "", SortOrder: 1, Enabled: true})
	models.DB.Create(&AiVendor{Name: "DeepSeek", Prefix: "", SortOrder: 2, Enabled: true})
	models.DB.Create(&AiVendor{Name: "My Vendor", Prefix: "", SortOrder: 3, Enabled: true})
	// 管理员手工改过的前缀不能被覆盖
	models.DB.Create(&AiVendor{Name: "Google", Prefix: "g-custom", SortOrder: 4, Enabled: true})

	SeedAiVendors()

	cases := map[string]string{
		"OpenAI":    "openai",
		"DeepSeek":  "deepseek",
		"My Vendor": "myvendor",
		"Google":    "g-custom", // 已有值保持不动
	}
	for name, want := range cases {
		var v AiVendor
		if err := models.DB.Where("name = ?", name).First(&v).Error; err != nil {
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
	old := models.DB
	models.DB = db
	defer func() { models.DB = old }()

	def := aihub.GetAiFeatureFlags()
	if !def.AllowByok || def.ContextMsgs != 20 || def.DailyChatLimit != -1 {
		t.Fatalf("默认值异常：%+v", def)
	}

	if err := aihub.SetAiFeatureFlags(aihub.AiFeatureFlags{
		AllowByok: false, AllowI2I: false, AllowFaceSwap: true,
		ContextMsgs: 999, DailyChatLimit: -9, PublicGallery: false,
	}); err != nil {
		t.Fatalf("保存失败：%v", err)
	}
	got := aihub.GetAiFeatureFlags()
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
