package models

import (
	"strings"
	"time"
)

// BadWord 违禁词规则（管理员可配置，动态生效，无需改代码）
// 匹配方式 match_type：literal（字面关键词，自动转义）/ regex（Go RE2 正则）
// 动作 action：mark（标记待审，不删不改，写入 BadWordFlag 由管理员处理）
//
//	/ delete（删除匹配内容）/ replace（替换为 Replacement）
//
// 适用范围 scope：all（全部）或逗号分隔组合 chapter,comment,forum,blog
type BadWord struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:255;not null" json:"name"`             // 规则名称
	Category    string    `gorm:"size:32;default:''" json:"category"`        // 分类：政治/色情/暴力/赌博/诈骗/广告/违禁品/辱骂/其他（可扩展）
	Tags        string    `gorm:"size:255;default:''" json:"tags"`           // 组合标签（逗号分隔），用于「不同类型组合」的二次分类
	MatchType   string    `gorm:"size:32;default:literal" json:"match_type"` // literal / regex
	Pattern     string    `gorm:"type:text;not null" json:"pattern"`         // 关键词或正则表达式
	Action      string    `gorm:"size:32;default:mark" json:"action"`        // mark / delete / replace
	Replacement string    `gorm:"type:text" json:"replacement"`              // 仅 replace 动作时使用
	Scope       string    `gorm:"size:64;default:all" json:"scope"`          // all / chapter,comment,forum,blog
	Enabled     bool      `gorm:"default:true" json:"enabled"`               // 是否启用
	Description string    `gorm:"type:text" json:"description"`              // 规则说明
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// BadWordImportItem JSON 导入/导出结构（不含主键与时间戳）
type BadWordImportItem struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	Tags        string `json:"tags"`
	MatchType   string `json:"match_type"`
	Pattern     string `json:"pattern"`
	Action      string `json:"action"`
	Replacement string `json:"replacement"`
	Scope       string `json:"scope"`
	Enabled     *bool  `json:"enabled"`
	Description string `json:"description"`
}

func (BadWord) TableName() string { return "bad_words" }

// ListBadWords 列出所有违禁词规则（含禁用）
func ListBadWords() ([]BadWord, error) {
	var words []BadWord
	err := DB.Order("id asc").Find(&words).Error
	return words, err
}

// ListEnabledBadWords 列出所有启用的违禁词规则
func ListEnabledBadWords() ([]BadWord, error) {
	var words []BadWord
	err := DB.Where("enabled = ?", true).Order("id asc").Find(&words).Error
	return words, err
}

// GetBadWord 获取单个规则
func GetBadWord(id uint) (*BadWord, error) {
	var word BadWord
	if err := DB.First(&word, id).Error; err != nil {
		return nil, err
	}
	return &word, nil
}

// CreateBadWord 创建规则
func CreateBadWord(word *BadWord) error {
	return DB.Create(word).Error
}

// UpdateBadWord 更新规则
func UpdateBadWord(word *BadWord) error {
	return DB.Save(word).Error
}

// DeleteBadWord 删除规则
func DeleteBadWord(id uint) error {
	return DB.Delete(&BadWord{}, id).Error
}

// ════════════════ 导入 / 导出（JSON 互操作） ════════════════

// ExportBadWords 导出全部规则（含禁用），供前端下载为 JSON
func ExportBadWords() ([]BadWord, error) {
	return ListBadWords()
}

// ImportBadWords 从 JSON 批量导入：按 (name+match_type+pattern) 去重，
// 已存在（同名同方式同词）则跳过，否则新建。返回创建/跳过计数。
func ImportBadWords(items []BadWordImportItem) (created, skipped int, err error) {
	if len(items) == 0 {
		return 0, 0, nil
	}
	// 预载已有规则用于去重
	existing, err := ListBadWords()
	if err != nil {
		return 0, 0, err
	}
	seen := make(map[string]bool, len(existing))
	for _, w := range existing {
		seen[keyOf(w.Name, w.MatchType, w.Pattern)] = true
	}
	for _, it := range items {
		name := strings.TrimSpace(it.Name)
		pattern := strings.TrimSpace(it.Pattern)
		if name == "" || pattern == "" {
			skipped++
			continue
		}
		mt := it.MatchType
		if mt != "regex" {
			mt = "literal"
		}
		action := it.Action
		if action != "mark" && action != "delete" && action != "replace" {
			action = "mark"
		}
		scope := strings.TrimSpace(it.Scope)
		if scope == "" {
			scope = "all"
		}
		k := keyOf(name, mt, pattern)
		if seen[k] {
			skipped++
			continue
		}
		enabled := true
		if it.Enabled != nil {
			enabled = *it.Enabled
		}
		word := &BadWord{
			Name:        name,
			Category:    strings.TrimSpace(it.Category),
			Tags:        strings.TrimSpace(it.Tags),
			MatchType:   mt,
			Pattern:     pattern,
			Action:      action,
			Replacement: it.Replacement,
			Scope:       scope,
			Enabled:     enabled,
			Description: it.Description,
		}
		if err := DB.Create(word).Error; err != nil {
			return created, skipped, err
		}
		seen[k] = true
		created++
	}
	return created, skipped, nil
}

func keyOf(name, mt, pattern string) string {
	return name + "\x00" + mt + "\x00" + pattern
}

// ==================== 违禁词标记（待审） ====================

// BadWordFlag 违禁词命中标记记录：mark 动作命中后写入，进入管理员待审列表
type BadWordFlag struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	ContentType  string    `gorm:"size:32;not null;index" json:"content_type"`  // comment / chapter / forum / blog
	ContentID    uint      `gorm:"not null" json:"content_id"`                  // 对应内容 ID
	UserID       uint      `gorm:"not null;index" json:"user_id"`               // 内容作者
	ContentTitle string    `gorm:"size:256" json:"content_title"`               // 内容标题/摘要（评论为片段前缀，章节/帖子为标题）
	Snippet      string    `gorm:"type:text" json:"snippet"`                    // 命中片段快照
	MatchedWords string    `gorm:"type:text" json:"matched_words"`              // 命中的违禁词列表（JSON 数组）
	Status       string    `gorm:"size:16;default:pending;index" json:"status"` // pending（待审）/ confirmed（确认违规）/ dismissed（放行）
	Note         string    `gorm:"type:text" json:"note"`                       // 管理员处理备注
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (BadWordFlag) TableName() string { return "bad_word_flags" }

// ListBadWordFlags 查询标记记录，status 为空查全部，按创建时间倒序
func ListBadWordFlags(status string, limit, offset int) ([]BadWordFlag, error) {
	var flags []BadWordFlag
	q := DB.Order("created_at desc")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Limit(limit).Offset(offset).Find(&flags).Error
	return flags, err
}

// CountBadWordFlags 统计标记记录数量
func CountBadWordFlags(status string) (int64, error) {
	var count int64
	q := DB.Model(&BadWordFlag{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Count(&count).Error
	return count, err
}

// GetBadWordFlag 获取单个标记记录
func GetBadWordFlag(id uint) (*BadWordFlag, error) {
	var flag BadWordFlag
	if err := DB.First(&flag, id).Error; err != nil {
		return nil, err
	}
	return &flag, nil
}

// CreateBadWordFlag 创建标记记录
func CreateBadWordFlag(flag *BadWordFlag) error {
	return DB.Create(flag).Error
}

// UpdateBadWordFlag 更新标记记录
func UpdateBadWordFlag(flag *BadWordFlag) error {
	return DB.Save(flag).Error
}

// HasPendingFlag 判断某内容是否已有待审标记（避免重复标记）
func HasPendingFlag(contentType string, contentID uint) (bool, error) {
	var count int64
	err := DB.Model(&BadWordFlag{}).
		Where("content_type = ? AND content_id = ? AND status = ?", contentType, contentID, "pending").
		Count(&count).Error
	return count > 0, err
}
