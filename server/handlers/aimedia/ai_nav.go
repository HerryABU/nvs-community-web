package aimedia

import (
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	aiops "nvs-server/models/aiops"
)

// AiNavItems GET /api/ai/nav-items
//
// AI 工作台左栏的自定义菜单（站长设置）。只下发**已启用**的条目；
// 前台拿到就渲染，不需要再判一遍启用状态。
func AiNavItems(c *gin.Context) {
	utils.Success(c, gin.H{"items": aiops.GetAiNavItems()})
}

// AdminGetAiNavItems GET /api/admin/ai-nav-items
// 管理端读全部（含未启用），站长要能编辑被隐藏的条目。
func AdminGetAiNavItems(c *gin.Context) {
	utils.Success(c, gin.H{
		"items": aiops.GetAllAiNavItems(),
		"kinds": aiops.AiNavKinds,
	})
}

// AdminSaveAiNavItems PUT /api/admin/ai-nav-items
// body: { items: [...] }  —— 整体替换（顺序即显示顺序）。
func AdminSaveAiNavItems(c *gin.Context) {
	var req struct {
		Items []aiops.AiNavItem `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := aiops.SetAiNavItems(req.Items); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, gin.H{"message": "已保存，AI 工作台左栏立即生效", "items": aiops.GetAllAiNavItems()})
}
