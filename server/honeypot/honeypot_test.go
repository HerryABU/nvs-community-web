package honeypot

import (
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"nvs-server/models"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ===== (原 honeypot_test.go) =====

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

// ===== 合并自 archive_test.go =====

// TestArchiveBeforeDelete 超期事件必须先落 gzip 归档再删库
func TestArchiveBeforeDelete(t *testing.T) {
	setupHP(t)
	if _, err := SaveSetting(Setting{Enabled: true, Level: LevelMedium, RetentionDays: 30}); err != nil {
		t.Fatal(err)
	}
	ResetCache()

	// 一条超期 + 一条未超期
	old := &Event{Kind: EventDecoyHit, Decoy: "env", Path: "/.env", Risk: 70,
		CreatedAt: time.Now().AddDate(0, 0, -60)}
	recent := &Event{Kind: EventDecoyHit, Decoy: "env", Path: "/.env", Risk: 70, CreatedAt: time.Now()}
	models.DB.Create(old)
	models.DB.Create(recent)

	res, err := ArchiveAndCleanup()
	if err != nil {
		t.Fatalf("归档失败: %v", err)
	}
	if res.Skipped || res.Archived != 1 {
		t.Fatalf("应归档 1 条: %+v", res)
	}
	if res.File == "" || res.Bytes == 0 {
		t.Fatalf("归档文件缺失: %+v", res)
	}
	if _, err := os.Stat(res.File); err != nil {
		t.Fatalf("归档文件不存在: %v", err)
	}
	// 超期被删、未超期保留
	var oldCnt, recentCnt int64
	models.DB.Model(&Event{}).Where("path = ? AND created_at < ?", "/.env", time.Now().AddDate(0, 0, -10)).Count(&oldCnt)
	models.DB.Model(&Event{}).Count(&recentCnt)
	if oldCnt != 0 || recentCnt != 1 {
		t.Fatalf("清理结果错误 old=%d recent=%d", oldCnt, recentCnt)
	}
	// 归档列表可见
	list, err := ListArchives()
	if err != nil || len(list) == 0 {
		t.Fatalf("归档列表为空: %v", err)
	}
	_ = os.RemoveAll(archiveDir())
}

// TestIsolatedDBSwitch 独立库切换后读写都落在独立库，且拒绝与主库同路径
func TestIsolatedDBSwitch(t *testing.T) {
	setupHP(t)
	dir := t.TempDir()
	p := filepath.Join(dir, "honeypot.db")

	if err := OpenIsolated(p); err != nil {
		t.Fatalf("打开独立库失败: %v", err)
	}
	defer func() {
		ResetDB() // 复位到主库，避免污染其它测试
		_ = os.RemoveAll(dir)
	}()
	if !Isolated() || DBPath() != p {
		t.Fatalf("未切到独立库: isolated=%v path=%s", Isolated(), DBPath())
	}
	// 写配置进独立库
	if _, err := SaveSetting(Setting{Enabled: true, Level: LevelHigh, RetentionDays: 60}); err != nil {
		t.Fatal(err)
	}
	got := Current()
	if got.Level != LevelHigh || got.RetentionDays != 60 {
		t.Fatalf("独立库读写异常: %+v", got)
	}
	// 同路径应被拒（把主库路径指向同一文件）
	t.Setenv("DB_PATH", p)
	if err := SetDB(models.DB, p); err == nil {
		t.Fatal("同路径应被拒绝")
	}
	t.Setenv("DB_PATH", "")
	// 主库不应被写入这条配置
	var cnt int64
	models.DB.Model(&Setting{}).Where("level = ?", LevelHigh).Count(&cnt)
	if cnt != 0 {
		t.Fatalf("独立库配置泄漏到主库: %d", cnt)
	}
}

// ===== 合并自 sqli_test.go =====

func mustRequest(t *testing.T) *http.Request {
	t.Helper()
	return httptest.NewRequest("GET", "/api/v1/query?sql=1", nil)
}

// TestDetectSQLi 注入特征识别（正例/反例都要有，避免「正常长文本也判成注入」）
func TestDetectSQLi(t *testing.T) {
	positives := []string{
		"id=1 UNION SELECT username,password FROM nvs_users",
		"id=1' AND SLEEP(5)--",
		"id=1; DROP TABLE users",
		"q=1' or '1'='1",
		"id=1 AND extractvalue(1,concat(0x7e,version()))",
		"file=../../etc/passwd' UNION/**/SELECT load_file('/etc/passwd')",
		"id=1' AND (SELECT 1 FROM information_schema.tables)",
	}
	for _, p := range positives {
		if hits := DetectSQLi(p); len(hits) == 0 {
			t.Fatalf("应识别为注入: %s", p)
		}
	}
	negatives := []string{
		"keyword=小说",
		"page=2&size=20",
		"title=The Union of Two Cities",
		"",
	}
	for _, n := range negatives {
		if hits := DetectSQLi(n); len(hits) > 0 {
			t.Fatalf("不应识别为注入: %q -> %v", n, hits)
		}
	}
}

// TestSQLiRiskScoring 风险分：无命中=40 探路；命中越高分越高；多特征叠加加分
func TestSQLiRiskScoring(t *testing.T) {
	if got := SQLiRisk(nil); got != 40 {
		t.Fatalf("无命中应为探路分 40，实际 %d", got)
	}
	one := SQLiRisk(DetectSQLi("id=1 or 1=1"))
	multi := SQLiRisk(DetectSQLi("id=1 UNION SELECT 1 FROM information_schema.tables--"))
	if multi < one {
		t.Fatalf("多特征风险应不低于单特征：one=%d multi=%d", one, multi)
	}
	if multi > 100 {
		t.Fatalf("风险分不应超过 100：%d", multi)
	}
}

// TestFakeResultShape 预生成响应库：金丝雀必须在，且不含真实数据
func TestFakeResultShape(t *testing.T) {
	if len(sqliCanary) != 32 {
		t.Fatalf("溯源标记应为 32 位十六进制（伪装成普通 token），实得: %s", sqliCanary)
	}
	users := fakeUsers(6)
	if len(users) == 0 {
		t.Fatal("假用户库不应为空")
	}
	for _, u := range users {
		if !strings.HasSuffix(u.Email, "@example.internal") {
			t.Fatalf("假邮箱应使用 example.internal 域: %s", u.Email)
		}
	}
	// 固定种子：两次生成一致（更像同一个「库」）
	a, b := fakeUsers(6), fakeUsers(6)
	for i := range a {
		if a[i].Hash != b[i].Hash {
			t.Fatal("假数据应可复现（同一部署内一致）")
		}
	}
}

// TestCaptureTLSProxyHeader 反代透传时采集 JA3/JA4；缺失时标 missing 而不是伪造
func TestCaptureTLSProxyHeader(t *testing.T) {
	req := mustRequest(t)
	req.Header.Set("X-JA3-Hash", "cd08e31494f9531f560d64c695473da9")
	info := CaptureTLS(req)
	if info.Source != "proxy_header" || info.JA3 == "" || !info.ViaProxy {
		t.Fatalf("应采信反代头: %+v", info)
	}

	plain := mustRequest(t)
	info2 := CaptureTLS(plain)
	if info2.Source != "missing" || info2.JA3 != "" {
		t.Fatalf("缺失时必须标 missing 且不伪造: %+v", info2)
	}
}

func TestTLSStatsDiagnosis(t *testing.T) {
	if got := diagnoseJA3(0, 0); !strings.Contains(got, "还没有指纹") {
		t.Fatalf("空数据诊断不对: %s", got)
	}
	if got := diagnoseJA3(10, 0); !strings.Contains(got, "未透传") {
		t.Fatalf("零采集诊断不对: %s", got)
	}
	if got := diagnoseJA3(10, 10); !strings.Contains(got, "正常") {
		t.Fatalf("满采集诊断不对: %s", got)
	}
}
