package handlers

import (
	"html"
	"strconv"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	"github.com/microcosm-cc/bluemonday"
)


// POST /api/reports — 提交举报
func CreateReport(c *gin.Context) {
	userID := c.GetUint("userID")
	var req struct {
		TargetType string `json:"target_type" binding:"required"`
		TargetID   uint   `json:"target_id" binding:"required"`
		Reason     string `json:"reason" binding:"required"`
		Detail     string `json:"detail"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}

	r := &models.Report{
		ReporterID: userID, TargetType: req.TargetType, TargetID: req.TargetID,
		Reason: bluemonday.UGCPolicy().Sanitize(req.Reason), Detail: html.EscapeString(req.Detail),
	}
	models.DB.Create(r)
	utils.Success(c, r)
}

// GET /api/admin/reports — 举报列表
func ListReports(c *gin.Context) {
	if !ensureAdmin(c) { return }
	var reports []models.Report
	models.DB.Preload("Reporter").Order("created_at DESC").Find(&reports)
	utils.Success(c, reports)
}

// POST /api/admin/reports/:id/handle — 处理举报
func HandleReport(c *gin.Context) {
	if !ensureAdmin(c) { return }
	userID := c.GetUint("userID")
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	var req struct {
		Status  string `json:"status" binding:"required"`
		Verdict string `json:"verdict"`
	}
	c.ShouldBindJSON(&req)

	models.DB.Model(&models.Report{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status": req.Status, "handler_id": userID, "verdict": bluemonday.UGCPolicy().Sanitize(req.Verdict),
	})
	utils.Success(c, gin.H{"message": "已处理"})
}

// ============ 站长面板 ============

// GET /api/admin/stats — 平台统计