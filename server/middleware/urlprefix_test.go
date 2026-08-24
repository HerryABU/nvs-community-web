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

// TestInjectPrefixIntoHTML 校验 index.html 注入 window.__NVS_PREFIX__ 且正确转义。
func TestInjectPrefixIntoHTML(t *testing.T) {
	html := []byte("<html><head><title>t</title></head><body>x</body></html>")

	// 空前缀：原样返回
	if string(InjectPrefixIntoHTML(html, "")) != string(html) {
		t.Fatal("空前缀应原样返回")
	}

	// 正常前缀
	out := string(InjectPrefixIntoHTML(html, "/go-nvs"))
	if !strings.Contains(out, "window.__NVS_PREFIX__='/go-nvs';") {
		t.Fatalf("应注入前缀变量: %s", out)
	}
	if !strings.Contains(out, "</head>") {
		t.Fatal("注入不应破坏 head 结构")
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
