package handlers

import (
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// theme.go — F12 用户自定义网站主题（alist 式）API。
// 安全模型：写入一律经服务端白名单消毒（utils.SanitizeCSS）后落库；
// 前端仅以 <style> textContent 注入，不使用 innerHTML。

// GetSiteTheme GET /api/theme/site（公开）
// 站点默认主题：管理员配置，对所有访客生效（登录用户的个人主题覆盖之）。
func GetSiteTheme(c *gin.Context) {
	utils.Success(c, gin.H{"css": models.GetSiteTheme()})
}

// GetMyTheme GET /api/theme/me（需登录）
// 当前用户的个人主题 CSS；未配置返回空串。
func GetMyTheme(c *gin.Context) {
	userID := currentUserID(c)
	if userID == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}
	utils.Success(c, gin.H{"css": models.GetUserTheme(userID)})
}

// UpdateMyTheme PUT /api/theme/me（需登录）
// 保存个人主题：服务端强制消毒，返回消毒后的 CSS 与被移除的规则列表。
func UpdateMyTheme(c *gin.Context) {
	userID := currentUserID(c)
	if userID == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}
	var req struct {
		CSS string `json:"css"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	removed, err := models.SetUserTheme(userID, req.CSS)
	if err != nil {
		utils.InternalError(c, "保存个人主题失败")
		return
	}
	utils.Success(c, gin.H{"css": models.GetUserTheme(userID), "removed": removed})
}

// UpdateSiteTheme PUT /api/theme/site（仅管理员）
// 保存站点默认主题：对全部访客生效，需二次确认的醒目提示由前端负责。
func UpdateSiteTheme(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		CSS string `json:"css"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	removed, err := models.SetSiteTheme(req.CSS)
	if err != nil {
		utils.InternalError(c, "保存站点主题失败")
		return
	}
	utils.Success(c, gin.H{"css": models.GetSiteTheme(), "removed": removed})
}
