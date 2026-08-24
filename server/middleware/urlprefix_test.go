package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
