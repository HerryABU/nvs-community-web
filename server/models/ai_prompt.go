package models

import (
	"fmt"
	"strings"
	"time"
)

// ==================== AI 提示词库（§5.4 提示词管理） ====================
// 平台预设（UserID=0，Builtin=true，只读）+ 用户自定义（UserID=本人，可增删改）。
// 按用途分类：chat（对话/写作）、image（绘画）、video（视频）。

// AiPrompt 提示词
type AiPrompt struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;default:0;index:idx_ai_prompt_user" json:"user_id"` // 0 = 平台预设
	Kind      string    `gorm:"size:16;not null;default:chat;index" json:"kind"`            // chat/image/video
	Category  string    `gorm:"size:32;default:''" json:"category"`                         // 分组（写作/翻译/绘画风格…）
	Title     string    `gorm:"size:64;not null" json:"title"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	Builtin   bool      `gorm:"not null;default:false" json:"builtin"`
	SortOrder int       `gorm:"not null;default:0" json:"sort_order"`
	UsedCount int64     `gorm:"not null;default:0" json:"used_count"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (AiPrompt) TableName() string { return "ai_prompts" }

const (
	AiPromptKindChat  = "chat"
	AiPromptKindImage = "image"
	AiPromptKindVideo = "video"
	// MaxUserPrompts 每个用户自定义提示词上限（防滥用撑表）
	MaxUserPrompts = 200
)

// NormalizePromptKind 归一化用途（非法/空 → chat）
func NormalizePromptKind(kind string) string {
	switch strings.TrimSpace(kind) {
	case AiPromptKindImage:
		return AiPromptKindImage
	case AiPromptKindVideo:
		return AiPromptKindVideo
	default:
		return AiPromptKindChat
	}
}

// ---------- 查询 ----------

// ListAiPrompts 我的提示词 + 平台预设（kind 为空则全部）
func ListAiPrompts(userID uint, kind, keyword string) ([]AiPrompt, error) {
	q := DB.Model(&AiPrompt{}).Where("user_id = ? OR user_id = 0", userID)
	if k := strings.TrimSpace(kind); k != "" {
		q = q.Where("kind = ?", NormalizePromptKind(k))
	}
	if kw := strings.TrimSpace(keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("title LIKE ? OR content LIKE ? OR category LIKE ?", like, like, like)
	}
	var list []AiPrompt
	// 自定义优先，其次预设；同组按 sort_order
	err := q.Order("builtin ASC, sort_order DESC, id DESC").Limit(500).Find(&list).Error
	return list, err
}

// GetAiPrompt 取提示词（本人或平台预设）
func GetAiPrompt(id, userID uint) (*AiPrompt, error) {
	var p AiPrompt
	if err := DB.Where("id = ? AND (user_id = ? OR user_id = 0)", id, userID).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// ListAiPromptCategories 提示词库分类（去重、非空）
func ListAiPromptCategories(userID uint) []string {
	var cats []string
	DB.Model(&AiPrompt{}).
		Where("(user_id = ? OR user_id = 0) AND category <> ''", userID).
		Distinct().Order("category ASC").Pluck("category", &cats)
	return cats
}

// ---------- 增删改 ----------

// CreateAiPrompt 新建自定义提示词
func CreateAiPrompt(userID uint, kind, category, title, content string) (*AiPrompt, error) {
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	if title == "" {
		return nil, fmt.Errorf("标题不能为空")
	}
	if content == "" {
		return nil, fmt.Errorf("提示词内容不能为空")
	}
	if len([]rune(title)) > 64 {
		title = string([]rune(title)[:64])
	}
	if len([]rune(content)) > 8000 {
		return nil, fmt.Errorf("提示词过长（上限 8000 字）")
	}
	var n int64
	DB.Model(&AiPrompt{}).Where("user_id = ?", userID).Count(&n)
	if n >= MaxUserPrompts {
		return nil, fmt.Errorf("自定义提示词数量已达上限（%d 条）", MaxUserPrompts)
	}
	p := &AiPrompt{
		UserID: userID, Kind: NormalizePromptKind(kind),
		Category: strings.TrimSpace(category), Title: title, Content: content,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := DB.Create(p).Error; err != nil {
		return nil, err
	}
	return p, nil
}

// UpdateAiPrompt 修改自定义提示词（平台预设不可改，校验归属）
func UpdateAiPrompt(id, userID uint, kind, category, title, content string) error {
	p, err := GetAiPrompt(id, userID)
	if err != nil {
		return fmt.Errorf("提示词不存在")
	}
	if p.UserID == 0 {
		return fmt.Errorf("平台预设提示词不可修改")
	}
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	if title == "" || content == "" {
		return fmt.Errorf("标题与内容都不能为空")
	}
	if len([]rune(title)) > 64 {
		title = string([]rune(title)[:64])
	}
	if len([]rune(content)) > 8000 {
		return fmt.Errorf("提示词过长（上限 8000 字）")
	}
	return DB.Model(&AiPrompt{}).Where("id = ? AND user_id = ?", id, userID).Updates(map[string]interface{}{
		"kind": NormalizePromptKind(kind), "category": strings.TrimSpace(category),
		"title": title, "content": content, "updated_at": time.Now(),
	}).Error
}

// DeleteAiPrompt 删除自定义提示词
func DeleteAiPrompt(id, userID uint) error {
	p, err := GetAiPrompt(id, userID)
	if err != nil {
		return fmt.Errorf("提示词不存在")
	}
	if p.UserID == 0 {
		return fmt.Errorf("平台预设提示词不可删除")
	}
	return DB.Where("id = ? AND user_id = ?", id, userID).Delete(&AiPrompt{}).Error
}

// TouchAiPrompt 记录使用次数（用于「常用提示词」排序）
func TouchAiPrompt(id uint) {
	DB.Model(&AiPrompt{}).Where("id = ?", id).UpdateColumn("used_count", DB.Raw("used_count + 1"))
}

// ---------- 平台预设种子 ----------

// InitAiPrompts 首次启动写入平台预设提示词（幂等：按 title 判重）
func InitAiPrompts() {
	seeds := []AiPrompt{
		// 写作 / 对话
		{Kind: AiPromptKindChat, Category: "写作", Title: "小说续写",
			Content: "你是资深小说作者。请在保持既有文风、人称与时态一致的前提下续写下面的内容，约 600 字。注意：延续人物性格与前文伏笔，不要引入新设定，不要总结前文。\n\n【前文】\n{{前文}}\n\n【续写要求】\n{{要求}}"},
		{Kind: AiPromptKindChat, Category: "写作", Title: "章节大纲",
			Content: "请为以下题材设计一份章节大纲，包含：一句话主线、主要人物（性格/动机/弧光）、三幕结构与关键转折点、每章 2-3 句摘要。\n\n题材：{{题材}}\n章节数：{{数量}}"},
		{Kind: AiPromptKindChat, Category: "写作", Title: "润色改写",
			Content: "请在不改变原意的前提下润色下面这段文字：去掉冗余修饰、统一语体、修正病句与标点，保持原有信息量与语气。直接给出润色后的文本，不要解释。\n\n{{原文}}"},
		{Kind: AiPromptKindChat, Category: "写作", Title: "标题党生成",
			Content: "为下面内容生成 10 个标题，覆盖不同风格（悬念式/数字式/对比式/提问式/故事式），每个不超过 24 字，不要夸张失实。\n\n{{内容}}"},
		{Kind: AiPromptKindChat, Category: "编程", Title: "代码审查",
			Content: "请审查下面的代码，按「安全漏洞 / 逻辑错误 / 性能问题 / 可读性」四类给出问题清单，每条注明行号、风险等级（高/中/低）与修复建议。不要重写整个文件。\n\n语言：{{语言}}\n```{{语言}}\n{{代码}}\n```"},
		{Kind: AiPromptKindChat, Category: "编程", Title: "解释报错",
			Content: "我在运行时遇到以下报错，请按「直接原因 → 根本原因 → 最小修复方案 → 如何避免」四段解释。\n\n报错：\n```\n{{报错}}\n```\n相关代码：\n```\n{{代码}}\n```"},
		{Kind: AiPromptKindChat, Category: "学习", Title: "费曼式讲解",
			Content: "请用费曼学习法讲解「{{概念}}」：先给 12 岁能懂的解释，再举一个生活类比，然后指出常见误解，最后给 3 道自测题（附答案）。"},
		{Kind: AiPromptKindChat, Category: "翻译", Title: "中英互译（保留格式）",
			Content: "请把下面内容翻译成{{目标语言}}。要求：保留原文的 Markdown 结构、代码块与专有名词不译（首次出现可加括号注释）、术语前后一致。只输出译文。\n\n{{原文}}"},

		// 绘画
		{Kind: AiPromptKindImage, Category: "人像", Title: "电影感人像",
			Content: "cinematic portrait of {{subject}}, 85mm lens, shallow depth of field, soft rim light, film grain, muted teal-orange color grading, ultra detailed skin texture, 8k"},
		{Kind: AiPromptKindImage, Category: "场景", Title: "国风山水",
			Content: "traditional Chinese ink painting, misty mountains, {{subject}}, xuan paper texture, minimal composition, large negative space, subtle gradient wash, masterpiece"},
		{Kind: AiPromptKindImage, Category: "场景", Title: "赛博朋克街景",
			Content: "cyberpunk street at night, neon signs reflection on wet asphalt, volumetric fog, {{subject}}, wide angle, high contrast, blade runner atmosphere, octane render"},
		{Kind: AiPromptKindImage, Category: "插画", Title: "日系轻小说插画",
			Content: "anime light novel illustration, {{subject}}, soft pastel palette, clean lineart, dramatic sky, dynamic pose, detailed eyes, studio quality"},
		{Kind: AiPromptKindImage, Category: "设计", Title: "极简产品图",
			Content: "minimal product photography of {{subject}}, seamless light gray backdrop, soft shadow, top-down composition, commercial advertising style, ultra sharp"},
		{Kind: AiPromptKindImage, Category: "风格", Title: "Niji 动漫风格",
			Content: "{{subject}}, anime style, vibrant colors, dynamic composition, detailed background, niji journey --ar 3:2"},

		// 视频
		{Kind: AiPromptKindVideo, Category: "空镜", Title: "电影感空镜",
			Content: "cinematic slow push-in shot of {{scene}}, golden hour light, subtle camera shake, shallow focus, filmic grain, 24fps"},
		{Kind: AiPromptKindVideo, Category: "人物", Title: "人物特写运镜",
			Content: "medium close-up of {{subject}}, slow dolly-in, natural window light, soft bokeh background, subtle breathing motion, realistic skin detail"},
		{Kind: AiPromptKindVideo, Category: "转场", Title: "无缝转场",
			Content: "seamless transition from {{sceneA}} to {{sceneB}}, match cut on shape and color, smooth 2 second blend, cinematic color grade"},
	}
	now := time.Now()
	for i := range seeds {
		s := seeds[i]
		var n int64
		DB.Model(&AiPrompt{}).Where("user_id = 0 AND title = ? AND kind = ?", s.Title, s.Kind).Count(&n)
		if n > 0 {
			continue
		}
		s.UserID = 0
		s.Builtin = true
		s.SortOrder = 100 - i
		s.CreatedAt = now
		s.UpdatedAt = now
		DB.Create(&s)
	}
}
