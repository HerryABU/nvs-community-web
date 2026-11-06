package authorai

import "time"

// Chapter 一个 AI 写作工程下的独立章节实体（与 NovelProject 的扁平物料字段解耦）。
//
// 设计要点（对齐阶段流水线 + 真源划分）：
//   - 章节是「独立可检索、可单独编辑」的关系表，不是塞进工程 JSON 字段的数组；
//   - Order 决定章节顺序；Outline 是本章章纲，Body 是本章正文（落点）；
//   - 生成章节时由 handler 把工程物料（设定/人物/大纲/时间线）+ 前序章节作为上下文打包，
//     逐章落盘，崩溃可从当前章节续跑（无需重跑整本）。
type Chapter struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ProjectID uint      `gorm:"not null;index:idx_authorai_chapter_project" json:"project_id"`
	Order     int       `gorm:"not null;default:0" json:"order"` // 章节序号（从 1 开始）
	Title     string    `gorm:"size:256;not null;default:''" json:"title"`
	Outline   string    `gorm:"type:text" json:"outline"`         // 本章章纲
	Body      string    `gorm:"type:text" json:"body"`            // 本章正文
	Status    string    `gorm:"size:32;default:''" json:"status"` // ""=未生成 / outline=仅有章纲 / generated=已生成正文
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Chapter) TableName() string { return "authorai_chapters" }
