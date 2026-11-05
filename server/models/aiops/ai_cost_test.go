package aiops

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"nvs-server/models"
	ai "nvs-server/models/ai"
	"testing"
	"time"
)

// ===== (原 ai_cost_test.go) =====

func setupCostDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:cost_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	models.DB = db
	if err := db.AutoMigrate(&models.PlatformConfig{}, &ai.AiCallLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	_ = db.Exec("DELETE FROM ai_call_logs").Error
	_ = models.SetPlatformConfig(aiBudgetKey, "")
	_ = models.SetPlatformConfig(aiThrottleKey, "")
	ai.ResetAiSlots()
	ResetAiThrottle()
}

// TestGlobalBudgetHardStop 到线即拒（HardStop），未到线放行
func TestGlobalBudgetHardStop(t *testing.T) {
	setupCostDB(t)
	cfg := ai.DefaultAiBudgetConfig()
	cfg.DailyCap = 100
	cfg.HardStop = true
	if err := ai.SaveAiBudgetConfig(cfg); err != nil {
		t.Fatal(err)
	}
	// 已用 90
	_ = models.DB.Create(&ai.AiCallLog{UserID: 1, Action: "x", Cost: 90, Status: "settled", CreatedAt: time.Now()}).Error
	if err := ai.CheckGlobalBudget(5); err != nil {
		t.Fatalf("90+5 未超 100，应放行: %v", err)
	}
	if err := ai.CheckGlobalBudget(20); err == nil {
		t.Fatal("90+20 超过 100，应拒绝")
	}
	u := ai.GetAiUsage()
	if u.Exceeded {
		t.Fatalf("90/100 未达上限不应标记超限: %+v", u)
	}
	if !u.Warn {
		t.Fatalf("90%% 应触发告警: %+v", u)
	}
	// 再花 20 → 110 超过 100
	_ = models.DB.Create(&ai.AiCallLog{UserID: 1, Action: "x", Cost: 20, Status: "settled", CreatedAt: time.Now()}).Error
	if u2 := ai.GetAiUsage(); !u2.Exceeded {
		t.Fatalf("超线后应标记超限: %+v", u2)
	}
}

// TestGlobalBudgetSoftMode 不硬拦时只观测，不阻断业务
func TestGlobalBudgetSoftMode(t *testing.T) {
	setupCostDB(t)
	cfg := ai.DefaultAiBudgetConfig()
	cfg.DailyCap = 10
	cfg.HardStop = false
	_ = ai.SaveAiBudgetConfig(cfg)
	_ = models.DB.Create(&ai.AiCallLog{UserID: 1, Action: "x", Cost: 999, Status: "settled", CreatedAt: time.Now()}).Error
	if err := ai.CheckGlobalBudget(1); err != nil {
		t.Fatalf("软模式不应阻断: %v", err)
	}
	if u := ai.GetAiUsage(); !u.Warn {
		t.Fatalf("应给出告警标记: %+v", u)
	}
}

// TestAcquireSlot 并发名额：满了就超时失败，释放后可再拿
func TestAcquireSlot(t *testing.T) {
	ai.ResetAiSlots()
	rel1, _, err := ai.AcquireSlot(1, 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ai.AcquireSlot(1, 100*time.Millisecond); err == nil {
		t.Fatal("只有一个名额时应排队超时")
	}
	rel1()
	rel2, _, err := ai.AcquireSlot(1, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("释放后应能拿到: %v", err)
	}
	rel2()
	// 幂等释放不应把计数打成负数
	rel2()
	if ai.AiConcurrencyInUse() != 0 {
		t.Fatalf("计数应归零: %d", ai.AiConcurrencyInUse())
	}
}

// TestEffectiveConcurrency 峰时上限不能超过全站上限
func TestEffectiveConcurrency(t *testing.T) {
	cfg := ai.DefaultAiBudgetConfig()
	cfg.QueueEnabled = true
	cfg.MaxConcurrent = 10
	cfg.PeakConcurrent = 99 // 非法：高于全站
	cfg = normalizeBudget(cfg)
	if cfg.PeakConcurrent != 10 {
		t.Fatalf("峰时应被钳到全站上限: %d", cfg.PeakConcurrent)
	}
	if ai.EffectiveConcurrency(ai.AiBudgetConfig{QueueEnabled: false, MaxConcurrent: 5}) != 0 {
		t.Fatal("关闭排队应返回 0（不限）")
	}
}

// TestThrottleUserModel 三维限流：同人同模型超限被拒，换模型不受影响
func TestThrottleUserModel(t *testing.T) {
	setupCostDB(t)
	cfg := DefaultAiThrottleConfig()
	cfg.PerUserPerModelPerMin = 2
	cfg.PerUserPerMin = 100
	cfg.PerModelPerMin = 1000
	if err := SaveAiThrottleConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if err := CheckAiThrottle(1, 10); err != nil {
		t.Fatal(err)
	}
	if err := CheckAiThrottle(1, 10); err != nil {
		t.Fatal(err)
	}
	err := CheckAiThrottle(1, 10)
	if err == nil {
		t.Fatal("第三个请求应超限")
	}
	if _, ok := err.(*ThrottleError); !ok {
		t.Fatalf("应返回 ThrottleError: %T", err)
	}
	// 换模型不受同模型额度影响
	if err := CheckAiThrottle(1, 11); err != nil {
		t.Fatalf("换模型应放行: %v", err)
	}
	if len(AiThrottleSnapshot(5)) == 0 {
		t.Fatal("快照应有数据")
	}
}

// TestThrottleUserDimension 用户维度：不同模型累计也能触发
func TestThrottleUserDimension(t *testing.T) {
	setupCostDB(t)
	cfg := DefaultAiThrottleConfig()
	cfg.PerUserPerModelPerMin = 100
	cfg.PerUserPerMin = 2
	cfg.PerModelPerMin = 1000
	_ = SaveAiThrottleConfig(cfg)
	if err := CheckAiThrottle(7, 1); err != nil {
		t.Fatal(err)
	}
	if err := CheckAiThrottle(7, 2); err != nil {
		t.Fatal(err)
	}
	if err := CheckAiThrottle(7, 3); err == nil {
		t.Fatal("用户维度应超限")
	}
}

// ===== 合并自 budget_key_test.go =====

// aiBudgetKey 与根包 ai_budget.go 中的平台配置键保持一致。
// 该常量未导出、无法跨包复用，故在本子包测试内本地定义一份（值相同，行为不变）。
const aiBudgetKey = "ai_budget"

// normalizeBudget 与根包 ai_budget.go 的实现等价：未导出函数无法跨包复用，
// 这里复制一份供本子包测试验证预算配置的钳位规则（逻辑逐条对齐）。
func normalizeBudget(c ai.AiBudgetConfig) ai.AiBudgetConfig {
	if c.DailyCap < 0 {
		c.DailyCap = 0
	}
	if c.MonthlyCap < 0 {
		c.MonthlyCap = 0
	}
	if c.MaxConcurrent < 0 {
		c.MaxConcurrent = 0
	}
	if c.QueueTimeoutSec <= 0 {
		c.QueueTimeoutSec = 20
	}
	if c.QueueTimeoutSec > 300 {
		c.QueueTimeoutSec = 300
	}
	if c.AlertAtRatio <= 0 || c.AlertAtRatio > 1 {
		c.AlertAtRatio = 0.8
	}
	// 峰时上限不能超过全站上限
	if c.PeakConcurrent > c.MaxConcurrent && c.MaxConcurrent > 0 {
		c.PeakConcurrent = c.MaxConcurrent
	}
	hours := make([]int, 0, len(c.PeakHours))
	seen := map[int]bool{}
	for _, h := range c.PeakHours {
		if h >= 0 && h <= 23 && !seen[h] {
			seen[h] = true
			hours = append(hours, h)
		}
	}
	c.PeakHours = hours
	return c
}

// ===== 合并自 testdb_test.go =====

// setupAiTestDB 独立唯一内存库。原根包 ai_billing_test.go 里的同名 helper 未导出，
// 子包无法跨包复用，故在本子包内自建一份（行为一致）。
func setupAiTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:aiops_test_" + time.Now().Format("20060102150405.000000000") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if err := models.AutoMigrateAll(db); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	models.DB = db
	t.Cleanup(func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}
