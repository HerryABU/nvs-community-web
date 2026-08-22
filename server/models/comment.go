package models

import (
	"strconv"
	"time"

	"gorm.io/gorm"
)

type Comment struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	UserID         uint      `gorm:"not null;index" json:"user_id"`
	NovelID        uint      `gorm:"default:0;index:idx_comments_novel_created,priority:1" json:"novel_id"`
	BlogID         uint      `gorm:"default:0;index" json:"blog_id"`
	ChapterNumber  int       `gorm:"default:0" json:"chapter_number"`
	Content        string    `gorm:"type:text;not null" json:"content"`
	QuoteText      string    `gorm:"size:1024;default:''" json:"quote_text"`
	QuoteOffset    int       `gorm:"default:0" json:"quote_offset"`
	ParentID       *uint     `gorm:"index:idx_comments_parent" json:"parent_id"`
	IsMarkdown     bool      `gorm:"default:true" json:"is_markdown"`
	Username       string    `gorm:"-" json:"username,omitempty"`
	DeletedBy      string    `gorm:"size:16;default:''" json:"deleted_by,omitempty"` // user=用户自删 / admin=管理员封禁删除 / 空=未删除或旧数据
	CreatedAt      time.Time `gorm:"index:idx_comments_novel_created,priority:2" json:"created_at"`
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
	if err := DB.Create(comment).Error; err != nil {
		return err
	}
	// 站内通知（best-effort，失败不影响评论本身）
	content := comment.Content
	if len(content) > 60 {
		content = content[:60] + "…"
	}
	// 1) 评论被回复 → 通知被回复评论的作者
	if comment.ParentID != nil && *comment.ParentID != 0 {
		var parent Comment
		if DB.Select("user_id").First(&parent, *comment.ParentID).Error == nil {
			_ = PushNotification(comment.UserID, parent.UserID, "comment_reply",
				"你的评论收到回复", content, linkForComment(comment))
		}
	}
	// 2) 作品收到新评论 → 通知作品作者
	if comment.NovelID != 0 {
		var n Novel
		if DB.Select("author_id", "title").First(&n, comment.NovelID).Error == nil {
			_ = PushNotification(comment.UserID, n.AuthorID, "novel_comment",
				"你的作品收到新评论", "《"+n.Title+"》", "/novel/"+strconv.FormatUint(uint64(comment.NovelID), 10))
		}
	}
	// 3) 博客收到新评论 → 通知博客作者
	if comment.BlogID != 0 {
		var b AuthorBlog
		if DB.Select("author_id", "title").First(&b, comment.BlogID).Error == nil {
			_ = PushNotification(comment.UserID, b.AuthorID, "blog_comment",
				"你的博客收到新评论", "《"+b.Title+"》", "/author/"+strconv.FormatUint(uint64(b.AuthorID), 10)+"/blogs/"+strconv.FormatUint(uint64(comment.BlogID), 10))
		}
	}
	return nil
}

// linkForComment 评论跳转链接：章评带章节号，作品级评论不带
func linkForComment(c *Comment) string {
	base := "/novel/" + strconv.FormatUint(uint64(c.NovelID), 10)
	if c.NovelID == 0 {
		return ""
	}
	if c.ChapterNumber > 0 {
		base += "/read/" + strconv.Itoa(c.ChapterNumber)
	}
	return base + "#comment-" + strconv.FormatUint(uint64(c.ID), 10)
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
