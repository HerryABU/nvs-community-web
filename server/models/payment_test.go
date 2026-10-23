package models

import (
	"testing"
	"time"
)

func TestPaymentDefaultOff(t *testing.T) {
	setupAiTestDB(t)
	// 默认关闭
	if IsPaymentEnabled() {
		t.Fatal("在线充值默认应为关闭")
	}
	// 默认配置：mock、1 元 = 1000 代币
	cfg := GetPaymentConfig()
	if cfg.Provider != PayProviderMock {
		t.Fatalf("默认 provider 应为 mock，实际 %s", cfg.Provider)
	}
	if cfg.TokenPerFen != 10 {
		t.Fatalf("默认每分 10 代币，实际 %d", cfg.TokenPerFen)
	}
	// 默认档位：1/5/10/50/100 元
	tiers := GetPaymentTiers()
	if len(tiers) != 5 || tiers[0].Fen != 100 || tiers[4].Fen != 10000 {
		t.Fatalf("默认档位异常：%+v", tiers)
	}
	// 关闭状态下不允许下单
	if _, err := CreateRechargeOrder(1, PayChannelWeChat, 1000); err == nil {
		t.Fatal("关闭状态下不应允许下单")
	}
}

func TestRechargeOrderFlowAndIdempotentCredit(t *testing.T) {
	setupAiTestDB(t)
	if err := SetTokenEnabled(true); err != nil {
		t.Fatalf("开启代币失败: %v", err)
	}
	if err := SetPaymentEnabled(true); err != nil {
		t.Fatalf("开启支付失败: %v", err)
	}
	// 10 元档：1000 分 ×10 + 赠送 150 = 10150
	order, err := CreateRechargeOrder(42, PayChannelMock, 1000)
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	if order.Status != PayStatusPending {
		t.Fatalf("新单应为 pending，实际 %s", order.Status)
	}
	if order.TokenAmount != 10150 {
		t.Fatalf("10 元档应到账 10150 代币，实际 %d", order.TokenAmount)
	}
	if order.PayURL == "" {
		_ = UpdateOrderPayURL(order.OrderNo, "/pay/mock/"+order.OrderNo)
	}
	// 非档位金额拒绝
	if _, err := CreateRechargeOrder(42, PayChannelMock, 999); err == nil {
		t.Fatal("非档位金额应被拒绝")
	}
	// 首次到账
	paid, credited, err := MarkOrderPaid(order.OrderNo, "TXN-001", "notify raw 1")
	if err != nil {
		t.Fatalf("到账失败: %v", err)
	}
	if !credited {
		t.Fatal("首次回调应 credited=true")
	}
	if paid.Status != PayStatusPaid || paid.TransactionID != "TXN-001" {
		t.Fatalf("订单状态/流水号异常：%s/%s", paid.Status, paid.TransactionID)
	}
	if bal := GetTokenBalance(42); bal != 10150 {
		t.Fatalf("到账后余额应为 10150，实际 %d", bal)
	}
	// 重复回调：幂等，不重复入账
	_, credited2, err := MarkOrderPaid(order.OrderNo, "TXN-001", "notify raw 2")
	if err != nil {
		t.Fatalf("重复回调应幂等成功，实际报错: %v", err)
	}
	if credited2 {
		t.Fatal("重复回调不应再次入账")
	}
	if bal := GetTokenBalance(42); bal != 10150 {
		t.Fatalf("重复回调后余额应仍为 10150，实际 %d", bal)
	}
}

func TestCloseExpiredOrders(t *testing.T) {
	setupAiTestDB(t)
	if err := SetTokenEnabled(true); err != nil {
		t.Fatalf("开启代币失败: %v", err)
	}
	if err := SetPaymentEnabled(true); err != nil {
		t.Fatalf("开启支付失败: %v", err)
	}
	order, err := CreateRechargeOrder(7, PayChannelMock, 500)
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	// 手动置为已过期
	past := time.Now().Add(-time.Minute)
	if err := DB.Model(&RechargeOrder{}).Where("order_no = ?", order.OrderNo).
		Update("expires_at", past).Error; err != nil {
		t.Fatalf("置过期失败: %v", err)
	}
	n, err := CloseExpiredOrders()
	if err != nil {
		t.Fatalf("关单失败: %v", err)
	}
	if n != 1 {
		t.Fatalf("应关闭 1 笔，实际 %d", n)
	}
	// 已关单订单不可到账
	if _, _, err := MarkOrderPaid(order.OrderNo, "TXN-X", ""); err == nil {
		t.Fatal("已关单订单不应允许到账")
	}
	if bal := GetTokenBalance(7); bal != 0 {
		t.Fatalf("关单后余额应仍为 0，实际 %d", bal)
	}
}

func TestPaymentTiersValidation(t *testing.T) {
	setupAiTestDB(t)
	// 合法档位
	if err := SetPaymentTiers([]PaymentTier{{Fen: 100, Bonus: 0, Label: "1 元"}, {Fen: 1000, Bonus: 200, Label: "10 元"}}); err != nil {
		t.Fatalf("合法档位应保存成功: %v", err)
	}
	// 空列表拒绝
	if err := SetPaymentTiers(nil); err == nil {
		t.Fatal("空档位应被拒绝")
	}
	// 面额过小拒绝
	if err := SetPaymentTiers([]PaymentTier{{Fen: 1, Bonus: 0, Label: "1 分"}}); err == nil {
		t.Fatal("1 分档应被拒绝")
	}
	// 重复面额拒绝
	if err := SetPaymentTiers([]PaymentTier{{Fen: 1000, Bonus: 0, Label: "甲"}, {Fen: 1000, Bonus: 1, Label: "乙"}}); err == nil {
		t.Fatal("重复面额应被拒绝")
	}
	// 超额赠送拒绝（bonus > fen*10）
	if err := SetPaymentTiers([]PaymentTier{{Fen: 100, Bonus: 1001, Label: "坑"}}); err == nil {
		t.Fatal("超额赠送应被拒绝")
	}
}
