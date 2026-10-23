package models

import "testing"

func TestAiFreeQuotaDefaultOff(t *testing.T) {
	setupAiTestDB(t)
	// 默认关闭：任何用户都不可走免费
	if IsAiFreeQuotaEnabled() {
		t.Fatal("免费额度默认应为关闭")
	}
	if ok, _, _ := CheckAiFreeQuota(1, 100); ok {
		t.Fatal("关闭状态下不应判定为可免费")
	}
	// 开启后：默认每日 5 次
	if err := SetAiFreeQuotaEnabled(true); err != nil {
		t.Fatalf("开启失败: %v", err)
	}
	cfg := GetAiFreeQuotaConfig()
	if cfg.CallsPerDay != 5 {
		t.Fatalf("默认每日免费次数应为 5，实际 %d", cfg.CallsPerDay)
	}
	if ok, callsLeft, _ := CheckAiFreeQuota(1, 100); !ok || callsLeft != 5 {
		t.Fatalf("开启后应可免费且剩余 5 次，实际 ok=%v left=%d", ok, callsLeft)
	}
}

func TestAiFreeQuotaConsumeAndExhaust(t *testing.T) {
	setupAiTestDB(t)
	if err := SetAiFreeQuotaEnabled(true); err != nil {
		t.Fatalf("开启失败: %v", err)
	}
	if err := SetAiFreeQuotaConfig(AiFreeQuotaConfig{CallsPerDay: 2, MaxTokensPerDay: 0}); err != nil {
		t.Fatalf("配置失败: %v", err)
	}
	// 第 1、2 次：可免费，记用量
	for i := 0; i < 2; i++ {
		if ok, _, _ := CheckAiFreeQuota(7, 50); !ok {
			t.Fatalf("第 %d 次应可免费", i+1)
		}
		if err := RecordAiFreeUsage(7, 1, 50); err != nil {
			t.Fatalf("记用量失败: %v", err)
		}
	}
	// 第 3 次：次数用尽 → 不可免费
	if ok, _, _ := CheckAiFreeQuota(7, 50); ok {
		t.Fatal("次数用尽后不应可免费")
	}
	// 换用户不受影响
	if ok, callsLeft, _ := CheckAiFreeQuota(8, 50); !ok || callsLeft != 2 {
		t.Fatalf("其他用户应仍有 2 次，实际 ok=%v left=%d", ok, callsLeft)
	}
}

func TestAiFreeQuotaTokenBudget(t *testing.T) {
	setupAiTestDB(t)
	if err := SetAiFreeQuotaEnabled(true); err != nil {
		t.Fatalf("开启失败: %v", err)
	}
	// tokens 预算 100/日，单次预估 60：第一次可，第二次（60+60>100）不可
	if err := SetAiFreeQuotaConfig(AiFreeQuotaConfig{CallsPerDay: -1, MaxTokensPerDay: 100}); err != nil {
		t.Fatalf("配置失败: %v", err)
	}
	if ok, _, tokensLeft := CheckAiFreeQuota(9, 60); !ok || tokensLeft != 100 {
		t.Fatalf("首次应可免费且 tokens 剩 100，实际 ok=%v left=%d", ok, tokensLeft)
	}
	_ = RecordAiFreeUsage(9, 1, 60)
	if ok, _, _ := CheckAiFreeQuota(9, 60); ok {
		t.Fatal("tokens 预算不足时不应可免费")
	}
	if ok, _, _ := CheckAiFreeQuota(9, 40); !ok {
		t.Fatal("tokens 预算内应可免费")
	}
}

func TestAiFreeQuotaOverride(t *testing.T) {
	setupAiTestDB(t)
	if err := SetAiFreeQuotaEnabled(true); err != nil {
		t.Fatalf("开启失败: %v", err)
	}
	// 按人覆盖：禁止免费（0 次）
	if err := SetUserAiFreeQuotaOverride(10, 0, 0, "测试封禁免费"); err != nil {
		t.Fatalf("覆盖失败: %v", err)
	}
	if ok, _, _ := CheckAiFreeQuota(10, 10); ok {
		t.Fatal("覆盖为 0 次后不应可免费")
	}
	// 覆盖为不限次数
	if err := SetUserAiFreeQuotaOverride(10, -1, 0, ""); err != nil {
		t.Fatalf("覆盖失败: %v", err)
	}
	if ok, callsLeft, _ := CheckAiFreeQuota(10, 10); !ok || callsLeft != -1 {
		t.Fatalf("覆盖为不限后应可免费，实际 ok=%v left=%d", ok, callsLeft)
	}
	// 删除覆盖 → 恢复平台默认（5 次）
	if err := DeleteUserAiFreeQuotaOverride(10); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if ok, callsLeft, _ := CheckAiFreeQuota(10, 10); !ok || callsLeft != 5 {
		t.Fatalf("删除覆盖后应恢复默认 5 次，实际 ok=%v left=%d", ok, callsLeft)
	}
}

func TestAiCallLogFreeSettlement(t *testing.T) {
	setupAiTestDB(t)
	if err := SetAiFreeQuotaEnabled(true); err != nil {
		t.Fatalf("开启失败: %v", err)
	}
	m := seedAiModel(t, 2, 4, 1024)
	// 免费调用：结算后 cost=0、不产生代币流水
	freeLog, err := CreateAiCallLog(1, m, "continue", "", 0, 1000, 500, true)
	if err != nil {
		t.Fatalf("创建免费调用失败: %v", err)
	}
	nb, err := SettleAiCallLog(freeLog.ID, 1000, 500)
	if err != nil {
		t.Fatalf("结算失败: %v", err)
	}
	if nb != 0 {
		t.Fatalf("免费调用不应改变余额，实际 %d", nb)
	}
	var log AiCallLog
	if err := DB.Where("id = ?", freeLog.ID).First(&log).Error; err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if log.Status != AiCallSettled || log.Cost != 0 {
		t.Fatalf("免费调用应 settled 且 cost=0，实际 %s/%d", log.Status, log.Cost)
	}
	// 付费调用：正常结算扣款（模型价 2/4 → 1000 入 = 2，500 出 = 2，共 4）
	payLog, err := CreateAiCallLog(2, m, "continue", "", 0, 1000, 500, false)
	if err != nil {
		t.Fatalf("创建付费调用失败: %v", err)
	}
	AppendTokenLedger(2, TokenTypeRechargeCard, 10, "", 0, "测试充值", "")
	nb, err = SettleAiCallLog(payLog.ID, 1000, 500)
	if err != nil {
		t.Fatalf("付费结算失败: %v", err)
	}
	if nb != 6 {
		t.Fatalf("付费调用后余额应为 6，实际 %d", nb)
	}
}
