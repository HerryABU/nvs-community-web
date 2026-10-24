package models

import (
	"time"
)

// ReadingNote 精读笔记：书签 + 批注。
// Type: bookmark（书签：收藏阅读位置，Content 为备注）/ annotation（批注：选中原文+笔记）。
// Visibility: private（仅自己可见，默认）/ public（公开，其他读者可见）。
type ReadingNote struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	UserID        uint      `gorm:"not null;index:idx_note_user_novel" json:"user_id"`
	NovelID       uint      `gorm:"not null;index:idx_note_user_novel" json:"novel_id"`
	ChapterNumber int       `gorm:"default:0" json:"chapter_number"` // 0=作品级书签
	Type          string    `gorm:"size:16;not null" json:"type"`    // bookmark / annotation
	Content       string    `gorm:"size:2000;default:''" json:"content"`
	SelectedText  string    `gorm:"size:1000;default:''" json:"selected_text"`       // 批注选中的原文片段
	Anchor        int       `gorm:"default:0" json:"anchor"`                         // 原文中的字符偏移（仅批注用，粗定位）
	Visibility    string    `gorm:"size:16;default:private;index" json:"visibility"` // private / public
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	// 运行时补充（不落库）
	NovelTitle   string `gorm:"-" json:"novel_title,omitempty"`
	ChapterTitle string `gorm:"-" json:"chapter_title,omitempty"`
	Username     string `gorm:"-" json:"username,omitempty"`
}

func (ReadingNote) TableName() string { return "reading_notes" }

// CreateReadingNote 新建书签/批注
func CreateReadingNote(n *ReadingNote) error {
	return DB.Create(n).Error
}

// ListMyReadingNotes 我的笔记（可按作品/章节/类型筛选）
func ListMyReadingNotes(userID, novelID uint, chapterNumber int, noteType string) ([]ReadingNote, error) {
	var list []ReadingNote
	q := DB.Model(&ReadingNote{}).Where("user_id = ?", userID)
	if novelID != 0 {
		q = q.Where("novel_id = ?", novelID)
	}
	if chapterNumber > 0 {
		q = q.Where("chapter_number = ?", chapterNumber)
	}
	if noteType != "" {
		q = q.Where("type = ?", noteType)
	}
	err := q.Order("created_at DESC").Find(&list).Error
	return list, err
}

// ListPublicNotes 公开批注/书签（按作品+章节）
//
// ⚠️ 不要 Preload("User")：ReadingNote 没有 User 关联（Username 是 gorm:"-" 的运行时字段），
// Preload 一个不存在的关联会让整个接口 500 —— 阅读器的公开划线因此一直是空的（后端报错，
// 前端 catch 里静默降级成"本章没有别人的划线"）。用户名单独批量查。
func ListPublicNotes(novelID uint, chapterNumber int) ([]ReadingNote, error) {
	var list []ReadingNote
	q := DB.Model(&ReadingNote{}).
		Where("visibility = ? AND novel_id = ?", "public", novelID)
	if chapterNumber > 0 {
		q = q.Where("chapter_number = ?", chapterNumber)
	}
	if err := q.Order("created_at ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	fillNoteUsernames(list)
	return list, nil
}

// fillNoteUsernames 批量补用户名：一次查询取全部作者，避免 N+1。
func fillNoteUsernames(list []ReadingNote) {
	ids := make([]uint, 0, len(list))
	seen := map[uint]bool{}
	for i := range list {
		uid := list[i].UserID
		if uid == 0 || seen[uid] {
			continue
		}
		seen[uid] = true
		ids = append(ids, uid)
	}
	if len(ids) == 0 {
		return
	}
	var users []User
	if err := DB.Select("id", "username", "nickname").Where("id IN ?", ids).Find(&users).Error; err != nil {
		return // 补名字失败不影响批注本体
	}
	nameOf := make(map[uint]string, len(users))
	for _, u := range users {
		n := u.Nickname
		if n == "" {
			n = u.Username
		}
		nameOf[u.ID] = n
	}
	for i := range list {
		list[i].Username = nameOf[list[i].UserID]
	}
}

// ListAllMyNotes 我的全部笔记（精读页，分页，带作品名/章节名）
func ListAllMyNotes(userID uint, page, pageSize int) ([]ReadingNote, int64, error) {
	var list []ReadingNote
	var total int64
	q := DB.Model(&ReadingNote{}).Where("user_id = ?", userID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	if err != nil {
		return nil, 0, err
	}
	// 补充作品名/章节名（批量查一次）
	novelIDs := map[uint]bool{}
	for i := range list {
		novelIDs[list[i].NovelID] = true
	}
	novelTitles := map[uint]string{}
	if len(novelIDs) > 0 {
		ids := make([]uint, 0, len(novelIDs))
		for id := range novelIDs {
			ids = append(ids, id)
		}
		var novels []Novel
		DB.Select("id", "title").Where("id IN ?", ids).Find(&novels)
		for _, n := range novels {
			novelTitles[n.ID] = n.Title
		}
	}
	for i := range list {
		list[i].NovelTitle = novelTitles[list[i].NovelID]
		// 章节名
		if list[i].ChapterNumber > 0 {
			var ch Chapter
			if DB.Select("title").Where("novel_id = ? AND chapter_number = ?", list[i].NovelID, list[i].ChapterNumber).
				First(&ch).Error == nil {
				list[i].ChapterTitle = ch.Title
			}
		}
	}
	return list, total, nil
}

// GetReadingNote 单条笔记
func GetReadingNote(id uint) (*ReadingNote, error) {
	var n ReadingNote
	err := DB.First(&n, id).Error
	return &n, err
}

// UpdateReadingNote 更新内容/可见性
func UpdateReadingNote(id uint, content, visibility string) error {
	updates := map[string]interface{}{}
	if content != "" {
		updates["content"] = content
	}
	if visibility == "private" || visibility == "public" {
		updates["visibility"] = visibility
	}
	if len(updates) == 0 {
		return nil
	}
	return DB.Model(&ReadingNote{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteReadingNote 删除笔记
func DeleteReadingNote(id uint) error {
	return DB.Delete(&ReadingNote{}, id).Error
}

// CountReadingNotes 笔记数（个人中心用）
func CountReadingNotes(userID uint) int64 {
	var c int64
	DB.Model(&ReadingNote{}).Where("user_id = ?", userID).Count(&c)
	return c
}
