package models

import "time"

// VipApplication VIP 申请
type VipApplication struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	UserID     uint       `gorm:"not null;uniqueIndex" json:"user_id"`
	Status     string     `gorm:"size:16;default:pending" json:"status"`
	Reason     string     `gorm:"type:text" json:"reason"`
	ReviewedAt *time.Time `json:"reviewed_at"`
	CreatedAt  time.Time  `json:"created_at"`
	User       *User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (VipApplication) TableName() string { return "vip_applications" }
