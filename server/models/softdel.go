package models

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// ─────────────────────────────────────────────────────────────
// 软删除留证层（softdel_* 表）
//
// 设计目标：所有管理员/抢救台/举报处置产生的「删除/封禁」操作，在真正
// 隐藏（软删除）原始记录之前，先把完整快照归档到对应的 softdel_ 表，
// 作为不可轻易丢失的证据留存（留证），满足 176 号令追溯要求。
// 原始表仍保留 deleted_at 软删除标记，二者叠加 = 双重可追溯。
// ─────────────────────────────────────────────────────────────

// SoftDelMeta 软删除留证元数据（嵌入各 softdel_ 表）
type SoftDelMeta struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	OriginalID uint      `gorm:"index" json:"original_id"` // 原记录主键
	DeletedBy  uint      `gorm:"index" json:"deleted_by"`  // 操作者用户 ID
	Operator   string    `gorm:"size:64" json:"operator"`  // 操作者用户名
	Reason     string    `gorm:"size:512;default:''" json:"reason"`
	Source     string    `gorm:"size:16;default:admin" json:"source"` // admin / emergency / report / moderator
	Summary    string    `gorm:"type:text" json:"summary"`            // 内容摘要（标题/正文开头）
	Snapshot   string    `gorm:"type:text" json:"snapshot"`           // 完整记录 JSON 快照
	DeletedAt  time.Time `json:"deleted_at"`
}

// 各内容类型对应的留证表（结构一致，仅表名不同，便于统一查询）
type SoftDelComment struct{ SoftDelMeta }

func (SoftDelComment) TableName() string { return "softdel_comments" }

type SoftDelNovel struct{ SoftDelMeta }

func (SoftDelNovel) TableName() string { return "softdel_novels" }

type SoftDelChapter struct{ SoftDelMeta }

func (SoftDelChapter) TableName() string { return "softdel_chapters" }

type SoftDelUser struct{ SoftDelMeta }

func (SoftDelUser) TableName() string { return "softdel_users" }

type SoftDelThread struct{ SoftDelMeta }

func (SoftDelThread) TableName() string { return "softdel_threads" }

type SoftDelPost struct{ SoftDelMeta }

func (SoftDelPost) TableName() string { return "softdel_posts" }

type SoftDelBlog struct{ SoftDelMeta }

func (SoftDelBlog) TableName() string { return "softdel_blogs" }

type SoftDelForum struct{ SoftDelMeta }

func (SoftDelForum) TableName() string { return "softdel_forums" }

// SoftDelTables 内容类型 → 留证表名 映射（保持声明顺序稳定）
var SoftDelTables = map[string]string{
	"comment": "softdel_comments",
	"novel":   "softdel_novels",
	"chapter": "softdel_chapters",
	"user":    "softdel_users",
	"thread":  "softdel_threads",
	"post":    "softdel_posts",
	"blog":    "softdel_blogs",
	"forum":   "softdel_forums",
}

// SoftDelItem 留证记录（跨表查询的统一结果）
type SoftDelItem struct {
	Type       string    `json:"type"`
	ID         uint      `json:"id"`
	OriginalID uint      `json:"original_id"`
	DeletedBy  uint      `json:"deleted_by"`
	Operator   string    `json:"operator"`
	Reason     string    `json:"reason"`
	Source     string    `json:"source"`
	Summary    string    `json:"summary"`
	Snapshot   string    `json:"snapshot"`
	DeletedAt  time.Time `json:"deleted_at"`
}

// softDelRow 用于跨表统一扫描的匿名结构（各 softdel_ 表列结构一致）
type softDelRow struct {
	ID         uint      `gorm:"column:id"`
	OriginalID uint      `gorm:"column:original_id"`
	DeletedBy  uint      `gorm:"column:deleted_by"`
	Operator   string    `gorm:"column:operator"`
	Reason     string    `gorm:"column:reason"`
	Source     string    `gorm:"column:source"`
	Summary    string    `gorm:"column:summary"`
	Snapshot   string    `gorm:"column:snapshot"`
	DeletedAt  time.Time `gorm:"column:deleted_at"`
}

// ArchiveSoftDelete 归档一条软删除留证记录。
// 参数：source=处置来源(admin/emergency/report/moderator)，ctype=内容类型，
// originalID=原记录 ID，deletedBy/operator=操作者，reason=处置原因，
// summary=摘要，snapshot=完整记录快照（任意可 JSON 序列化的结构）。
func ArchiveSoftDelete(source, ctype string, originalID, deletedBy uint, operator, reason, summary string, snapshot interface{}) error {
	if _, ok := SoftDelTables[ctype]; !ok {
		return fmt.Errorf("未知留证类型: %s", ctype)
	}
	snap := ""
	if snapshot != nil {
		if b, err := json.Marshal(snapshot); err == nil {
			snap = string(b)
		}
	}
	meta := SoftDelMeta{
		OriginalID: originalID,
		DeletedBy:  deletedBy,
		Operator:   operator,
		Reason:     reason,
		Source:     source,
		Summary:    summary,
		Snapshot:   snap,
		DeletedAt:  time.Now(),
	}
	switch ctype {
	case "comment":
		return DB.Create(&SoftDelComment{SoftDelMeta: meta}).Error
	case "novel":
		return DB.Create(&SoftDelNovel{SoftDelMeta: meta}).Error
	case "chapter":
		return DB.Create(&SoftDelChapter{SoftDelMeta: meta}).Error
	case "user":
		return DB.Create(&SoftDelUser{SoftDelMeta: meta}).Error
	case "thread":
		return DB.Create(&SoftDelThread{SoftDelMeta: meta}).Error
	case "post":
		return DB.Create(&SoftDelPost{SoftDelMeta: meta}).Error
	case "blog":
		return DB.Create(&SoftDelBlog{SoftDelMeta: meta}).Error
	case "forum":
		return DB.Create(&SoftDelForum{SoftDelMeta: meta}).Error
	default:
		return fmt.Errorf("未知留证类型: %s", ctype)
	}
}

// ListSoftDel 列出留证记录；ctype 为空则合并全部类型并按时间倒序返回。
func ListSoftDel(ctype string, limit int) ([]SoftDelItem, error) {
	if limit <= 0 {
		limit = 500
	}
	var tables []string
	if ctype != "" {
		t, ok := SoftDelTables[ctype]
		if !ok {
			return nil, fmt.Errorf("未知留证类型: %s", ctype)
		}
		tables = []string{t}
	} else {
		for _, t := range SoftDelTables {
			tables = append(tables, t)
		}
	}

	var items []SoftDelItem
	for _, t := range tables {
		var rows []softDelRow
		if err := DB.Table(t).Order("deleted_at DESC").Limit(limit).Find(&rows).Error; err != nil {
			continue // 单表查询失败不阻断整体
		}
		for _, r := range rows {
			items = append(items, SoftDelItem{
				Type:       typeOfSoftDelTable(t),
				ID:         r.ID,
				OriginalID: r.OriginalID,
				DeletedBy:  r.DeletedBy,
				Operator:   r.Operator,
				Reason:     r.Reason,
				Source:     r.Source,
				Summary:    r.Summary,
				Snapshot:   r.Snapshot,
				DeletedAt:  r.DeletedAt,
			})
		}
	}
	// 跨表合并后按删除时间倒序
	sort.Slice(items, func(i, j int) bool {
		return items[i].DeletedAt.After(items[j].DeletedAt)
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

// typeOfSoftDelTable 表名 → 内容类型
func typeOfSoftDelTable(table string) string {
	for ctype, t := range SoftDelTables {
		if t == table {
			return ctype
		}
	}
	return table
}
