package honeypot

import (
	"net/http/httptest"
	"strings"
	"testing"

	"nvs-server/models"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupHP(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:honeypot_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if err := models.AutoMigrateAll(db); err != nil {
		t.Fatalf("主迁移失败: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("蜜罐迁移失败: %v", err)
	}
	old := models.DB
	models.DB = db
	ResetDB()
	t.Cleanup(func() {
		models.DB = old
		ResetCache()
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func newCtx(path string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", path, nil)
	c.Request.Header.Set("User-Agent", "sqlmap/1.7")
	c.Request.Header.Set("X-Forwarded-For", "10.1.2.3, 10.9.9.9")
	c.Request.Header.Set("Cookie", "session=secret")
	return c, w
}

// TestCurrentWithoutDB DB 未初始化（未建站）时不得 panic，一律按 off 处理
func TestCurrentWithoutDB(t *testing.T) {
	old := models.DB
	models.DB = nil
	ResetCache()
	defer func() { models.DB = old; ResetCache() }()

	set := Current()
	if set.Level != LevelOff || set.Enabled {
		t.Fatalf("无 DB 时应为 off: %+v", set)
	}
	// 诱饵 handler 也要能安全返回（不 panic）
	c, _ := newCtx("/.env")
	if ev, err := Record(set, c, allDecoys(DefaultDecoyProfile())["/.env"]); err != nil || ev != nil {
		t.Fatalf("off 态不应记录: %+v err=%v", ev, err)
	}
}

// TestLevelControlsDecoys 等级决定诱饵数量：off 无、low 1 个、medium 4 个、high 全量
func TestLevelControlsDecoys(t *testing.T) {
	setupHP(t)
	if got := len(decoysForLevel(LevelOff, DefaultDecoyProfile())); got != 0 {
		t.Fatalf("off 不应有诱饵: %d", got)
	}
	if got := len(decoysForLevel(LevelLow, DefaultDecoyProfile())); got != 1 {
		t.Fatalf("low 应为 1 个: %d", got)
	}
	if got := len(decoysForLevel(LevelMedium, DefaultDecoyProfile())); got != 4 {
		t.Fatalf("medium 应为 4 个: %d", got)
	}
	if got := len(decoysForLevel(LevelHigh, DefaultDecoyProfile())); got < 6 {
		t.Fatalf("high 应全量: %d", got)
	}
}

// TestSaveSettingNormalize 开启但等级 off → 自动降到 low；关闭 → off
func TestSaveSettingNormalize(t *testing.T) {
	setupHP(t)
	s, err := SaveSetting(Setting{Enabled: true, Level: "bogus"})
	if err != nil || s.Level != LevelLow {
		t.Fatalf("非法等级应回落 low: %+v err=%v", s, err)
	}
	s2, _ := SaveSetting(Setting{Enabled: false, Level: LevelHigh})
	if s2.Level != LevelOff {
		t.Fatalf("关闭应为 off: %+v", s2)
	}
}

// TestLogPolicyFields 勾选字段：关掉 UA/XFF/Query 就不落库；请求头只记白名单且不带 Cookie
func TestLogPolicyFields(t *testing.T) {
	setupHP(t)
	set := Setting{
		ID: 1, Enabled: true, Level: LevelMedium,
		RecordUA: false, RecordXFF: false, RecordQuery: false,
		RecordReferer: true, RecordHeaders: true, RecordBodySnippet: false,
		HeaderAllowlist: "X-Forwarded-For, Cookie, Authorization",
	}
	c, _ := newCtx("/.env?token=abc")
	f := BuildLogFields(set, c)
	if f.UA != "" || f.XFF != "" || f.Query != "" {
		t.Fatalf("关掉的字段不应记录: %+v", f)
	}
	if strings.Contains(f.Headers, "session=secret") {
		t.Fatalf("Cookie 绝不应记录: %s", f.Headers)
	}
	if !strings.Contains(f.Headers, "x-forwarded-for") {
		t.Fatalf("白名单头应记录: %s", f.Headers)
	}

	// 全开
	set.RecordUA, set.RecordXFF, set.RecordQuery = true, true, true
	f2 := BuildLogFields(set, c)
	if f2.UA == "" || f2.XFF == "" || f2.Query != "token=abc" {
		t.Fatalf("开启后应记录: %+v", f2)
	}
}

// TestRecordAndBan 命中记录会话/事件；高等级 + auto_ban 触发封禁
func TestRecordAndBan(t *testing.T) {
	setupHP(t)
	set, err := SaveSetting(Setting{Enabled: true, Level: LevelHigh, AutoBan: true, BanMinutes: 30,
		RecordUA: true, RecordReferer: true, RecordQuery: true})
	if err != nil {
		t.Fatal(err)
	}
	c, _ := newCtx("/.env")
	d := allDecoys(DefaultDecoyProfile())["/.env"]
	ev, err := Record(*set, c, d)
	if err != nil || ev == nil {
		t.Fatalf("记录失败: %+v err=%v", ev, err)
	}
	if ev.Action != "ban" || ev.Risk != d.Risk {
		t.Fatalf("应触发封禁: %+v", ev)
	}
	if !IsBanned(c.ClientIP()) {
		t.Fatal("IP 未进入封禁列表")
	}
	Unban(c.ClientIP())
	if IsBanned(c.ClientIP()) {
		t.Fatal("解封失败")
	}
	// 会话聚合
	list, total, err := ListSessions(1, 10, "", 0)
	if err != nil || total != 1 || len(list) != 1 {
		t.Fatalf("会话聚合错误: %d %v", total, err)
	}
	if list[0].EventCount < 1 {
		t.Fatalf("会话计数未更新: %+v", list[0])
	}
}

// TestServeDecoyByLevel 等级不覆盖的诱饵按 404 处理，覆盖的返回仿真内容
func TestServeDecoyByLevel(t *testing.T) {
	setupHP(t)
	if _, err := SaveSetting(Setting{Enabled: true, Level: LevelLow}); err != nil {
		t.Fatal(err)
	}
	// low 只覆盖 .env
	// 诱饵必须「看起来是真的」：要有正常配置项，且**不得**出现任何自曝标记。
	// 这是回归保护 —— 早期版本写过 `# jwt（泄漏示例 / 已轮换）` 和 `CANARY=...`，
	// 攻击者扫到 .env 第一眼就知道是蜜罐，整份诱饵白做。
	content := envDecoy(DefaultDecoyProfile())
	for _, must := range []string{"APP_NAME", "APP_ENV=production", "JWT_SECRET=", "DB_HOST=", "DB_PASSWORD=", "REDIS_HOST="} {
		if !strings.Contains(content, must) {
			t.Fatalf(".env 诱饵缺少正常配置项 %q://n%s", must, content)
		}
	}
	leakMarkers := []string{"CANARY", "HONEY", "DECOY", "EXAMPLE", "PLACEHOLDER", "蜜罐", "示例", "已轮换", "模拟", "占位"}
	upper := strings.ToUpper(content)
	for _, m := range leakMarkers {
		if strings.Contains(upper, strings.ToUpper(m)) {
			t.Fatalf(".env 诱饵出现自曝标记 %q，攻击者会立刻识破:\n%s", m, content)
		}
	}
	// 金丝雀取值唯一，且 stealth 风格不得带 CANARY 字样
	a, bb := newCanary(""), newCanary("")
	if a == bb {
		t.Fatal("金丝雀应每次唯一")
	}
	if strings.Contains(strings.ToUpper(a), "CANARY") {
		t.Fatalf("stealth 风格的金丝雀不应带 CANARY 字样: %s", a)
	}
	// 探测路径不返回内容（与 404 一致）
	c, w := newCtx("/wp-admin/")
	serveProbe(c, "/wp-admin/")
	if w.Code != 0 && w.Code != 404 && w.Code != 200 {
		t.Fatalf("异常状态码: %d", w.Code)
	}
}

// TestExportLogsAction 导出会写一条 export 事件（留痕）
func TestExportLogsAction(t *testing.T) {
	setupHP(t)
	if _, err := SaveSetting(Setting{Enabled: true, Level: LevelMedium}); err != nil {
		t.Fatal(err)
	}
	c, _ := newCtx("/.git/config")
	if _, err := Record(Current(), c, allDecoys(DefaultDecoyProfile())["/.git/config"]); err != nil {
		t.Fatal(err)
	}
	var sess Session
	if err := models.DB.First(&sess).Error; err != nil {
		t.Fatal(err)
	}
	data, err := Export(sess.ID, 99)
	if err != nil || data["session"] == nil {
		t.Fatalf("导出失败: %v", err)
	}
	LogExport(sess.ID, 99)
	var n int64
	models.DB.Model(&Event{}).Where("kind = ?", EventExport).Count(&n)
	if n != 1 {
		t.Fatalf("导出未留痕: %d", n)
	}
}
