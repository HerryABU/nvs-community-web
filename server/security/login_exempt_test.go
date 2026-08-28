package security

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nvs-server/config"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// 验证：已登录浏览器用户免限流；访客仍走匿名通道（本次未改动其逻辑）。
func TestIsLoggedInBrowser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	config.JWTSecret = "test-secret-login-exempt"

	tok, err := utils.GenerateToken(1, "a@b.c", "reader")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	mk := func(cookie *http.Cookie) *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/api/novels", nil)
		if cookie != nil {
			c.Request.AddCookie(cookie)
		}
		return c
	}

	if !isLoggedInBrowser(mk(&http.Cookie{Name: "token", Value: tok})) {
		t.Error("期望：有效 token cookie → 已登录")
	}
	if isLoggedInBrowser(mk(nil)) {
		t.Error("期望：无 cookie（访客）→ 未登录")
	}
	if isLoggedInBrowser(mk(&http.Cookie{Name: "token", Value: "garbage.token.value"})) {
		t.Error("期望：无效 token → 未登录")
	}
	if isLoggedInBrowser(mk(&http.Cookie{Name: "token", Value: ""})) {
		t.Error("期望：空 token → 未登录")
	}
}

// 验证：匿名配额被打满时，已登录用户仍被放行，且 CtxKeySkipIP 被置位（路由级限流同步跳过）。
func TestAPIKeyGuardBypassesLoggedInUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	config.JWTSecret = "test-secret-login-exempt"
	tok, _ := utils.GenerateToken(1, "a@b.c", "reader")

	// 预置：把该来源的匿名/爬虫配额打满（模拟访客已超限）
	rateLimitStore["anon:"] = &rateLimiterEntry{count: 999999, windowStart: time.Now()}
	rateLimitStore["bot:"] = &rateLimiterEntry{count: 999999, windowStart: time.Now()}

	var skipIP bool
	r := gin.New()
	r.Use(APIKeyGuard())
	// 再挂一个极严的路由级限流（1 次/60 秒）——若 CtxKeySkipIP 生效则应被跳过
	r.GET("/api/novels", RateLimit(1, 60), func(c *gin.Context) {
		skipIP = c.GetBool(CtxKeySkipIP)
		c.Status(200)
	})

	req := httptest.NewRequest("GET", "/api/novels", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: tok})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("已登录用户应免限流（期望 200），实际 %d", w.Code)
	}
	if !skipIP {
		t.Error("已登录用户应置 CtxKeySkipIP=true，使路由级 RateLimit 一并跳过")
	}

	// 连续多次：已登录用户无论多少请求都不应被 429
	for i := 0; i < 50; i++ {
		req := httptest.NewRequest("GET", "/api/novels", nil)
		req.AddCookie(&http.Cookie{Name: "token", Value: tok})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code == 429 {
			t.Fatalf("已登录用户第 %d 次请求被限流（429），免限流未生效", i+1)
		}
	}
}
