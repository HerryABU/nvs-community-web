// Package testutil 提供各 handlers 子包测试共用的夹具。
//
// 背景：拆包前这些辅助函数定义在处理函数根包的某个 _test.go 里，同包测试可直接调用；
// 按域拆成多个子包后，_test.go 内的标识符无法跨包可见，故统一抽到本包并导出。
// 本包只依赖 models / middleware / utils，不依赖任何 handlers 子包，避免循环引用。
package testutil

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"nvs-server/utils/infra"
	"testing"

	"nvs-server/middleware"
	"nvs-server/models"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// SetupHandlerTestDB 建立内存 sqlite 测试库并替换全局 models.DB。
func SetupHandlerTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{Logger: infra.NewDBLogger()})
	if err != nil {
		t.Fatalf("打开内存测试库失败: %v", err)
	}
	if err := models.AutoMigrateAll(db); err != nil {
		t.Fatalf("迁移测试库失败: %v", err)
	}
	models.DB = db
	t.Cleanup(func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
}

// AsUser 构造带登录身份的 gin context（c 用于调用 handler，w 用于读取响应）。
func AsUser(role string, userID uint) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(middleware.CtxUserID, userID)
	c.Set(middleware.CtxUserRole, role)
	c.Request = httptest.NewRequest("POST", "/", bytes.NewBufferString("{}"))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, w
}

// SetJSONBody 为 context 设置 JSON 请求体。
func SetJSONBody(c *gin.Context, payload interface{}) {
	b, _ := json.Marshal(payload)
	c.Request = httptest.NewRequest("POST", "/", bytes.NewBuffer(b))
	c.Request.Header.Set("Content-Type", "application/json")
}

// SeedTestUser 造一个用户（回收站/封禁流程需要 operator 名字查询；EnsureAdmin 需 DB 角色校验）。
func SeedTestUser(t *testing.T, id uint, username, role string) {
	t.Helper()
	u := models.User{}
	u.ID = id
	u.Username = username
	u.Nickname = username
	u.Email = username + "@test.local"
	u.Role = role
	if err := models.DB.Create(&u).Error; err != nil {
		t.Fatalf("创建测试用户失败: %v", err)
	}
}

// MustJSON 读取响应体并断言 code==0。
func MustJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("响应状态应为 200，实际 %d：%s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应非 JSON: %v", err)
	}
	if code, _ := resp["code"].(float64); code != 0 {
		t.Fatalf("业务 code 应为 0，实际 %v：%s", resp["code"], w.Body.String())
	}
	return resp
}
