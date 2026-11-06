// Package authorai 作者工作台内置 AI 写作工程的领域模型。
//
// 与 AI 中台（models/ai、models/aihub）数据完全隔离：本包只存「作者自己的写作物料」
// （设定 / 人物设定 / 大事年表 / 大纲 / 风格 / 正文草稿），不依赖任何中台会话或智能体表。
// 调用上游生成时走 aiclient，但工程数据独立建表，满足「与 ai 中台数据分开」的硬要求。
package authorai

import (
	"time"
)

// NovelProject 一个 AI 写作工程。作者可在工作台内逐步打磨，最后「生成小说」落地成 Novel。
type NovelProject struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	UserID      uint   `gorm:"not null;index:idx_authorai_user" json:"user_id"`
	NovelID     uint   `gorm:"index;default:0" json:"novel_id"` // 已生成/关联的小说 ID，0=尚未生成
	Title       string `gorm:"size:256;not null;default:''" json:"title"`
	Setting     string `gorm:"type:text" json:"setting"`               // 世界观 / 设定
	Characters  string `gorm:"type:text" json:"characters"`            // JSON 数组：人物设定卡
	Timeline    string `gorm:"type:text" json:"timeline"`              // JSON：大事年表 + 嵌套循环时间线
	Outline     string `gorm:"type:text" json:"outline"`               // JSON：大纲层级
	StylePreset string `gorm:"size:32;default:''" json:"style_preset"` // realism / hard_sci_fi / light_novel / custom
	StyleDetail string `gorm:"type:text" json:"style_detail"`          // 更详细的风格要求
	BodyDraft   string `gorm:"type:text" json:"body_draft"`            // 正文草稿
	// 四个素材模块的「用户输入状态（补充要求）」：空=待 AI 生成，非空=用户已填写。
	// 与对应内容字段（Setting/Characters/Outline/Timeline）解耦：内容由 AI 产出，补充要求由用户持久化。
	SettingInput   string    `gorm:"type:text" json:"setting_input"`
	CharacterInput string    `gorm:"type:text" json:"character_input"`
	OutlineInput   string    `gorm:"type:text" json:"outline_input"`
	TimelineInput  string    `gorm:"type:text" json:"timeline_input"`
	Status         string    `gorm:"size:32;default:draft" json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (NovelProject) TableName() string { return "authorai_projects" }

// Models 返回本域需要迁移的模型列表（由根包 AllModels 运行时聚合）。
func Models() []interface{} {
	return []interface{}{&NovelProject{}, &Chapter{}}
}
