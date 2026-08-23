package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nvs-server/middleware"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// ─────────────────────────────────────────────────────────────
// 测试辅助：内存 sqlite + gin context 模拟登录态
// ─────────────────────────────────────────────────────────────

func setupHandlerTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{Logger: utils.NewDBLogger()})
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

// asUser 构造带登录身份的 gin context（c 用于调用 handler，w 用于读取响应）
func asUser(role string, userID uint) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(middleware.CtxUserID, userID)
	c.Set(middleware.CtxUserRole, role)
	c.Request = httptest.NewRequest("POST", "/", bytes.NewBufferString("{}"))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, w
}

// setJSONBody 为 context 设置 JSON 请求体
func setJSONBody(c *gin.Context, payload interface{}) {
	b, _ := json.Marshal(payload)
	c.Request = httptest.NewRequest("POST", "/", bytes.NewBuffer(b))
	c.Request.Header.Set("Content-Type", "application/json")
}

// seedTestUser 造一个用户（回收站/封禁流程需要 operator 名字查询；ensureAdmin 需 DB 角色校验）
func seedTestUser(t *testing.T, id uint, username, role string) {
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

// mustJSON 读取响应体并断言 code==0
func mustJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
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

// ─────────────────────────────────────────────────────────────
// 软删除区分：用户自删(user) vs 管理员封禁(admin)
// ─────────────────────────────────────────────────────────────

// TestDeletedByDistinction 用户删除小说 → deleted_by=user；管理员封禁 → deleted_by=admin（含级联章节）
func TestDeletedByDistinction(t *testing.T) {
	setupHandlerTestDB(t)
	seedTestUser(t, 1, "author1", "author")
	seedTestUser(t, 2, "adminuser", "admin")

	// 作者建小说
	c, _ := asUser("author", 1)
	setJSONBody(c, map[string]interface{}{"title": "测试书", "category": "玄幻"})
	CreateNovel(c)
	var novel models.Novel
	models.DB.First(&novel)
	if novel.ID == 0 {
		t.Fatal("创建小说失败")
	}

	// 1) 作者手动删（DeleteNovel 用户侧链路）+ 回收站语义标记 deleted_by=user
	dc, _ := asUser("author", 1)
	dc.Params = gin.Params{gin.Param{Key: "id", Value: "1"}}
	dc.Request = httptest.NewRequest("DELETE", "/", nil)
	DeleteNovel(dc)
	models.DB.Unscoped().Model(&models.Novel{}).Where("id = ?", novel.ID).Update("deleted_by", "user")
	var afterUser models.Novel
	models.DB.Unscoped().First(&afterUser, novel.ID)
	if afterUser.DeletedBy != "user" {
		t.Fatalf("用户自删后 deleted_by 应为 user，实际 %q", afterUser.DeletedBy)
	}

	// 2) 管理员封禁另一本书（softDeleteContent 全链路：留证 + 软删 + deleted_by=admin + 级联章节）
	c2, _ := asUser("author", 1)
	setJSONBody(c2, map[string]interface{}{"title": "测试书2"})
	CreateNovel(c2)
	var novel2 models.Novel
	models.DB.Where("title = ?", "测试书2").First(&novel2)
	models.DB.Create(&models.Chapter{NovelID: novel2.ID, ChapterNumber: 1, Title: "第一章", ContentPath: "x", Status: "published"})

	banCtx, _ := asUser("admin", 2)
	banCtx.Request = httptest.NewRequest("POST", "/", nil)
	if err := softDeleteContent(banCtx, "novel", novel2.ID, "涉黄违规", "admin", "ban"); err != nil {
		t.Fatalf("管理员封禁失败: %v", err)
	}
	var afterBan models.Novel
	models.DB.Unscoped().First(&afterBan, novel2.ID)
	if afterBan.DeletedBy != "admin" || afterBan.BanReason == "" {
		t.Fatalf("管理员封禁后 deleted_by=admin 且 ban_reason 非空，实际 deleted_by=%q ban_reason=%q", afterBan.DeletedBy, afterBan.BanReason)
	}
	var chap models.Chapter
	models.DB.Unscoped().First(&chap, "novel_id = ?", novel2.ID)
	if chap.DeletedBy != "admin" {
		t.Fatalf("管理员封禁作品后章节 deleted_by 应为 admin，实际 %q", chap.DeletedBy)
	}
	// 留证记录应存在
	var sdc int64
	models.DB.Table("softdel_novels").Where("original_id = ?", novel2.ID).Count(&sdc)
	if sdc != 1 {
		t.Errorf("管理员封禁应写入 softdel_novels 留证，实际 %d", sdc)
	}
}

// TestRestoreAuthorRecycle 回收站恢复权限：
// 用户只能恢复自己删除(deleted_by=user)的内容；管理员封禁的拒绝；他人内容拒绝。
func TestRestoreAuthorRecycle(t *testing.T) {
	setupHandlerTestDB(t)
	seedTestUser(t, 1, "author1", "author")
	seedTestUser(t, 2, "other", "author")

	// 作者1 的小说：user 删除
	n1 := models.Novel{AuthorID: 1, Title: "自删书"}
	models.DB.Create(&n1)
	models.DB.Delete(&n1)
	models.DB.Unscoped().Model(&models.Novel{}).Where("id = ?", n1.ID).Update("deleted_by", "user")

	// 作者1 的小说：admin 封禁
	n2 := models.Novel{AuthorID: 1, Title: "封禁书"}
	models.DB.Create(&n2)
	models.DB.Delete(&n2)
	models.DB.Unscoped().Model(&models.Novel{}).Where("id = ?", n2.ID).Update("deleted_by", "admin")

	// 作者2 的小说：user 删除（他人内容）
	n3 := models.Novel{AuthorID: 2, Title: "他人书"}
	models.DB.Create(&n3)
	models.DB.Delete(&n3)
	models.DB.Unscoped().Model(&models.Novel{}).Where("id = ?", n3.ID).Update("deleted_by", "user")

	restore := func(id uint) *httptest.ResponseRecorder {
		c, w := asUser("author", 1)
		setJSONBody(c, map[string]interface{}{"type": "novel", "id": id})
		RestoreAuthorRecycle(c)
		return w
	}

	// 1) 恢复自己的 user 删除 → 成功
	w := restore(n1.ID)
	if w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte("已恢复")) {
		t.Errorf("恢复自己的自删内容应成功: %d %s", w.Code, w.Body.String())
	}
	var chk models.Novel
	if err := models.DB.First(&chk, n1.ID).Error; err != nil {
		t.Errorf("恢复后小说应可见（deleted_at 已清）: %v", err)
	}

	// 2) 尝试恢复 admin 封禁 → 拒绝
	w = restore(n2.ID)
	if !bytes.Contains(w.Body.Bytes(), []byte("管理员封禁")) {
		t.Errorf("恢复被封禁内容应被拒绝并提示管理员: %s", w.Body.String())
	}

	// 3) 恢复他人内容 → 拒绝
	w = restore(n3.ID)
	if !bytes.Contains(w.Body.Bytes(), []byte("只能恢复自己的")) {
		t.Errorf("恢复他人内容应被拒绝: %s", w.Body.String())
	}
}

// TestAdminListRecycle 管理员回收站监管：能看到用户自删内容并带作者名
func TestAdminListRecycle(t *testing.T) {
	setupHandlerTestDB(t)
	seedTestUser(t, 1, "author1", "author")
	seedTestUser(t, 9, "superadmin", "admin")

	n := models.Novel{AuthorID: 1, Title: "回收站监管书"}
	models.DB.Create(&n)
	models.DB.Delete(&n)
	models.DB.Unscoped().Model(&models.Novel{}).Where("id = ?", n.ID).Update("deleted_by", "user")

	c, w := asUser("admin", 9)
	c.Request = httptest.NewRequest("GET", "/", nil)
	AdminListRecycle(c)

	resp := mustJSON(t, w)
	items, _ := resp["data"].([]interface{})
	found := false
	for _, it := range items {
		m := it.(map[string]interface{})
		if m["type"] == "novel" && m["id"] == float64(n.ID) {
			found = true
			if m["author_name"] != "author1" {
				t.Errorf("回收站监管应带作者名，实际 %v", m["author_name"])
			}
		}
	}
	if !found {
		t.Errorf("管理员回收站监管应能看到用户自删小说")
	}
}

// ─────────────────────────────────────────────────────────────
// 博客分类管理 API（一/二级）+ 博客点赞 API
// ─────────────────────────────────────────────────────────────

func TestBlogCategoryAdminAPI(t *testing.T) {
	setupHandlerTestDB(t)
	seedTestUser(t, 1, "admin1", "admin")

	// 创建一级
	c, w := asUser("admin", 1)
	setJSONBody(c, map[string]interface{}{"name": "技术"})
	AdminCreateBlogCategory(c)
	resp := mustJSON(t, w)
	catID := uint(resp["data"].(map[string]interface{})["id"].(float64))

	// 同名重复 → 拒绝
	c, w = asUser("admin", 1)
	setJSONBody(c, map[string]interface{}{"name": "技术"})
	AdminCreateBlogCategory(c)
	if !bytes.Contains(w.Body.Bytes(), []byte("同名")) {
		t.Errorf("同名分类应被拒绝: %s", w.Body.String())
	}

	// 二级挂一级 → 成功
	c, w = asUser("admin", 1)
	setJSONBody(c, map[string]interface{}{"name": "Go", "parent_id": catID})
	AdminCreateBlogCategory(c)
	mustJSON(t, w)
	// 记录二级分类 id（catID+1）
	subID := catID + 1

	// 二级挂二级 → 拒绝
	c, w = asUser("admin", 1)
	setJSONBody(c, map[string]interface{}{"name": "非法", "parent_id": subID})
	AdminCreateBlogCategory(c)
	if !bytes.Contains(w.Body.Bytes(), []byte("一级")) {
		t.Errorf("二级挂二级应被拒绝: %s", w.Body.String())
	}

	// 删除一级（有子分类）→ 拒绝
	c, w = asUser("admin", 1)
	c.Params = gin.Params{gin.Param{Key: "id", Value: "1"}}
	c.Request = httptest.NewRequest("DELETE", "/", nil)
	AdminDeleteBlogCategory(c)
	if !bytes.Contains(w.Body.Bytes(), []byte("子分类")) {
		t.Errorf("一级有子分类应拒绝删除: %s", w.Body.String())
	}

	// 删除二级 → 成功
	c, w = asUser("admin", 1)
	c.Params = gin.Params{gin.Param{Key: "id", Value: "2"}}
	c.Request = httptest.NewRequest("DELETE", "/", nil)
	AdminDeleteBlogCategory(c)
	mustJSON(t, w)
}

// TestBlogCategoryFilterAndLike 博客带分类创建 → 列表一级过滤含二级 → 点赞 toggle → 点赞数返回
func TestBlogCategoryFilterAndLike(t *testing.T) {
	setupHandlerTestDB(t)
	seedTestUser(t, 1, "author1", "author")

	// 建分类：技术(1) / Go(2)
	models.DB.Create(&models.BlogCategory{Name: "技术"})
	models.DB.Create(&models.BlogCategory{Name: "Go", ParentID: 1})

	// 作者创建博客挂二级分类
	c, w := asUser("author", 1)
	setJSONBody(c, map[string]interface{}{"title": "Go 并发博客", "content": "内容", "category_id": 2})
	CreateBlog(c)
	resp := mustJSON(t, w)
	if resp["data"].(map[string]interface{})["category"] != "技术/Go" {
		t.Errorf("返回的 category 应为 技术/Go，实际 %v", resp["data"].(map[string]interface{})["category"])
	}

	// 列表按一级分类过滤 → 应包含该博客（一级含二级）
	c, w = asUser("author", 1)
	c.Request = httptest.NewRequest("GET", "/?category_id=1", nil)
	ListPublicBlogs(c)
	resp = mustJSON(t, w)
	list := resp["data"].(map[string]interface{})["list"].([]interface{})
	if len(list) != 1 {
		t.Errorf("按一级分类过滤应返回 1 篇（含二级），实际 %d", len(list))
	}

	// 点赞 → count=1 liked=true
	c, w = asUser("author", 1)
	c.Params = gin.Params{gin.Param{Key: "id", Value: "1"}}
	c.Request = httptest.NewRequest("POST", "/", nil)
	ToggleBlogLike(c)
	resp = mustJSON(t, w)
	d := resp["data"].(map[string]interface{})
	if d["count"] != float64(1) || d["liked"] != true {
		t.Errorf("点赞后 count=1 liked=true，实际 %v", d)
	}

	// 再点取消 → count=0 liked=false
	c, w = asUser("author", 1)
	c.Params = gin.Params{gin.Param{Key: "id", Value: "1"}}
	c.Request = httptest.NewRequest("POST", "/", nil)
	ToggleBlogLike(c)
	resp = mustJSON(t, w)
	d = resp["data"].(map[string]interface{})
	if d["count"] != float64(0) || d["liked"] != false {
		t.Errorf("取消点赞后 count=0 liked=false，实际 %v", d)
	}
}
