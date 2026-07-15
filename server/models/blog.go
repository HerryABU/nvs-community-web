package models

import "time"

// AuthorBlog 作者博客文章
type AuthorBlog struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	AuthorID    uint      `gorm:"not null;index:idx_author_blog" json:"author_id"`
	Title       string    `gorm:"size:256;not null" json:"title"`
	Content     string    `gorm:"-" json:"content,omitempty"`
	ContentPath string    `gorm:"size:512;not null" json:"-"`
	ContentHash string    `gorm:"size:64;default:''" json:"content_hash,omitempty"`
	Summary     string    `gorm:"size:512;default:''" json:"summary"`
	IsPinned    bool      `gorm:"default:false;index" json:"is_pinned"`
	ViewCount   int       `gorm:"default:0" json:"view_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

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

func ListAllBlogs(page, pageSize int) ([]AuthorBlog, int64, error) {
	var blogs []AuthorBlog
	var total int64

	query := DB.Model(&AuthorBlog{})
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
