package models

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupCostDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:cost_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	DB = db
	if err := db.AutoMigrate(&PlatformConfig{}, &AiCallLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	_ = db.Exec("DELETE FROM ai_call_logs").Error
	_ = SetPlatformConfig(aiBudgetKey, "")
	_ = SetPlatformConfig(aiThrottleKey, "")
	ResetAiSlots()
	ResetAiThrottle()
}

// TestGlobalBudgetHardStop 到线即拒（HardStop），未到线放行
func TestGlobalBudgetHardStop(t *testing.T) {
	setupCostDB(t)
	cfg := DefaultAiBudgetConfig()
	cfg.DailyCap = 100
	cfg.HardStop = true
	if err := SaveAiBudgetConfig(cfg); err != nil {
		t.Fatal(err)
	}
	// 已用 90
	_ = DB.Create(&AiCallLog{UserID: 1, Action: "x", Cost: 90, Status: "settled", CreatedAt: time.Now()}).Error
	if err := CheckGlobalBudget(5); err != nil {
		t.Fatalf("90+5 未超 100，应放行: %v", err)
	}
	if err := CheckGlobalBudget(20); err == nil {
		t.Fatal("90+20 超过 100，应拒绝")
	}
	u := GetAiUsage()
	if u.Exceeded {
		t.Fatalf("90/100 未达上限不应标记超限: %+v", u)
	}
	if !u.Warn {
		t.Fatalf("90%% 应触发告警: %+v", u)
	}
	// 再花 20 → 110 超过 100
	_ = DB.Create(&AiCallLog{UserID: 1, Action: "x", Cost: 20, Status: "settled", CreatedAt: time.Now()}).Error
	if u2 := GetAiUsage(); !u2.Exceeded {
		t.Fatalf("超线后应标记超限: %+v", u2)
	}
}

// TestGlobalBudgetSoftMode 不硬拦时只观测，不阻断业务
func TestGlobalBudgetSoftMode(t *testing.T) {
	setupCostDB(t)
	cfg := DefaultAiBudgetConfig()
	cfg.DailyCap = 10
	cfg.HardStop = false
	_ = SaveAiBudgetConfig(cfg)
	_ = DB.Create(&AiCallLog{UserID: 1, Action: "x", Cost: 999, Status: "settled", CreatedAt: time.Now()}).Error
	if err := CheckGlobalBudget(1); err != nil {
		t.Fatalf("软模式不应阻断: %v", err)
	}
	if u := GetAiUsage(); !u.Warn {
		t.Fatalf("应给出告警标记: %+v", u)
	}
}

// TestAcquireSlot 并发名额：满了就超时失败，释放后可再拿
func TestAcquireSlot(t *testing.T) {
	ResetAiSlots()
	rel1, _, err := AcquireSlot(1, 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := AcquireSlot(1, 100*time.Millisecond); err == nil {
		t.Fatal("只有一个名额时应排队超时")
	}
	rel1()
	rel2, _, err := AcquireSlot(1, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("释放后应能拿到: %v", err)
	}
	rel2()
	// 幂等释放不应把计数打成负数
	rel2()
	if AiConcurrencyInUse() != 0 {
		t.Fatalf("计数应归零: %d", AiConcurrencyInUse())
	}
}

// TestEffectiveConcurrency 峰时上限不能超过全站上限
func TestEffectiveConcurrency(t *testing.T) {
	cfg := DefaultAiBudgetConfig()
	cfg.QueueEnabled = true
	cfg.MaxConcurrent = 10
	cfg.PeakConcurrent = 99 // 非法：高于全站
	cfg = normalizeBudget(cfg)
	if cfg.PeakConcurrent != 10 {
		t.Fatalf("峰时应被钳到全站上限: %d", cfg.PeakConcurrent)
	}
	if EffectiveConcurrency(AiBudgetConfig{QueueEnabled: false, MaxConcurrent: 5}) != 0 {
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
