// Package handlers — 认证相关功能已拆分到以下文件：
//   auth_register.go  — Register 函数（含 RegisterInput struct）
//   auth_login.go     — Login, GetCurrentUser, Logout 函数（含 LoginInput struct）
//   auth_password.go  — SendVerificationCode, VerifyEmailCode, ForgotPassword, ResetPassword, ChangePassword 函数
package handlers

import (
	"nvs-server/models"

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

