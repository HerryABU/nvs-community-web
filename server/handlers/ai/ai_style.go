package ai

import (
	"nvs-server/handlers/admin"
	"nvs-server/utils"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	aihub "nvs-server/models/aihub"
)

// ==================== 作品风格字典 ====================

// AiStyleList GET /api/ai/styles?kind=image|video — 前台取启用风格（发布作品用）
func AiStyleList(c *gin.Context) {
	list, err := aihub.ListAiStyles(c.Query("kind"), true)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	names := make([]string, 0, len(list))
	for _, s := range list {
		names = append(names, s.Name)
	}
	utils.Success(c, gin.H{"list": list, "names": names})
}

// AdminListAiStyles GET /api/admin/ai/styles?kind=
func AdminListAiStyles(c *gin.Context) {
	if !admin.EnsureAdmin(c) {
		return
	}
	list, err := aihub.ListAiStyles(c.Query("kind"), false)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{"list": list})
}

// AdminCreateAiStyle POST /api/admin/ai/styles
func AdminCreateAiStyle(c *gin.Context) {
	if !admin.EnsureAdmin(c) {
		return
	}
	var req struct {
		Kind      string `json:"kind"`
		Name      string `json:"name"`
		SortOrder int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	st, err := aihub.CreateAiStyle(req.Kind, req.Name, req.SortOrder)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, st)
}

// AdminUpdateAiStyle PUT /api/admin/ai/styles/:id
func AdminUpdateAiStyle(c *gin.Context) {
	if !admin.EnsureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Name      string `json:"name"`
		SortOrder int    `json:"sort_order"`
		Enabled   *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if err := aihub.UpdateAiStyle(uint(id), strings.TrimSpace(req.Name), req.SortOrder, enabled); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, gin.H{"message": "已保存"})
}

// AdminDeleteAiStyle DELETE /api/admin/ai/styles/:id
func AdminDeleteAiStyle(c *gin.Context) {
	if !admin.EnsureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := aihub.DeleteAiStyle(uint(id)); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, gin.H{"message": "已删除"})
}
