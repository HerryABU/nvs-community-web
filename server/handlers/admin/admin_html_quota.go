package admin

import (
	"strconv"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	quota "nvs-server/models/quota"
)

// ==================== HTML / WASM 托管额度（管理端） ====================

// AdminGetHTMLQuota GET /api/admin/html-quota
// 返回全局额度配置 + 按人覆盖列表。
func AdminGetHTMLQuota(c *gin.Context) {
	if !EnsureAdmin(c) {
		return
	}
	var overrides []quota.HTMLQuotaOverride
	models.DB.Order("updated_at DESC").Limit(500).Find(&overrides)
	utils.Success(c, gin.H{
		"config":    quota.LoadHTMLQuotaSetting(),
		"overrides": overrides,
	})
}

// AdminSaveHTMLQuota PUT /api/admin/html-quota
func AdminSaveHTMLQuota(c *gin.Context) {
	if !EnsureAdmin(c) {
		return
	}
	var in quota.HTMLQuotaSetting
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数不合法")
		return
	}
	if in.MaxProjects < 0 {
		in.MaxProjects = 0
	}
	if in.MaxTotalSizeMB < 0 {
		in.MaxTotalSizeMB = 0
	}
	if err := quota.SaveHTMLQuotaSetting(in); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.SuccessMessage(c, "已保存", quota.LoadHTMLQuotaSetting())
}

// AdminSetHTMLQuotaOverride POST /api/admin/html-quota/overrides
// action=set：按人设定（0=跟随全局，-1=不限）；action=remove：删除覆盖。
func AdminSetHTMLQuotaOverride(c *gin.Context) {
	if !EnsureAdmin(c) {
		return
	}
	var req struct {
		Action         string `json:"action"`
		UserID         uint   `json:"user_id"`
		MaxProjects    int    `json:"max_projects"`
		MaxTotalSizeMB int    `json:"max_total_size_mb"`
		Note           string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.UserID == 0 {
		utils.BadRequest(c, "参数不合法")
		return
	}
	switch req.Action {
	case "remove":
		if err := models.DB.Where("user_id = ?", req.UserID).
			Delete(&quota.HTMLQuotaOverride{}).Error; err != nil {
			utils.InternalError(c, "删除失败")
			return
		}
		utils.SuccessMessage(c, "已恢复为全局额度", nil)
	case "set":
		var ov quota.HTMLQuotaOverride
		err := models.DB.Where("user_id = ?", req.UserID).First(&ov).Error
		if err != nil {
			ov = quota.HTMLQuotaOverride{UserID: req.UserID}
		}
		ov.MaxProjects = req.MaxProjects
		ov.MaxTotalSizeMB = req.MaxTotalSizeMB
		ov.Note = req.Note
		if err := models.DB.Save(&ov).Error; err != nil {
			utils.InternalError(c, "保存失败")
			return
		}
		utils.SuccessMessage(c, "已保存", ov)
	default:
		utils.BadRequest(c, "action 只能是 set / remove")
	}
}

// AdminHTMLQuotaUsage GET /api/admin/html-quota/usage?user_id=X
// 查某人的实际额度与已用量（站长在审核页核对「他到底还能放多少」）。
func AdminHTMLQuotaUsage(c *gin.Context) {
	if !EnsureAdmin(c) {
		return
	}
	uid, _ := strconv.ParseUint(c.Query("user_id"), 10, 64)
	if uid == 0 {
		utils.BadRequest(c, "缺少 user_id")
		return
	}
	utils.Success(c, quota.HTMLQuotaSnapshot(uint(uid)))
}
