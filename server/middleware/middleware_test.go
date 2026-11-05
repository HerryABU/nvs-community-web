package middleware

import (
	"crypto/tls"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ===== (原 access_log_test.go) =====

// effectiveHost 决定「访问域名」这一列记的是不是用户真正看到的地址。
// cpolar / nginx 转发时 Host 常被改写成上游，所以必须优先信 X-Forwarded-Host。
func TestEffectiveHost(t *testing.T) {
	cases := []struct {
		name    string
		host    string
		headers map[string]string
		tls     bool
		wantH   string
		wantS   string
	}{
		{
			name: "直连：用 Host",
			host: "192.168.1.3:8000", wantH: "192.168.1.3:8000", wantS: "http",
		},
		{
			name:    "nginx 透传域名（优先于被改写的 Host）",
			host:    "127.0.0.1:8000",
			headers: map[string]string{"X-Forwarded-Host": "nvs.example.com"},
			wantH:   "nvs.example.com", wantS: "http",
		},
		{
			name: "cpolar 域名 + https",
			host: "127.0.0.1:8000",
			headers: map[string]string{
				"X-Forwarded-Host":  "3f2a-1-2-3-4.ngrok-free.app",
				"X-Forwarded-Proto": "https",
			},
			wantH: "3f2a-1-2-3-4.ngrok-free.app", wantS: "https",
		},
		{
			name:    "多级代理逗号列表取第一个",
			host:    "127.0.0.1:8000",
			headers: map[string]string{"X-Forwarded-Host": "a.com, b.com, c.com"},
			wantH:   "a.com", wantS: "http",
		},
		{
			name:    "只有 X-Real-Host 时也能取到",
			host:    "127.0.0.1:8000",
			headers: map[string]string{"X-Real-Host": "real.example.com"},
			wantH:   "real.example.com", wantS: "http",
		},
		{
			name:  "大小写与结尾点归一",
			host:  "NVS.Example.COM.",
			wantH: "nvs.example.com", wantS: "http",
		},
		{
			name:    "个别反代把 scheme 写进 host",
			host:    "127.0.0.1:8000",
			headers: map[string]string{"X-Forwarded-Host": "https://tunnel.example.com"},
			wantH:   "tunnel.example.com", wantS: "http",
		},
		{
			name: "TLS 直连识别为 https",
			host: "nvs.example.com", tls: true,
			wantH: "nvs.example.com", wantS: "https",
		},
		{
			name:    "X-Forwarded-Ssl: on",
			host:    "nvs.example.com",
			headers: map[string]string{"X-Forwarded-Ssl": "on"},
			wantH:   "nvs.example.com", wantS: "https",
		},
	}
	for _, c := range cases {
		r := &http.Request{Host: c.host, Header: http.Header{}}
		for k, v := range c.headers {
			r.Header.Set(k, v)
		}
		if c.tls {
			r.TLS = &tls.ConnectionState{}
		}
		gotH, gotS := effectiveHost(r)
		if gotH != c.wantH || gotS != c.wantS {
			t.Errorf("%s: 得到 (%q, %q)，期望 (%q, %q)", c.name, gotH, gotS, c.wantH, c.wantS)
		}
	}
}

// 超长 Host 必须截断到列宽，否则写库会失败/被静默截断。
func TestEffectiveHost_Truncates(t *testing.T) {
	r := &http.Request{Host: strings.Repeat("a", 300), Header: http.Header{}}
	h, _ := effectiveHost(r)
	if len(h) != 128 {
		t.Fatalf("期望截断到 128 字节，实际 %d", len(h))
	}
}

// nil 请求不能 panic（中间件理论上拿不到 nil，但不该靠这个假设）。
func TestEffectiveHost_Nil(t *testing.T) {
	h, s := effectiveHost(nil)
	if h != "" || s != "http" {
		t.Fatalf("nil 请求应返回空 host + http，实际 (%q, %q)", h, s)
	}
}

// ===== 合并自 csrf_test.go =====

// TestCSRFGuard 校验 CSRF 防护行为（安全审计）：
// 登录态写操作必须携带 X-Requested-With 头；匿名请求与 GET 不受影响。
func TestCSRFGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)

	setup := func() *gin.Engine {
		r := gin.New()
		r.Use(CSRFGuard())
		r.POST("/api/echo", func(c *gin.Context) { c.String(200, "ok") })
		r.GET("/api/ping", func(c *gin.Context) { c.String(200, "pong") })
		return r
	}

	t.Run("带登录Cookie的POST缺少CSRF头应拒绝", func(t *testing.T) {
		r := setup()
		req := httptest.NewRequest("POST", "/api/echo", nil)
		req.AddCookie(&http.Cookie{Name: "token", Value: "sometoken"})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("期望 403，实际 %d", w.Code)
		}
	})

	t.Run("带登录Cookie的POST携带CSRF头应放行", func(t *testing.T) {
		r := setup()
		req := httptest.NewRequest("POST", "/api/echo", nil)
		req.AddCookie(&http.Cookie{Name: "token", Value: "sometoken"})
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("期望 200，实际 %d", w.Code)
		}
	})

	t.Run("匿名POST不受限", func(t *testing.T) {
		r := setup()
		req := httptest.NewRequest("POST", "/api/echo", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("匿名 POST 应放行，实际 %d", w.Code)
		}
	})

	t.Run("GET不受限", func(t *testing.T) {
		r := setup()
		req := httptest.NewRequest("GET", "/api/ping", nil)
		req.AddCookie(&http.Cookie{Name: "token", Value: "sometoken"})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("GET 应放行，实际 %d", w.Code)
		}
	})
}

// ===== 合并自 governance_test.go =====

// 白名单必须「只放该放的」：任何一条漏网都等于给警方开了 admin 全量。
func TestPoliceGovernanceWhitelist(t *testing.T) {
	allowed := []struct{ method, path string }{
		{"GET", "/api/admin/moderation/records"},
		{"GET", "/api/admin/moderation/stats"},
		{"POST", "/api/admin/moderation/records/12/decide"},
		{"GET", "/api/admin/reports"},
		{"POST", "/api/admin/reports/3/handle"},
		{"GET", "/api/admin/novels"},
		{"GET", "/api/admin/novels/7/chapters"},
		{"POST", "/api/admin/softdel"},
		{"POST", "/api/admin/softdel/restore"},
		{"GET", "/api/admin/hotwords"},
	}
	for _, c := range allowed {
		if !policeAllowed(c.method, c.path) {
			t.Fatalf("应放行却被拒: %s %s", c.method, c.path)
		}
	}

	denied := []struct{ method, path string }{
		// 合规与安全：警方一律不可见
		{"GET", "/api/admin/compliance"},
		{"GET", "/api/admin/compliance/bundle"},
		{"GET", "/api/admin/realname/records"},
		{"GET", "/api/admin/redact/config"},
		{"GET", "/api/admin/audit-logs"},
		// 密钥 / 支付 / 角色提权
		{"GET", "/api/admin/apikeys"},
		{"POST", "/api/admin/users/1/promote-police"},
		{"GET", "/api/admin/users"},
		{"GET", "/api/admin/payment/orders"},
		// AI 配置与舆情配置（站长决策，不给警方改）
		{"PUT", "/api/admin/sentiment/config"},
		{"GET", "/api/admin/ai/models"},
		{"PUT", "/api/admin/ai/safety/config"},
		// 方法不匹配：同一路径 GET 放行，DELETE 不该放行
		{"DELETE", "/api/admin/reports/3/handle"},
		// 前缀混淆应被拒（正则锚定，不接受子路径拼接）
		{"GET", "/api/admin/moderation/records/export"},
		{"GET", "/api/admin/novels/7/chapters/1"},
	}
	for _, c := range denied {
		if policeAllowed(c.method, c.path) {
			t.Fatalf("不该放行却放行了: %s %s", c.method, c.path)
		}
	}
}

// 尾部斜杠不能成为绕过手段。
func TestPoliceGovernanceTrailingSlash(t *testing.T) {
	if !policeAllowed("GET", "/api/admin/reports/") {
		t.Fatalf("尾部斜杠应归一化后放行")
	}
	// 双重斜杠不做归一化 → 应当被拒（宁可 403，也不放宽匹配）
	if policeAllowed("GET", "/api/admin//reports") {
		t.Fatalf("双斜杠不应被放行")
	}
}

// ===== 合并自 urlprefix_test.go =====

// TestStripURLPrefixHandler 校验站点路径前缀剥离（反向代理挂载场景）：
// /go-nvs/login → /login、/go-nvs/api/health → /api/health、/go-nvs/ → /、
// 未命中前缀的路径保持不变、未配置前缀时 no-op。
func TestStripURLPrefixHandler(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Path", r.URL.Path)
		w.WriteHeader(200)
		_, _ = w.Write([]byte(r.URL.Path))
	})

	check := func(t *testing.T, h http.Handler, path, want string) {
		t.Helper()
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("%s 期望 200 实际 %d", path, w.Code)
		}
		if w.Body.String() != want {
			t.Fatalf("%s 期望剥离为 %q 实际 %q", path, want, w.Body.String())
		}
	}

	t.Run("配置前缀 /go-nvs", func(t *testing.T) {
		h := StripURLPrefixHandler("/go-nvs", inner)
		check(t, h, "/go-nvs/login", "/login")
		check(t, h, "/go-nvs/api/health", "/api/health")
		check(t, h, "/go-nvs/assets/x.js", "/assets/x.js")
		check(t, h, "/go-nvs/", "/")
		check(t, h, "/login", "/login") // 无前缀访问兼容
	})

	t.Run("未配置前缀 no-op", func(t *testing.T) {
		h := StripURLPrefixHandler("", inner)
		check(t, h, "/login", "/login")
		check(t, h, "/go-nvs/login", "/go-nvs/login")
	})
}

// TestInjectPrefixIntoHTML 校验 index.html 注入 <base href> 与 window.__NVS_PREFIX__ 且正确转义。
// 核心：非空前缀时资源引用 ./assets/xxx 必须被改写为 {prefix}/assets/xxx（不依赖浏览器 base 解析）。
func TestInjectPrefixIntoHTML(t *testing.T) {
	html := []byte(`<html><head><title>t</title>
<link rel="stylesheet" crossorigin href="./assets/index-abc.css">
<script type="module" crossorigin src="./assets/index-abc.js"></script>
<link rel="modulepreload" href="./assets/vendor-xyz.js">
<link rel="icon" href="./favicon.ico">
</head><body>x</body></html>`)

	// 正常前缀：资源引用改写 + base + __NVS_PREFIX__
	out := string(InjectPrefixIntoHTML(html, "/go-nvs"))
	// 资源引用必须带前缀（主方案）
	for _, expect := range []string{
		`href="/go-nvs/assets/index-abc.css"`,
		`src="/go-nvs/assets/index-abc.js"`,
		`href="/go-nvs/assets/vendor-xyz.js"`,
		`href="/go-nvs/favicon.ico"`,
	} {
		if !strings.Contains(out, expect) {
			t.Fatalf("资源引用应改写为带前缀 %q: %s", expect, out)
		}
	}
	// 不应残留裸 ./assets/ 引用
	if strings.Contains(out, `"./assets/`) || strings.Contains(out, `'./assets/`) {
		t.Fatalf("不应残留相对 ./assets/ 引用: %s", out)
	}
	if !strings.Contains(out, `<base href="/go-nvs/">`) {
		t.Fatalf("应注入 base href（兜底）: %s", out)
	}
	if !strings.Contains(out, "window.__NVS_PREFIX__='/go-nvs';") {
		t.Fatalf("应注入前缀变量: %s", out)
	}
	// base 标签必须位于 <head> 内部且在其他内容之前
	if strings.Index(out, "<head>") > strings.Index(out, `<base href="/go-nvs/">`) {
		t.Fatal("base 标签应位于 <head> 之后紧跟")
	}

	// 空前缀：不改写资源引用，注入 base href="/"，不注入前缀变量
	plain := string(InjectPrefixIntoHTML(html, ""))
	if !strings.Contains(plain, `href="./assets/index-abc.css"`) {
		t.Fatalf("空前缀应保持 Vite 相对资源引用原样: %s", plain)
	}
	if !strings.Contains(plain, `<base href="/">`) {
		t.Fatalf("空前缀应注入 base href=\"/\": %s", plain)
	}
	if strings.Contains(plain, "__NVS_PREFIX__='/'") {
		t.Fatalf("空前缀不应注入非空前缀变量: %s", plain)
	}

	// 恶意前缀转义（单引号/反斜杠/</script>）
	evil := string(InjectPrefixIntoHTML(html, "/x';alert(1)//</script><script>alert(2)</script>"))
	if !strings.Contains(evil, `\'`) {
		t.Fatal("前缀中的单引号应被转义")
	}
	if !strings.Contains(evil, `<\/script>`) {
		t.Fatal("前缀中的 </script> 应被转义")
	}
}
