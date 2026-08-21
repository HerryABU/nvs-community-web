package models

import (
	"time"

	"gorm.io/gorm"
)

type Comment struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	UserID         uint      `gorm:"not null;index" json:"user_id"`
	NovelID        uint      `gorm:"default:0;index" json:"novel_id"`
	BlogID         uint      `gorm:"default:0;index" json:"blog_id"`
	ChapterNumber  int       `gorm:"default:0" json:"chapter_number"`
	Content        string    `gorm:"type:text;not null" json:"content"`
	QuoteText      string    `gorm:"size:1024;default:''" json:"quote_text"`
	QuoteOffset    int       `gorm:"default:0" json:"quote_offset"`
	ParentID       *uint     `json:"parent_id"`
	IsMarkdown     bool      `gorm:"default:true" json:"is_markdown"`
	Username       string    `gorm:"-" json:"username,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"` // 合规留存：软删除标记

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

// GetAllCommentsByNovel 获取作品的「全部」评论（书评 + 所有章评，按时间倒序）。
// 供内容抢救台等场景一次性拉取作品下所有评论，便于审核/排查。
func GetAllCommentsByNovel(novelID uint, limit int) ([]Comment, error) {
	if limit <= 0 {
		limit = 500
	}
	var comments []Comment
	err := DB.Where("novel_id = ?", novelID).
		Preload("User").
		Order("created_at DESC").
		Limit(limit).
		Find(&comments).Error
	return comments, err
}

func DeleteComment(id uint) error {
	return DB.Delete(&Comment{}, id).Error
}

// GetCommentsByBlog 获取博客评论
func GetCommentsByBlog(blogID uint, page, pageSize int) ([]Comment, int64, error) {
	var comments []Comment
	var total int64
	query := DB.Model(&Comment{}).Where("blog_id = ?", blogID)
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
