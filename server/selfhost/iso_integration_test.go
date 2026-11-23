package selfhost

// NVS 接入层集成测试：selfhost_iso_services 表 → config.Table → 完整隔离引擎。
// 验证三种去头模式在真实 HTTP 转发下的语义与 GWh 一致。

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nvs-server/selfhost/isolation/config"
	"nvs-server/selfhost/isolation/engine"
	"nvs-server/selfhost/target"
)

func newIsoBackend(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Set-Cookie", "session=abc; Path=/")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"path":"`+r.URL.Path+`","host":"`+r.Host+`"}`)
	}))
}

func isoTableFor(t *testing.T, name, targetURL, mode string, extra func(*config.Service)) *config.Table {
	t.Helper()
	m, ok := config.ParseMode(mode)
	if !ok {
		t.Fatalf("非法模式 %q", mode)
	}
	p, err := target.ParseBackend(targetURL)
	if err != nil {
		t.Fatalf("后端地址解析失败: %v", err)
	}
	svc := &config.Service{
		Name: name, Prefix: "/" + name, Target: targetURL, Backend: p,
		Scheme: p.Scheme, Host: p.Host, Root: p.Root,
		Mode: m, Timeout: 5, HostMode: config.HostModeKeep, Enabled: true,
		CookieHat: name + "__", MaxRewriteBytes: config.DefaultMaxRewriteBytes,
	}
	if extra != nil {
		extra(svc)
	}
	return &config.Table{Version: 1, Services: []*config.Service{svc}, LoadedAt: time.Now()}
}

// TestIsoIntegrationThreeModes 三种模式在完整引擎下的真实转发：
// strip 剥前缀、pass-through 保留前缀、isolated 剥前缀但不改响应体。
func TestIsoIntegrationThreeModes(t *testing.T) {
	backend := newIsoBackend(t)
	defer backend.Close()

	cases := []struct {
		mode       string
		path       string
		wantPath   string
		wantCookie string
	}{
		{config.ModePassThrough.Canonical(), "/passthru/api/x", "/passthru/api/x", "Path=/passthru"},
		{config.ModeStrip.Canonical(), "/strip/api/x", "/api/x", "Path=/strip"},
		{config.ModeIsolated.Canonical(), "/isolated/api/x", "/api/x", "Path=/isolated"},
	}
	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			e := engine.New(engine.Options{})
			e.Reload(isoTableFor(t, strings.Split(tc.path, "/")[1], backend.URL, tc.mode, nil), "test")
			if !e.Matches(tc.path) {
				t.Fatalf("引擎未匹配路径 %q", tc.path)
			}
			req := httptest.NewRequest("GET", tc.path, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("转发失败: status=%d body=%s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), `"path":"`+tc.wantPath+`"`) {
				t.Errorf("后端收到的路径不符: %s", rec.Body.String())
			}
			cookie := rec.Header().Get("Set-Cookie")
			if !strings.Contains(cookie, tc.wantCookie) {
				t.Errorf("Cookie 路径未收紧: %s", cookie)
			}
		})
	}
}

// TestIsoIntegrationBodyRewrite strip 模式下 HTML 绝对链接改写生效（响应体真改），
// isolated 模式响应体保持原样（头层修正而已）。
func TestIsoIntegrationBodyRewrite(t *testing.T) {
	html := `<html><a href="/api/x">go</a></html>`
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, html)
	}))
	defer backend.Close()

	withRewrite := func(s *config.Service) { s.HTMLRewrite = true }
	e := engine.New(engine.Options{})
	e.Reload(isoTableFor(t, "rw", backend.URL, config.ModeStrip.Canonical(), withRewrite), "test")
	req := httptest.NewRequest("GET", "/rw/page", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("strip 改写转发失败: %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "/rw/api/x") {
		t.Errorf("strip 模式应改写绝对链接为带前缀: %s", rec.Body.String())
	}

	e2 := engine.New(engine.Options{})
	e2.Reload(isoTableFor(t, "norw", backend.URL, config.ModeIsolated.Canonical(), withRewrite), "test")
	req2 := httptest.NewRequest("GET", "/norw/page", nil)
	rec2 := httptest.NewRecorder()
	e2.ServeHTTP(rec2, req2)
	if rec2.Body.String() != html {
		t.Errorf("isolated 模式响应体应原样: %s", rec2.Body.String())
	}
}

// TestIsoEngineTableFromDB 验证 isoEngineTable 把数据库模型字段完整映射到引擎配置。
func TestIsoEngineTableFromDB(t *testing.T) {
	list := []IsoService{{
		Name: "svc", Target: "http://127.0.0.1:9000/app", Mode: "strip",
		Timeout: 8, Enabled: true, CookieHat: "svc__", HTMLRewrite: true,
		KnownAbsPrefix: "/a, /b", HostMode: "rewrite", NoFullCompat: true,
		MaxRewriteBytes: 1024, DropForeign: true, PassThroughFix: true,
	}}
	tbl := isoEngineTable(list)
	if len(tbl.Services) != 1 {
		t.Fatalf("服务数错误: %d", len(tbl.Services))
	}
	s := tbl.Services[0]
	if s.Prefix != "/svc" || s.Mode != config.ModeStrip || s.HostMode != config.HostModeRewrite {
		t.Errorf("配置映射错误: %+v", s)
	}
	if len(s.KnownAbsPrefix) != 2 || s.KnownAbsPrefix[0] != "/a" {
		t.Errorf("已知绝对前缀映射错误: %+v", s.KnownAbsPrefix)
	}
	if s.Root != "/app" || s.Host != "127.0.0.1:9000" {
		t.Errorf("后端解析错误: root=%q host=%q", s.Root, s.Host)
	}
	if !s.HTMLRewrite || !s.DropForeign || s.MaxRewriteBytes != 1024 {
		t.Errorf("高级字段映射错误: %+v", s)
	}
}
