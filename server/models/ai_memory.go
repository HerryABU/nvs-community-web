package models

import (
	"strings"
	"time"
)

// ==================== 长篇 AI 续写上下文记忆（§13.5） ====================
// 按作品维护「大纲 + 章节摘要」两级记忆，AI 调用时按 token 预算组装注入，
// 解决长篇续写「前文遗忘」问题。记忆仅作者本人可读写（handler 层校验归属）。

const (
	// AiMemoryOutline 作品大纲（novel 级唯一）
	AiMemoryOutline = "outline"
	// AiMemoryChapterSummary 章节摘要（按 chapter 唯一）
	AiMemoryChapterSummary = "chapter_summary"
	// AiMemoryCharacters 角色设定（novel 级唯一）：供模板/续写注入「角色信息」
	AiMemoryCharacters = "characters"

	// AiContextBudget 上下文组装默认 token 预算
	AiContextBudget = 3000
	// AiContextRecentChapters 默认注入的最近章节摘要数
	AiContextRecentChapters = 5
	// AiContextTailRunes 当前章尾部窗口（rune 数）
	AiContextTailRunes = 1200
)

// AiNovelMemory 作品级 AI 记忆（大纲 / 章节摘要）
type AiNovelMemory struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	NovelID   uint      `gorm:"uniqueIndex:idx_ai_memory,priority:1;not null" json:"novel_id"`
	Kind      string    `gorm:"size:24;uniqueIndex:idx_ai_memory,priority:2;not null" json:"kind"`         // outline / chapter_summary
	ChapterID uint      `gorm:"uniqueIndex:idx_ai_memory,priority:3;not null;default:0" json:"chapter_id"` // 0=大纲
	Content   string    `gorm:"type:text;not null" json:"content"`
	Tokens    int       `gorm:"not null;default:0" json:"tokens"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (AiNovelMemory) TableName() string { return "ai_novel_memories" }

// UpsertNovelOutline 保存/更新作品大纲
func UpsertNovelOutline(novelID uint, content string) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return DB.Where("novel_id = ? AND kind = ?", novelID, AiMemoryOutline).
			Delete(&AiNovelMemory{}).Error
	}
	return upsertAiMemory(novelID, AiMemoryOutline, 0, content)
}

// GetNovelOutline 读取作品大纲（无则空串）
func GetNovelOutline(novelID uint) string {
	var m AiNovelMemory
	if err := DB.Where("novel_id = ? AND kind = ?", novelID, AiMemoryOutline).First(&m).Error; err != nil {
		return ""
	}
	return m.Content
}

// UpsertNovelCharacters 保存/更新角色设定（人物卡合集，纯文本）
func UpsertNovelCharacters(novelID uint, content string) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return DB.Where("novel_id = ? AND kind = ?", novelID, AiMemoryCharacters).
			Delete(&AiNovelMemory{}).Error
	}
	return upsertAiMemory(novelID, AiMemoryCharacters, 0, content)
}

// GetNovelCharacters 读取角色设定（无则空串）
func GetNovelCharacters(novelID uint) string {
	var m AiNovelMemory
	if err := DB.Where("novel_id = ? AND kind = ?", novelID, AiMemoryCharacters).First(&m).Error; err != nil {
		return ""
	}
	return m.Content
}

// UpsertChapterSummary 保存/更新章节摘要
func UpsertChapterSummary(novelID, chapterID uint, content string) error {
	content = strings.TrimSpace(content)
	if content == "" || chapterID == 0 {
		return nil
	}
	return upsertAiMemory(novelID, AiMemoryChapterSummary, chapterID, content)
}

// GetChapterSummary 读取章节摘要（无则空串）
func GetChapterSummary(novelID, chapterID uint) string {
	if chapterID == 0 {
		return ""
	}
	var m AiNovelMemory
	if err := DB.Where("novel_id = ? AND kind = ? AND chapter_id = ?", novelID, AiMemoryChapterSummary, chapterID).
		First(&m).Error; err != nil {
		return ""
	}
	return m.Content
}

func upsertAiMemory(novelID uint, kind string, chapterID uint, content string) error {
	now := time.Now()
	tokens := EstimateTokens(content)
	return DB.Exec(
		"INSERT INTO ai_novel_memories (novel_id, kind, chapter_id, content, tokens, created_at, updated_at) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?) "+
			"ON CONFLICT(novel_id, kind, chapter_id) DO UPDATE SET content=excluded.content, "+
			"tokens=excluded.tokens, updated_at=excluded.updated_at",
		novelID, kind, chapterID, content, tokens, now, now).Error
}

// ListRecentChapterSummaries 最近 N 条章节摘要（不含 excludeChapter，按章节倒序）
func ListRecentChapterSummaries(novelID, excludeChapter uint, limit int) ([]AiNovelMemory, error) {
	if limit <= 0 {
		limit = AiContextRecentChapters
	}
	var list []AiNovelMemory
	q := DB.Where("novel_id = ? AND kind = ?", novelID, AiMemoryChapterSummary)
	if excludeChapter > 0 {
		q = q.Where("chapter_id <> ?", excludeChapter)
	}
	err := q.Order("chapter_id DESC").Limit(limit).Find(&list).Error
	return list, err
}

// ListNovelMemories 作品全部记忆（管理预览：大纲 + 摘要列表）
func ListNovelMemories(novelID uint) ([]AiNovelMemory, error) {
	var list []AiNovelMemory
	err := DB.Where("novel_id = ?", novelID).
		Order("kind ASC, chapter_id ASC").Find(&list).Error
	return list, err
}

// CountNovelMemories 作品记忆数量（status 展示）
func CountNovelMemories(novelID uint) (outline bool, summaries int64) {
	outline = GetNovelOutline(novelID) != ""
	DB.Model(&AiNovelMemory{}).
		Where("novel_id = ? AND kind = ?", novelID, AiMemoryChapterSummary).
		Count(&summaries)
	return
}

// DeleteChapterMemory 删除某章记忆（章节删除时清理）
func DeleteChapterMemory(novelID, chapterID uint) error {
	return DB.Where("novel_id = ? AND kind = ? AND chapter_id = ?", novelID, AiMemoryChapterSummary, chapterID).
		Delete(&AiNovelMemory{}).Error
}

// ---------- 上下文组装 ----------

// BuildNovelContext 组装长篇上下文（token 预算内）：
//  1. 当前章尾部窗口（固定保留）
//  2. 最近 K 章摘要（新→旧，预算不足从最旧开始丢）
//  3. 作品大纲（仍有余量则附加）
// 返回注入文本与估算 tokens（调用方应计入 tokens_in 预检）。
func BuildNovelContext(novelID, currentChapterID uint, currentText string, budget int) (string, int) {
	if budget <= 0 {
		budget = AiContextBudget
	}

	// 1. 当前章尾部窗口
	runes := []rune(strings.TrimSpace(currentText))
	if len(runes) > AiContextTailRunes {
		runes = runes[len(runes)-AiContextTailRunes:]
	}
	tailPart := ""
	tailTokens := 0
	if len(runes) > 0 {
		tailPart = "【当前章节结尾】\n" + string(runes)
		tailTokens = EstimateTokens(tailPart)
	}

	// 2. 最近 K 章摘要（新→旧遍历，放不下的更旧的直接跳过）
	summaries, _ := ListRecentChapterSummaries(novelID, currentChapterID, AiContextRecentChapters)
	var kept []string
	sumUsed := 0
	for _, s := range summaries {
		line := "【前文摘要】" + s.Content
		t := EstimateTokens(line)
		if tailTokens+sumUsed+t > budget {
			continue
		}
		kept = append(kept, line)
		sumUsed += t
	}
	// kept 为新→旧，倒序为旧→新符合阅读顺序
	for i, j := 0, len(kept)-1; i < j; i, j = i+1, j-1 {
		kept[i], kept[j] = kept[j], kept[i]
	}

	// 3. 大纲（有余量才附加）
	outlinePart := ""
	used := tailTokens + sumUsed
	if outline := GetNovelOutline(novelID); outline != "" {
		part := "【作品大纲】\n" + outline
		if t := EstimateTokens(part); used+t <= budget {
			outlinePart = part
			used += t
		}
	}

	if tailPart == "" && len(kept) == 0 && outlinePart == "" {
		return "", 0
	}
	var parts []string
	if outlinePart != "" {
		parts = append(parts, outlinePart)
	}
	if len(kept) > 0 {
		parts = append(parts, "【前文脉络】\n"+strings.Join(kept, "\n"))
	}
	if tailPart != "" {
		parts = append(parts, tailPart)
	}
	return strings.Join(parts, "\n\n"), used
}
