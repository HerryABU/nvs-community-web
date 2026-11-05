package theme

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"nvs-server/models"
)

// newMapTestDB 独立的唯一内存库：`file::memory:?cache=shared` 是全局同名库，
// 同包内所有测试会互相污染，所以这里带时间戳唯一命名。
func newMapTestDB(t *testing.T) {
	t.Helper()
	dsn := fmt.Sprintf("file:map_setting_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存测试库失败: %v", err)
	}
	if err := models.AutoMigrateAll(db); err != nil {
		t.Fatalf("迁移测试库失败: %v", err)
	}
	models.DB = db
	t.Cleanup(func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
}

// TestMapSettingDefault 没配任何东西时默认离线示意（不要求站长先申请 key）
func TestMapSettingDefault(t *testing.T) {
	newMapTestDB(t)
	t.Setenv("TIANDITU_TK", "")
	t.Setenv("MAP_REVIEW_NO", "")

	c := LoadMapSetting()
	if c.Provider != "offline" || c.Scope != "china" {
		t.Fatalf("默认应为 offline/china，实际 %s/%s", c.Provider, c.Scope)
	}
	if c.TiandituKey != "" {
		t.Fatalf("不应有内置 key，实际 %q", c.TiandituKey)
	}
	if got, ok := PublicMapSetting()["key_ready"]; ok && got != false {
		t.Fatalf("未配 key 时 key_ready 应为 false，实际 %v", got)
	}
}

// TestMapSettingEnvFallback env 注入 TIANDITU_TK 时自动启用天地图（容器化部署免进后台）
func TestMapSettingEnvFallback(t *testing.T) {
	newMapTestDB(t)
	t.Setenv("TIANDITU_TK", "env-tk-123")
	t.Setenv("MAP_REVIEW_NO", "GS(2024)0650号")

	c := LoadMapSetting()
	if c.Provider != "tianditu" {
		t.Fatalf("注入了 key 应自动切到 tianditu，实际 %s", c.Provider)
	}
	if c.TiandituKey != "env-tk-123" {
		t.Fatalf("key 未从 env 读取，实际 %q", c.TiandituKey)
	}
	if c.ReviewNo != "GS(2024)0650号" {
		t.Fatalf("审图号未从 env 读取，实际 %q", c.ReviewNo)
	}
}

// TestMapSettingDBWinsOverEnv 后台显式填过就以数据库为准，env 不得覆盖
func TestMapSettingDBWinsOverEnv(t *testing.T) {
	newMapTestDB(t)
	t.Setenv("TIANDITU_TK", "env-tk-123")

	if err := SaveMapSetting(MapSetting{
		Provider: "tianditu", Scope: "world", TiandituKey: "db-tk-999", ReviewNo: "GS(2023)0001号",
	}); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	c := LoadMapSetting()
	if c.TiandituKey != "db-tk-999" || c.Scope != "world" || c.ReviewNo != "GS(2023)0001号" {
		t.Fatalf("数据库配置应优先于 env，实际 %+v", c)
	}
}

// TestMapSettingSanitize 坏值回退（provider 乱写 → offline；scope 乱写 → china）
func TestMapSettingSanitize(t *testing.T) {
	newMapTestDB(t)
	if err := models.SetPlatformConfig(mapSettingKey, `{"provider":"baidu","scope":"mars","tianditu_key":"k"}`); err != nil {
		t.Fatalf("写入坏配置失败: %v", err)
	}
	c := LoadMapSetting()
	if c.Provider != "offline" {
		t.Fatalf("非法 provider 应回退 offline，实际 %s", c.Provider)
	}
	if c.Scope != "china" {
		t.Fatalf("非法 scope 应回退 china，实际 %s", c.Scope)
	}
	if c.TiandituKey != "k" {
		t.Fatalf("key 不应被丢弃，实际 %q", c.TiandituKey)
	}
}

// TestPublicMapSettingHidesNothingButFlags key_ready 反映真实配置状态
func TestPublicMapSettingKeyReady(t *testing.T) {
	newMapTestDB(t)
	t.Setenv("TIANDITU_TK", "")
	if err := SaveMapSetting(MapSetting{Provider: "tianditu", TiandituKey: "abc"}); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	if got := PublicMapSetting()["key_ready"]; got != true {
		t.Fatalf("配了 key 时 key_ready 应为 true，实际 %v", got)
	}
}
