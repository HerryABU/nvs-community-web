// Package handlers — 认证相关功能已拆分到以下文件：
//   auth_register.go  — Register 函数（含 RegisterInput struct）
//   auth_login.go     — Login, GetCurrentUser, Logout 函数（含 LoginInput struct）
//   auth_password.go  — SendVerificationCode, VerifyEmailCode, ForgotPassword, ResetPassword, ChangePassword 函数
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
// 支持 user_id、zone 筛选，返回「谁、何时、进入哪个分区」。
func ListZoneAccessLogs(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	userID, _ := strconv.ParseUint(c.Query("user_id"), 10, 64)
	zone := c.Query("zone")
	logs, err := models.ListZoneAccess(uint(userID), zone, 500)
	if err != nil {
		utils.InternalError(c, "获取隔离墙进入记录失败")
		return
	}
	utils.Success(c, logs)
}

