package sentiment

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"nvs-server/handlers/admincommon"
	"nvs-server/models"
	"nvs-server/sentiment/store"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupCampChangeDB 用内存 SQLite 注入 store.DB 与 models.DB，并造好「超级管理员 + 普通管理员」两个账号。
func setupCampChangeDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:campchange_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate sentiment: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("migrate user: %v", err)
	}
	store.SetDB(db)
	models.DB = db
	// 超级管理员：id 最小的管理员（站长）
	db.Create(&models.User{ID: 1, Username: "boss", Email: "boss@test.local", Role: admincommon.RoleAdmin})
	// 普通管理员：id 更大，不是超级管理员
	db.Create(&models.User{ID: 2, Username: "mgr", Email: "mgr@test.local", Role: admincommon.RoleAdmin})
	return db
}

// newCampChangeRouter 注册阵营标记相关路由，并用 X-Test-UID 头模拟登录用户。
func newCampChangeRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if h := c.GetHeader("X-Test-UID"); h != "" {
			if id, err := strconv.ParseUint(h, 10, 64); err == nil {
				c.Set("userID", uint(id))
			}
		}
		c.Next()
	})
	g := r.Group("/api/admin/sentiment")
	g.POST("/camp-marks", CreateCampMark)
	g.GET("/camp-marks", ListCampMarks)
	g.POST("/camp-marks/:id/request-update", RequestUpdateCampMark)
	g.POST("/camp-marks/:id/request-delete", RequestDeleteCampMark)
	g.PUT("/camp-marks/:id", UpdateCampMark)
	g.DELETE("/camp-marks/:id", DeleteCampMark)
	g.GET("/delete-requests", ListDeleteRequests)
	g.POST("/delete-requests/:id/approve", ApproveDeleteRequest)
	g.POST("/delete-requests/:id/reject", RejectDeleteRequest)
	return r
}

func doReq(t *testing.T, r *gin.Engine, method, path, uid string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if uid != "" {
		req.Header.Set("X-Test-UID", uid)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestCampMarkChangeFlow(t *testing.T) {
	db := setupCampChangeDB(t)
	r := newCampChangeRouter()

	// 1) 超级管理员新增一条阵营标记
	w := doReq(t, r, "POST", "/api/admin/sentiment/camp-marks", "1", map[string]interface{}{
		"camp_id": "A", "camp_label": "支持方", "lang_format": "fandom", "confidence": 60,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create camp mark: code=%d body=%s", w.Code, w.Body.String())
	}
	var created struct {
		Data store.CampMark `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	markID := created.Data.ID
	if markID == 0 {
		t.Fatalf("mark id is 0")
	}

	// 2) 普通管理员（uid=2）不能直接修改（应 403）
	w = doReq(t, r, "PUT", "/api/admin/sentiment/camp-marks/"+strconv.Itoa(int(markID)), "2", map[string]interface{}{
		"camp_id": "A", "camp_label": "支持方改", "lang_format": "political", "confidence": 80,
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("regular admin PUT should be 403, got %d", w.Code)
	}

	// 3) 普通管理员不能直接删除（应 403）
	w = doReq(t, r, "DELETE", "/api/admin/sentiment/camp-marks/"+strconv.Itoa(int(markID)), "2", nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("regular admin DELETE should be 403, got %d", w.Code)
	}

	// 4) 普通管理员提交「修改申请」
	w = doReq(t, r, "POST", "/api/admin/sentiment/camp-marks/"+strconv.Itoa(int(markID))+"/request-update", "2", map[string]interface{}{
		"camp_id": "A", "camp_label": "支持方改", "lang_format": "political", "confidence": 80, "reason": "语言格式判断错了",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("request-update: code=%d body=%s", w.Code, w.Body.String())
	}
	var reqCreated struct {
		Data struct {
			Pending   bool `json:"pending"`
			RequestID uint `json:"request_id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &reqCreated)
	if !reqCreated.Data.Pending || reqCreated.Data.RequestID == 0 {
		t.Fatalf("request-update should return pending request: %s", w.Body.String())
	}
	// 修改申请未批准前，标记不应变化
	var m store.CampMark
	db.First(&m, markID)
	if m.CampLabel == "支持方改" {
		t.Fatalf("camp mark changed before approval")
	}

	// 5) 超级管理员批准修改申请 → 标记应更新
	w = doReq(t, r, "POST", "/api/admin/sentiment/delete-requests/"+strconv.Itoa(int(reqCreated.Data.RequestID))+"/approve", "1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("approve update: code=%d body=%s", w.Code, w.Body.String())
	}
	db.First(&m, markID)
	if m.CampLabel != "支持方改" || m.LangFormat != "political" || m.Confidence != 80 {
		t.Fatalf("camp mark not updated after approve: %+v", m)
	}

	// 6) 普通管理员提交「删除申请」
	w = doReq(t, r, "POST", "/api/admin/sentiment/camp-marks/"+strconv.Itoa(int(markID))+"/request-delete", "2", map[string]interface{}{"reason": "不再需要"})
	if w.Code != http.StatusOK {
		t.Fatalf("request-delete: code=%d body=%s", w.Code, w.Body.String())
	}
	var delReq struct {
		Data struct {
			RequestID uint `json:"request_id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &delReq)
	// 删除申请未批准前，标记仍存在
	if err := db.First(&m, markID).Error; err != nil {
		t.Fatalf("camp mark deleted before approval: %v", err)
	}

	// 7) 超级管理员驳回删除申请（验证驳回路径）
	w = doReq(t, r, "POST", "/api/admin/sentiment/delete-requests/"+strconv.Itoa(int(delReq.Data.RequestID))+"/reject", "1", map[string]string{"reason": "保留观察"})
	if w.Code != http.StatusOK {
		t.Fatalf("reject delete: code=%d body=%s", w.Code, w.Body.String())
	}
	// 驳回后标记仍存在
	if err := db.First(&m, markID).Error; err != nil {
		t.Fatalf("camp mark wrongly deleted after reject: %v", err)
	}

	// 8) 再次提交删除申请并批准 → 标记被真正删除
	w = doReq(t, r, "POST", "/api/admin/sentiment/camp-marks/"+strconv.Itoa(int(markID))+"/request-delete", "2", map[string]interface{}{"reason": "确认删除"})
	_ = json.Unmarshal(w.Body.Bytes(), &delReq)
	w = doReq(t, r, "POST", "/api/admin/sentiment/delete-requests/"+strconv.Itoa(int(delReq.Data.RequestID))+"/approve", "1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("approve delete: code=%d body=%s", w.Code, w.Body.String())
	}
	if err := db.First(&m, markID).Error; err == nil {
		t.Fatalf("camp mark should be deleted after approve")
	}
}
