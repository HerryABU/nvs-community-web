package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nvs-server/models"
	"nvs-server/security/slidecaptcha"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupLoginCaptchaTestDB 独立命名内存 SQLite 库（按名隔离，避免与其他测试共享缓存串数据），
// 并临时替换 models.DB。
func setupLoginCaptchaTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:login_captcha_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存测试库失败: %v", err)
	}
	if err := models.AutoMigrateAll(db); err != nil {
		t.Fatalf("迁移测试库失败: %v", err)
	}
	old := models.DB
	models.DB = db
	t.Cleanup(func() {
		models.DB = old
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

// solveChallenge 解出一次滑块挑战，返回服务端签发的一次性 pass token（测试用，循环试探 correctX）。
func solveChallenge(t *testing.T) string {
	t.Helper()
	id, _, _, err := slidecaptcha.Issue()
	if err != nil {
		t.Fatalf("签发滑块挑战失败: %v", err)
	}
	for x := 0; x <= 320; x++ {
		if token, ok := slidecaptcha.Verify(id, x); ok {
			return token
		}
	}
	t.Fatal("无法解出滑块挑战")
	return ""
}

// TestLoginCaptchaGate 验证：站长开启滑块验证码时，邮箱密码登录必须先过人机校验。
func TestLoginCaptchaGate(t *testing.T) {
	setupLoginCaptchaTestDB(t)
	if err := models.SetLoginMethods(models.LoginMethods{Email: true, Phone: true}); err != nil {
		t.Fatal(err)
	}
	if err := models.SetPlatformConfig("captcha_enabled", "true"); err != nil {
		t.Fatal(err)
	}

	loginBody := func(token string) []byte {
		b, _ := json.Marshal(LoginInput{Email: "someone@example.com", Password: "whatever", CaptchaToken: token})
		return b
	}
	doLogin := func(token string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody(token)))
		c.Request.Header.Set("Content-Type", "application/json")
		Login(c)
		return w
	}

	// 1) 未带 captcha_token → 403（滑块闸门拦截，且不应泄漏到账号查询）
	w := doLogin("")
	if w.Code != http.StatusForbidden {
		t.Fatalf("开启滑块后未带 token 应 403，实际 %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "滑块验证") {
		t.Errorf("应提示先完成滑块验证: %s", w.Body.String())
	}

	// 2) 非法 token → 403
	w = doLogin("9999999999.foo.ffffff")
	if w.Code != http.StatusForbidden {
		t.Fatalf("非法 token 应 403，实际 %d", w.Code)
	}

	// 3) 合法一次性 token → 越过滑块闸门（该邮箱不存在，应到达账号查询并返回 400 邮箱或密码错误，而非 403）
	goodToken := solveChallenge(t)
	w = doLogin(goodToken)
	if w.Code == http.StatusForbidden {
		t.Fatalf("合法 token 不应被滑块闸门拦截: %s", w.Body.String())
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("合法 token 后无该邮箱应返回 400，实际 %d: %s", w.Code, w.Body.String())
	}

	// 4) 同一 token 复用 → 403（一次性已消费）
	w = doLogin(goodToken)
	if w.Code != http.StatusForbidden {
		t.Errorf("pass token 一次性使用，复用应 403，实际 %d", w.Code)
	}
}

// TestLoginNoCaptchaWhenDisabled 验证：站长关闭滑块时，登录不要求 captcha_token（向后兼容）。
func TestLoginNoCaptchaWhenDisabled(t *testing.T) {
	setupLoginCaptchaTestDB(t)
	if err := models.SetLoginMethods(models.LoginMethods{Email: true, Phone: true}); err != nil {
		t.Fatal(err)
	}
	if err := models.SetPlatformConfig("captcha_enabled", "false"); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body, _ := json.Marshal(LoginInput{Email: "someone@example.com", Password: "whatever"})
	c.Request = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	Login(c)

	// 关闭滑块时不应被 403 拦截（无该邮箱 → 400）
	if w.Code == http.StatusForbidden {
		t.Fatalf("关闭滑块后不应要求滑块验证: %s", w.Body.String())
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("无该邮箱应返回 400，实际 %d: %s", w.Code, w.Body.String())
	}
}
