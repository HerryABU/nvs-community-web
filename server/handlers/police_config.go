package handlers

import (
	"regexp"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// 警方取证台「已删除内容高亮色」配置键（存于 platform_configs）。
const (
	PoliceDelColorKey     = "police_del_color"
	DefaultPoliceDelColor = "#ff4d4f"
)

// hexColorRe 校验颜色：#RGB 或 #RRGGBB（大小写不敏感）。
var hexColorRe = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// GET /api/police/del-color — 读取已删除内容高亮色（警方可读，管理员亦可）
// 返回 { color: "#ff4d4f" }。未配置时回退默认色。
func PoliceGetDelColor(c *gin.Context) {
	if !ensurePoliceOrAdmin(c) {
		return
	}
	color := models.GetPlatformConfig(PoliceDelColorKey)
	if color == "" {
		color = DefaultPoliceDelColor
	}
	utils.Success(c, gin.H{"color": color})
}

// PUT /api/police/del-color — 设置已删除内容高亮色（仅管理员可写）
// 入参 { color: "#ff4d4f" }。警方/其他角色无写权限。
func PoliceSetDelColor(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		Color string `json:"color" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Color == "" {
		utils.BadRequest(c, "请提供 color 字段")
		return
	}
	if !hexColorRe.MatchString(req.Color) {
		utils.BadRequest(c, "颜色格式非法，需为 #RGB 或 #RRGGBB")
		return
	}
	if err := models.SetPlatformConfig(PoliceDelColorKey, req.Color); err != nil {
		utils.InternalError(c, "保存配置失败")
		return
	}
	utils.Success(c, gin.H{"color": req.Color, "message": "已更新删除内容高亮色"})
}
