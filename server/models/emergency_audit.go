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

// 审计动作枚举（176号令合规：规范化 action 与 detail 填写）
// detail 格式建议：操作描述 | 违规内容摘要 | 处置依据 | 是否报告
const (
	AuditSetPassword    = "set_password"   // 设置/修改抢救台二级密码
	AuditUnlock         = "unlock"         // 抢救台解锁
	AuditUnlockFail     = "unlock_fail"    // 抢救台解锁失败
	AuditPreview        = "preview"        // 抢救台内容预览
	AuditEmergencyDelete = "delete"        // 抢救台物理硬删除
	AuditContentDelete  = "content_delete" // 内容删除（管理员/版主处置）
	AuditContentHide    = "content_hide"   // 内容屏蔽
	AuditContentRestore = "content_restore" // 内容恢复（误删恢复）
	AuditUserMute       = "user_mute"      // 禁言用户
	AuditUserUnmute     = "user_unmute"    // 解除禁言
	AuditUserBan        = "user_ban"       // 封禁/删除用户
	AuditUserUnban      = "user_unban"     // 解封用户
	AuditUserPromotePolice = "user_promote_police" // 提权为警方取管台账号
	AuditUserRevokePolice  = "user_revoke_police"  // 撤销警方取管台权限
	AuditReportHandle   = "report_handle"  // 举报处理
)

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
