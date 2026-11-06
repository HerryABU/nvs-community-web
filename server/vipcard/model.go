// Package vipcard VIP 等级卡密（#66）。
//
// 与 wallet.CardKey（代币充值卡密）语义不同：这里发放的是「会员等级 + 有效天数」，
// 故独立建表、独立批次，避免把两种权益混进同一张表。
// 安全模式沿用代币卡密：明文只在生成时返回一次，库里只存规范化后的 sha256。
package vipcard

import (
	"time"
)

const (
	StatusUnused = "unused"
	StatusUsed   = "used"
	StatusVoid   = "void"
)

// VipCard VIP 等级卡密。
type VipCard struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	CodeHash   string     `gorm:"size:64;uniqueIndex;not null" json:"-"`               // sha256(规范化卡密) —— 不明文
	CodePrefix string     `gorm:"size:8;default:''" json:"code_prefix"`                // 前缀（列表展示用）
	Level      int        `gorm:"not null;default:1;index" json:"level"`               // 兑换后达到的会员等级
	Days       int        `gorm:"not null;default:30" json:"days"`                     // 会员有效天数
	Status     string     `gorm:"size:16;not null;default:unused;index" json:"status"` // unused/used/void
	BatchID    string     `gorm:"size:32;index;default:''" json:"batch_id"`            // 批次号
	CreatedBy  uint       `gorm:"default:0" json:"created_by"`                         // 生成管理员
	UsedBy     *uint      `json:"used_by"`                                             // 兑换用户
	UsedAt     *time.Time `json:"used_at"`
	ExpiresAt  *time.Time `json:"expires_at"` // 卡密本身的有效期（null = 永久）
	Note       string     `gorm:"size:256;default:''" json:"note"`
	CreatedAt  time.Time  `json:"created_at"`

	UsedByUsername string `gorm:"-" json:"used_by_username,omitempty"` // 列表填充
}

func (VipCard) TableName() string { return "vip_cards" }

// Models 供根包 AutoMigrateAll 聚合。
func Models() []interface{} { return []interface{}{&VipCard{}} }
