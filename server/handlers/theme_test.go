package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nvs-server/models"

	"github.com/gin-gonic/gin"
)

// setThemeJSONBody 为 context 设置 PUT JSON 请求体（主题保存均为 PUT 语义）
func setThemeJSONBody(c *gin.Context, payload map[string]interface{}) {
	b, _ := json.Marshal(payload)
	c.Request = httptest.NewRequest("PUT", "/", bytes.NewBuffer(b))
	c.Request.Header.Set("Content-Type", "application/json")
}

// setupThemeTestDB 复用公共测试库初始化，并补建主题表。
// 说明：SiteTheme/UserTheme 需主 agent 注册进 models.AllModels 后才会被
// AutoMigrateAll 覆盖，测试侧在此显式补迁移，保证本文件可独立运行。
func setupThemeTestDB(t *testing.T) {
	t.Helper()
	setupHandlerTestDB(t)
	if err := models.DB.AutoMigrate(&models.SiteTheme{}, &models.UserTheme{}); err != nil {
		t.Fatalf("迁移主题表失败: %v", err)
	}
}

// TestGetSiteThemeEmpty 未配置站点主题时公开接口返回空串
func TestGetSiteThemeEmpty(t *testing.T) {
	setupThemeTestDB(t)
	c, w := asUser("admin", 1)
	GetSiteTheme(c)
	data := mustJSON(t, w)["data"].(map[string]interface{})
	if css, _ := data["css"].(string); css != "" {
		t.Fatalf("未配置站点主题应返回空串，实际 %q", css)
	}
}

// TestUpdateMyThemeSanitizes 个人主题保存必须经服务端消毒，且返回 removed 列表
func TestUpdateMyThemeSanitizes(t *testing.T) {
	setupThemeTestDB(t)
	seedTestUser(t, 1, "alice", "reader")

	c, w := asUser("reader", 1)
	setThemeJSONBody(c, map[string]interface{}{
		"css": ".a { color: red; background: url(https://evil.example/x.png); behavior: url(x.htc); }",
	})
	UpdateMyTheme(c)
	data := mustJSON(t, w)["data"].(map[string]interface{})
	css, _ := data["css"].(string)
	if strings.Contains(css, "url(") || strings.Contains(css, "behavior") {
		t.Fatalf("保存时必须经服务端消毒，实际 %q", css)
	}
	if !strings.Contains(css, "color: red") {
		t.Fatalf("合法声明应保留，实际 %q", css)
	}
	removed, ok := data["removed"].([]interface{})
	if !ok || len(removed) == 0 {
		t.Fatalf("应返回被移除规则列表，实际 %v", data["removed"])
	}

	// 落库后再读，应与保存的消毒结果一致（跨设备全局生效的数据基础）
	c2, w2 := asUser("reader", 1)
	GetMyTheme(c2)
	data2 := mustJSON(t, w2)["data"].(map[string]interface{})
	if css2, _ := data2["css"].(string); css2 != css {
		t.Fatalf("读回应与保存的消毒结果一致，实际 %q vs %q", css2, css)
	}
}

// TestUpdateMyThemeRequiresLogin 未登录保存个人主题 → 401
func TestUpdateMyThemeRequiresLogin(t *testing.T) {
	setupThemeTestDB(t)
	c, w := asUser("", 0)
	setThemeJSONBody(c, map[string]interface{}{"css": ".a { color: red; }"})
	UpdateMyTheme(c)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("未登录应返回 401，实际 %d: %s", w.Code, w.Body.String())
	}
}

// TestUpdateSiteThemeForbiddenForNonAdmin 非管理员保存站点主题 → 403 且不落库
func TestUpdateSiteThemeForbiddenForNonAdmin(t *testing.T) {
	setupThemeTestDB(t)
	seedTestUser(t, 1, "alice", "reader")

	c, w := asUser("reader", 1)
	setThemeJSONBody(c, map[string]interface{}{"css": ".a { color: red; }"})
	UpdateSiteTheme(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("非管理员应返回 403，实际 %d: %s", w.Code, w.Body.String())
	}
	var st models.SiteTheme
	if err := models.DB.First(&st, 1).Error; err == nil {
		t.Fatal("越权写入不应产生站点主题记录")
	}
}

// TestUpdateSiteThemeByAdmin 管理员保存站点主题：消毒后落库，公开可读
func TestUpdateSiteThemeByAdmin(t *testing.T) {
	setupThemeTestDB(t)
	seedTestUser(t, 2, "root", "admin")

	c, w := asUser("admin", 2)
	setThemeJSONBody(c, map[string]interface{}{
		"css": "body { background-color: #102030; } .a { behavior: url(x.htc); }",
	})
	UpdateSiteTheme(c)
	data := mustJSON(t, w)["data"].(map[string]interface{})
	css, _ := data["css"].(string)
	if !strings.Contains(css, "background-color: #102030") {
		t.Fatalf("合法声明应保留，实际 %q", css)
	}
	if strings.Contains(css, "behavior") {
		t.Fatalf("危险声明应被消毒，实际 %q", css)
	}
	if removed, _ := data["removed"].([]interface{}); len(removed) == 0 {
		t.Fatal("应返回被移除规则列表")
	}

	// 公开读取（普通用户身份）应拿到管理员配置的站点主题
	c2, w2 := asUser("reader", 3)
	c2.Request = httptest.NewRequest("GET", "/", nil)
	GetSiteTheme(c2)
	data2 := mustJSON(t, w2)["data"].(map[string]interface{})
	if css2, _ := data2["css"].(string); css2 != css {
		t.Fatalf("公开读回应拿到站点主题，实际 %q vs %q", css2, css)
	}
}

// TestUpdateSiteThemeBadRequest 非法 JSON 体 → 400
func TestUpdateSiteThemeBadRequest(t *testing.T) {
	setupThemeTestDB(t)
	seedTestUser(t, 2, "root", "admin")

	c, w := asUser("admin", 2)
	c.Request = httptest.NewRequest("PUT", "/", bytes.NewBufferString("not-json"))
	c.Request.Header.Set("Content-Type", "application/json")
	UpdateSiteTheme(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("非法 JSON 应返回 400，实际 %d", w.Code)
	}
}
