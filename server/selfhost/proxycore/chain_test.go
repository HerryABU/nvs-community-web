package proxycore

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestChainPrefixStrip 一条路径里连续出现「中转 / 目标 / 服务自身路由」时，
// 应逐段剥离并交给最后一个服务，不能拼出 /target/target/...。
func TestChainPrefixStrip(t *testing.T) {
	var seen string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL.Path
		_, _ = w.Write([]byte("ok:" + r.URL.Path))
	}))
	defer backend.Close()

	m := newTestManager("",
		activeRoute("relay", "http://127.0.0.1:1"), // 中转自身不会被真正访问
		activeRoute("target", backend.URL),
	)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.NoRoute(func(c *gin.Context) {
		if !m.Matchable(c.Request.URL.Path, c.Request.Host) {
			c.String(http.StatusNotFound, "nf")
			return
		}
		m.Handle(c)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/relay/target/app", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("链式请求应 200，got %d body=%s", w.Code, w.Body.String())
	}
	if seen != "/app" {
		t.Fatalf("后端应收到 /app，实际 %q（说明服务名没有被逐段剥掉）", seen)
	}
}

// TestChainGzipRewrite 中间任意一跳返回 gzip 文本时，必须解压→改写→重新压缩回包，
// 且 <base>/shim 的前缀是完整的外层链，而不是最后一个服务名。
func TestChainGzipRewrite(t *testing.T) {
	html := `<!doctype html><html><head></head><body><script src="/assets/app.js"></script><a href="/api/x">x</a></body></html>`
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, _ = zw.Write([]byte(html))
	_ = zw.Close()
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Length", strconvItoa(buf.Len()))
		_, _ = w.Write(buf.Bytes())
	}))
	defer backend.Close()

	m := newTestManager("", activeRoute("relay", "http://127.0.0.1:1"), activeRoute("target", backend.URL))
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.NoRoute(func(c *gin.Context) {
		if !m.Matchable(c.Request.URL.Path, c.Request.Host) {
			c.String(http.StatusNotFound, "nf")
			return
		}
		m.Handle(c)
	})
	req := httptest.NewRequest("GET", "/relay/target/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("gzip 链式请求应 200，got %d", w.Code)
	}
	body := w.Body.Bytes()
	// httptest 不会自动解压：先确认仍是 gzip，再解压校验改写结果。
	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("改写后应保持 gzip，实际 %q", w.Header().Get("Content-Encoding"))
	}
	gr, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("回包不是合法 gzip: %v", err)
	}
	decoded, err := io.ReadAll(gr)
	if err != nil {
		t.Fatal(err)
	}
	text := string(decoded)
	if !strings.Contains(text, `<base href="/relay/target/">`) {
		t.Errorf("<base> 未使用完整链路前缀: %s", text[:minLen(text, 200)])
	}
	if !strings.Contains(text, `P="/relay/target"`) {
		t.Errorf("前缀 shim 未使用完整链路前缀: %s", text[:minLen(text, 200)])
	}
	if strings.Contains(text, "/target/target") {
		t.Error("出现重复服务名，链式剥离失败")
	}
	if w.Header().Get("ETag") == "" {
		t.Error("改写后应重算 ETag")
	}
}

func strconvItoa(n int) string {
	if n == 0 {
		return "0"
	}
	out := make([]byte, 0, 12)
	for n > 0 {
		out = append([]byte{byte('0' + n%10)}, out...)
		n /= 10
	}
	return string(out)
}

func minLen(s string, n int) int {
	if len(s) < n {
		return len(s)
	}
	return n
}

// TestThreeLevelChain 三层服务名也应逐段剥离，最终只把 /x 交给第三个服务。
func TestThreeLevelChain(t *testing.T) {
	var seen string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()
	m := newTestManager("",
		activeRoute("a", "http://127.0.0.1:1"),
		activeRoute("b", "http://127.0.0.1:2"),
		activeRoute("c", backend.URL),
	)
	prefix, chain, e, rest := m.resolveChain("/a/b/c/x")
	if prefix != "c" || chain != "a/b/c" || e == nil || rest != "/x" {
		t.Fatalf("三层链解析错误: prefix=%q chain=%q rest=%q", prefix, chain, rest)
	}
	_ = seen
}

// TestRealSelfHopGzip 真实跨 HTTP 自指链：第一跳 alias → 本平台 /target/，
// 第二跳 target → gzip 后端；最终回包仍应正确解压改写、重压并带上完整外层前缀。
func TestRealSelfHopGzip(t *testing.T) {
	html := `<html><head></head><body><script src="/assets/app.js"></script></body></html>`
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	_, _ = zw.Write([]byte(html))
	_ = zw.Close()
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write(gz.Bytes())
	}))
	defer backend.Close()

	gin.SetMode(gin.TestMode)
	var m *Manager
	r := gin.New()
	r.NoRoute(func(c *gin.Context) {
		if !m.Matchable(c.Request.URL.Path, c.Request.Host) {
			c.String(http.StatusNotFound, "nf")
			return
		}
		m.Handle(c)
	})
	gw := httptest.NewServer(r)
	defer gw.Close()
	m = newTestManager("",
		activeRoute("alias", gw.URL+"/target/"),
		activeRoute("target", backend.URL),
	)

	client := &http.Client{Transport: &http.Transport{DisableCompression: true}}
	resp, err := client.Get(gw.URL + "/alias/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("自指 gzip 链应 200，got %d", resp.StatusCode)
	}
	if resp.Header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("回包应保持 gzip，实际 %q", resp.Header.Get("Content-Encoding"))
	}
	gr, err := gzip.NewReader(resp.Body)
	if err != nil {
		t.Fatalf("回包 gzip 损坏: %v", err)
	}
	decoded, _ := io.ReadAll(gr)
	text := string(decoded)
	if !strings.Contains(text, `P="/alias/target"`) {
		t.Errorf("跨跳后应保留完整前缀链 /alias/target: %s", text[:minLen(text, 120)])
	}
	if strings.Contains(text, "/target/target") {
		t.Error("跨跳后出现重复服务名")
	}
}

// TestCrossProcessChain 模拟 NVS → 另一个网关进程 → 后端 的跨 HTTP 嵌套。
// 内层网关会自己注入 shim/base（局部前缀），外层必须用完整链更新，
// 且请求侧链头必须跨进程传递，最终页面的 <base> 与 shim P 都是 /outer/inner。
func TestCrossProcessChain(t *testing.T) {
	html := `<!doctype html><html><head></head><body><script src="/assets/app.js"></script></body></html>`
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, html)
	}))
	defer backend.Close()

	// 内层网关（如 GWh）：一条 inner 路由指向后端，自己会改写并注入 shim。
	innerMgr := newTestManager("", activeRoute("inner", backend.URL))
	innerGin := gin.New()
	innerGin.NoRoute(func(c *gin.Context) {
		if !innerMgr.Matchable(c.Request.URL.Path, c.Request.Host) {
			c.String(http.StatusNotFound, "nf")
			return
		}
		innerMgr.Handle(c)
	})
	innerSrv := httptest.NewServer(innerGin)
	defer innerSrv.Close()

	// 外层（NVS）：outer 路由指向内层网关进程。
	outerMgr := newTestManager("", activeRoute("outer", innerSrv.URL))
	outerGin := gin.New()
	outerGin.NoRoute(func(c *gin.Context) {
		if !outerMgr.Matchable(c.Request.URL.Path, c.Request.Host) {
			c.String(http.StatusNotFound, "nf")
			return
		}
		outerMgr.Handle(c)
	})

	req := httptest.NewRequest("GET", "/outer/inner/", nil)
	w := httptest.NewRecorder()
	outerGin.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("跨进程嵌套应 200，got %d body=%s", w.Code, w.Body.String())
	}
	text := w.Body.String()
	if !strings.Contains(text, `<base href="/outer/inner/">`) {
		t.Errorf("<base> 应为完整链 /outer/inner/，实际: %s", text[:minLen(text, 200)])
	}
	if !strings.Contains(text, `P="/outer/inner"`) {
		t.Errorf("shim 应为完整链 /outer/inner，实际: %s", text[:minLen(text, 200)])
	}
	if n := strings.Count(text, "data-gateway-prefix-shim"); n != 1 {
		t.Errorf("shim 应恰好一个，got %d", n)
	}
}

// TestProxyCookieHat 普通路径代理的 GWh 同款 cookie 帽子：
// 请求侧只脱本服务帽并丢弃外域，响应侧加帽 + 收紧 Path + 保留属性。
func TestProxyCookieHat(t *testing.T) {
	var gotCookie string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		w.Header().Add("Set-Cookie", "session=abc; Path=/; HttpOnly; SameSite=Lax")
		w.Header().Add("Set-Cookie", "theme=dark; Path=/")
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "ok")
	}))
	defer backend.Close()

	m := newTestManager("", Route{Prefix: "app", Target: backend.URL, Timeout: 5, Status: StatusActive, CookieHat: "app__", DropForeign: true})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.NoRoute(func(c *gin.Context) {
		if !m.Matchable(c.Request.URL.Path, c.Request.Host) {
			c.String(http.StatusNotFound, "nf")
			return
		}
		m.Handle(c)
	})
	req := httptest.NewRequest("GET", "/app/x", nil)
	req.Header.Set("Cookie", "app__session=1; other__x=2; plain=3")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("应 200，got %d", w.Code)
	}
	if gotCookie != "session=1" {
		t.Errorf("请求侧应只脱本服务帽并丢外域，got %q", gotCookie)
	}
	sc := w.Header().Values("Set-Cookie")
	if len(sc) != 2 {
		t.Fatalf("Set-Cookie 条数应 2，got %d: %v", len(sc), sc)
	}
	if !strings.Contains(sc[0], "app__session=abc") || !strings.Contains(sc[0], "Path=/app") {
		t.Errorf("响应侧应加帽并收紧 Path: %s", sc[0])
	}
	if !strings.Contains(sc[0], "HttpOnly") || !strings.Contains(sc[0], "SameSite=Lax") {
		t.Errorf("应保留属性: %s", sc[0])
	}
}
