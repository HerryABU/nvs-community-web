package aiops

import (
	"fmt"
	"nvs-server/models"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupAigcTestDB 独立内存库（不用 setupTestDB 的共享库：同名 cache=shared 会串数据）
func setupAigcTestDB(t *testing.T) {
	t.Helper()
	prev := models.DB
	dsn := fmt.Sprintf("file:aigc_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存库失败: %v", err)
	}
	if err := db.AutoMigrate(&AigcAsset{}, &AigcExport{}, &AigcWaiverLog{}, &models.AiChannelAccount{}, &models.PlatformConfig{}); err != nil {
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
