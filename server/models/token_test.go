package models

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTokenTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:token_test?mode=memory&cache=shared"), &gorm.Config{})
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

func TestTokenLedgerAndBalance(t *testing.T) {
	setupTokenTestDB(t)
	if b := GetTokenBalance(1); b != 0 {
		t.Errorf("初始余额应为 0，实际 %d", b)
	}
	nb, err := AppendTokenLedger(1, TokenTypeGrant, 100, "admin", 9, "测试增发", "")
	if err != nil || nb != 100 {
		t.Fatalf("增发 100 后期望余额 100，实际 nb=%d err=%v", nb, err)
	}
	if GetTokenBalance(1) != 100 {
		t.Errorf("缓存余额应为 100，实际 %d", GetTokenBalance(1))
	}
	nb, err = AppendTokenLedger(1, TokenTypeConsume, -60, "ai", 0, "", "")
	if err != nil || nb != 40 {
		t.Fatalf("消费 60 后期望余额 40，实际 nb=%d err=%v", nb, err)
	}
	// 超额扣款应失败且不改变余额
	if _, err := AppendTokenLedger(1, TokenTypeConsume, -999, "ai", 0, "", ""); err == nil {
		t.Errorf("超额扣款应返回余额不足错误")
	}
	if GetTokenBalance(1) != 40 {
		t.Errorf("扣款失败后余额应保持 40，实际 %d", GetTokenBalance(1))
	}
	// 对账：流水合计 = 缓存余额
	if s := SumTokenLedger(1); s != 40 {
		t.Errorf("流水合计应为 40，实际 %d", s)
	}
	rb, err := ReconcileTokenBalance(1)
	if err != nil || rb != 40 {
		t.Errorf("对账应得 40，实际 %d err=%v", rb, err)
	}
}

func TestCardKeyLifecycle(t *testing.T) {
	setupTokenTestDB(t)
	codes, err := GenerateCardKeys(9, 2, 50, nil, "批次测试")
	if err != nil || len(codes) != 2 {
		t.Fatalf("生成 2 张卡密失败: %v (len=%d)", err, len(codes))
	}
	if !strings.Contains(codes[0], "-") {
		t.Errorf("卡密应含分组分隔符，实际 %s", codes[0])
	}
	// 哈希存储：库里 code_hash != 明文
	var ck CardKey
	if e := DB.Where("code_hash = ?", hashCardCode(codes[0])).First(&ck).Error; e != nil {
		t.Fatalf("按哈希查卡密失败: %v", e)
	}
	if ck.CodeHash == codes[0] {
		t.Errorf("卡密不应明文存储")
	}
	if ck.Value != 50 || ck.Status != CardKeyUnused {
		t.Errorf("卡密面值/状态异常: value=%d status=%s", ck.Value, ck.Status)
	}
	// 兑换 codes[0]（用户1）
	nb, err := RedeemCardKey(1, codes[0])
	if err != nil || nb != 50 {
		t.Fatalf("兑换期望余额 50，实际 nb=%d err=%v", nb, err)
	}
	// 重复兑换（含换用户）应失败
	if _, err := RedeemCardKey(1, codes[0]); err == nil {
		t.Errorf("重复兑换应失败")
	}
	if _, err := RedeemCardKey(2, codes[0]); err == nil {
		t.Errorf("他人兑换已用卡密应失败")
	}
	// codes[1] 用小写+去分隔符的规范化形式兑换（用户2）——验证 normalize
	norm := strings.ToLower(strings.ReplaceAll(codes[1], "-", ""))
	nb2, err := RedeemCardKey(2, norm)
	if err != nil || nb2 != 50 {
		t.Fatalf("规范化形式兑换期望成功且余额 50，实际 nb=%d err=%v", nb2, err)
	}
	// 无效卡密
	if _, err := RedeemCardKey(2, "ZZZZ-ZZZZ-ZZZZ-ZZZZ"); err == nil {
		t.Errorf("无效卡密应失败")
	}
	// 作废一张未用卡密后不可兑换
	codes2, _ := GenerateCardKeys(9, 1, 30, nil, "作废测试")
	var ck2 CardKey
	_ = DB.Where("code_hash = ?", hashCardCode(codes2[0])).First(&ck2)
	if e := VoidCardKey(ck2.ID); e != nil {
		t.Fatalf("作废失败: %v", e)
	}
	if _, err := RedeemCardKey(3, codes2[0]); err == nil {
		t.Errorf("已作废卡密兑换应失败")
	}
	// 再次作废应失败（仅可作废未使用）
	if e := VoidCardKey(ck2.ID); e == nil {
		t.Errorf("重复作废应失败")
	}
}

func TestTokenConfig(t *testing.T) {
	setupTokenTestDB(t)
	if IsTokenEnabled() {
		t.Errorf("代币系统默认应关闭")
	}
	_ = SetTokenEnabled(true)
	if !IsTokenEnabled() {
		t.Errorf("开启后应生效")
	}
	if GetTokenName() != DefaultTokenName {
		t.Errorf("默认代币名应为 %s，实际 %s", DefaultTokenName, GetTokenName())
	}
	_ = SetTokenName("  文学币  ")
	if GetTokenName() != "文学币" {
		t.Errorf("代币名应 trim 为 文学币，实际 %s", GetTokenName())
	}
	_ = SetTokenName("") // 空回退默认
	if GetTokenName() != DefaultTokenName {
		t.Errorf("空名应回退默认，实际 %s", GetTokenName())
	}
}
