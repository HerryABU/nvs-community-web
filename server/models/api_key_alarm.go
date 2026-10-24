package models

import "time"

// ApiKeyAlarm API Key 审计告警（多 IP / 解密失败 / 封禁后仍使用 / 指纹不匹配）
// 只做「提醒」不做「自动封禁」：移动网络切换与出口 IP 池都会触发多 IP，
// 自动封禁会误伤真实用户；处置权交给管理员（见 security/apikey_alarm.go）。
type ApiKeyAlarm struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	Kind      string     `gorm:"size:24;not null;index" json:"kind"`
	KeyID     uint       `gorm:"not null;index" json:"key_id"`
	UserID    uint       `gorm:"not null;index" json:"user_id"`
	Detail    string     `gorm:"size:512;default:''" json:"detail"`
	Acked     bool       `gorm:"not null;default:false;index" json:"acked"`
	AckedBy   uint       `gorm:"not null;default:0" json:"acked_by"`
	AckedAt   *time.Time `json:"acked_at"`
	Note      string     `gorm:"size:256;default:''" json:"note"`
	CreatedAt time.Time  `gorm:"index" json:"created_at"`
}

func (ApiKeyAlarm) TableName() string { return "api_key_alarms" }
