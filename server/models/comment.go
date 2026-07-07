package models

import (
	"time"
)

type Comment struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	UserID         uint      `gorm:"not null;index" json:"user_id"`
	NovelID        uint      `gorm:"not null;index" json:"novel_id"`
	ChapterNumber  int       `gorm:"default:0" json:"chapter_number"`
	Content        string    `gorm:"type:text;not null" json:"content"`
	QuoteText      string    `gorm:"size:1024;default:''" json:"quote_text"`
	QuoteOffset    int       `gorm:"default:0" json:"quote_offset"`
	ParentID       *uint     `json:"parent_id"`
	IsMarkdown     bool      `gorm:"default:true" json:"is_markdown"`
	Username       string    `gorm:"-" json:"username,omitempty"`
	CreatedAt      time.Time `json:"created_at"`

	// 关联
	User   *User     `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Novel  *Novel    `gorm:"foreignKey:NovelID" json:"novel,omitempty"`
	Parent *Comment  `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
}

func (Comment) TableName() string {
	return "comments"
}

func CreateComment(comment *Comment) error {
	return DB.Create(comment).Error
}

func GetCommentsByNovel(novelID uint, chapterNumber int, page, pageSize int) ([]Comment, int64, error) {
	var comments []Comment
	var total int64

	query := DB.Model(&Comment{}).Where("novel_id = ?", novelID)

	if chapterNumber > 0 {
		query = query.Where("chapter_number = ?", chapterNumber)
	} else {
		// 如果没有指定章节号，只查作品级评论（chapter_number = 0）
		query = query.Where("chapter_number = 0")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Preload("User").Preload("Parent").
		Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&comments).Error; err != nil {
		return nil, 0, err
	}

	return comments, total, nil
}

func GetCommentByID(id uint) (*Comment, error) {
	var comment Comment
	err := DB.Preload("User").First(&comment, id).Error
	if err != nil {
		return nil, err
	}
	return &comment, nil
}

func DeleteComment(id uint) error {
	return DB.Delete(&Comment{}, id).Error
}
