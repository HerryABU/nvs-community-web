package models

import "time"

// Announcement 站点公告（公告栏）。
// 管理后台维护，前台首页/公告栏展示；置顶公告优先显示。
type Announcement struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	Title     string     `gorm:"size:256;not null" json:"title"` // 公告标题
	Content   string     `gorm:"type:text" json:"content"`       // 公告内容（支持 Markdown）
	Pinned    bool       `gorm:"default:false" json:"pinned"`    // 是否置顶
	Published bool       `gorm:"default:true" json:"published"`  // 是否发布（false=草稿不展示）
	CreatedBy uint       `json:"created_by"`                     // 发布管理员 ID
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"-"` // 软删除
}

func (Announcement) TableName() string { return "announcements" }

// ListPublishedAnnouncements 前台获取已发布公告（置顶优先，时间倒序）
func ListPublishedAnnouncements(limit int) []Announcement {
	var list []Announcement
	if limit <= 0 {
		limit = 20
	}
	DB.Where("published = ?", true).
		Order("pinned DESC, created_at DESC").
		Limit(limit).
		Find(&list)
	return list
}
