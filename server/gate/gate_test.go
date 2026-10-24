package gate

import (
	"testing"
	"time"

	"nvs-server/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupGateDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:gate_test_x?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	models.DB = db
	if err := db.AutoMigrate(&models.PlatformConfig{}); err != nil {
		t.Fatalf("platform_configs: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	_ = db.Exec("DELETE FROM gate_device_bindings").Error
	_ = db.Exec("DELETE FROM gate_invites").Error
	_ = models.SetPlatformConfig("register_gate", "")
}

func TestCanRegisterDeviceLimit(t *testing.T) {
	setupGateDB(t)
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.MaxAccountsPerDevice = 2
	cfg.BlockHoneypotIP = false
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := Bind(models.DB, "dev-abc", uint(i+1), "1.1.1.1", "ua", "header"); err != nil {
			t.Fatal(err)
		}
	}
	if err := CanRegister(RegisterInput{IP: "1.1.1.1", DeviceID: "dev-abc"}); err == nil {
		t.Fatal("达到设备上限应拒绝")
	}
	// 换设备放行
	if err := CanRegister(RegisterInput{IP: "1.1.1.1", DeviceID: "dev-other"}); err != nil {
		t.Fatalf("新设备应放行: %v", err)
	}
}

func TestCanRegisterInviteAndDisposable(t *testing.T) {
	setupGateDB(t)
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.RequireInvite = true
	cfg.BlockDisposableEmail = true
	cfg.BlockHoneypotIP = false
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := CanRegister(RegisterInput{IP: "2.2.2.2", DeviceID: "d1", Email: "a@b.com"}); err == nil {
		t.Fatal("未填邀请码应拒绝")
	}
	if err := CanRegister(RegisterInput{IP: "2.2.2.2", DeviceID: "d1",
		Email: "a@mailinator.com", Invite: "INV-X"}); err == nil {
		t.Fatal("临时邮箱应拒绝")
	}
	if err := CanRegister(RegisterInput{IP: "2.2.2.2", DeviceID: "d1",
		Email: "a@b.com", Invite: "INV-X"}); err != nil {
		t.Fatalf("应放行: %v", err)
	}
}

func TestInviteUseLimit(t *testing.T) {
	setupGateDB(t)
	inv, err := CreateInvite(1, 1, "test", 24)
	if err != nil {
		t.Fatal(err)
	}
	if err := UseInvite(nil, inv.Code); err != nil {
		t.Fatalf("首次应成功: %v", err)
	}
	if err := UseInvite(nil, inv.Code); err == nil {
		t.Fatal("用尽后应失败")
	}
	if err := RevokeInvite(inv.ID); err != nil {
		t.Fatal(err)
	}
}

func TestCanPostAgeAndDailyLimit(t *testing.T) {
	setupGateDB(t)
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.PostMinAgeHours = 24
	cfg.CommentMinAgeHours = 1
	cfg.NewUserDailyPosts = 2
	cfg.NewUserDays = 3
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	fresh := time.Now().Add(-90 * time.Minute)
	if err := CanPost(PostGateInput{UserID: 1, CreatedAt: fresh, Kind: "post"}); err == nil {
		t.Fatal("未满 24 小时不应能发文")
	}
	if err := CanPost(PostGateInput{UserID: 1, CreatedAt: fresh, Kind: "comment"}); err != nil {
		t.Fatalf("满 1 小时应能评论: %v", err)
	}
	// 新号每日上限（评论路径，账号已满 1 小时）
	if err := CanPost(PostGateInput{UserID: 1, CreatedAt: fresh, Kind: "comment", TodayCount: 2}); err == nil {
		t.Fatal("新号达到每日上限应拒绝")
	}
	// 老账号不受新号上限限制
	old := time.Now().Add(-10 * 24 * time.Hour)
	if err := CanPost(PostGateInput{UserID: 1, CreatedAt: old, Kind: "comment", TodayCount: 99}); err != nil {
		t.Fatalf("老账号不应受限: %v", err)
	}
}

func TestDeviceIDFromFallback(t *testing.T) {
	// 无头时用 ip+ua 兜底，且必须稳定（同一请求两次结果一致）
	if got := shortHash("1.2.3.4|ua"); got != shortHash("1.2.3.4|ua") {
		t.Fatal("兜底指纹应稳定")
	}
	if shortHash("1.2.3.4|ua") == shortHash("1.2.3.5|ua") {
		t.Fatal("不同 IP 应产生不同兜底指纹")
	}
}

func TestIsDisposableEmail(t *testing.T) {
	cases := map[string]bool{
		"a@mailinator.com":  true,
		"a@sub.yopmail.com": true,
		"a@gmail.com":       false,
		"a@qq.com":          false,
		"not-an-email":      false,
	}
	for in, want := range cases {
		if got := IsDisposableEmail(in); got != want {
			t.Fatalf("%s: got %v want %v", in, got, want)
		}
	}
}
