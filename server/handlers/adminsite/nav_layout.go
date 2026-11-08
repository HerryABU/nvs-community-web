package adminsite

// nav_layout.go —— 导航模块布局（超级管理员可配：主导航 / 更多）。
//
// 公开端点供前台导航渲染；管理端仅超级管理员（站长）可读写。
// 布局持久化在 platform_configs.nav_module_layout（JSON），读取时与默认目录对账，
// 文案/图标/路径以后端目录为准，前端无法借配置注入任意跳转。

import (
	"nvs-server/handlers/admincommon"
	"nvs-server/models"
	"nvs-server/utils/infra"

	"github.com/gin-gonic/gin"
)

// GetNavLayout GET /api/nav-layout —— 公开：前台导航按布局渲染。
func GetNavLayout(c *gin.Context) {
	infra.Success(c, gin.H{"modules": models.GetNavModules()})
}

// AdminGetNavLayout GET /api/admin/nav-layout —— 管理端读取（仅超级管理员）。
func AdminGetNavLayout(c *gin.Context) {
	if !admincommon.EnsureSuperAdmin(c) {
		return
	}
	infra.Success(c, gin.H{"modules": models.GetNavModules()})
}

// AdminUpdateNavLayout PUT /api/admin/nav-layout —— 管理端保存（仅超级管理员，立即生效）。
func AdminUpdateNavLayout(c *gin.Context) {
	if !admincommon.EnsureSuperAdmin(c) {
		return
	}
	var req struct {
		Modules []models.NavModule `json:"modules"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	if len(req.Modules) == 0 {
		infra.BadRequest(c, "模块列表不能为空")
		return
	}
	if err := models.SetNavModules(req.Modules); err != nil {
		infra.InternalError(c, "保存失败")
		return
	}
	infra.Success(c, gin.H{"message": "已保存（立即生效）", "modules": models.GetNavModules()})
}
