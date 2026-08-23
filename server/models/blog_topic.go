package models

import (
	"time"

	"gorm.io/gorm"
)

// ─────────────────────────────────────────────────────────────
// 博客话题（#话题，CSDN 式）：比标签更正式的内容聚合单元
//   blog_topics：话题字典（name 唯一）
//   blog_topic_links：博客 ↔ 话题 多对多
// ─────────────────────────────────────────────────────────────

type BlogTopic struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:48;not null;uniqueIndex" json:"name"`
	Description string    `gorm:"size:256;default:''" json:"description"`
	CreatedBy   uint      `gorm:"default:0" json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

func (BlogTopic) TableName() string { return "blog_topics" }

type BlogTopicLink struct {
	BlogID  uint `gorm:"primaryKey" json:"blog_id"`
	TopicID uint `gorm:"primaryKey" json:"topic_id"`
}

func (BlogTopicLink) TableName() string { return "blog_topic_links" }

// SyncBlogTopics 为博客同步话题（按名创建缺失话题并重建关联；空=清空）
func SyncBlogTopics(blogID uint, topicNames []string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("blog_id = ?", blogID).Delete(&BlogTopicLink{}).Error; err != nil {
			return err
		}
		seen := map[string]bool{}
		for _, raw := range topicNames {
			name := trimTopicName(raw)
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			var tp BlogTopic
			err := tx.Where("name = ?", name).First(&tp).Error
			if err != nil {
				tp = BlogTopic{Name: name}
				if err := tx.Create(&tp).Error; err != nil {
					continue
				}
			}
			if err := tx.Create(&BlogTopicLink{BlogID: blogID, TopicID: tp.ID}).Error; err != nil {
				continue
			}
		}
		return nil
	})
}

func trimTopicName(s string) string {
	t := s
	for len(t) > 0 && (t[0] == '#' || t[0] == ' ' || t[0] == '\t') {
		t = t[1:]
	}
	t = string([]rune(t))
	for len(t) > 0 && (t[len(t)-1] == ' ' || t[len(t)-1] == '\t') {
		t = t[:len(t)-1]
	}
	if len([]rune(t)) > 48 {
		t = string([]rune(t)[:48])
	}
	return t
}

// GetBlogTopics 取单篇博客的话题
func GetBlogTopics(blogID uint) []BlogTopic {
	var topics []BlogTopic
	DB.Joins("JOIN blog_topic_links ON blog_topic_links.topic_id = blog_topics.id").
		Where("blog_topic_links.blog_id = ?", blogID).
		Order("blog_topics.id ASC").Find(&topics)
	return topics
}

// FillBlogTopics 批量填充博客话题
func FillBlogTopics(blogs []AuthorBlog) {
	if len(blogs) == 0 {
		return
	}
	ids := make([]uint, 0, len(blogs))
	for i := range blogs {
		ids = append(ids, blogs[i].ID)
	}
	var links []BlogTopicLink
	DB.Where("blog_id IN ?", ids).Find(&links)
	if len(links) == 0 {
		return
	}
	topicIDs := make([]uint, 0, len(links))
	for _, l := range links {
		topicIDs = append(topicIDs, l.TopicID)
	}
	var topics []BlogTopic
	DB.Where("id IN ?", topicIDs).Find(&topics)
	topicOf := map[uint]BlogTopic{}
	for _, t := range topics {
		topicOf[t.ID] = t
	}
	byBlog := map[uint][]BlogTopic{}
	for _, l := range links {
		if t, ok := topicOf[l.TopicID]; ok {
			byBlog[l.BlogID] = append(byBlog[l.BlogID], t)
		}
	}
	for i := range blogs {
		if ts, ok := byBlog[blogs[i].ID]; ok {
			blogs[i].Topics = ts
		} else {
			blogs[i].Topics = []BlogTopic{}
		}
	}
}

// TopicWithCount 话题 + 文章数
type TopicWithCount struct {
	BlogTopic
	BlogCount int64 `json:"blog_count"`
}

// ListBlogTopics 全站话题列表（按文章数降序；仅含未删除博客）
func ListBlogTopics(limit int) []TopicWithCount {
	if limit <= 0 {
		limit = 50
	}
	rows := []TopicWithCount{}
	DB.Table("blog_topics").
		Select("blog_topics.*, COUNT(blog_topic_links.blog_id) AS blog_count").
		Joins("JOIN blog_topic_links ON blog_topic_links.topic_id = blog_topics.id").
		Joins("JOIN author_blogs ON author_blogs.id = blog_topic_links.blog_id").
		Where("author_blogs.deleted_at IS NULL").
		Group("blog_topics.id").
		Order("blog_count DESC, blog_topics.id ASC").
		Limit(limit).Scan(&rows)
	return rows
}

// GetBlogTopicByName 按名取话题
func GetBlogTopicByName(name string) (*BlogTopic, error) {
	var t BlogTopic
	if err := DB.Where("name = ?", name).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// GetBlogTopicByID 按 id 取话题
func GetBlogTopicByID(id uint) (*BlogTopic, error) {
	var t BlogTopic
	if err := DB.First(&t, id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}
