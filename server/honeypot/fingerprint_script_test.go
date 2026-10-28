package honeypot

import (
	"net/http/httptest"
	"strings"
	"testing"

	"nvs-server/models"

	"github.com/gin-gonic/gin"
)

// TestDecoyFingerprintCoverage HTML 类诱饵必须带指纹采集脚本。
// 背景：采集脚本原先只内联在验证页，只有走到风控卡口的攻击者才有指纹；
// 而「浏览型攻击者」（直接访问 /admin.php、/wp-login.php 等 HTML 诱饵，
// 这是最常见的路径枚举/扫描行为）完全采不到任何浏览器指纹 —— 覆盖率缺口。
// 纯文本类诱饵（.env/.git/backup）不执行 JS，不应注入。
func TestDecoyFingerprintCoverage(t *testing.T) {
	setupHP(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r)
	withLevel(t, LevelHigh) // 全量诱饵

	for _, p := range []string{"/admin.php", "/wp-login.php", "/phpinfo.php"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", p, nil))
		if w.Code != 200 {
			t.Fatalf("%s 应命中诱饵: %d", p, w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, "/hp/fp") {
			t.Errorf("%s 缺少指纹采集脚本（指纹覆盖率缺口）", p)
		}
		if !strings.Contains(body, "DOMContentLoaded") {
			t.Errorf("%s 的采集应在 DOMContentLoaded 即采", p)
		}
		if !strings.Contains(body, "canvas") || !strings.Contains(body, "webgl") {
			t.Errorf("%s 的采集脚本应含 canvas/webgl 指纹项", p)
		}
	}

	for _, p := range []string{"/.env", "/.git/config", "/backup/"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", p, nil))
		if strings.Contains(w.Body.String(), "/hp/fp") {
			t.Errorf("%s 是纯文本诱饵，不应注入采集脚本", p)
		}
	}
}

// TestInjectFingerprintScript 注入器行为：HTML 注入 / 纯文本不注入 / 幂等 / 不污染入参
func TestInjectFingerprintScript(t *testing.T) {
	// HTML 诱饵：注入，且锚点前插入
	html := Decoy{ID: "admin", Path: "/admin.php", Content: "<html><body><h1>x</h1></body></html>",
		Headers: map[string]string{"Content-Type": "text/html; charset=utf-8"}}
	out := withFingerprintInjection(html)
	if !strings.Contains(out.Content, "/hp/fp") {
		t.Fatal("HTML 诱饵应注入采集脚本")
	}
	if !strings.Contains(out.Content, "</h1></script>") && !strings.Contains(out.Content, "<script>") {
		t.Fatal("注入脚本位置异常")
	}

	// 幂等：已注入的再注入不重复
	out2 := withFingerprintInjection(out)
	if strings.Count(out2.Content, "/hp/fp") != strings.Count(out.Content, "/hp/fp") {
		t.Fatal("重复注入应幂等")
	}

	// 纯文本诱饵：不注入
	txt := Decoy{ID: "env", Path: "/.env", Content: "CANARY=abc\n",
		Headers: map[string]string{"Content-Type": "text/plain; charset=utf-8"}}
	if strings.Contains(withFingerprintInjection(txt).Content, "/hp/fp") {
		t.Fatal("纯文本诱饵不应注入采集脚本")
	}

	// 自定义诱饵未声明 Content-Type 但内容是 HTML：补类型 + 注入，且不修改入参 map
	custom := Decoy{ID: "custom", Path: "/x.html", Content: "<!doctype html><html><body>hi</body></html>"}
	outC := withFingerprintInjection(custom)
	if !strings.Contains(outC.Content, "/hp/fp") {
		t.Fatal("自定义 HTML 诱饵应注入采集脚本")
	}
	if outC.Headers["Content-Type"] == "" {
		t.Fatal("未声明类型时应补 text/html")
	}
	if custom.Headers != nil {
		t.Fatal("不应修改入参的 Headers map（会污染注册时共享的 map）")
	}
}

// TestFingerprintEventKind 指纹上报记为独立事件类型，便于后台区分来源
func TestFingerprintEventKind(t *testing.T) {
	setupHP(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterVerify(r)
	withLevel(t, LevelMedium)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/hp/fp", strings.NewReader(`{"canvas":"ab12","webgl":"cd34"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	var n int64
	models.DB.Model(&Event{}).Where("kind = ?", EventFingerprint).Count(&n)
	if n == 0 {
		t.Fatalf("指纹事件类型应为 %s", EventFingerprint)
	}
	var fp int64
	models.DB.Model(&Fingerprint{}).Count(&fp)
	if fp == 0 {
		t.Fatal("指纹未落库")
	}
}
