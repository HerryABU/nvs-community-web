package models

import (
	"fmt"
	"strings"
	"time"
)

// ==================== 作品广场（灵感广场） ====================
// 作者把成功的绘画/视频任务主动发布到公开广场，其他人可浏览、点赞、一键复刻提示词。

// ListPublicMedia 公开作品分页（kind: image/video；style 为空表示全部）
func ListPublicMedia(kind, style string, page, size int) ([]AiMediaTask, int64, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 60 {
		size = 24
	}
	q := DB.Model(&AiMediaTask{}).Where("is_public = ? AND status = ?", true, AiMediaSucceeded)
	if k := strings.TrimSpace(kind); k != "" {
		q = q.Where("kind = ?", NormalizeAiKind(k))
	}
	if s := strings.TrimSpace(style); s != "" {
		q = q.Where("style = ?", s)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []AiMediaTask
	err := q.Order("likes DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

// PublishAiMedia 发布/取消发布作品（仅作者本人）
func PublishAiMedia(id, userID uint, public bool, title, style string) error {
	var t AiMediaTask
	if err := DB.Where("id = ? AND user_id = ?", id, userID).First(&t).Error; err != nil {
		return fmt.Errorf("作品不存在")
	}
	if t.Status != AiMediaSucceeded {
		return fmt.Errorf("仅成功的作品可发布")
	}
	title = strings.TrimSpace(title)
	if len([]rune(title)) > 128 {
		title = string([]rune(title)[:128])
	}
	if title == "" {
		title = string([]rune(strings.TrimSpace(t.Prompt)))
		if len([]rune(title)) > 40 {
			title = string([]rune(title)[:40]) + "…"
		}
	}
	return DB.Model(&AiMediaTask{}).Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]interface{}{
			"is_public": public, "title": title,
			"style": strings.TrimSpace(style), "updated_at": time.Now(),
		}).Error
}

// LikeAiMedia 点赞（同一用户可多次，前端另做本地去重；此处只累加计数）
func LikeAiMedia(id uint) error {
	res := DB.Model(&AiMediaTask{}).Where("id = ? AND is_public = ?", id, true).
		UpdateColumn("likes", DB.Raw("likes + 1"))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("作品不存在或未公开")
	}
	return nil
}

// GalleryStyles 公开作品出现过的风格标签（用于筛选 chips）
func GalleryStyles(kind string) []string {
	var styles []string
	q := DB.Model(&AiMediaTask{}).Where("is_public = ? AND style <> ''", true)
	if k := strings.TrimSpace(kind); k != "" {
		q = q.Where("kind = ?", NormalizeAiKind(k))
	}
	q.Distinct().Order("style ASC").Pluck("style", &styles)
	return styles
}
