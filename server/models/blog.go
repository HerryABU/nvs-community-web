package models

import (
	"time"

	"gorm.io/gorm"
)

// AuthorBlog 作者博客文章
type AuthorBlog struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	AuthorID    uint      `gorm:"not null;index:idx_author_blog" json:"author_id"`
	Title       string    `gorm:"size:256;not null" json:"title"`
	Content     string    `gorm:"-" json:"content,omitempty"`
	ContentPath string    `gorm:"size:512;not null" json:"-"`
	ContentHash string    `gorm:"size:64;default:''" json:"content_hash,omitempty"`
	Summary     string    `gorm:"size:512;default:''" json:"summary"`
	CategoryID  uint      `gorm:"default:0;index:idx_blog_category" json:"category_id"` // 博客分类（blog_categories 表，0=未分类）
	Category    string    `gorm:"-" json:"category,omitempty"`                         // 运行时填充：分类名（含父级前缀）
	LikeCount   int       `gorm:"default:0" json:"like_count"`                         // 点赞数（blog_likes 冗余计数）
	IsPinned    bool      `gorm:"default:false;index" json:"is_pinned"`
	ViewCount   int       `gorm:"default:0" json:"view_count"`
	DeletedBy   string    `gorm:"size:16;default:''" json:"deleted_by,omitempty"` // user=用户自删 / admin=管理员封禁删除 / 空=未删除或旧数据
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"` // 合规留存：软删除标记

	Author *User `gorm:"foreignKey:AuthorID" json:"author,omitempty"`
}

func (AuthorBlog) TableName() string { return "author_blogs" }

func CreateBlog(blog *AuthorBlog) error {
	err := DB.Create(blog).Error
	return err
}

func GetBlogByID(id uint) (*AuthorBlog, error) {
	var blog AuthorBlog
	err := DB.Preload("Author").First(&blog, id).Error
	if err != nil {
		return nil, err
	}
	return &blog, nil
}

func GetBlogsByAuthor(authorID uint, page, pageSize int) ([]AuthorBlog, int64, error) {
	var blogs []AuthorBlog
	var total int64

	query := DB.Model(&AuthorBlog{}).Where("author_id = ?", authorID)
	query.Count(&total)

	offset := (page - 1) * pageSize
	err := query.Preload("Author").Order("is_pinned DESC, created_at DESC").
		Offset(offset).Limit(pageSize).Find(&blogs).Error
	return blogs, total, err
}

func UpdateBlog(blog *AuthorBlog) error {
	return DB.Save(blog).Error
}

func DeleteBlog(id uint) error {
	return DB.Delete(&AuthorBlog{}, id).Error
}

// ListAllBlogs 博客列表（categoryID>0 时按分类过滤；一级分类自动包含其下全部子孙分类）
func ListAllBlogs(categoryID uint, page, pageSize int) ([]AuthorBlog, int64, error) {
	var blogs []AuthorBlog
	var total int64

	query := DB.Model(&AuthorBlog{})
	if categoryID > 0 {
		// 收集该分类全部子孙（最多三级），过滤其下所有博客
		ids := []uint{categoryID}
		var kids1 []uint
		DB.Model(&BlogCategory{}).Where("parent_id = ?", categoryID).Pluck("id", &kids1)
		ids = append(ids, kids1...)
		if len(kids1) > 0 {
			var kids2 []uint
			DB.Model(&BlogCategory{}).Where("parent_id IN ?", kids1).Pluck("id", &kids2)
			ids = append(ids, kids2...)
		}
		query = query.Where("category_id IN ?", ids)
	}
	query.Count(&total)

	offset := (page - 1) * pageSize
	err := query.Preload("Author").Order("is_pinned DESC, created_at DESC").
		Offset(offset).Limit(pageSize).Find(&blogs).Error
	// 脱敏
	for i := range blogs {
		if blogs[i].Author != nil {
			blogs[i].Author.Email = ""
		}
	}
	return blogs, total, err
}

func IncrementBlogView(id uint) {
	DB.Model(&AuthorBlog{}).Where("id = ?", id).UpdateColumn("view_count", DB.Raw("view_count + 1"))
}
