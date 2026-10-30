package handlers

import (
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// GetHomeView GET /api/home-view —— 公开：当前首页形态（前端据此决定渲染哪套视图）
func GetHomeView(c *gin.Context) {
	utils.Success(c, gin.H{
		"view":  models.GetHomeView(),
		"views": models.HomeViews,
	})
}

// AdminGetHomeView GET /api/admin/home-view
func AdminGetHomeView(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	utils.Success(c, gin.H{
		"view":  models.GetHomeView(),
		"views": models.HomeViews,
	})
}

// AdminSaveHomeView PUT /api/admin/home-view
func AdminSaveHomeView(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		View string `json:"view"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数不合法")
		return
	}
	if err := models.SetHomeView(req.View); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "已保存，刷新首页生效", models.GetHomeView())
}
