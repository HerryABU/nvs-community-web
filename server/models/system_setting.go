package models

import (
	"strconv"
	"time"
)

// ─────────────────────────────────────────────────────────────
// 系统设置（key/value 表）：站点级配置项
// 目前用途：博客分类审核模式（blog_category_review_mode）
//   - post_review（默认）：事后审核 —— 普通作者创建分类先待审核，管理员通过后公开
//   - pre_public：先公开，事后封禁 —— 普通作者创建分类直接公开，管理员发现违规再下架
// 以及 @ 提及来源（mention_source）：站长可设站点默认，楼主可对本作品覆盖
// ─────────────────────────────────────────────────────────────

const (
	// 博客分类审核模式
	CategoryReviewModeKey     = "blog_category_review_mode"
	CategoryReviewModePost    = "post_review" // 事后审核（默认）
	CategoryReviewModePreOpen = "pre_public"  // 先公开，事后封禁

	// @ 提及来源模式
	MentionSourceKey            = "mention_source"
	MentionSourceParticipants   = "participants" // 本作品/博客参与者（默认）
	MentionSourceGlobal         = "global"       // 全站用户搜索
	MentionSourceManual         = "manual"       // 仅手动输入，不联想
)

// MentionSourceWorkKey 某作品/博客的 @ 提及来源覆盖键（楼主可设）
func MentionSourceWorkKey(targetType string, targetID uint) string {
	return "mention_source:" + targetType + ":" + strconv.FormatUint(uint64(targetID), 10)
}

// GetMentionSource 站点默认 @ 提及来源
func GetMentionSource() string {
	return GetSetting(MentionSourceKey, MentionSourceParticipants)
}

// GetEffectiveMentionSource 解析某作品/博客生效的 @ 提及来源：优先作品覆盖，否则站点默认
func GetEffectiveMentionSource(targetType string, targetID uint) string {
	if targetType != "" && targetID > 0 {
		if v := GetSetting(MentionSourceWorkKey(targetType, targetID), ""); v != "" {
			return v
		}
	}
	return GetMentionSource()
}

type SystemSetting struct {
	Key       string    `gorm:"primaryKey;size:64" json:"key"`
	Value     string    `gorm:"size:512;default:''" json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (SystemSetting) TableName() string { return "system_settings" }

// GetSetting 读取配置项，不存在返回 def
func GetSetting(key, def string) string {
	var s SystemSetting
	if err := DB.First(&s, "key = ?", key).Error; err != nil {
		return def
	}
	if s.Value == "" {
		return def
	}
	return s.Value
}

// SetSetting 写入配置项（存在则更新）
func SetSetting(key, value string) error {
	var s SystemSetting
	err := DB.Where("key = ?", key).First(&s).Error
	if err == nil {
		s.Value = value
		return DB.Save(&s).Error
	}
	return DB.Create(&SystemSetting{Key: key, Value: value}).Error
}

// GetCategoryReviewMode 获取博客分类审核模式
func GetCategoryReviewMode() string {
	return GetSetting(CategoryReviewModeKey, CategoryReviewModePost)
}
