package home

import (
	"nvs-server/handlers/admin"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	theme "nvs-server/models/theme"
)

// GetHomeView GET /api/home-view —— 公开：当前首页形态（前端据此决定渲染哪套视图）
func GetHomeView(c *gin.Context) {
	utils.Success(c, gin.H{
		"view":  theme.GetHomeView(),
		"views": theme.HomeViews,
	})
}

// AdminGetHomeView GET /api/admin/home-view
func AdminGetHomeView(c *gin.Context) {
	if !admin.EnsureAdmin(c) {
		return
	}
	utils.Success(c, gin.H{
		"view":  theme.GetHomeView(),
		"views": theme.HomeViews,
	})
}

// AdminSaveHomeView PUT /api/admin/home-view
func AdminSaveHomeView(c *gin.Context) {
	if !admin.EnsureAdmin(c) {
		return
	}
	var req struct {
		View string `json:"view"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数不合法")
		return
	}
	if err := theme.SetHomeView(req.View); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "已保存，刷新首页生效", theme.GetHomeView())
}
