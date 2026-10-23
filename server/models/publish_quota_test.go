package models

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupPublishQuotaTestDB 独立内存库（唯一名，避免与同包其他测试共享污染）
func setupPublishQuotaTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:publishquota_test?mode=memory&cache=shared"), &gorm.Config{})
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

func TestDailyPublishDefaultDisabled(t *testing.T) {
	setupPublishQuotaTestDB(t)
	cfg := GetDailyPublishConfig()
	if cfg.Enabled {
		t.Errorf("每日发文限额默认应关闭，实际 enabled=%v", cfg.Enabled)
	}
	if cfg.Roles[RoleAdmin].Count != -1 {
		t.Errorf("管理员等级默认应不限（-1），实际 %d", cfg.Roles[RoleAdmin].Count)
	}
	// 关闭时一律放行
	allowed, _, _, _, _ := CheckDailyPublish(1, RoleAuthor, PublishKindBlog, 99999)
	if !allowed {
		t.Errorf("关闭状态应放行，实际被拒")
	}
}

func TestDailyPublishEnforceCount(t *testing.T) {
	setupPublishQuotaTestDB(t)
	if err := SetDailyPublishConfig(DailyPublishConfig{
		Enabled: true,
		Roles: map[string]DailyPublishRoleLimit{
			RoleAuthor: {Count: 2},
			RoleAdmin:  {Count: -1},
		},
	}); err != nil {
		t.Fatalf("保存配置失败: %v", err)
	}
	for i := 0; i < 2; i++ {
		allowed, used, limit, _, _ := CheckDailyPublish(1, RoleAuthor, PublishKindBlog, 0)
		if !allowed {
			t.Fatalf("第 %d 次应放行，实际被拒", i+1)
		}
		if used != i || limit != 2 {
			t.Errorf("第 %d 次 used=%d limit=%d，期望 used=%d limit=2", i+1, used, limit, i)
		}
		if err := RecordPublish(1, PublishKindBlog, uint(i+1), 10); err != nil {
			t.Fatalf("记账失败: %v", err)
		}
	}
	// 第 3 次应被拒
	allowed, used, limit, _, _ := CheckDailyPublish(1, RoleAuthor, PublishKindBlog, 0)
	if allowed {
		t.Errorf("达到数量上限后应被拒，实际放行")
	}
	if used != 2 || limit != 2 {
		t.Errorf("超额时 used=%d limit=%d，期望 2/2", used, limit)
	}
}

func TestDailyPublishWordCap(t *testing.T) {
	setupPublishQuotaTestDB(t)
	if err := SetDailyPublishConfig(DailyPublishConfig{
		Enabled: true,
		Roles: map[string]DailyPublishRoleLimit{
			RoleAuthor: {Count: -1, WordCap: 100},
		},
	}); err != nil {
		t.Fatalf("保存配置失败: %v", err)
	}
	_ = RecordPublish(1, PublishKindChapter, 1, 60)
	// 60 + 50 > 100 → 拒
	allowed, _, _, usedWords, wordLimit := CheckDailyPublish(1, RoleAuthor, PublishKindChapter, 50)
	if allowed {
		t.Errorf("超出字数上限应被拒，实际放行")
	}
	if usedWords != 60 || wordLimit != 100 {
		t.Errorf("usedWords=%d wordLimit=%d，期望 60/100", usedWords, wordLimit)
	}
	// 60 + 30 = 90 ≤ 100 → 放行
	allowed2, _, _, _, _ := CheckDailyPublish(1, RoleAuthor, PublishKindChapter, 30)
	if !allowed2 {
		t.Errorf("未超字数上限应放行，实际被拒")
	}
}

// TestDailyPublishOverrideZeroForbids 验证「0 = 禁止发布」的零值纪律：
// 必须用 map 精确写库，不能被列默认值 -1 顶掉。
func TestDailyPublishOverrideZeroForbids(t *testing.T) {
	setupPublishQuotaTestDB(t)
	_ = SetDailyPublishConfig(DailyPublishConfig{
		Enabled: true,
		Roles:   map[string]DailyPublishRoleLimit{RoleAuthor: {Count: 5}},
	})
	// 按人覆盖：count=0（禁止发布）、word_cap=0（不限字数）
	if err := SetUserPublishQuotaOverride(1, 0, 0, "测试禁止"); err != nil {
		t.Fatalf("写入覆盖失败: %v", err)
	}
	o := GetUserPublishQuotaOverride(1)
	if o == nil || o.Count != 0 {
		t.Fatalf("覆盖 count 应精确为 0，实际 %v", o)
	}
	countLimit, _ := EffectivePublishLimit(1, RoleAuthor)
	if countLimit != 0 {
		t.Errorf("覆盖后生效数量应为 0（禁止），实际 %d（疑似被默认值顶掉）", countLimit)
	}
	allowed, _, _, _, _ := CheckDailyPublish(1, RoleAuthor, PublishKindBlog, 0)
	if allowed {
		t.Errorf("count=0 应禁止发布，实际放行")
	}
	// 撤销覆盖 → 恢复跟随角色默认 5
	if err := DeleteUserPublishQuotaOverride(1); err != nil {
		t.Fatalf("撤销覆盖失败: %v", err)
	}
	countLimit2, _ := EffectivePublishLimit(1, RoleAuthor)
	if countLimit2 != 5 {
		t.Errorf("撤销后应恢复角色默认 5，实际 %d", countLimit2)
	}
}

func TestNormalizeFillsMissingRoles(t *testing.T) {
	cfg := normalizeDailyPublishConfig(DailyPublishConfig{Enabled: true, Roles: map[string]DailyPublishRoleLimit{}})
	for _, r := range []string{RoleReader, RoleAuthor, RoleVipAuthor, RoleArbitrator, RolePolice, RoleAdmin} {
		if _, ok := cfg.Roles[r]; !ok {
			t.Errorf("规范化应补齐缺失等级 %s", r)
		}
	}
	// 数值下限保护
	cfg2 := normalizeDailyPublishConfig(DailyPublishConfig{Roles: map[string]DailyPublishRoleLimit{RoleAuthor: {Count: -9, WordCap: -3}}})
	if cfg2.Roles[RoleAuthor].Count != -1 || cfg2.Roles[RoleAuthor].WordCap != 0 {
		t.Errorf("数值应被钳制，实际 count=%d word_cap=%d", cfg2.Roles[RoleAuthor].Count, cfg2.Roles[RoleAuthor].WordCap)
	}
}
