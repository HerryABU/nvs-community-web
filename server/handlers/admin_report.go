package handlers

import (
	"html"
	"strconv"
	"time"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	"github.com/microcosm-cc/bluemonday"
)

// GetReportReasons GET /api/reports/reasons — 预置举报理由
func GetReportReasons(c *gin.Context) {
	utils.Success(c, models.PredefinedReportReasons())
}

// CreateReport POST /api/reports — 提交举报（理由选择 + 提交）
func CreateReport(c *gin.Context) {
	userID := c.GetUint("userID")
	var req struct {
		TargetType string `json:"target_type" binding:"required"`
		TargetID   uint   `json:"target_id" binding:"required"`
		ReasonCode string `json:"reason_code"`
		Reason     string `json:"reason" binding:"required"`
		Detail     string `json:"detail"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}

	// 目标类型白名单
	switch req.TargetType {
	case "novel", "comment", "thread", "post":
	default:
		utils.BadRequest(c, "不支持的举报对象类型")
		return
	}

	// 理由：优先预置编码，其次回退为自由文本
	reasonLabel := bluemonday.UGCPolicy().Sanitize(req.Reason)
	reasonCode := ""
	if r, ok := models.LookupReportReason(req.ReasonCode); ok {
		reasonCode = r.Code
		reasonLabel = r.Label
	}

	// 防止同一用户对同一目标重复举报
	var dupCount int64
	models.DB.Model(&models.Report{}).
		Where("reporter_id = ? AND target_type = ? AND target_id = ? AND status = ?",
			userID, req.TargetType, req.TargetID, "pending").
		Count(&dupCount)
	if dupCount > 0 {
		utils.BadRequest(c, "您已举报过该内容，请等待处理结果")
		return
	}

	r := &models.Report{
		ReporterID: userID,
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
		ReasonCode: reasonCode,
		Reason:     reasonLabel,
		Detail:     html.EscapeString(req.Detail),
	}
	if err := models.DB.Create(r).Error; err != nil {
		utils.InternalError(c, "提交举报失败")
		return
	}
	utils.Success(c, r)
}

// ListMyReports GET /api/reports/mine — 我的举报（状态追踪）
func ListMyReports(c *gin.Context) {
	userID := c.GetUint("userID")
	var reports []models.Report
	models.DB.Where("reporter_id = ?", userID).
		Order("created_at DESC").Limit(200).Find(&reports)

	type reportItem struct {
		models.Report
		TargetPreview string `json:"target_preview"`
	}
	items := make([]reportItem, 0, len(reports))
	for _, r := range reports {
		items = append(items, reportItem{Report: r, TargetPreview: targetPreview(r.TargetType, r.TargetID)})
	}
	utils.Success(c, items)
}

// targetPreview 根据举报目标类型返回内容预览，便于用户追踪
func targetPreview(targetType string, targetID uint) string {
	const maxLen = 80
	truncate := func(s string) string {
		if len(s) > maxLen {
			return s[:maxLen] + "…"
		}
		return s
	}
	switch targetType {
	case "comment":
		if c, err := models.GetCommentByID(targetID); err == nil {
			return truncate(c.Content)
		}
	case "thread":
		if t, err := models.GetThreadByID(targetID); err == nil {
			return truncate(t.Title)
		}
	case "novel":
		if n, err := models.GetNovelByID(targetID); err == nil {
			return truncate(n.Title)
		}
	case "post":
		var p models.Post
		if err := models.DB.First(&p, targetID).Error; err == nil {
			return truncate(p.Content)
		}
	}
	return ""
}

// ListReports GET /api/admin/reports — 举报列表
func ListReports(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	status := c.Query("status")
	query := models.DB.Preload("Reporter").Order("created_at DESC")
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var reports []models.Report
	query.Find(&reports)
	utils.Success(c, reports)
}

// HandleReport POST /api/admin/reports/:id/handle — 处理举报（人工判定 + 可选删除目标）
func HandleReport(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	userID := c.GetUint("userID")
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	var req struct {
		Status       string `json:"status" binding:"required"`
		Verdict      string `json:"verdict"`
		DeleteTarget bool   `json:"delete_target"` // 判定成立时是否删除被举报内容
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if req.Status != "accepted" && req.Status != "rejected" {
		utils.BadRequest(c, "无效的处理状态")
		return
	}

	var report models.Report
	if err := models.DB.First(&report, id).Error; err != nil {
		utils.NotFound(c, "举报不存在")
		return
	}

	// 判定成立且要求删除目标内容时，执行删除
	if req.Status == "accepted" && req.DeleteTarget {
		deleteReportTarget(report.TargetType, report.TargetID)
	}

	now := time.Now()
	models.DB.Model(&models.Report{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":      req.Status,
		"handler_id":  userID,
		"verdict":     bluemonday.UGCPolicy().Sanitize(req.Verdict),
		"resolved_at": &now,
	})
	utils.Success(c, gin.H{"message": "已处理"})
}

// deleteReportTarget 删除被举报的目标内容（管理员判定成立时）
func deleteReportTarget(targetType string, targetID uint) {
	switch targetType {
	case "comment":
		_ = models.DeleteComment(targetID)
	case "thread":
		models.DB.Delete(&models.Thread{}, targetID)
		models.DB.Where("thread_id = ?", targetID).Delete(&models.Post{})
	case "post":
		models.DB.Delete(&models.Post{}, targetID)
	case "novel":
		_ = models.DeleteNovel(targetID)
	}
}
