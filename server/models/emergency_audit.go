package models

import "time"

// EmergencyAuditLog 内容抢救台审计日志
// 记录所有高危应急操作（解锁/预览/硬删除/改密码），供日后溯源，独立于业务库语义。
type EmergencyAuditLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	OperatorID uint      `json:"operator_id"`          // 操作者用户 ID
	Operator   string    `gorm:"size:64" json:"operator"` // 操作者用户名
	IP         string    `gorm:"size:64" json:"ip"`       // 来源 IP
	Action     string    `gorm:"size:32" json:"action"`   // unlock/preview/delete/set_password
	TargetType string    `gorm:"size:16" json:"target_type"` // comment/thread/post
	TargetID   uint      `json:"target_id"`
	Detail     string    `gorm:"type:text" json:"detail"` // 内容摘要/备注
	CreatedAt  time.Time `json:"created_at"`
}

func (EmergencyAuditLog) TableName() string { return "emergency_audit_logs" }

// RecordEmergencyAudit 写入一条抢救台审计日志
func RecordEmergencyAudit(log *EmergencyAuditLog) {
	_ = DB.Create(log).Error
}

// ListEmergencyAuditLogs 列出审计日志（最近 N 条）
func ListEmergencyAuditLogs(limit int) ([]EmergencyAuditLog, error) {
	if limit <= 0 {
		limit = 200
	}
	var logs []EmergencyAuditLog
	err := DB.Order("id desc").Limit(limit).Find(&logs).Error
	return logs, err
}
