package feed

import (
	"strings"
	"sync"
	"time"

	"nvs-server/models"
)

// Tag 动态标签的倒排索引。
//
// Post.Tags 是逗号分隔字符串，LIKE '%x%' 既会误命中（"测试" 命中 "测试版"）
// 也不能走索引。高性能机器上按标签筛选必须走这张表的 tag 索引。
type Tag struct {
	ID        uint      `gorm:"primaryKey"`
	PostID    uint      `gorm:"not null;uniqueIndex:idx_feed_tag_post_tag;index:idx_feed_tag_post" json:"post_id"`
	Tag       string    `gorm:"size:64;not null;uniqueIndex:idx_feed_tag_post_tag;index:idx_feed_tag_name" json:"tag"`
	CreatedAt time.Time `json:"-"`
}

func (Tag) TableName() string { return "feed_tags" }

// ParseTags 统一中英文逗号分隔，去空、去重、限长，与前端 splitTags 对齐
func ParseTags(raw string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == '，' }) {
		t := strings.TrimSpace(part)
		if t == "" || seen[t] {
			continue
		}
		if len([]rune(t)) > 32 {
			t = string([]rune(t)[:32])
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

// SyncPostTags 发布/编辑动态时同步倒排索引；先删后插，调用方在事务外也能安全重试。
func SyncPostTags(postID uint, raw string) error {
	if models.DB == nil {
		return nil
	}
	tags := ParseTags(raw)
	if err := models.DB.Where("post_id = ?", postID).Delete(&Tag{}).Error; err != nil {
		return err
	}
	if len(tags) == 0 {
		return nil
	}
	rows := make([]Tag, len(tags))
	for i, t := range tags {
		rows[i] = Tag{PostID: postID, Tag: t}
	}
	return models.DB.Create(&rows).Error
}

var rebuildOnce sync.Once

// EnsureTagIndex 首次按标签查询时补齐历史数据的倒排索引（老库没有这张表的数据）。
// 只在空表时重建，之后靠 SyncPostTags 增量维护。
func EnsureTagIndex() {
	rebuildOnce.Do(func() {
		if models.DB == nil {
			return
		}
		var n int64
		if models.DB.Model(&Tag{}).Count(&n) == nil && n > 0 {
			return
		}
		RebuildTagIndex()
	})
}

// RebuildTagIndex 从 posts.tags 全量重建（迁移/修复入口）
func RebuildTagIndex() error {
	if models.DB == nil {
		return nil
	}
	if err := models.DB.Where("1 = 1").Delete(&Tag{}).Error; err != nil {
		return err
	}
	var posts []Post
	if err := models.DB.Select("id, tags").Find(&posts).Error; err != nil {
		return err
	}
	for _, p := range posts {
		if err := SyncPostTags(p.ID, p.Tags); err != nil {
			return err
		}
	}
	return nil
}
