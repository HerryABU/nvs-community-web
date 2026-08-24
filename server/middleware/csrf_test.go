package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

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
