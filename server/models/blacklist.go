package models

import (
	"time"

	"gorm.io/gorm"
)

// ============ 用户黑名单（F13） ============
//
// 语义：UserID（拉黑方）把 BlockedUserID（被拉黑方）加入黑名单。
//   Mode = block：禁止对方在自己的评论区发言（创建评论时拦截）
//   Mode = hide ：在评论区/内容流中屏蔽对方的展示与互动
// 站长/警方/管理员角色不可被拉黑（保障平台治理通道，见 handlers.AddBlacklist）。

const (
	BlacklistModeBlock = "block" // 禁止在其评论区评论
	BlacklistModeHide  = "hide"  // 屏蔽其内容不看
)

type Blacklist struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	UserID        uint      `gorm:"not null;index;uniqueIndex:idx_blacklists_pair,priority:1" json:"user_id"`         // 拉黑方
	BlockedUserID uint      `gorm:"not null;index;uniqueIndex:idx_blacklists_pair,priority:2" json:"blocked_user_id"` // 被拉黑方
	Mode          string    `gorm:"size:16;default:'block'" json:"mode"`                                              // block=禁止评论 / hide=屏蔽内容
	Reason        string    `gorm:"size:256;default:''" json:"reason"`                                                // 拉黑理由（可选，256 字符）
	CreatedAt     time.Time `json:"created_at"`

	// 运行时填充（ListBlacklist 通过 LEFT JOIN users 带出对方信息，不落库）
	BlockedUsername  string `gorm:"-" json:"blocked_username,omitempty"`
	BlockedNickname  string `gorm:"-" json:"blocked_nickname,omitempty"`
	BlockedAvatarURL string `gorm:"-" json:"blocked_avatar_url,omitempty"`
}

func (Blacklist) TableName() string {
	return "blacklists"
}

// AddBlacklist 添加黑名单；同一 (UserID, BlockedUserID) 已存在时更新 mode/reason（幂等）。
// 成功后 b.ID 为记录主键（新建或既有行）。
func AddBlacklist(b *Blacklist) error {
	var existing Blacklist
	err := DB.Where("user_id = ? AND blocked_user_id = ?", b.UserID, b.BlockedUserID).First(&existing).Error
	if err == nil {
		// 已拉黑 → 仅更新模式与理由，保留原创建时间
		if uerr := DB.Model(&existing).Updates(map[string]interface{}{
			"mode":   b.Mode,
			"reason": b.Reason,
		}).Error; uerr != nil {
			return uerr
		}
		b.ID = existing.ID
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	return DB.Create(b).Error
}

// RemoveBlacklist 解除拉黑（仅限记录归属者本人：userID 必须匹配）。
func RemoveBlacklist(userID, id uint) error {
	return DB.Where("id = ? AND user_id = ?", id, userID).Delete(&Blacklist{}).Error
}

// GetBlacklistByID 按主键查询黑名单记录（供 handler 做归属校验）。
func GetBlacklistByID(id uint) (*Blacklist, error) {
	var b Blacklist
	if err := DB.First(&b, id).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

// BlacklistPage 黑名单分页结果。
type BlacklistPage struct {
	List  []Blacklist
	Total int64
}

// ListBlacklist 分页查询某用户的全部黑名单（按拉黑时间倒序），
// LEFT JOIN users 带出对方的 username/nickname/avatar_url 供前端展示。
func ListBlacklist(userID uint, page, pageSize int) (BlacklistPage, error) {
	var out BlacklistPage
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	if err := DB.Model(&Blacklist{}).Where("user_id = ?", userID).Count(&out.Total).Error; err != nil {
		return out, err
	}
	if out.Total == 0 {
		return out, nil
	}
	// LEFT JOIN：即使对方账号被软删除，黑名单记录本身仍要可见可解除
	err := DB.Model(&Blacklist{}).
		Select("blacklists.*, u.username AS blocked_username, u.nickname AS blocked_nickname, u.avatar_url AS blocked_avatar_url").
		Joins("LEFT JOIN users u ON u.id = blacklists.blocked_user_id").
		Where("blacklists.user_id = ?", userID).
		Order("blacklists.created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Scan(&out.List).Error
	return out, err
}

// IsBlacklisted 单向判定：blockedID 是否被 blockerID 拉黑（创建评论时拦截用）。
func IsBlacklisted(blockerID, blockedID uint) bool {
	if blockerID == 0 || blockedID == 0 {
		return false
	}
	var n int64
	DB.Model(&Blacklist{}).
		Where("user_id = ? AND blocked_user_id = ?", blockerID, blockedID).
		Count(&n)
	return n > 0
}

// IsBlockedEither 双向判定：a 与 b 之间任一方拉黑了对方即返回 true（内容隐藏用）。
func IsBlockedEither(a, b uint) bool {
	if a == 0 || b == 0 {
		return false
	}
	var n int64
	DB.Model(&Blacklist{}).
		Where("(user_id = ? AND blocked_user_id = ?) OR (user_id = ? AND blocked_user_id = ?)", a, b, b, a).
		Count(&n)
	return n > 0
}

// CountBlacklist 统计某用户已拉黑的用户数。
func CountBlacklist(userID uint) int64 {
	var n int64
	DB.Model(&Blacklist{}).Where("user_id = ?", userID).Count(&n)
	return n
}
