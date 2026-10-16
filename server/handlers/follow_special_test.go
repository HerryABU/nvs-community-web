package handlers

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"nvs-server/middleware"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// ─────────────────────────────────────────────────────────────
// 特别关注（F14）测试：独立命名的内存库，避免与其它测试共享内存库互相污染
// ─────────────────────────────────────────────────────────────

func setupSpecialTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:follow_special_test?mode=memory&cache=shared"), &gorm.Config{Logger: utils.NewDBLogger()})
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

// asUserWithQuery 构造带登录身份 + 查询串的 gin context（GET 类 handler 用）
func asUserWithQuery(userID uint, rawQuery string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(middleware.CtxUserID, userID)
	c.Set(middleware.CtxUserRole, "reader")
	c.Request = httptest.NewRequest("GET", "/?"+rawQuery, nil)
	return c, w
}

// toggleSpecial 调用 ToggleSpecialFollow（target 为用户 ID）
func toggleSpecial(t *testing.T, userID, target uint, special bool) *httptest.ResponseRecorder {
	t.Helper()
	c, w := asUser("reader", userID)
	c.Params = gin.Params{gin.Param{Key: "id", Value: strconv.FormatUint(uint64(target), 10)}}
	setJSONBody(c, map[string]interface{}{"special": special})
	ToggleSpecialFollow(c)
	return w
}

// TestToggleSpecialFollow 特别关注置位/取消/幂等 + 边界校验
func TestToggleSpecialFollow(t *testing.T) {
	setupSpecialTestDB(t)
	seedTestUser(t, 1, "me", "reader")
	seedTestUser(t, 2, "author2", "author")
	seedTestUser(t, 3, "author3", "author")

	// 1) 置位：未关注时自动建立关注关系并标记特别关注
	w := toggleSpecial(t, 1, 2, true)
	resp := mustJSON(t, w)
	if data, _ := resp["data"].(map[string]interface{}); data == nil || data["is_special"] != true {
		t.Fatalf("置位响应应带 is_special=true: %s", w.Body.String())
	}
	if !models.IsSpecialFollowing(1, 2) {
		t.Error("置位后 IsSpecialFollowing 应为 true")
	}
	if !models.IsFollowing(1, 2) {
		t.Error("置位特别关注时应自动建立普通关注")
	}

	// 2) 幂等：重复置位不报错且仍为特别关注
	w = toggleSpecial(t, 1, 2, true)
	mustJSON(t, w)
	if !models.IsSpecialFollowing(1, 2) {
		t.Error("重复置位后应仍为特别关注")
	}
	var followRows int64
	models.DB.Model(&models.Follow{}).Where("follower_id = ? AND followed_id = ?", 1, 2).Count(&followRows)
	if followRows != 1 {
		t.Errorf("重复置位不应产生重复关注行，实际 %d 行", followRows)
	}

	// 3) 取消：is_special 置 false 但保留普通关注行
	w = toggleSpecial(t, 1, 2, false)
	resp = mustJSON(t, w)
	if data, _ := resp["data"].(map[string]interface{}); data == nil || data["is_special"] != false {
		t.Fatalf("取消响应应带 is_special=false: %s", w.Body.String())
	}
	if models.IsSpecialFollowing(1, 2) {
		t.Error("取消后 IsSpecialFollowing 应为 false")
	}
	if !models.IsFollowing(1, 2) {
		t.Error("取消特别关注后普通关注应保留")
	}

	// 4) 边界：不能对自己操作
	c, w := asUser("reader", 1)
	c.Params = gin.Params{gin.Param{Key: "id", Value: "1"}}
	setJSONBody(c, map[string]interface{}{"special": true})
	ToggleSpecialFollow(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("对自己操作应返回 400，实际 %d: %s", w.Code, w.Body.String())
	}

	// 5) 边界：目标用户不存在 → 404
	c, w = asUser("reader", 1)
	c.Params = gin.Params{gin.Param{Key: "id", Value: "999"}}
	setJSONBody(c, map[string]interface{}{"special": true})
	ToggleSpecialFollow(c)
	if w.Code != http.StatusNotFound {
		t.Errorf("目标不存在应返回 404，实际 %d: %s", w.Code, w.Body.String())
	}

	// 6) 边界：id 非法 → 400
	c, w = asUser("reader", 1)
	c.Params = gin.Params{gin.Param{Key: "id", Value: "abc"}}
	setJSONBody(c, map[string]interface{}{"special": true})
	ToggleSpecialFollow(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("id 非法应返回 400，实际 %d: %s", w.Code, w.Body.String())
	}

	// 7) 边界：未关注时取消特别关注 = 无操作（不得顺手建出普通关注行）
	w = toggleSpecial(t, 1, 3, false)
	mustJSON(t, w)
	if models.IsFollowing(1, 3) {
		t.Error("未关注时取消特别关注不应创建普通关注行")
	}
	if models.IsSpecialFollowing(1, 3) {
		t.Error("未关注时不应存在特别关注关系")
	}
}

// TestListMySpecialFollows 特别关注列表：只含特别关注对象，不含普通关注
func TestListMySpecialFollows(t *testing.T) {
	setupSpecialTestDB(t)
	seedTestUser(t, 1, "me", "reader")
	seedTestUser(t, 2, "special2", "author")
	seedTestUser(t, 3, "special3", "author")
	seedTestUser(t, 4, "normal4", "author")

	// 2/3 特别关注，4 仅普通关注
	if err := models.SetSpecialFollow(1, 2, true); err != nil {
		t.Fatalf("设置特别关注失败: %v", err)
	}
	if err := models.SetSpecialFollow(1, 3, true); err != nil {
		t.Fatalf("设置特别关注失败: %v", err)
	}
	if err := models.CreateFollow(1, 4); err != nil {
		t.Fatalf("创建普通关注失败: %v", err)
	}

	c, w := asUserWithQuery(1, "page=1&size=20")
	ListMySpecialFollows(c)
	resp := mustJSON(t, w)
	data, _ := resp["data"].(map[string]interface{})
	if data == nil {
		t.Fatalf("响应缺少 data: %s", w.Body.String())
	}
	list, _ := data["list"].([]interface{})
	if len(list) != 2 {
		t.Fatalf("特别关注列表应含 2 人，实际 %d: %s", len(list), w.Body.String())
	}
	if total, _ := data["total"].(float64); int(total) != 2 {
		t.Errorf("total 应为 2，实际 %v", data["total"])
	}
	ids := map[float64]bool{}
	for _, it := range list {
		if m, ok := it.(map[string]interface{}); ok {
			if id, ok := m["id"].(float64); ok {
				ids[id] = true
			}
			if m["email"] != nil && m["email"] != "" {
				t.Errorf("列表不应回显邮箱: %v", m["email"])
			}
		}
	}
	if !ids[2] || !ids[3] || ids[4] {
		t.Errorf("列表应含用户 2/3 且不含普通关注用户 4，实际 %v", ids)
	}

	// 分页：size=1 时本页 1 条、total 仍为 2
	c, w = asUserWithQuery(1, "page=1&size=1")
	ListMySpecialFollows(c)
	resp = mustJSON(t, w)
	data, _ = resp["data"].(map[string]interface{})
	if list, _ := data["list"].([]interface{}); len(list) != 1 {
		t.Errorf("size=1 应返回 1 条，实际 %d", len(list))
	}
	if total, _ := data["total"].(float64); int(total) != 2 {
		t.Errorf("size=1 时 total 应仍为 2，实际 %v", data["total"])
	}
}

// TestGetSpecialFeed 动态流：三类内容聚合、时间倒序、非特别关注内容排除、URL 正确
func TestGetSpecialFeed(t *testing.T) {
	setupSpecialTestDB(t)
	seedTestUser(t, 1, "me", "reader")
	seedTestUser(t, 2, "star", "author")
	seedTestUser(t, 3, "stranger", "author")

	// 用户 1 特别关注用户 2；用户 3 未被关注
	if err := models.SetSpecialFollow(1, 2, true); err != nil {
		t.Fatalf("设置特别关注失败: %v", err)
	}

	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	// 特别关注用户的小说（最早）
	models.DB.Create(&models.Novel{AuthorID: 2, Title: "特别关注的小说", Summary: "小说摘要", Status: "published", CreatedAt: base})
	// 特别关注用户的博客（中间）
	models.DB.Create(&models.AuthorBlog{AuthorID: 2, Title: "特别关注的博客", Summary: "博客摘要", CreatedAt: base.Add(time.Hour)})
	// 特别关注用户的帖子（最新）
	models.DB.Create(&models.Thread{ForumID: 1, UserID: 2, Title: "特别关注的帖子", Content: "帖子内容", CreatedAt: base.Add(2 * time.Hour)})
	// 未关注用户的内容（应被排除）
	models.DB.Create(&models.Novel{AuthorID: 3, Title: "路人的小说", Status: "published", CreatedAt: base.Add(3 * time.Hour)})
	// 草稿小说（应被排除）
	models.DB.Create(&models.Novel{AuthorID: 2, Title: "草稿小说", Status: "draft", CreatedAt: base.Add(4 * time.Hour)})

	c, w := asUserWithQuery(1, "page=1&size=20")
	GetSpecialFeed(c)
	resp := mustJSON(t, w)
	data, _ := resp["data"].(map[string]interface{})
	if data == nil {
		t.Fatalf("响应缺少 data: %s", w.Body.String())
	}
	list, _ := data["list"].([]interface{})
	if len(list) != 3 {
		t.Fatalf("动态应聚合 3 条（小说/博客/帖子），实际 %d: %s", len(list), w.Body.String())
	}
	if total, _ := data["total"].(float64); int(total) != 3 {
		t.Errorf("total 应为 3，实际 %v", data["total"])
	}

	// 校验时间倒序：帖子 → 博客 → 小说
	wantOrder := []string{"thread", "blog", "novel"}
	for i, it := range list {
		m, _ := it.(map[string]interface{})
		if m["type"] != wantOrder[i] {
			t.Errorf("第 %d 条类型应为 %s，实际 %v", i, wantOrder[i], m["type"])
		}
		if m["user_name"] != "star" {
			t.Errorf("第 %d 条作者名应为 star，实际 %v", i, m["user_name"])
		}
	}
	// URL 校验（与前端路由对齐）
	if url, _ := list[0].(map[string]interface{})["url"].(string); url != "/thread/1" {
		t.Errorf("帖子 URL 应为 /thread/1，实际 %v", url)
	}
	if url, _ := list[1].(map[string]interface{})["url"].(string); url != "/author/2/blogs/1" {
		t.Errorf("博客 URL 应为 /author/2/blogs/1，实际 %v", url)
	}
	if url, _ := list[2].(map[string]interface{})["url"].(string); url != "/author/2/novel/1" {
		t.Errorf("小说 URL 应为 /author/2/novel/1，实际 %v", url)
	}

	// 分页：size=2 → 第 1 页取最新 2 条（帖子、博客）
	c, w = asUserWithQuery(1, "page=1&size=2")
	GetSpecialFeed(c)
	resp = mustJSON(t, w)
	data, _ = resp["data"].(map[string]interface{})
	list, _ = data["list"].([]interface{})
	if len(list) != 2 {
		t.Fatalf("size=2 应返回 2 条，实际 %d", len(list))
	}
	if list[0].(map[string]interface{})["type"] != "thread" || list[1].(map[string]interface{})["type"] != "blog" {
		t.Errorf("第 1 页应为 thread/blog，实际 %v/%v", list[0].(map[string]interface{})["type"], list[1].(map[string]interface{})["type"])
	}
	// 第 2 页剩 1 条（小说）
	c, w = asUserWithQuery(1, "page=2&size=2")
	GetSpecialFeed(c)
	resp = mustJSON(t, w)
	data, _ = resp["data"].(map[string]interface{})
	list, _ = data["list"].([]interface{})
	if len(list) != 1 || list[0].(map[string]interface{})["type"] != "novel" {
		t.Errorf("第 2 页应为 1 条 novel，实际 %v", list)
	}

	// 无特别关注 → 空列表
	c, w = asUserWithQuery(3, "page=1&size=20")
	GetSpecialFeed(c)
	resp = mustJSON(t, w)
	data, _ = resp["data"].(map[string]interface{})
	if list, _ := data["list"].([]interface{}); len(list) != 0 {
		t.Errorf("无特别关注应返回空列表，实际 %v", list)
	}
}
