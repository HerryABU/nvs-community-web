// Package handlers — 认证相关功能已拆分到以下文件：
//
//	auth_register.go  — Register 函数（含 RegisterInput struct）
//	auth_login.go     — Login, GetCurrentUser, Logout 函数（含 LoginInput struct）
//	auth_password.go  — SendVerificationCode, VerifyEmailCode, ForgotPassword, ResetPassword, ChangePassword 函数
package handlers

import (
	"strconv"
	"strings"
	"time"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// recordUserAudit 记录用户行为审计日志（176号令合规）。
// 从 gin context 提取 userID、IP、User-Agent 后写入 user_audit_logs。
func recordUserAudit(c *gin.Context, userID uint, action, detail, result string) {
	models.RecordUserAudit(&models.UserAuditLog{
		UserID:    userID,
		Action:    action,
		IP:        c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
		Detail:    detail,
		Result:    result,
	})
}

// RecordZoneEnter POST /api/zone/enter — 记录用户进入隔离墙（敏感分区）的确认时间（176号令合规）。
// 用户在通过同人区/政治文学区等敏感分区的确认弹窗后调用，落库「何时进入」以备审计追溯。
func RecordZoneEnter(c *gin.Context) {
	userID := currentUserID(c)
	var req struct {
		Zone string `json:"zone"`
	}
	_ = c.ShouldBindJSON(&req)
	zone := strings.TrimSpace(req.Zone)
	if zone == "" {
		utils.BadRequest(c, "缺少隔离墙分区名")
		return
	}
	// 仅写独立隔离墙进入表：含 user_id / zone / entered_at / ip / UA，已覆盖原审计日志 zone_enter 条目
	models.RecordZoneEnterLog(userID, zone, c.ClientIP(), c.Request.UserAgent(), time.Now())
	utils.Success(c, gin.H{"message": "已记录进入时间"})
}

// ListZoneAccessLogs GET /api/admin/zone-access — 查询隔离墙（敏感分区）进入记录（176号令专项）。
// 支持 user_id、zone、from、to 筛选与分页，返回「谁、何时、进入哪个分区」及用户名。
func ListZoneAccessLogs(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	userID, _ := strconv.ParseUint(c.Query("user_id"), 10, 64)
	zone := strings.TrimSpace(c.Query("zone"))
	from := parseDateParam(c.Query("from"))
	to := parseDateParam(c.Query("to"))
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	items, total, err := models.ListZoneAccessPage(models.ZoneAccessFilter{
		UserID:   uint(userID),
		Zone:     zone,
		From:     from,
		To:       to,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		utils.InternalError(c, "获取隔离墙进入记录失败")
		return
	}
	utils.Success(c, gin.H{"total": total, "items": items})
}

// GetZoneAccessStats GET /api/admin/zone-access/stats — 隔离墙进入记录聚合统计（仪表盘卡片）。
func GetZoneAccessStats(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	stats, err := models.GetZoneAccessStats()
	if err != nil {
		utils.InternalError(c, "获取隔离墙进入统计失败")
		return
	}
	utils.Success(c, stats)
}

// parseDateParam 解析日期查询参数，支持 RFC3339 / 2006-01-02T15:04:05 / 2006-01-02，空或非法返回零值。
func parseDateParam(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
