package readperm

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"nvs-server/models"
	wallet "nvs-server/models/wallet"
)

func setupPurchaseDB(t *testing.T) uint {
	t.Helper()
	prev := models.DB
	dsn := fmt.Sprintf("file:readperm_purchase_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存库失败: %v", err)
	}
	if err := models.AutoMigrateAll(db); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	models.DB = db
	InvalidateCache()
	t.Cleanup(func() {
		models.DB = prev
		InvalidateCache()
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
	// 开放 paid 等级，最低价 1
	cfg := DefaultConfig()
	cfg.EnabledLevels = append(cfg.EnabledLevels, LevelPaid)
	cfg.PaidMinPrice = 1
	if err := Save(cfg); err != nil {
		t.Fatalf("保存配置失败: %v", err)
	}
	InvalidateCache()

	u := models.User{Username: "buyer", Email: "buyer@example.com", PasswordHash: "x", Role: "reader"}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	return u.ID
}

func TestPurchaseContentAtomicAndIdempotent(t *testing.T) {
	uid := setupPurchaseDB(t)
	if _, err := wallet.AppendTokenLedger(uid, "recharge_card", 100, "test", 0, "初始充值", ""); err != nil {
		t.Fatalf("充值失败: %v", err)
	}
	before := wallet.GetTokenBalance(uid)

	if err := PurchaseContent(uid, TypeNovel, 7, 10); err != nil {
		t.Fatalf("购买失败: %v", err)
	}
	if !HasPurchased(uid, TypeNovel, 7) {
		t.Fatal("购买后应记录解锁")
	}
	if got := wallet.GetTokenBalance(uid); got != before-10 {
		t.Fatalf("扣费不符: before=%d after=%d", before, got)
	}
	// 重复购买：幂等，不再扣费
	if err := PurchaseContent(uid, TypeNovel, 7, 10); err != nil {
		t.Fatalf("重复购买不应报错: %v", err)
	}
	if got := wallet.GetTokenBalance(uid); got != before-10 {
		t.Fatalf("重复购买不应再扣费: %d", got)
	}
	var n int64
	_ = models.DB.Model(&ContentPurchase{}).Count(&n).Error
	if n != 1 {
		t.Fatalf("购买记录应只有 1 条，实际 %d", n)
	}
	// 判定层应放行
	d := Check(NewActor(uid, "reader"), Content{
		Type: TypeNovel, ID: 7, AuthorID: 999,
		Permission: LevelPaid, PaidPrice: 10,
	})
	if !d.Allowed {
		t.Fatalf("购买后应放行: %+v", d)
	}
}

func TestPurchaseContentInsufficientBalanceRollsBack(t *testing.T) {
	uid := setupPurchaseDB(t)
	if _, err := wallet.AppendTokenLedger(uid, "recharge_card", 5, "test", 0, "初始充值", ""); err != nil {
		t.Fatalf("充值失败: %v", err)
	}
	err := PurchaseContent(uid, TypeBlog, 33, 10)
	if err == nil {
		t.Fatal("余额不足应失败")
	}
	if HasPurchased(uid, TypeBlog, 33) {
		t.Fatal("扣费失败不应留下解锁记录")
	}
	if got := wallet.GetTokenBalance(uid); got != 5 {
		t.Fatalf("失败后余额应不变，实际 %d", got)
	}
}

func TestPurchaseContentRejectsBadParams(t *testing.T) {
	uid := setupPurchaseDB(t)
	for _, tc := range []struct {
		uid uint
		ct  string
		id  uint
		p   int64
	}{
		{0, TypeNovel, 1, 10},
		{uid, "", 1, 10},
		{uid, TypeNovel, 0, 10},
		{uid, TypeNovel, 1, 0},
		{uid, TypeNovel, 1, -1},
	} {
		if err := PurchaseContent(tc.uid, tc.ct, tc.id, tc.p); err == nil {
			t.Fatalf("非法参数应被拒绝: %+v", tc)
		}
	}
	if IsKnownContentType("forum") || !IsKnownContentType(TypeNovel) || !IsKnownContentType(TypeBlog) {
		t.Fatal("内容类型白名单不符")
	}
}
