package quota

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"nvs-server/models"
)

// setupQuotaTestDB 独立内存库（唯一名，避免与其它包的共享内存库互相污染）
func setupQuotaTestDB(t *testing.T) {
	t.Helper()
	dsn := fmt.Sprintf("file:htmlquota_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存测试库失败: %v", err)
	}
	if err := db.AutoMigrate(&models.UserHTML{}, &HTMLQuotaOverride{}, &models.PlatformConfig{}); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	old := models.DB
	models.DB = db
	t.Cleanup(func() { models.DB = old })
}

func seedHTML(t *testing.T, userID uint, size int64) {
	t.Helper()
	h := &models.UserHTML{
		UserID: userID, Name: "p", ExtractDir: "/tmp/p", EntryFile: "index.html", TotalSize: size,
	}
	if err := models.DB.Create(h).Error; err != nil {
		t.Fatalf("造数据失败: %v", err)
	}
}

// 默认配置下，第 11 个项目应被拒（默认上限 10）
func TestHTMLQuota_ProjectLimit(t *testing.T) {
	setupQuotaTestDB(t)
	_ = SaveHTMLQuotaSetting(DefaultHTMLQuotaSetting())
	for i := 0; i < 10; i++ {
		seedHTML(t, 7, 1024)
	}
	if err := CheckHTMLQuota(7, 1024); err == nil {
		t.Fatal("已满 10 个项目，应被拒绝")
	}
	// 另一个用户不受影响
	if err := CheckHTMLQuota(8, 1024); err != nil {
		t.Fatalf("用户 8 不应被拒: %v", err)
	}
}

// 空间上限：已用接近上限时，再加就超
func TestHTMLQuota_SizeLimit(t *testing.T) {
	setupQuotaTestDB(t)
	cfg := DefaultHTMLQuotaSetting()
	cfg.MaxProjects = 0 // 只测空间
	cfg.MaxTotalSizeMB = 10
	_ = SaveHTMLQuotaSetting(cfg)

	seedHTML(t, 9, 9*1024*1024) // 已用 9MB
	if err := CheckHTMLQuota(9, 512*1024); err != nil {
		t.Fatalf("9.5MB < 10MB 不该拒: %v", err)
	}
	if err := CheckHTMLQuota(9, 2*1024*1024); err == nil {
		t.Fatal("9MB+2MB > 10MB 应被拒")
	}
}

// 按人覆盖：收紧到 1 个项目
func TestHTMLQuota_OverrideTighten(t *testing.T) {
	setupQuotaTestDB(t)
	_ = SaveHTMLQuotaSetting(DefaultHTMLQuotaSetting())
	if err := models.DB.Create(&HTMLQuotaOverride{UserID: 11, MaxProjects: 1, Note: "试用"}).Error; err != nil {
		t.Fatalf("写入覆盖失败: %v", err)
	}
	seedHTML(t, 11, 1024)
	if err := CheckHTMLQuota(11, 1024); err == nil {
		t.Fatal("覆盖为 1 个项目后，第 2 个应被拒")
	}
}

// 按人覆盖：-1 表示不限
func TestHTMLQuota_OverrideUnlimited(t *testing.T) {
	setupQuotaTestDB(t)
	_ = SaveHTMLQuotaSetting(DefaultHTMLQuotaSetting())
	_ = models.DB.Create(&HTMLQuotaOverride{UserID: 12, MaxProjects: -1, MaxTotalSizeMB: -1}).Error
	for i := 0; i < 20; i++ {
		seedHTML(t, 12, 50*1024*1024)
	}
	if err := CheckHTMLQuota(12, 100*1024*1024); err != nil {
		t.Fatalf("覆盖为不限后不该拒绝: %v", err)
	}
}

// 全局关闭 → 不做任何限制
func TestHTMLQuota_Disabled(t *testing.T) {
	setupQuotaTestDB(t)
	cfg := DefaultHTMLQuotaSetting()
	cfg.Enabled = false
	_ = SaveHTMLQuotaSetting(cfg)
	for i := 0; i < 30; i++ {
		seedHTML(t, 13, 100*1024*1024)
	}
	if err := CheckHTMLQuota(13, 100*1024*1024); err != nil {
		t.Fatalf("总开关关闭后不该拒绝: %v", err)
	}
}

// 用量快照字段齐全（前端据此展示进度）
func TestHTMLQuotaSnapshot(t *testing.T) {
	setupQuotaTestDB(t)
	_ = SaveHTMLQuotaSetting(DefaultHTMLQuotaSetting())
	seedHTML(t, 14, 1048576)
	s := HTMLQuotaSnapshot(14)
	if s["used_projects"].(int) != 1 {
		t.Errorf("used_projects = %v，期望 1", s["used_projects"])
	}
	if s["used_bytes"].(int64) != 1048576 {
		t.Errorf("used_bytes = %v，期望 1048576", s["used_bytes"])
	}
	if s["enabled"].(bool) != true {
		t.Errorf("enabled 应为 true")
	}
}
