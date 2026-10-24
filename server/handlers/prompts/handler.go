package prompthandler

import (
	"strconv"
	"strings"

	"nvs-server/middleware"
	"nvs-server/models"
	"nvs-server/prompttpl"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== 场景化提示词模板：接口层 ====================
// 用户侧：按场景列出启用模板 + 变量渲染（编辑器「模板」下拉用）。
// 管理侧：CRUD + 版本历史/回滚 + 实时预览。

func requireAdmin(c *gin.Context) bool {
	user, _ := models.GetUserByID(middleware.UserID(c))
	if user == nil || user.Role != "admin" {
		utils.Forbidden(c, "需要管理员权限")
		c.Abort()
		return false
	}
	return true
}

// List GET /api/ai/templates?scene=novel_chapter&kind=chat
func List(c *gin.Context) {
	if middleware.UserID(c) == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	list, err := prompttpl.List(c.Query("scene"), c.Query("kind"), false)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{
		"list":       list,
		"categories": prompttpl.Categories(c.Query("scene")),
	})
}

// Render POST /api/ai/templates/:id/render { vars: {name: value} }
func Render(c *gin.Context) {
	if middleware.UserID(c) == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	t, err := prompttpl.Get(uint(id))
	if err != nil {
		utils.NotFound(c, err.Error())
		return
	}
	if !t.Enabled {
		utils.BadRequest(c, "该模板已停用")
		return
	}
	var req struct {
		Vars map[string]string `json:"vars"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Vars == nil {
		req.Vars = map[string]string{}
	}
	res := prompttpl.Render(t.Content, t.Vars, req.Vars)
	utils.Success(c, gin.H{
		"template_id": t.ID, "title": t.Title, "version": t.Version,
		"text": res.Text, "missing": res.Missing, "unused": res.Unused,
	})
}

// RenderContext POST /api/ai/templates/:id/render-context
// 自动注入作品上下文：作品名/类型/大纲/角色设定/前文摘要/前文/正文/作者样本。
// 编辑器正在写的内容（vars 里的 原文/正文）优先，不会被旧章节覆盖。
func RenderContext(c *gin.Context) {
	uid := middleware.UserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	t, err := prompttpl.Get(uint(id))
	if err != nil {
		utils.NotFound(c, err.Error())
		return
	}
	if !t.Enabled {
		utils.BadRequest(c, "该模板已停用")
		return
	}
	var req struct {
		NovelID   uint              `json:"novel_id"`
		ChapterID uint              `json:"chapter_id"`
		Vars      map[string]string `json:"vars"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if req.NovelID == 0 {
		utils.BadRequest(c, "novel_id 无效")
		return
	}
	novel, nerr := models.GetNovelByID(req.NovelID)
	if nerr != nil || novel == nil {
		utils.NotFound(c, "作品不存在")
		return
	}
	// 归属校验：作品作者本人或平台管理员（上下文含未公开正文，不能跨用户读取）
	if novel.AuthorID != uid && middleware.UserRole(c) != "admin" {
		utils.Forbidden(c, "仅作者本人可使用该作品的上下文")
		return
	}
	if req.Vars == nil {
		req.Vars = map[string]string{}
	}
	ctxRes, cerr := prompttpl.BuildNovelVars(req.NovelID, req.ChapterID, req.Vars, 0)
	if cerr != nil {
		utils.BadRequest(c, cerr.Error())
		return
	}
	vars := prompttpl.FilterVars(t.Content, ctxRes.Vars)
	rendered := prompttpl.Render(t.Content, t.Vars, vars)
	// 只回传实际用到且自动填充的变量（避免把作者大纲整段回显到前端日志）
	autoUsed := make([]string, 0, len(ctxRes.AutoFilled))
	for _, name := range ctxRes.AutoFilled {
		if _, ok := vars[name]; ok {
			autoUsed = append(autoUsed, name)
		}
	}
	utils.Success(c, gin.H{
		"template_id": t.ID, "title": t.Title, "version": t.Version,
		"text": rendered.Text, "missing": rendered.Missing,
		"auto_filled": autoUsed, "sources": ctxRes.Sources,
		"novel_title": ctxRes.NovelTitle, "chapter_title": ctxRes.ChapterTitle,
	})
}

// ---------- 管理端 ----------

// AdminList GET /api/admin/ai/templates?scene=&kind=
func AdminList(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	list, err := prompttpl.List(c.Query("scene"), c.Query("kind"), true)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{"list": list})
}

// AdminCreate POST /api/admin/ai/templates
func AdminCreate(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	var in prompttpl.Input
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	t, err := prompttpl.Create(in)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "已创建", t)
}

// AdminUpdate PUT /api/admin/ai/templates/:id
func AdminUpdate(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var in prompttpl.Input
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	t, err := prompttpl.Update(uint(id), in)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "已保存（版本 "+strconv.Itoa(t.Version)+"）", t)
}

// AdminDelete DELETE /api/admin/ai/templates/:id
func AdminDelete(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := prompttpl.Delete(uint(id)); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "已删除", nil)
}

// AdminVersions GET /api/admin/ai/templates/:id/versions
func AdminVersions(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	list, err := prompttpl.ListVersions(uint(id))
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{"list": list})
}

// AdminRestore POST /api/admin/ai/templates/:id/restore { version }
func AdminRestore(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Version int `json:"version"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Version <= 0 {
		utils.BadRequest(c, "请指定要回滚的版本号")
		return
	}
	t, err := prompttpl.Restore(uint(id), req.Version)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "已回滚（当前版本 "+strconv.Itoa(t.Version)+"）", t)
}

// AdminPreview POST /api/admin/ai/templates/preview { content, variables, vars }
// 未保存内容也能先渲染验证（含变量缺失提示）。
func AdminPreview(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	var req struct {
		Content   string                  `json:"content"`
		Variables []prompttpl.TemplateVar `json:"variables"`
		Vars      map[string]string       `json:"vars"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		utils.BadRequest(c, "模板内容不能为空")
		return
	}
	if req.Vars == nil {
		req.Vars = map[string]string{}
	}
	res := prompttpl.Render(req.Content, req.Variables, req.Vars)
	utils.Success(c, gin.H{"text": res.Text, "missing": res.Missing, "unused": res.Unused, "used": res.Used})
}
