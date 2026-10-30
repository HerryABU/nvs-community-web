package handlers

import (
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// GetMapSetting GET /api/map-setting —— 公开：前端据此决定用哪套地图
func GetMapSetting(c *gin.Context) {
	utils.Success(c, models.PublicMapSetting())
}

// AdminGetMapSetting GET /api/admin/map-setting —— 含完整配置（key 明文给站长自己核对）
func AdminGetMapSetting(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	utils.Success(c, models.LoadMapSetting())
}

// AdminSaveMapSetting PUT /api/admin/map-setting
func AdminSaveMapSetting(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var in models.MapSetting
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数不合法")
		return
	}
	if err := models.SaveMapSetting(in); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.SuccessMessage(c, "已保存", models.LoadMapSetting())
}
