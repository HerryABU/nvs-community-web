package models

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupLiveDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:live_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	DB = db
	if err := db.AutoMigrate(&PlatformConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	_ = SetPlatformConfig(liveCfgKey, "")
}

// TestLiveValidateEmbed 嵌入地址必须 https（http 会被浏览器拦，且不安全）
func TestLiveValidateEmbed(t *testing.T) {
	cases := map[string]bool{
		"https://live.example.com/room/1":   true,
		"http://live.example.com/room/1":    false,
		"javascript:alert(1)":               false,
		"data:text/html,<script>x</script>": false,
		"":                                  true, // 不填允许（可只配 video_url）
	}
	for u, ok := range cases {
		err := ValidateLive(LiveSetting{EmbedURL: u})
		if ok && err != nil {
			t.Fatalf("%q 应通过: %v", u, err)
		}
		if !ok && err == nil {
			t.Fatalf("%q 应被拒绝", u)
		}
	}
}

// TestLiveValidateVideo 直链只允许 http(s) 或站内 uploads
func TestLiveValidateVideo(t *testing.T) {
	if err := ValidateLive(LiveSetting{VideoURL: "/uploads/2026/10/a.mp4"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateLive(LiveSetting{VideoURL: "https://cdn.example.com/a.mp4"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateLive(LiveSetting{VideoURL: "file:///etc/passwd"}); err == nil {
		t.Fatal("file 协议应拒绝")
	}
}

// TestLiveSaveNormalize 状态非法回落 offline；标题超长截断
func TestLiveSaveNormalize(t *testing.T) {
	setupLiveDB(t)
	long := ""
	for i := 0; i < 80; i++ {
		long += "长"
	}
	if err := SaveLiveSetting(LiveSetting{Enabled: true, Status: "weird", Title: long}); err != nil {
		t.Fatal(err)
	}
	got := LoadLiveSetting()
	if got.Status != LiveOffline {
		t.Fatalf("非法状态应回落 offline: %s", got.Status)
	}
	if len([]rune(got.Title)) > 60 {
		t.Fatalf("标题应截断到 60 字: %d", len([]rune(got.Title)))
	}
	if !got.Enabled {
		t.Fatal("enabled 应保留")
	}
}

func TestLiveDefaultDisabled(t *testing.T) {
	setupLiveDB(t)
	if LoadLiveSetting().Enabled {
		t.Fatal("默认应关闭")
	}
}
