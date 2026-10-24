package models

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupOpsDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:ops_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	DB = db
	if err := db.AutoMigrate(&PlatformConfig{}, &NavMenu{}, &Carousel{}, &CheckInRecord{},
		&UserTokenBalance{}, &TokenLedger{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, tbl := range []string{"nav_menus", "carousels", "checkin_records", "token_ledger", "user_token_balances"} {
		_ = db.Exec("DELETE FROM " + tbl).Error
	}
	_ = SetPlatformConfig(checkInCfgKey, "")
	_ = SetTokenEnabled(false)
}

// TestCheckInIdempotent 同一天重复签到不再发币（这是最重要的一条）
func TestCheckInIdempotent(t *testing.T) {
	setupOpsDB(t)
	cfg := DefaultCheckInConfig()
	cfg.Enabled = true
	cfg.BaseReward = 2
	cfg.StreakBonus = 1
	if err := SaveCheckInConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if err := SetTokenEnabled(true); err != nil {
		t.Fatal(err)
	}
	r1, err := DoCheckIn(1)
	if err != nil {
		t.Fatal(err)
	}
	if r1.Already || r1.Reward != 2 || r1.Streak != 1 {
		t.Fatalf("首签结果错: %+v", r1)
	}
	r2, err := DoCheckIn(1)
	if err != nil {
		t.Fatal(err)
	}
	if !r2.Already {
		t.Fatalf("重复签到应标记 already: %+v", r2)
	}
	// 账本只有一条
	var n int64
	DB.Model(&TokenLedger{}).Where("user_id = ? AND type = ?", 1, "checkin").Count(&n)
	if n != 1 {
		t.Fatalf("重复签到不应重复发币，账本 %d 条", n)
	}
}

// TestCheckInStreak 连签加成（昨天有记录 → streak+1，奖励叠加）
func TestCheckInStreak(t *testing.T) {
	setupOpsDB(t)
	cfg := DefaultCheckInConfig()
	cfg.Enabled = true
	cfg.BaseReward = 1
	cfg.StreakBonus = 2
	if err := SaveCheckInConfig(cfg); err != nil {
		t.Fatal(err)
	}
	y := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	if err := DB.Create(&CheckInRecord{UserID: 5, Date: y, Streak: 3, Reward: 5, CreatedAt: time.Now()}).Error; err != nil {
		t.Fatal(err)
	}
	r, err := DoCheckIn(5)
	if err != nil {
		t.Fatal(err)
	}
	if r.Streak != 4 {
		t.Fatalf("连签应为 4: %+v", r)
	}
	// 1 + 2*(4-1) = 7
	if r.Reward != 7 {
		t.Fatalf("奖励应为 7: %+v", r)
	}
}

func TestCheckInDisabled(t *testing.T) {
	setupOpsDB(t)
	if _, err := DoCheckIn(1); err == nil {
		t.Fatal("未开启时应报错")
	}
}

// TestNavURLValidation 菜单地址校验：拒绝 javascript:/data:/协议相对
func TestNavURLValidation(t *testing.T) {
	ok := []string{"/novels", "https://example.com/x"}
	for _, u := range ok {
		if _, _, err := NormalizeNavURL(u, "self"); err != nil {
			t.Fatalf("应允许 %s: %v", u, err)
		}
	}
	bad := []string{"javascript:alert(1)", "data:text/html,x", "//evil.com", "ftp://x/y", "no-slash"}
	for _, u := range bad {
		if _, _, err := NormalizeNavURL(u, "self"); err == nil {
			t.Fatalf("应拒绝 %s", u)
		}
	}
}

// TestNavMenuTree 两级树：子项挂在父项下，隐藏项前台不可见
func TestNavMenuTree(t *testing.T) {
	setupOpsDB(t)
	parent, err := SaveNavMenu(NavMenu{Title: "创作", URL: "/write", SortOrder: 1, Visible: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SaveNavMenu(NavMenu{Title: "我的作品", URL: "/author", ParentID: parent.ID,
		SortOrder: 1, Visible: true, RequireLogin: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveNavMenu(NavMenu{Title: "隐藏项", URL: "/hidden", Visible: false}); err != nil {
		t.Fatal(err)
	}
	pub, err := PublicNavMenus(false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(pub) != 1 || pub[0].Title != "创作" {
		t.Fatalf("未登录前台应只见 1 个一级项: %+v", pub)
	}
	// 子项要求登录：未登录时应被过滤掉（不是挂一个空壳）
	if len(pub[0].Children) != 0 {
		t.Fatalf("未登录不应看到需登录子项: %+v", pub[0].Children)
	}
	// 登录后子项可见
	pub2, _ := PublicNavMenus(true, false)
	if len(pub2[0].Children) != 1 {
		t.Fatalf("登录后子项应可见: %+v", pub2[0].Children)
	}
	all, _ := AllNavMenus()
	if len(all) != 2 {
		t.Fatalf("管理端应见 2 个一级项（含隐藏）: %d", len(all))
	}
	// 删除父项连带子项
	if err := DeleteNavMenu(parent.ID); err != nil {
		t.Fatal(err)
	}
	var n int64
	DB.Model(&NavMenu{}).Where("parent_id = ?", parent.ID).Count(&n)
	if n != 0 {
		t.Fatalf("子项应被连带删除: %d", n)
	}
}

// TestCarouselActiveWindow 轮播时段过滤（后端过滤，不依赖前端）
func TestCarouselActiveWindow(t *testing.T) {
	setupOpsDB(t)
	past := time.Now().AddDate(0, 0, -2)
	pastEnd := time.Now().AddDate(0, 0, -1)
	// 已过期
	if _, err := SaveCarousel(Carousel{Title: "过期", ImageURL: "/uploads/a.png",
		Enabled: true, StartAt: &past, EndAt: &pastEnd}); err != nil {
		t.Fatal(err)
	}
	// 进行中
	if _, err := SaveCarousel(Carousel{Title: "进行中", ImageURL: "/uploads/b.png", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	// 关闭
	if _, err := SaveCarousel(Carousel{Title: "关闭", ImageURL: "/uploads/c.png", Enabled: false}); err != nil {
		t.Fatal(err)
	}
	list, err := ActiveCarousels()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Title != "进行中" {
		t.Fatalf("只应返回进行中的一条: %+v", list)
	}
	// 非法图片地址
	if _, err := SaveCarousel(Carousel{Title: "x", ImageURL: "javascript:alert(1)"}); err == nil {
		t.Fatal("非法图片地址应拒绝")
	}
}
