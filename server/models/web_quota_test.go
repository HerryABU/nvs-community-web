package models

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupWebQuotaTestDB 独立内存库（唯一名，避免与同包其他测试共享污染）
func setupWebQuotaTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:webquota_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if err := AutoMigrateAll(db); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	DB = db
	t.Cleanup(func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestWebQuotaSizeConfigDefault(t *testing.T) {
	setupWebQuotaTestDB(t)
	// 缺省 0 = 不限
	if got := GetWebPublishQuotaSizeConfig(); got != 0 {
		t.Errorf("缺省大小额度应为 0（不限），实际 %d", got)
	}
	// 非法值回退 0
	_ = SetPlatformConfig(WebPublishQuotaSizeKey, "abc")
	if got := GetWebPublishQuotaSizeConfig(); got != 0 {
		t.Errorf("非法值应回退 0，实际 %d", got)
	}
	// 负值回退 0
	_ = SetPlatformConfig(WebPublishQuotaSizeKey, "-5")
	if got := GetWebPublishQuotaSizeConfig(); got != 0 {
		t.Errorf("负值应回退 0，实际 %d", got)
	}
	// 上限钳制
	if err := SetWebPublishQuotaSizeConfig(999999); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	if got := GetWebPublishQuotaSizeConfig(); got != MaxWebQuotaSizeMB {
		t.Errorf("应钳制到 %d，实际 %d", MaxWebQuotaSizeMB, got)
	}
}

func TestSetUserWebQuotaOverrideZeroValues(t *testing.T) {
	setupWebQuotaTestDB(t)
	// 0=禁止公开 与 0=不限大小 都是零值，必须能显式写入（GORM map upsert）
	if err := SetUserWebQuotaOverride(1, 0, 0, "测试零值"); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	ov := GetUserWebQuotaOverride(1)
	if ov == nil {
		t.Fatalf("覆盖记录应存在")
	}
	if ov.Quota != 0 {
		t.Errorf("quota 应为 0（禁止），实际 %d", ov.Quota)
	}
	if ov.QuotaSizeMB != 0 {
		t.Errorf("quota_size_mb 应为 0（不限），实际 %d", ov.QuotaSizeMB)
	}
	// 更新为专属上限
	if err := SetUserWebQuotaOverride(1, 5, 100, "调整"); err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	ov = GetUserWebQuotaOverride(1)
	if ov.Quota != 5 || ov.QuotaSizeMB != 100 {
		t.Errorf("更新后应为 5/100，实际 %d/%d", ov.Quota, ov.QuotaSizeMB)
	}
	// 生效值：按人覆盖优先
	if got := GetWebPublishQuotaSizeMB(1); got != 100 {
		t.Errorf("生效大小额度应为 100，实际 %d", got)
	}
	// -1 = 跟随全局
	if err := SetUserWebQuotaOverride(1, -1, -1, "跟随全局"); err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	_ = SetWebPublishQuotaSizeConfig(50)
	if got := GetWebPublishQuotaSizeMB(1); got != 50 {
		t.Errorf("跟随全局应得 50，实际 %d", got)
	}
}

func TestPathSizeBytes(t *testing.T) {
	dir := t.TempDir()
	// 单文件
	f := filepath.Join(dir, "a.txt")
	os.WriteFile(f, make([]byte, 1234), 0644)
	if got := pathSizeBytes(f); got != 1234 {
		t.Errorf("文件大小应 1234，实际 %d", got)
	}
	// 目录递归
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(sub, "b.bin"), make([]byte, 2000), 0644)
	os.WriteFile(filepath.Join(sub, "c.bin"), make([]byte, 3000), 0644)
	want := int64(1234 + 2000 + 3000)
	if got := pathSizeBytes(dir); got != want {
		t.Errorf("目录大小应 %d，实际 %d", want, got)
	}
	// 不存在 / 空路径
	if got := pathSizeBytes(filepath.Join(dir, "nope")); got != 0 {
		t.Errorf("不存在路径应 0，实际 %d", got)
	}
	if got := pathSizeBytes(""); got != 0 {
		t.Errorf("空路径应 0，实际 %d", got)
	}
}

func TestCheckAndApplyWebPublishSizeQuota(t *testing.T) {
	db := setupWebQuotaTestDB(t)

	// 造一个用户 + 一个已公开 UserHTML（5MB）+ 一个待公开 UserHTML（5MB）
	db.Create(&User{Username: "u1", Email: "u1@test.com", PasswordHash: "x"})
	h1 := UserHTML{UserID: 1, Name: "h1", ExtractDir: "x", EntryFile: "index.html", IsPublic: true, TotalSize: 5 * 1024 * 1024}
	db.Create(&h1)
	h2 := UserHTML{UserID: 1, Name: "h2", ExtractDir: "y", EntryFile: "index.html", TotalSize: 5 * 1024 * 1024}
	db.Create(&h2)

	// 场景 1：数量够、大小不限 → 直接公开
	_ = SetWebPublishQuotaConfig(100)
	_ = SetWebPublishQuotaSizeConfig(0)
	allowed, needReview, _, err := CheckAndApplyWebPublish(1, WebQuotaTargetHTML, h2.ID)
	if err != nil || !allowed || needReview {
		t.Fatalf("不限额度应直接公开: allowed=%v needReview=%v err=%v", allowed, needReview, err)
	}

	// 场景 2：大小额度 6MB，已用 5MB + 新项目 5MB = 10MB > 6MB → 超额审核
	_ = SetWebPublishQuotaSizeConfig(6)
	// 先把 h2 改回私有
	db.Model(&UserHTML{}).Where("id = ?", h2.ID).Updates(map[string]interface{}{"is_public": false, "review_status": "approved"})
	allowed, needReview, reason, err := CheckAndApplyWebPublish(1, WebQuotaTargetHTML, h2.ID)
	if err != nil {
		t.Fatalf("不应出错: %v", err)
	}
	if allowed || !needReview {
		t.Errorf("大小超额应转审核: allowed=%v needReview=%v", allowed, needReview)
	}
	if reason == "" {
		t.Errorf("应返回超额原因")
	}
	// 目标应进入 quota_review 且不公开
	var h2After UserHTML
	db.First(&h2After, h2.ID)
	if h2After.IsPublic || h2After.ReviewStatus != ReviewStatusQuotaReview {
		t.Errorf("目标应 quota_review 且不公开，实际 public=%v status=%s", h2After.IsPublic, h2After.ReviewStatus)
	}
	// 审核单已建
	reqs, _ := ListWebQuotaRequests(WebQuotaStatusPending)
	if len(reqs) != 1 {
		t.Errorf("应有 1 条待审单，实际 %d", len(reqs))
	}

	// 场景 3：按人覆盖解除大小限制（0=不限）→ 直接公开
	if err := SetUserWebQuotaOverride(1, 100, 0, "豁免大小"); err != nil {
		t.Fatalf("覆盖失败: %v", err)
	}
	allowed, needReview, _, err = CheckAndApplyWebPublish(1, WebQuotaTargetHTML, h2.ID)
	if err != nil || !allowed || needReview {
		t.Errorf("按人豁免大小后应直接公开: allowed=%v needReview=%v err=%v", allowed, needReview, err)
	}
}

func TestCheckAndApplyWebPublishCountQuotaStillWorks(t *testing.T) {
	db := setupWebQuotaTestDB(t)
	db.Create(&User{Username: "u2", Email: "u2@test.com", PasswordHash: "x"})
	// 数量额度 1，已公开 1 个 → 新的转审核（回归：数量维度不能坏）
	_ = SetWebPublishQuotaConfig(1)
	_ = SetWebPublishQuotaSizeConfig(0)
	db.Create(&UserHTML{UserID: 1, Name: "pub", ExtractDir: "x", EntryFile: "index.html", IsPublic: true, TotalSize: 1024})
	h2 := UserHTML{UserID: 1, Name: "new", ExtractDir: "y", EntryFile: "index.html", TotalSize: 1024}
	db.Create(&h2)
	allowed, needReview, reason, err := CheckAndApplyWebPublish(1, WebQuotaTargetHTML, h2.ID)
	if err != nil || allowed || !needReview {
		t.Errorf("数量超额应转审核: allowed=%v needReview=%v err=%v", allowed, needReview, err)
	}
	if reason == "" {
		t.Errorf("应返回超额原因")
	}
}
