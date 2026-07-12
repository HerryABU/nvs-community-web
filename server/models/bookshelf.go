package models

import (
	"time"
)

// BookShelf 用户书架（收藏/追读）
type BookShelf struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	UserID          uint      `gorm:"not null;uniqueIndex:uk_user_novel;index" json:"user_id"`
	NovelID         uint      `gorm:"not null;uniqueIndex:uk_user_novel;index" json:"novel_id"`
	LastReadChapter int       `gorm:"default:0" json:"last_read_chapter"` // 最后阅读章节号
	AddedAt         time.Time `gorm:"autoCreateTime" json:"added_at"`

	// 关联
	Novel *Novel `gorm:"foreignKey:NovelID" json:"novel,omitempty"`
}

func (BookShelf) TableName() string {
	return "bookshelf"
}

// AddToShelf 添加到书架（如果已存在则不重复添加）
func AddToShelf(userID, novelID uint) error {
	var existing BookShelf
	result := DB.Where("user_id = ? AND novel_id = ?", userID, novelID).First(&existing)
	if result.Error == nil {
		// 已存在，更新 added_at
		return DB.Model(&existing).Update("added_at", time.Now()).Error
	}
	return DB.Create(&BookShelf{UserID: userID, NovelID: novelID}).Error
}

// RemoveFromShelf 从书架移除
func RemoveFromShelf(userID, novelID uint) error {
	return DB.Where("user_id = ? AND novel_id = ?", userID, novelID).Delete(&BookShelf{}).Error
}

// IsOnShelf 检查是否已在书架
func IsOnShelf(userID, novelID uint) bool {
	var count int64
	DB.Model(&BookShelf{}).Where("user_id = ? AND novel_id = ?", userID, novelID).Count(&count)
	return count > 0
}

// GetShelfList 获取用户书架列表（含作品信息和阅读进度）
func GetShelfList(userID uint, page, pageSize int) ([]BookShelf, int64, error) {
	var items []BookShelf
	var total int64

	query := DB.Model(&BookShelf{}).Where("user_id = ?", userID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Preload("Novel").Preload("Novel.Author").
		Order("added_at DESC").
		Offset(offset).Limit(pageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}

	// 填充每个作品的多分类名
	for i := range items {
		if items[i].Novel != nil {
			items[i].Novel.CategoryNames = GetNovelCategoryNames(items[i].Novel.ID)
		}
	}

	return items, total, nil
}

// UpdateShelfProgress 更新阅读进度
func UpdateShelfProgress(userID, novelID uint, chapterNum int) error {
	return DB.Model(&BookShelf{}).
		Where("user_id = ? AND novel_id = ?", userID, novelID).
		Update("last_read_chapter", chapterNum).Error
}
