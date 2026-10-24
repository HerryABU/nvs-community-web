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
		Content   string                `json:"content"`
		Variables []prompttpl.TemplateVar `json:"variables"`
		Vars      map[string]string     `json:"vars"`
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
