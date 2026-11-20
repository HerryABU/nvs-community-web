package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"nvs-server/maintenance"
)

func TestMaintenanceWriteGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(MaintenanceWriteGuard())
	r.POST("/api/feed", func(c *gin.Context) { c.Status(200) })
	r.GET("/api/feed", func(c *gin.Context) { c.Status(200) })
	r.POST("/api/admin/db/storage-mode", func(c *gin.Context) { c.Status(200) })

	// 未开启时正常放行
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/feed", nil))
	if w.Code != 200 {
		t.Fatalf("未开启维护却拦截了写入: %d", w.Code)
	}

	maintenance.Begin()
	defer maintenance.End()

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/feed", nil))
	if w.Code != http.StatusLocked || !contains(w.Body.String(), maintenanceMessage) {
		t.Fatalf("维护期写操作应 423 且提示维护: %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/feed", nil))
	if w.Code != 200 {
		t.Fatalf("读操作不应被拦截: %d", w.Code)
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/admin/db/storage-mode", nil))
	if w.Code != 200 {
		t.Fatalf("迁移控制接口必须可用: %d", w.Code)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
