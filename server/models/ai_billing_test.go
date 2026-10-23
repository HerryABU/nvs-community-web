package models

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupAiTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:ai_billing_test?mode=memory&cache=shared"), &gorm.Config{})
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

// 造一个计费模型（直接落库，绕开 handler 入参绑定）
func seedAiModel(t *testing.T, priceIn, priceOut int64, maxTokens int) *AiModel {
	t.Helper()
	m := &AiModel{
		Name: "测试模型", ModelID: "test-model", BaseURL: "http://127.0.0.1:1/v1",
		Enabled: true, PriceIn: priceIn, PriceOut: priceOut, MaxTokens: maxTokens,
	}
	if err := DB.Create(m).Error; err != nil {
		t.Fatalf("创建模型失败: %v", err)
	}
	return m
}

func TestAiEstimateCost(t *testing.T) {
	setupAiTestDB(t)
	m := &AiModel{PriceIn: 2, PriceOut: 4} // 每 1K tokens：入 2 / 出 4
	// 1500 入 + 500 出：ceil(1.5×2)=3 + ceil(0.5×4)=2 = 5
	if c := EstimateAiCost(m, 1500, 500); c != 5 {
		t.Errorf("预估费用应为 5，实际 %d", c)
	}
	// 恰好整数倍不额外进位：1000 入 ×2 = 2
	if c := EstimateAiCost(m, 1000, 0); c != 2 {
		t.Errorf("1000 入 tokens 费用应为 2，实际 %d", c)
	}
	// 免费模型（价格 0）费用为 0
	free := &AiModel{}
	if c := EstimateAiCost(free, 9999, 9999); c != 0 {
		t.Errorf("免费模型费用应为 0，实际 %d", c)
	}
	// tokens 粗估：空串 0，非空 >0
	if n := EstimateTokens(""); n != 0 {
		t.Errorf("空文本 tokens 应为 0，实际 %d", n)
	}
	if n := EstimateTokens("一二三四五"); n <= 0 {
		t.Errorf("非空文本 tokens 应 >0，实际 %d", n)
	}
}

func TestAiPrecheckBalance(t *testing.T) {
	setupAiTestDB(t)
	m := seedAiModel(t, 2, 4, 1024)
	// 免费模型：cost=0 时即使余额 0 也放行
	free := seedAiModel(t, 0, 0, 1024)
	if _, ok := PrecheckAiBalance(1, free, 100, 100); !ok {
		t.Errorf("免费模型应放行")
	}
	// 付费模型：余额 0 → 不可负担
	cost, ok := PrecheckAiBalance(1, m, 1500, 500)
	if ok || cost != 5 {
		t.Errorf("余额 0 应不可负担，cost=%d ok=%v", cost, ok)
	}
	// 充值 4（不足 5）→ 仍不可负担
	if _, e := AppendTokenLedger(1, TokenTypeGrant, 4, "admin", 1, "", ""); e != nil {
		t.Fatalf("增发失败: %v", e)
	}
	if _, ok := PrecheckAiBalance(1, m, 1500, 500); ok {
		t.Errorf("余额 4 < 费用 5 应不可负担")
	}
	// 再充 1（=5）→ 可负担
	if _, e := AppendTokenLedger(1, TokenTypeGrant, 1, "admin", 1, "", ""); e != nil {
		t.Fatalf("增发失败: %v", e)
	}
	if _, ok := PrecheckAiBalance(1, m, 1500, 500); !ok {
		t.Errorf("余额 5 ≥ 费用 5 应可负担")
	}
}

func TestAiSettlePaidCall(t *testing.T) {
	setupAiTestDB(t)
	m := seedAiModel(t, 2, 4, 1024)
	if _, e := AppendTokenLedger(7, TokenTypeRechargeCard, 100, "card", 1, "卡密", ""); e != nil {
		t.Fatalf("充值失败: %v", e)
	}
	log, err := CreateAiCallLog(7, m, "polish", "chapter", 3, 1500, 500)
	if err != nil {
		t.Fatalf("创建 pending 流水失败: %v", err)
	}
	if log.Status != AiCallPending {
		t.Fatalf("初始状态应为 pending，实际 %s", log.Status)
	}
	// 上游真实 usage：1200 入 / 600 出 → ceil(1.2×2)=3 + ceil(0.6×4)=3 = 6
	nb, err := SettleAiCallLog(log.ID, 1200, 600)
	if err != nil || nb != 94 {
		t.Fatalf("结算后期望余额 94，实际 nb=%d err=%v", nb, err)
	}
	// 流水落 consume + 快照
	var entries []TokenLedger
	DB.Where("user_id = ? AND type = ?", 7, TokenTypeConsume).Find(&entries)
	if len(entries) != 1 || entries[0].Amount != -6 {
		t.Fatalf("应有 1 笔 -6 的消费流水，实际 %+v", entries)
	}
	if entries[0].PriceSnapshot == "" {
		t.Errorf("消费流水应带价格快照")
	}
	// 调用记录变 settled 且 tokens/cost 为真实值
	var got AiCallLog
	DB.Where("id = ?", log.ID).First(&got)
	if got.Status != AiCallSettled || got.TokensIn != 1200 || got.TokensOut != 600 || got.Cost != 6 {
		t.Errorf("结算记录异常: %+v", got)
	}
	// 重复结算应被拒绝（状态机保护）
	if _, e := SettleAiCallLog(log.ID, 1200, 600); e == nil {
		t.Errorf("重复结算应失败")
	}
	// 退款：+6 回滚，状态 refunded
	rb, err := RefundAiCall(log.ID, "测试退款")
	if err != nil || rb != 100 {
		t.Fatalf("退款后期望余额 100，实际 rb=%d err=%v", rb, err)
	}
	DB.Where("id = ?", log.ID).First(&got)
	if got.Status != AiCallRefunded {
		t.Errorf("退款后状态应为 refunded，实际 %s", got.Status)
	}
	// 重复退款应失败
	if _, e := RefundAiCall(log.ID, "再退"); e == nil {
		t.Errorf("重复退款应失败")
	}
}

func TestAiSettleInsufficientBalance(t *testing.T) {
	setupAiTestDB(t)
	m := seedAiModel(t, 2, 4, 1024)
	// 只充 2，费用需 6
	if _, e := AppendTokenLedger(8, TokenTypeGrant, 2, "admin", 1, "", ""); e != nil {
		t.Fatalf("增发失败: %v", e)
	}
	log, err := CreateAiCallLog(8, m, "continue", "", 0, 1200, 600)
	if err != nil {
		t.Fatalf("创建 pending 流水失败: %v", err)
	}
	if _, e := SettleAiCallLog(log.ID, 1200, 600); e == nil {
		t.Fatalf("余额不足结算应报错")
	}
	// 余额不变、记录 failed、无消费流水
	if GetTokenBalance(8) != 2 {
		t.Errorf("失败结算后余额应保持 2，实际 %d", GetTokenBalance(8))
	}
	var got AiCallLog
	DB.Where("id = ?", log.ID).First(&got)
	if got.Status != AiCallFailed {
		t.Errorf("状态应为 failed，实际 %s", got.Status)
	}
	var n int64
	DB.Model(&TokenLedger{}).Where("user_id = ? AND type = ?", 8, TokenTypeConsume).Count(&n)
	if n != 0 {
		t.Errorf("失败结算不应产生消费流水，实际 %d 笔", n)
	}
}

func TestAiSettleFreeModel(t *testing.T) {
	setupAiTestDB(t)
	m := seedAiModel(t, 0, 0, 1024)
	log, err := CreateAiCallLog(9, m, "summarize", "", 0, 100, 100)
	if err != nil {
		t.Fatalf("创建 pending 流水失败: %v", err)
	}
	nb, err := SettleAiCallLog(log.ID, 100, 100)
	if err != nil || nb != 0 {
		t.Fatalf("免费模型结算应返回余额 0，实际 nb=%d err=%v", nb, err)
	}
	var got AiCallLog
	DB.Where("id = ?", log.ID).First(&got)
	if got.Status != AiCallSettled || got.Cost != 0 {
		t.Errorf("免费模型记录异常: %+v", got)
	}
	// 免费模型不碰代币账本（无余额行）
	if GetTokenBalance(9) != 0 {
		t.Errorf("免费模型不应建余额行")
	}
}

func TestAiModelKeyEncryption(t *testing.T) {
	setupAiTestDB(t)
	m := &AiModel{Name: "密钥测试", ModelID: "k", BaseURL: "http://x", Enabled: true}
	if err := CreateAiModel(m, "sk-super-secret-123"); err != nil {
		t.Fatalf("创建模型失败: %v", err)
	}
	var got AiModel
	DB.Where("id = ?", m.ID).First(&got)
	if got.ApiKeyEnc == "" {
		t.Fatalf("密钥应加密落库")
	}
	if got.ApiKeyEnc == "sk-super-secret-123" {
		t.Errorf("密钥不应明文落库")
	}
	plain, err := got.DecryptApiKey()
	if err != nil || plain != "sk-super-secret-123" {
		t.Errorf("解密应还原明文，实际 %q err=%v", plain, err)
	}
	// 留空轮换：不应改动原密钥
	if err := UpdateAiModelFields(m.ID, map[string]interface{}{"note": "x"}, ""); err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	DB.Where("id = ?", m.ID).First(&got)
	plain2, _ := got.DecryptApiKey()
	if plain2 != "sk-super-secret-123" {
		t.Errorf("留空 api_key 不应轮换，实际 %q", plain2)
	}
	// 非空轮换：生效
	if err := UpdateAiModelFields(m.ID, map[string]interface{}{}, "sk-new-key-456"); err != nil {
		t.Fatalf("轮换失败: %v", err)
	}
	DB.Where("id = ?", m.ID).First(&got)
	plain3, _ := got.DecryptApiKey()
	if plain3 != "sk-new-key-456" {
		t.Errorf("轮换后密钥应更新，实际 %q", plain3)
	}
}

func TestAiConfigAndQueries(t *testing.T) {
	setupAiTestDB(t)
	if IsAiEnabled() {
		t.Errorf("AI 默认应关闭")
	}
	if err := SetAiEnabled(true); err != nil {
		t.Fatalf("开失败: %v", err)
	}
	if !IsAiEnabled() {
		t.Errorf("开启后应生效")
	}
	// 启用/禁用筛选
	_ = DB.Create(&AiModel{Name: "a", ModelID: "a", BaseURL: "http://x", Enabled: true}).Error
	_ = DB.Model(&AiModel{}).Where("name = ?", "b").Create(&AiModel{Name: "b", ModelID: "b", BaseURL: "http://x", Enabled: false}).Error
	// b 的 Enabled=false 会被 default:true 顶掉（struct Create 零值跳过），改用 map 精确写
	_ = DB.Model(&AiModel{}).Where("model_id = ?", "b").Updates(map[string]interface{}{"enabled": false}).Error
	all, _ := ListAiModels(false)
	only, _ := ListAiModels(true)
	if len(all) != 2 {
		t.Errorf("管理员列表应 2 个，实际 %d", len(all))
	}
	if len(only) != 1 {
		t.Errorf("启用列表应 1 个，实际 %d", len(only))
	}
	n, _ := CountEnabledAiModels()
	if n != 1 {
		t.Errorf("启用计数应为 1，实际 %d", n)
	}
	// GetEnabledAiModel：禁用模型返回 nil
	var b AiModel
	_ = DB.Where("model_id = ?", "b").First(&b)
	em, err := GetEnabledAiModel(b.ID)
	if err != nil || em != nil {
		t.Errorf("禁用模型应返回 nil,nil，实际 %v err=%v", em, err)
	}
	// 调用历史分页
	_, _ = CreateAiCallLog(5, &AiModel{ID: 1, Name: "a"}, "custom", "", 0, 10, 10)
	list, total, err := ListAiCallLogs(5, 1, 20)
	if err != nil || total != 1 || len(list) != 1 {
		t.Errorf("调用历史异常: total=%d len=%d err=%v", total, len(list), err)
	}
}
