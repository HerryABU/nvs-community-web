package handlers

import (
	"strconv"

	"nvs-server/moderation"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// GetModerationConfig GET /api/admin/moderation/config
func GetModerationConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	cfg, err := moderation.GetConfig()
	if err != nil {
		utils.InternalError(c, "读取配置失败")
		return
	}
	utils.Success(c, cfg)
}

// SaveModerationConfig PUT /api/admin/moderation/config
func SaveModerationConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var cfg moderation.ModerationConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if cfg.BatchIntervalHours <= 0 {
		cfg.BatchIntervalHours = 6
	}
	if cfg.RecheckReportCount <= 0 {
		cfg.RecheckReportCount = 5
	}
	if cfg.MaliciousThreshold <= 0 {
		cfg.MaliciousThreshold = 5
	}
	if cfg.StopReviewAfter <= 0 {
		cfg.StopReviewAfter = 20
	}
	if err := moderation.SaveConfig(&cfg); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.Success(c, cfg)
}

// ListModerationRecords GET /api/admin/moderation/records
func ListModerationRecords(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	f := moderation.RecordFilter{
		Scope:    c.Query("scope"),
		Verdict:  c.Query("verdict"),
		Trigger:  c.Query("trigger"),
		Page:     page,
		PageSize: 50,
	}
	list, total, err := moderation.ListRecords(f)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{"items": list, "total": total})
}

// RunModerationBatch POST /api/admin/moderation/batch
func RunModerationBatch(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	n, err := moderation.RunBatchOnce()
	if err != nil {
		utils.InternalError(c, "批量审核失败")
		return
	}
	utils.Success(c, gin.H{"reviewed": n})
}

// GetModerationStats GET /api/admin/moderation/stats
func GetModerationStats(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	s, err := moderation.GetStats()
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, s)
}

// ListMaliciousReporters GET /api/admin/moderation/malicious
func ListMaliciousReporters(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	list, err := moderation.ListMaliciousReporters()
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, list)
}

// UnbanReporter POST /api/admin/moderation/malicious/:id/unban
func UnbanReporter(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := moderation.UnbanReporter(uint(id)); err != nil {
		utils.InternalError(c, "操作失败")
		return
	}
	utils.Success(c, gin.H{"message": "已恢复举报功能"})
}
