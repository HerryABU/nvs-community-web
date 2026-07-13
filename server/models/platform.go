package models

import "time"

// Report 举报
type Report struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ReporterID uint      `gorm:"not null;index" json:"reporter_id"`
	TargetType string    `gorm:"size:16;not null" json:"target_type"` // novel/comment/thread
	TargetID   uint      `gorm:"not null" json:"target_id"`
	Reason     string    `gorm:"size:64;not null" json:"reason"`
	Detail     string    `gorm:"type:text" json:"detail"`
	Status     string    `gorm:"size:16;default:pending" json:"status"` // pending/accepted/rejected
	HandlerID  *uint     `json:"handler_id"`
	Verdict    string    `gorm:"type:text" json:"verdict"`
	CreatedAt  time.Time `json:"created_at"`
	Reporter   *User     `gorm:"foreignKey:ReporterID" json:"reporter,omitempty"`
}

func (Report) TableName() string { return "reports" }

// WithdrawalRequest 提现申请
type WithdrawalRequest struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	Amount    float64   `gorm:"not null" json:"amount"`
	Method    string    `gorm:"size:32" json:"method"`
	Account   string    `gorm:"size:128" json:"account"`
	Status    string    `gorm:"size:16;default:pending" json:"status"` // pending/approved/paid/rejected
	CreatedAt time.Time `json:"created_at"`
	User      *User     `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (WithdrawalRequest) TableName() string { return "withdrawal_requests" }

// EarningsRecord 收益记录
type EarningsRecord struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	NovelID   *uint     `json:"novel_id"`
	Amount    float64   `gorm:"not null" json:"amount"`
	Type      string    `gorm:"size:32;not null" json:"type"` // tip/subscription
	CreatedAt time.Time `json:"created_at"`
}

func (EarningsRecord) TableName() string { return "earnings_records" }

// BlacklistIP 黑名单 IP
type BlacklistIP struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	IP        string    `gorm:"size:45;uniqueIndex;not null" json:"ip"`
	Reason    string    `gorm:"size:256" json:"reason"`
	ExpiresAt *time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

func (BlacklistIP) TableName() string { return "blacklist_ips" }
