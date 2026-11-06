package vipcard

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"nvs-server/models"
	"nvs-server/readperm"
)

func setupVipCardDB(t *testing.T) uint {
	t.Helper()
	prev := models.DB
	dsn := fmt.Sprintf("file:vipcard_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存库失败: %v", err)
	}
	if err := models.AutoMigrateAll(db); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	models.DB = db
	readperm.InvalidateCache()
	t.Cleanup(func() {
		models.DB = prev
		readperm.InvalidateCache()
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})

	// 站长配置：最高可发行 3 级
	cfg := readperm.DefaultConfig()
	cfg.VipMaxLevel = 3
	if err := readperm.Save(cfg); err != nil {
		t.Fatalf("保存配置失败: %v", err)
	}
	readperm.InvalidateCache()

	u := models.User{Username: "vipuser", Email: "vip@example.com", PasswordHash: "x", Role: "reader"}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	return u.ID
}

func TestGenerateValidatesLevelByConfig(t *testing.T) {
	setupVipCardDB(t)
	if _, err := Generate(1, 1, 4, 30, nil, "超限"); err == nil {
		t.Fatal("超过站长设定上限的等级应被拒绝")
	}
	if _, err := Generate(1, 1, 0, 30, nil, "非法"); err == nil {
		t.Fatal("等级 0 应被拒绝")
	}
	if _, err := Generate(1, 0, 1, 30, nil, "零数量"); err == nil {
		t.Fatal("数量 0 应被拒绝")
	}
	codes, err := Generate(1, 3, 2, 30, nil, "正常")
	if err != nil {
		t.Fatalf("正常生成失败: %v", err)
	}
	if len(codes) != 3 {
		t.Fatalf("应生成 3 张，实际 %d", len(codes))
	}
	// 库里只存哈希，不存明文
	var n int64
	_ = models.DB.Model(&VipCard{}).Where("code_hash = ?", hashCode(codes[0])).Count(&n).Error
	if n != 1 {
		t.Fatal("卡密哈希未正确落库")
	}
	var stored VipCard
	_ = models.DB.Where("code_hash = ?", hashCode(codes[0])).First(&stored).Error
	if stored.CodePrefix == "" || stored.Level != 2 || stored.Days != 30 || stored.Status != StatusUnused {
		t.Fatalf("卡密字段不符: %+v", stored)
	}
}

func TestRedeemGrantsLevelAndDays(t *testing.T) {
	uid := setupVipCardDB(t)

	codes2, _ := Generate(1, 1, 2, 30, nil, "")
	r, err := Redeem(uid, codes2[0])
	if err != nil {
		t.Fatalf("兑换失败: %v", err)
	}
	if r.Level != 2 || r.Days != 30 {
		t.Fatalf("兑换结果不符: %+v", r)
	}
	if got := readperm.VipLevelOf(uid); got != 2 {
		t.Fatalf("钩子应返回 2 级，实际 %d", got)
	}
	// 规范化：带空格 / 小写 / 无分隔符也应可用
	codes3, _ := Generate(1, 1, 3, 10, nil, "")
	r2, err := Redeem(uid, "  "+codes3[0]+"  ")
	if err != nil {
		t.Fatalf("带空格兑换失败: %v", err)
	}
	if r2.Level != 3 {
		t.Fatalf("等级应取 max(2,3)=3，实际 %d", r2.Level)
	}
	if !r2.Until.After(r.Until) {
		t.Fatal("连兑第二张应累加时长")
	}
	if got := readperm.VipLevelOf(uid); got != 3 {
		t.Fatalf("钩子应返回 3 级，实际 %d", got)
	}
}

func TestRedeemRejectsReusedVoidExpired(t *testing.T) {
	uid := setupVipCardDB(t)

	codes, _ := Generate(1, 1, 1, 10, nil, "")
	if _, err := Redeem(uid, codes[0]); err != nil {
		t.Fatalf("首次兑换失败: %v", err)
	}
	if _, err := Redeem(uid, codes[0]); err == nil {
		t.Fatal("重复兑换应失败")
	}
	if _, err := Redeem(uid, "NOPE-NOPE-NOPE-NOPE"); err == nil {
		t.Fatal("不存在的卡密应失败")
	}

	voidCodes, _ := Generate(1, 1, 1, 10, nil, "")
	var v VipCard
	_ = models.DB.Where("code_hash = ?", hashCode(voidCodes[0])).First(&v).Error
	if err := Void(v.ID); err != nil {
		t.Fatalf("作废失败: %v", err)
	}
	if _, err := Redeem(uid, voidCodes[0]); err == nil {
		t.Fatal("已作废卡密应失败")
	}

	past := time.Now().Add(-time.Hour)
	expired, _ := Generate(1, 1, 1, 10, &past, "")
	if _, err := Redeem(uid, expired[0]); err == nil {
		t.Fatal("过期卡密应失败")
	}
}

func TestMembershipAndList(t *testing.T) {
	uid := setupVipCardDB(t)
	codes, _ := Generate(1, 2, 1, 5, nil, "批次备注")
	_, _ = Redeem(uid, codes[0])

	level, until, err := Membership(uid)
	if err != nil || level != 1 || until == nil {
		t.Fatalf("会员状态不符: level=%d until=%v err=%v", level, until, err)
	}
	list, err := List("", "", 10)
	if err != nil || len(list) != 2 {
		t.Fatalf("列表应为 2 张: len=%d err=%v", len(list), err)
	}
	used, err := List(StatusUsed, "", 10)
	if err != nil || len(used) != 1 {
		t.Fatalf("已使用筛选应 1 张: len=%d err=%v", len(used), err)
	}
	if used[0].UsedByUsername != "vipuser" {
		t.Fatalf("使用者用户名未填充: %+v", used[0])
	}
}

func TestExpiredMembershipIsLevelZero(t *testing.T) {
	uid := setupVipCardDB(t)
	codes, _ := Generate(1, 1, 2, 5, nil, "")
	_, _ = Redeem(uid, codes[0])
	// 把到期时间拨到过去
	past := time.Now().Add(-time.Hour)
	if err := models.DB.Model(&models.User{}).Where("id = ?", uid).
		Update("member_until", past).Error; err != nil {
		t.Fatalf("拨快到期失败: %v", err)
	}
	if got := readperm.VipLevelOf(uid); got != 0 {
		t.Fatalf("会员过期后应为 0 级，实际 %d", got)
	}
}
