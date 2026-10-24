package models

import (
	"strconv"
	"strings"
	"time"
)

// ==================== AI 生成内容标识（176 号令合规） ====================
// 侧表设计：不改动小说章节/博客/论坛/评论主表结构，用 (target_type,target_id) 关联，
// 保存内容时写入，展示时批量查询。记录"用了哪个模型"，不记录用户输入正文。

// AI 内容类型
const (
	AiMarkBlog    = "blog"
	AiMarkChapter = "novel_chapter"
	AiMarkForum   = "forum_post"
	AiMarkComment = "comment"
)

// AiContentMark AI 辅助生成标记
type AiContentMark struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	TargetType string    `gorm:"size:24;not null;index:idx_ai_mark_target,priority:1" json:"target_type"`
	TargetID   uint      `gorm:"not null;index:idx_ai_mark_target,priority:2" json:"target_id"`
	AiModel    string    `gorm:"size:64;default:''" json:"ai_model"` // 上游模型展示名（可空）
	CreatedAt  time.Time `json:"created_at"`
}

func (AiContentMark) TableName() string { return "ai_content_marks" }

// MarkAiContent 记录/更新 AI 标记（幂等：同目标只留一条）
func MarkAiContent(targetType string, targetID uint, aiModel string) error {
	if targetID == 0 || strings.TrimSpace(targetType) == "" {
		return nil
	}
	aiModel = strings.TrimSpace(aiModel)
	if len([]rune(aiModel)) > 64 {
		aiModel = string([]rune(aiModel)[:64])
	}
	var n int64
	DB.Model(&AiContentMark{}).Where("target_type = ? AND target_id = ?", targetType, targetID).Count(&n)
	if n > 0 {
		return DB.Model(&AiContentMark{}).
			Where("target_type = ? AND target_id = ?", targetType, targetID).
			UpdateColumn("ai_model", aiModel).Error
	}
	return DB.Create(&AiContentMark{
		TargetType: targetType, TargetID: targetID, AiModel: aiModel, CreatedAt: time.Now(),
	}).Error
}

// ClearAiContentMark 取消标记（作者选择"这是我写的"时）
func ClearAiContentMark(targetType string, targetID uint) error {
	return DB.Where("target_type = ? AND target_id = ?", targetType, targetID).Delete(&AiContentMark{}).Error
}

// AiMarkItem 查询项
type AiMarkItem struct {
	Type string
	ID   uint
}

// GetAiContentMarks 批量查询标记，返回 map["type:id"]模型名
func GetAiContentMarks(items []AiMarkItem) map[string]string {
	out := map[string]string{}
	if len(items) == 0 {
		return out
	}
	// 按类型分组，IN 查询（避免 N+1）
	byType := map[string][]uint{}
	for _, it := range items {
		if it.Type == "" || it.ID == 0 {
			continue
		}
		byType[it.Type] = append(byType[it.Type], it.ID)
	}
	for t, ids := range byType {
		var rows []AiContentMark
		DB.Where("target_type = ? AND target_id IN ?", t, ids).Find(&rows)
		for _, r := range rows {
			out[t+":"+strconv.FormatUint(uint64(r.TargetID), 10)] = r.AiModel
		}
	}
	return out
}
