package authorai

import "time"

// Detail 一个 AI 写作工程下的「独立细节卡」（ADRO 创作向的 Deconstruct 产物）。
//
// 设计要点（对标 NovelForge 卡片式结构化创作 / 织叙工作台独立实体）：
//   - 世界观、人物、大纲、时间线不再是一整块 textarea，而是拆成一条条可独立检索、
//     独立编辑、独立让 AI 微调的实体；作者可对「每一个细节」做苏格拉底式追问或费曼式复述；
//   - Module 归属四个素材模块；SortOrder 决定卡片顺序；
//   - 工程表 NovelProject 的 Setting/Characters/Outline/Timeline 四个大字段是「投影」：
//     由本表按模块重编译（compileMaterials）而成，供既有 generate / timeline-check /
//     publish 等链路继续读取，无需改动它们；
//   - 真源在本表。写回投影是单向的（detail → material），手工在大字段里改的内容
//     不会被反向拆卡，避免双写不一致。
type Detail struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ProjectID uint      `gorm:"not null;index:idx_authorai_detail_project" json:"project_id"`
	Module    string    `gorm:"size:32;not null;index:idx_authorai_detail_module" json:"module"` // setting/character/outline/timeline
	Title     string    `gorm:"size:256;not null;default:''" json:"title"`
	Content   string    `gorm:"type:text" json:"content"`
	SortOrder int       `gorm:"not null;default:0" json:"sort_order"`
	Status    string    `gorm:"size:32;default:draft" json:"status"` // draft 手填 / generated AI 生成 / refined 已微调
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Detail) TableName() string { return "authorai_details" }

// DetailModule 素材模块（与 handler 的 CompSetting/CompCharacter/... 对齐）。
const (
	ModSetting   = "setting"
	ModCharacter = "character"
	ModOutline   = "outline"
	ModTimeline  = "timeline"
	ModStoryline = "storyline" // 故事线（主线/支线，可多条、可交联）
)

// DetailModules 全部合法模块，用于校验与编译投影。
var DetailModules = []string{ModSetting, ModCharacter, ModOutline, ModTimeline, ModStoryline}

func ValidDetailModule(m string) bool {
	for _, v := range DetailModules {
		if v == m {
			return true
		}
	}
	return false
}

// ModuleLabel 模块中文名（提示词与前端共用语义由前端负责，这里仅用于错误信息）。
func ModuleLabel(m string) string {
	switch m {
	case ModSetting:
		return "世界观 / 设定"
	case ModCharacter:
		return "人物设定"
	case ModOutline:
		return "大纲"
	case ModTimeline:
		return "大事年表 / 时间线"
	case ModStoryline:
		return "故事线（主线 / 支线）"
	}
	return m
}
