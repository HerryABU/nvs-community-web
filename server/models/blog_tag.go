package models

import (
	"time"

	"gorm.io/gorm"
)

// ─────────────────────────────────────────────────────────────
// 博客标签（CSDN 式 #tag）：作者打标签，全站自动聚合
//   blog_tags：标签字典（name 唯一，首个创建者记录）
//   blog_tag_links：博客 ↔ 标签 多对多关联
// ─────────────────────────────────────────────────────────────

type BlogTag struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:32;not null;uniqueIndex" json:"name"`
	CreatedBy uint      `gorm:"default:0" json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

func (BlogTag) TableName() string { return "blog_tags" }

type BlogTagLink struct {
	BlogID uint `gorm:"primaryKey" json:"blog_id"`
	TagID  uint `gorm:"primaryKey" json:"tag_id"`
}

func (BlogTagLink) TableName() string { return "blog_tag_links" }

// SyncBlogTags 为博客同步标签：按名创建缺失标签并重建关联。
// tags 为空则清空全部关联。
func SyncBlogTags(blogID uint, tags []string) error {
	// 重建关联（事务）
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("blog_id = ?", blogID).Delete(&BlogTagLink{}).Error; err != nil {
			return err
		}
		seen := map[string]bool{}
		for _, raw := range tags {
			name := trimTagName(raw)
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			var tag BlogTag
			err := tx.Where("name = ?", name).First(&tag).Error
			if err != nil {
				tag = BlogTag{Name: name}
				if err := tx.Create(&tag).Error; err != nil {
					continue
				}
			}
			if err := tx.Create(&BlogTagLink{BlogID: blogID, TagID: tag.ID}).Error; err != nil {
				continue
			}
		}
		return nil
	})
}

func trimTagName(s string) string {
	t := s
	for len(t) > 0 && (t[0] == '#' || t[0] == ' ' || t[0] == '\t') {
		t = t[1:]
	}
	t = string([]rune(t))
	for len(t) > 0 && (t[len(t)-1] == ' ' || t[len(t)-1] == '\t') {
		t = t[:len(t)-1]
	}
	if len([]rune(t)) > 32 {
		t = string([]rune(t)[:32])
	}
	return t
}

// GetBlogTags 取单篇博客的标签
func GetBlogTags(blogID uint) []BlogTag {
	var tags []BlogTag
	DB.Joins("JOIN blog_tag_links ON blog_tag_links.tag_id = blog_tags.id").
		Where("blog_tag_links.blog_id = ?", blogID).
		Order("blog_tags.id ASC").Find(&tags)
	return tags
}

// FillBlogTags 批量填充博客标签
func FillBlogTags(blogs []AuthorBlog) {
	if len(blogs) == 0 {
		return
	}
	ids := make([]uint, 0, len(blogs))
	for i := range blogs {
		ids = append(ids, blogs[i].ID)
	}
	var links []BlogTagLink
	DB.Where("blog_id IN ?", ids).Find(&links)
	if len(links) == 0 {
		return
	}
	tagIDs := make([]uint, 0, len(links))
	for _, l := range links {
		tagIDs = append(tagIDs, l.TagID)
	}
	var tags []BlogTag
	DB.Where("id IN ?", tagIDs).Find(&tags)
	tagOf := map[uint]BlogTag{}
	for _, t := range tags {
		tagOf[t.ID] = t
	}
	byBlog := map[uint][]BlogTag{}
	for _, l := range links {
		if t, ok := tagOf[l.TagID]; ok {
			byBlog[l.BlogID] = append(byBlog[l.BlogID], t)
		}
	}
	for i := range blogs {
		if ts, ok := byBlog[blogs[i].ID]; ok {
			blogs[i].Tags = ts
		} else {
			blogs[i].Tags = []BlogTag{}
		}
	}
}

// GetBlogTagCloud 全站标签云：标签名 + 文章数（按文章数降序）
func GetBlogTagCloud(limit int) []struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
} {
	if limit <= 0 {
		limit = 30
	}
	rows := []struct {
		Name  string `json:"name"`
		Count int64  `json:"count"`
	}{}
	DB.Table("blog_tags").
		Select("blog_tags.name, COUNT(blog_tag_links.blog_id) AS count").
		Joins("JOIN blog_tag_links ON blog_tag_links.tag_id = blog_tags.id").
		Joins("JOIN author_blogs ON author_blogs.id = blog_tag_links.blog_id").
		Where("author_blogs.deleted_at IS NULL").
		Group("blog_tags.id").
		Order("count DESC, blog_tags.id ASC").
		Limit(limit).Scan(&rows)
	return rows
}

// GetBlogIDsByTag 按标签名查博客 id 集合（未删除）
func GetBlogIDsByTag(tagName string) ([]uint, error) {
	var ids []uint
	err := DB.Table("blog_tag_links").
		Select("blog_tag_links.blog_id").
		Joins("JOIN blog_tags ON blog_tags.id = blog_tag_links.tag_id").
		Joins("JOIN author_blogs ON author_blogs.id = blog_tag_links.blog_id").
		Where("blog_tags.name = ? AND author_blogs.deleted_at IS NULL", tagName).
		Pluck("blog_tag_links.blog_id", &ids).Error
	return ids, err
}



