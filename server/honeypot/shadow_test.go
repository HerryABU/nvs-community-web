package honeypot

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"nvs-server/models"

	"github.com/gin-gonic/gin"
)

// withLevel 设置等级并清缓存
func withLevel(t *testing.T, level string) {
	t.Helper()
	if _, err := SaveSetting(Setting{Enabled: true, Level: level, AutoBan: false,
		RecordUA: true, RecordReferer: true, RecordQuery: true}); err != nil {
		t.Fatalf("设置等级失败: %v", err)
	}
	ResetCache()
}

// TestShadowSecretIsIsolated 蜜罐密钥必须能签出蜜罐认的 token（先甜），且与真实 JWT 无关
func TestShadowSecretIsIsolated(t *testing.T) {
	setupHP(t)
	if ShadowSecretB64() == "" {
		t.Fatal("蜜罐密钥未生成")
	}
	// .env 诱饵里泄露的正是这把密钥
	if !strings.Contains(envDecoy(), "JWT_SECRET=") {
		t.Fatal("诱饵 .env 缺 JWT_SECRET")
	}
	tok, err := SignShadow(ShadowClaims{UserID: 1, Email: "a@b.c", Role: "admin", IsAdmin: true})
	if err != nil {
		t.Fatalf("签发失败: %v", err)
	}
	cl, err := VerifyShadow(tok)
	if err != nil || !cl.IsAdmin {
		t.Fatalf("蜜罐 token 应验签通过: %+v err=%v", cl, err)
	}
	// alg=none 必须拒绝
	none := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJ1c2VyX2lkIjoxfQ."
	if _, err := VerifyShadow(none); err == nil {
		t.Fatal("alg=none 不应通过")
	}
}

// TestShadowRoutes 甜（读）→ 卡（写/提权），且卡口没有成功分支
func TestShadowRoutes(t *testing.T) {
	setupHP(t)
	withLevel(t, LevelHigh)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterShadow(r)

	tok, _ := SignShadow(ShadowClaims{UserID: 7, Email: "e@x.com", Role: "admin", IsAdmin: true})

	// 甜：/api/v2/user/me 返回合成用户
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v2/user/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("User-Agent", "sqlmap/1.7")
	r.ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "admin") {
		t.Fatalf("甜接口异常: %d %s", w.Code, w.Body.String())
	}

	// 卡：写入 → 403 + 验证页地址
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("POST", "/api/v2/admin/users/1/role", strings.NewReader(`{"role":"admin"}`))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusForbidden || !strings.Contains(w2.Body.String(), "/hp/verify") {
		t.Fatalf("卡口异常: %d %s", w2.Code, w2.Body.String())
	}

	// 事件应记到同一会话：jwt_forged + privilege_ops
	var kinds []string
	models.DB.Model(&Event{}).Pluck("kind", &kinds)
	joined := strings.Join(kinds, ",")
	if !strings.Contains(joined, EventJWTForged) || !strings.Contains(joined, EventPrivilege) {
		t.Fatalf("事件类型不全: %s", joined)
	}
}

// TestVerifyNeverSucceeds 验证接口恒失败；页面里不存在 success 分支
func TestVerifyNeverSucceeds(t *testing.T) {
	setupHP(t)
	withLevel(t, LevelMedium)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterVerify(r)

	// 指纹上报：恒 ok:false
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/hp/fp", strings.NewReader(`{"canvas":"ab12","webgl":"cd34","headless":"webdriver"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"ok":false`) {
		t.Fatalf("指纹接口应恒失败: %d %s", w.Code, w.Body.String())
	}

	// 验证提交：恒 ok:false 且 retry=true（诱导重试，但不放行）
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("POST", "/hp/verify", strings.NewReader(`{"t":1}`))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)
	body := w2.Body.String()
	if w2.Code != 200 || !strings.Contains(body, `"ok":false`) {
		t.Fatalf("验证提交应恒失败: %d %s", w2.Code, body)
	}
	if strings.Contains(body, `"ok":true`) {
		t.Fatal("验证接口不允许存在成功分支")
	}

	// 页面：必须内联采集且不含 success
	html := VerifyPageHTMLForTest("/admin")
	if !strings.Contains(html, "DOMContentLoaded") || !strings.Contains(html, "/hp/fp") {
		t.Fatal("验证页缺少前置采集")
	}
	if strings.Contains(html, "ok:true") || strings.Contains(html, "验证成功") {
		t.Fatal("验证页不应存在成功路径")
	}

	// 指纹落库
	var n int64
	models.DB.Model(&Fingerprint{}).Count(&n)
	if n == 0 {
		t.Fatal("指纹未落库")
	}
}

// TestForensicsZip 取证包含 manifest 与 SHA256
func TestForensicsZip(t *testing.T) {
	setupHP(t)
	withLevel(t, LevelMedium)
	c, _ := newCtx("/.env")
	if _, err := Record(Current(), c, allDecoys()["/.env"]); err != nil {
		t.Fatal(err)
	}
	var sess Session
	if err := models.DB.First(&sess).Error; err != nil {
		t.Fatal(err)
	}
	b, err := BuildForensicsZip(sess.ID, 42)
	if err != nil {
		t.Fatalf("生成取证包失败: %v", err)
	}
	if b.SHA256 == "" || b.Bytes == 0 || len(b.Files) < 3 {
		t.Fatalf("取证包不完整: %+v", b)
	}
	d, err := os.ReadFile(b.Path)
	if err != nil || len(d) < 100 {
		t.Fatalf("取证包不可读: %v", err)
	}
	_ = os.Remove(b.Path)
}

// TestSuggestAndApplyRules 规则反哺：只提炼明确恶意特征，写入后即时生效
func TestSuggestAndApplyRules(t *testing.T) {
	setupHP(t)
	withLevel(t, LevelHigh)
	c, _ := newCtx("/.env")
	c.Request.Header.Set("User-Agent", "sqlmap/1.7 (http://sqlmap.org)")
	if _, err := Record(Current(), c, allDecoys()["/.env"]); err != nil {
		t.Fatal(err)
	}
	list := SuggestRules(10)
	if len(list) == 0 {
		t.Fatal("应提炼出候选规则")
	}
	var kw []string
	for _, s := range list {
		if s.Kind == "ua" {
			kw = append(kw, s.Keyword)
		}
	}
	if len(kw) == 0 {
		t.Fatalf("应识别 UA 特征: %+v", list)
	}
	added, err := ApplyRules(kw)
	if err != nil {
		t.Fatalf("反哺失败: %v", err)
	}
	if len(added) == 0 {
		t.Fatal("应写入至少一条规则")
	}
	cfg := models.LoadAPIRateLimitConfig()
	joined := strings.Join(cfg.BotBlockKeywords, ",")
	if !strings.Contains(strings.ToLower(joined), "sqlmap") {
		t.Fatalf("规则未写入限流配置: %s", joined)
	}
	// 重复反哺应幂等
	again, _ := ApplyRules(kw)
	if len(again) != 0 {
		t.Fatalf("重复反哺应跳过: %v", again)
	}
}

// TestDecoyServingEndToEnd 端到端：off → 404；medium → 返回仿真内容并记事件
func TestDecoyServingEndToEnd(t *testing.T) {
	setupHP(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r)
	RegisterShadow(r)
	RegisterVerify(r)

	// off：与普通 404 无差别
	withLevel0 := func() {
		_, _ = SaveSetting(Setting{Enabled: false, Level: LevelOff})
		ResetCache()
	}
	withLevel0()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/.env", nil))
	if w.Code != 404 {
		t.Fatalf("off 态应 404: %d", w.Code)
	}

	// medium：.env 返回仿真内容且含金丝雀
	withLevel(t, LevelMedium)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest("GET", "/.env", nil))
	if w2.Code != 200 || !strings.Contains(w2.Body.String(), "CANARY=") {
		t.Fatalf("medium 态应返回诱饵: %d %s", w2.Code, w2.Body.String()[:min(80, w2.Body.Len())])
	}
	// low 不覆盖 .git
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, httptest.NewRequest("GET", "/phpinfo.php", nil))
	if w3.Code != 404 {
		t.Fatalf("medium 不覆盖 phpinfo: %d", w3.Code)
	}

	// 命中已记录
	var n int64
	models.DB.Model(&Event{}).Where("kind = ?", EventDecoyHit).Count(&n)
	if n == 0 {
		t.Fatal("诱饵命中未记录")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
