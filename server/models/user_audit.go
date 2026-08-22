package models

import "time"

// UserAuditLog 用户行为审计日志（176号令合规：可追溯用户关键操作）
// 记录登录成功/失败、登出、改密码、改邮箱、改资料、注册等操作，含 IP 与 User-Agent。
type UserAuditLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index:idx_user_audit_user_time,priority:1" json:"user_id"`
	Action    string    `gorm:"size:30;not null;index" json:"action"` // login_success/login_fail/logout/change_password/change_email/change_profile/register
	IP        string    `gorm:"size:45" json:"ip"`
	UserAgent string    `gorm:"type:text" json:"user_agent"`
	Detail    string    `gorm:"type:text" json:"detail"`
	Result    string    `gorm:"size:20" json:"result"` // success / fail
	CreatedAt time.Time `gorm:"index:idx_user_audit_user_time,priority:2" json:"created_at"`
}

func (UserAuditLog) TableName() string { return "user_audit_logs" }

// 操作类型枚举
const (
	AuditLoginSuccess   = "login_success"
	AuditLoginFail      = "login_fail"
	AuditLogout         = "logout"
	AuditChangePassword = "change_password"
	AuditChangeEmail    = "change_email"
	AuditChangeProfile  = "change_profile"
	AuditRegister       = "register"
)

// RecordUserAudit 写入一条用户行为审计日志（异步容错，不阻塞主流程）
func RecordUserAudit(log *UserAuditLog) {
	if log == nil {
		return
	}
	_ = DB.Create(log).Error
}

// ListUserAuditLogs 按用户/操作筛选审计日志（最近 N 条）
func ListUserAuditLogs(userID uint, action string, limit int) ([]UserAuditLog, error) {
	if limit <= 0 {
		limit = 500
	}
	query := DB.Model(&UserAuditLog{}).Order("id desc")
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	if action != "" {
		query = query.Where("action = ?", action)
	}
	var logs []UserAuditLog
	err := query.Limit(limit).Find(&logs).Error
	return logs, err
}
