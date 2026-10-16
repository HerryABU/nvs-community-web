package handlers

import (
	"strconv"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// author_theme.go — G4 作者公开主题（访客在作者页可见，区别于作者个人登录主题）。
//
// 安全模型：与个人/站点主题同源——写入一律经服务端白名单消毒（utils.SanitizeCSS），
// 前端仅以 <style> textContent 注入，杜绝任何原始用户 JS / 危险 CSS。

// GetAuthorTheme GET /api/author/:id/theme（公开，访客可见）
// 读取作者公开主题 CSS；无记录返回空串。
func GetAuthorTheme(c *gin.Context) {
	id, ok := parseAuthorID(c)
	if !ok {
		return
	}
	utils.Success(c, gin.H{"css": models.GetAuthorTheme(id)})
}

// UpdateAuthorTheme PUT /api/author/:id/theme（作者本人或管理员覆盖）
// 保存作者公开主题：服务端强制消毒后落库，访客在作者域页面可见。
func UpdateAuthorTheme(c *gin.Context) {
	id, ok := parseAuthorID(c)
	if !ok {
		return
	}
	uid := currentUserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}
	// 作者本人可改自己的；管理员可覆盖任意作者（override）
	if uid != id && !isAdminUser(c) {
		utils.Forbidden(c, "只能修改自己的作者主题")
		return
	}
	var req struct {
		CSS string `json:"css"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	removed, err := models.SetAuthorTheme(id, req.CSS)
	if err != nil {
		utils.InternalError(c, "保存作者主题失败")
		return
	}
	utils.Success(c, gin.H{"css": models.GetAuthorTheme(id), "removed": removed})
}

// parseAuthorID 解析路由中的作者 ID（uint），非法时直接响应 400。
func parseAuthorID(c *gin.Context) (uint, bool) {
	v, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || v == 0 {
		utils.BadRequest(c, "作者 ID 非法")
		return 0, false
	}
	return uint(v), true
}
