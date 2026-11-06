package readperm

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"nvs-server/models"
	"nvs-server/models/social" // 触发 follows 表注册，供粉丝可见判定
)

func setupReadPermDB(t *testing.T) {
	t.Helper()
	prev := models.DB
	dsn := fmt.Sprintf("file:readperm_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
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
}

// useConfig 写入一份配置并清缓存。
func useConfig(t *testing.T, c Config) {
	t.Helper()
	if err := Save(c); err != nil {
		t.Fatalf("保存配置失败: %v", err)
	}
	InvalidateCache()
}

func TestEffectivePermission(t *testing.T) {
	cases := []struct {
		perm, want string
		wall       bool
	}{
		{"", LevelPublic, false},
		{"", LevelLogin, true},
		{"not-a-level", LevelPublic, false},
		{LevelVip, LevelVip, true}, // 新字段优先于老开关
	}
	for _, c := range cases {
		if got := EffectivePermission(c.perm, c.wall); got != c.want {
			t.Errorf("EffectivePermission(%q,%v)=%q want %q", c.perm, c.wall, got, c.want)
		}
	}
}

func TestConfigRoundTrip(t *testing.T) {
	setupReadPermDB(t)
	c := DefaultConfig()
	c.EnabledLevels = []string{LevelVip, LevelPublic, LevelVip, "bogus", LevelPaid}
	c.VipMinLevel = 3
	c.Labels = map[string]string{LevelVip: "高级会员专享"}
	useConfig(t, c)

	got := Load()
	if got.EnabledLevels[0] != LevelPublic {
		t.Fatalf("public 必须恒在首位: %v", got.EnabledLevels)
	}
	seen := map[string]int{}
	for _, l := range got.EnabledLevels {
		seen[l]++
	}
	if seen[LevelVip] != 1 || seen["bogus"] != 0 || seen[LevelPaid] != 1 {
		t.Fatalf("等级未正确去重 / 过滤: %v", got.EnabledLevels)
	}
	if got.VipMinLevel != 3 || Label(got, LevelVip) != "高级会员专享" {
		t.Fatalf("配置往返丢失: %+v", got)
	}
	if !got.IsEnabled(LevelPaid) || got.IsEnabled(LevelPrivate) {
		t.Fatalf("启用集合不符: %v", got.EnabledLevels)
	}
}

func TestCheckAllLevels(t *testing.T) {
	setupReadPermDB(t)
	cfg := DefaultConfig()
	cfg.EnabledLevels = append(cfg.EnabledLevels, LevelPaid)
	useConfig(t, cfg)

	// 会员等级钩子固定：user 2 为 1 级会员，其余非会员。
	origHook := VipLevelOf
	VipLevelOf = func(id uint) int {
		if id == 2 {
			return 1
		}
		return 0
	}
	defer func() { VipLevelOf = origHook }()

	const author uint = 100
	anon := NewActor(0, "")
	reader := NewActor(1, "reader")
	vip := NewActor(2, "reader")
	admin := NewActor(9, "admin")
	owner := NewActor(author, "reader")

	content := func(level string) Content {
		return Content{Type: TypeNovel, ID: 7, AuthorID: author, Permission: level, PaidPrice: 10}
	}

	if d := Check(anon, content(LevelPublic)); !d.Allowed {
		t.Fatal("public 应放行匿名")
	}
	if d := Check(anon, content(LevelLogin)); d.Allowed || d.Reason != ReasonLoginRequired {
		t.Fatalf("login 应拦匿名: %+v", d)
	}
	if d := Check(reader, content(LevelLogin)); !d.Allowed {
		t.Fatal("login 应放行已登录用户")
	}
	if d := Check(reader, content(LevelFollower)); d.Allowed || d.Reason != ReasonFollowerOnly {
		t.Fatalf("未关注应被拦: %+v", d)
	}
	if d := Check(reader, content(LevelVip)); d.Allowed || d.Reason != ReasonVipRequired {
		t.Fatalf("非会员应被拦: %+v", d)
	}
	if d := Check(vip, content(LevelVip)); !d.Allowed {
		t.Fatal("会员应放行")
	}
	if d := Check(reader, content(LevelPaid)); d.Allowed || d.Reason != ReasonPaidRequired {
		t.Fatalf("未购买应被拦: %+v", d)
	}
	if d := Check(reader, content(LevelPrivate)); d.Allowed || d.Reason != ReasonPrivate {
		t.Fatalf("private 应拦普通读者: %+v", d)
	}
	if d := Check(owner, content(LevelPrivate)); !d.Allowed {
		t.Fatal("作者本人应放行")
	}
	if d := Check(admin, content(LevelPrivate)); !d.Allowed {
		t.Fatal("平台管理员合规放行")
	}
	if d := Check(anon, Content{Type: TypeNovel, ID: 8, AuthorID: author, WallEnabled: true}); d.Allowed {
		t.Fatal("老 wall_enabled=true 数据应等价于 login")
	}
}

func TestCheckFollowerAndPurchase(t *testing.T) {
	setupReadPermDB(t)
	cfg := DefaultConfig()
	cfg.EnabledLevels = append(cfg.EnabledLevels, LevelPaid)
	useConfig(t, cfg)

	const author, reader uint = 200, 201
	ct := Content{Type: TypeBlog, ID: 33, AuthorID: author, Permission: LevelFollower}
	if d := Check(NewActor(reader, "reader"), ct); d.Allowed {
		t.Fatal("尚未关注不应放行")
	}
	if err := social.CreateFollow(reader, author); err != nil {
		t.Fatalf("创建关注关系失败: %v", err)
	}
	if d := Check(NewActor(reader, "reader"), ct); !d.Allowed {
		t.Fatalf("关注后应放行: %+v", d)
	}

	paid := Content{Type: TypeBlog, ID: 33, AuthorID: author, Permission: LevelPaid, PaidPrice: 10}
	if d := Check(NewActor(reader, "reader"), paid); d.Allowed {
		t.Fatal("未购买不应放行")
	}
	if err := GrantPurchase(reader, TypeBlog, 33, 10); err != nil {
		t.Fatalf("发放购买记录失败: %v", err)
	}
	if err := GrantPurchase(reader, TypeBlog, 33, 10); err != nil {
		t.Fatalf("重复发放应幂等: %v", err)
	}
	var n int64
	_ = models.DB.Model(&ContentPurchase{}).Count(&n).Error
	if n != 1 {
		t.Fatalf("购买记录应幂等，实际 %d 行", n)
	}
	if d := Check(NewActor(reader, "reader"), paid); !d.Allowed {
		t.Fatalf("购买后应放行: %+v", d)
	}
	if HasPurchased(999, TypeBlog, 33) {
		t.Fatal("未购买用户不应命中")
	}
}

func TestCheckDisabledLevelFallsBackToLogin(t *testing.T) {
	setupReadPermDB(t)
	cfg := DefaultConfig() // 默认不含 paid
	useConfig(t, cfg)

	const author uint = 300
	ct := Content{Type: TypeNovel, ID: 1, AuthorID: author, Permission: LevelPaid, PaidPrice: 10}
	if d := Check(NewActor(0, ""), ct); d.Allowed || d.Reason != ReasonLoginRequired {
		t.Fatalf("被停用等级应保守降级为登录可见（匿名仍拦）: %+v", d)
	}
	if d := Check(NewActor(1, "reader"), ct); !d.Allowed {
		t.Fatalf("被停用等级降级后应放行登录用户: %+v", d)
	}
}

func TestValidateSelection(t *testing.T) {
	setupReadPermDB(t)
	useConfig(t, DefaultConfig()) // paid 未开放
	if err := ValidateSelection(LevelPaid, 10); err == nil {
		t.Fatal("停用的等级应被拒绝")
	}
	cfg := DefaultConfig()
	cfg.EnabledLevels = append(cfg.EnabledLevels, LevelPaid)
	cfg.PaidMinPrice = 20
	useConfig(t, cfg)
	if err := ValidateSelection(LevelPaid, 5); err == nil {
		t.Fatal("低于最低价应被拒绝")
	}
	if err := ValidateSelection(LevelPaid, 20); err != nil {
		t.Fatalf("合规选择应通过: %v", err)
	}
}
