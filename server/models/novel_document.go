package models

import "time"

// NovelDocument 小说附属文档（设定集 / 大纲 / 人物小传 / 时间线 / 故事线 / 资料等）。
// 与小说域绑定（novel_id），作者维护，可控制对读者是否公开。
// 与 authorai 的细节卡不同：这里是「成品小说」的附属资料，发布后仍可维护。
type NovelDocument struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	NovelID    uint      `gorm:"not null;index:idx_novel_doc_novel,priority:1" json:"novel_id"`
	Kind       string    `gorm:"size:32;not null;default:'setting';index:idx_novel_doc_novel,priority:2" json:"kind"` // setting/outline/character/timeline/storyline/extra/material
	Title      string    `gorm:"size:256;not null;default:''" json:"title"`
	Content    string    `gorm:"type:text" json:"content"`
	SortOrder  int       `gorm:"not null;default:0" json:"sort_order"`
	Visibility string    `gorm:"size:16;not null;default:'public'" json:"visibility"` // public / private
	Status     string    `gorm:"size:16;not null;default:'draft'" json:"status"`      // draft / published
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (NovelDocument) TableName() string { return "novel_documents" }

// Novel 文档分类
const (
	DocKindSetting   = "setting"   // 设定集
	DocKindOutline   = "outline"   // 大纲
	DocKindCharacter = "character" // 人物小传
	DocKindTimeline  = "timeline"  // 时间线 / 大事年表
	DocKindStoryline = "storyline" // 故事线
	DocKindExtra     = "extra"     // 番外 / 其他
	DocKindMaterial  = "material"  // 资料
)

func IsValidDocKind(k string) bool {
	switch k {
	case DocKindSetting, DocKindOutline, DocKindCharacter, DocKindTimeline, DocKindStoryline, DocKindExtra, DocKindMaterial:
		return true
	}
	return false
}

// ListNovelDocuments 小说文档（按排序）
func ListNovelDocuments(novelID uint) ([]NovelDocument, error) {
	var list []NovelDocument
	err := DB.Where("novel_id = ?", novelID).Order("sort_order ASC, id ASC").Find(&list).Error
	return list, err
}

// GetNovelDocument 取单条
func GetNovelDocument(id uint) (*NovelDocument, error) {
	var d NovelDocument
	if err := DB.First(&d, id).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

// CreateNovelDocument 新建
func CreateNovelDocument(d *NovelDocument) error {
	if d.SortOrder == 0 {
		var maxOrder int
		DB.Model(&NovelDocument{}).Where("novel_id = ?", d.NovelID).
			Select("COALESCE(MAX(sort_order), -1)").Scan(&maxOrder)
		d.SortOrder = maxOrder + 1
	}
	d.CreatedAt = time.Now()
	d.UpdatedAt = time.Now()
	return DB.Create(d).Error
}

// UpdateNovelDocumentFields 更新（限定小说，防越权）
func UpdateNovelDocumentFields(id, novelID uint, fields map[string]interface{}) error {
	fields["updated_at"] = time.Now()
	return DB.Model(&NovelDocument{}).Where("id = ? AND novel_id = ?", id, novelID).Updates(fields).Error
}

// DeleteNovelDocument 删除（限定小说，防越权）
func DeleteNovelDocument(id, novelID uint) error {
	return DB.Where("id = ? AND novel_id = ?", id, novelID).Delete(&NovelDocument{}).Error
}
