package aiapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// 站点根 /models 返回 SPA 首页（200 + HTML）时，必须自动退到 /v1/models。
// 这是真实踩到的：https://api.hcnsec.cn/models → HTML，/v1/models → JSON。
func TestPullProviderModelsFallbackToV1(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/models":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, "<!doctype html><html><body>SPA</body></html>")
		case "/v1/models":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":[{"id":"deepseek-v4-flash"},{"id":"gpt-4o"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	items, err := PullProviderModels(srv.URL, "sk-test")
	if err != nil {
		t.Fatalf("应回退到 /v1/models 并成功：%v", err)
	}
	if len(items) != 2 {
		t.Fatalf("模型数 = %d", len(items))
	}
	if items[0].RawID != "deepseek-v4-flash" {
		t.Fatalf("首个模型 = %q", items[0].RawID)
	}
}

// 已经是 API 根时不重复拼 /v1
func TestPullProviderModelsDirectV1(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `{"data":[{"id":"m1"}]}`)
	}))
	defer srv.Close()

	items, err := PullProviderModels(srv.URL+"/v1", "")
	if err != nil {
		t.Fatalf("直连 /v1 应成功：%v", err)
	}
	if len(items) != 1 {
		t.Fatalf("模型数 = %d", len(items))
	}
}
