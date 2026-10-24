package oc

// ==================== 世界书（条目化 + 版本） ====================
// 世界书是「设定数据」：条目化存储便于按需注入提示词（只取相关条目，不整本塞进上下文），
// 与角色卡同样过净化与违禁词。

import (
	"errors"
	"strings"
	"time"

	"nvs-server/models"
	"nvs-server/security"
)

// CreateWorldBook 创建世界书
func CreateWorldBook(userID, groupID uint, title, summary, visibility string) (*WorldBook, error) {
	title = strings.TrimSpace(security.SanitizeUserContent(title))
	summary = strings.TrimSpace(security.SanitizeUserContent(summary))
	if title == "" {
		return nil, errors.New("世界书标题不能为空")
	}
	vis := NormalizeVisibility(visibility)
	review := ReviewApproved
	if vis == VisPublic {
		review = ReviewPending
	}
	b := &WorldBook{
		UserID: userID, GroupID: groupID, Title: title,
		Summary: summary, Version: 1, Visibility: vis, Review: review,
		Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := models.DB.Create(b).Error; err != nil {
		return nil, err
	}
	return b, nil
}

// GetWorldBook 读取（含条目）
func GetWorldBook(id, viewerID uint, isAdmin bool) (*WorldBook, error) {
	var b WorldBook
	if err := models.DB.Where("id = ?", id).First(&b).Error; err != nil {
		return nil, errors.New("世界书不存在")
	}
	if b.Status != "active" && !isAdmin {
		return nil, errors.New("世界书已删除")
	}
	if b.Visibility == VisPublic {
		if b.Review != ReviewApproved && b.UserID != viewerID && !isAdmin {
			return nil, errors.New("世界书待审核")
		}
	} else if b.UserID != viewerID && !isAdmin {
		return nil, errors.New("世界书未公开")
	}
	entries, _ := ListEntries(b.ID)
	b.Entries = entries
	return &b, nil
}

// ListWorldBooks 列表（mine / public / group）
func ListWorldBooks(userID uint, scope string, groupID uint) ([]WorldBook, error) {
	q := models.DB.Model(&WorldBook{}).Where("status = ?", "active")
	switch scope {
	case "mine":
		q = q.Where("user_id = ? AND group_id = 0", userID)
	case "public":
		q = q.Where("visibility = ? AND review = ?", VisPublic, ReviewApproved)
	case "group":
		q = q.Where("group_id = ?", groupID)
	default:
		q = q.Where("user_id = ? OR (visibility = ? AND review = ?) OR group_id = ?",
			userID, VisPublic, ReviewApproved, groupID)
	}
	var list []WorldBook
	if err := q.Order("updated_at DESC").Limit(50).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// UpdateWorldBook 更新（每次改动版本号 +1，便于追溯）
func UpdateWorldBook(userID, id uint, title, summary, visibility string) (*WorldBook, error) {
	var b WorldBook
	if err := models.DB.Where("id = ? AND user_id = ? AND status = ?", id, userID, "active").First(&b).Error; err != nil {
		return nil, errors.New("世界书不存在")
	}
	if t := strings.TrimSpace(security.SanitizeUserContent(title)); t != "" {
		b.Title = t
	}
	if s := strings.TrimSpace(security.SanitizeUserContent(summary)); s != "" {
		b.Summary = s
	}
	vis := NormalizeVisibility(visibility)
	if vis == VisPublic && b.Visibility != VisPublic {
		b.Review = ReviewPending
	}
	b.Visibility = vis
	b.Version++
	b.UpdatedAt = time.Now()
	if err := models.DB.Save(&b).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

// DeleteWorldBook 软删除（留证）
func DeleteWorldBook(userID, id uint) error {
	return models.DB.Model(&WorldBook{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]interface{}{"status": "deleted", "visibility": VisPrivate}).Error
}

// ─── 条目 ───

// ListEntries 条目列表
func ListEntries(bookID uint) ([]WorldEntry, error) {
	var list []WorldEntry
	if err := models.DB.Where("book_id = ?", bookID).
		Order("sort_order ASC, id ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// EntryInput 条目入参（对齐酒馆 Lorebook 条目字段）
type EntryInput struct {
	Kind           string
	Key            string
	Content        string
	SecondaryKeys  string
	Selective      bool
	Logic          int
	Constant       bool
	Enabled        *bool
	Probability    int
	CaseSensitive  bool
	WholeWord      bool
	InsertionOrder int
	Priority       int
	Position       int
	Depth          int
	Group          string
	SortOrder      int
}

// SaveEntryFull 完整字段保存条目（供接口层使用）
func SaveEntryFull(userID, bookID, entryID uint, in EntryInput) (*WorldEntry, error) {
	var b WorldBook
	if err := models.DB.Where("id = ? AND user_id = ? AND status = ?", bookID, userID, "active").First(&b).Error; err != nil {
		return nil, errors.New("世界书不存在或无权编辑")
	}
	key := strings.TrimSpace(security.SanitizeUserContent(in.Key))
	content := strings.TrimSpace(security.SanitizeUserContent(in.Content))
	if key == "" || content == "" {
		return nil, errors.New("触发词与内容不能为空")
	}
	if len([]rune(content)) > MaxEntryRunes {
		return nil, errors.New("条目内容过长")
	}
	content, _ = security.ApplyBadWords(content, "oc")

	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	prob := in.Probability
	if prob <= 0 {
		prob = 100
	}
	if prob > 100 {
		prob = 100
	}
	order := in.InsertionOrder
	if order <= 0 {
		order = 100
	}
	priority := in.Priority
	if priority <= 0 {
		priority = 100
	}
	position := in.Position
	if position < 0 || position > 4 {
		position = PosAfterChar
	}

	patch := map[string]interface{}{
		"kind":            NormalizeEntryKind(in.Kind),
		"key":             key,
		"content":         content,
		"secondary_keys":  strings.TrimSpace(security.SanitizeUserContent(in.SecondaryKeys)),
		"selective":       in.Selective,
		"logic":           in.Logic,
		"constant":        in.Constant,
		"enabled":         enabled,
		"probability":     prob,
		"case_sensitive":  in.CaseSensitive,
		"whole_word":      in.WholeWord,
		"insertion_order": order,
		"priority":        priority,
		"position":        position,
		"depth":           in.Depth,
		"group":           strings.TrimSpace(security.SanitizeUserContent(in.Group)),
		"sort_order":      in.SortOrder,
		"updated_at":      time.Now(),
	}

	if entryID > 0 {
		var e WorldEntry
		if err := models.DB.Where("id = ? AND book_id = ?", entryID, bookID).First(&e).Error; err != nil {
			return nil, errors.New("条目不存在")
		}
		if err := models.DB.Model(&WorldEntry{}).Where("id = ?", entryID).Updates(patch).Error; err != nil {
			return nil, err
		}
		bumpBookVersion(bookID)
		_ = models.DB.Where("id = ?", entryID).First(&e).Error
		return &e, nil
	}
	e := &WorldEntry{BookID: bookID, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := models.DB.Create(e).Error; err != nil {
		return nil, err
	}
	if err := models.DB.Model(&WorldEntry{}).Where("id = ?", e.ID).Updates(patch).Error; err != nil {
		return nil, err
	}
	bumpBookVersion(bookID)
	_ = models.DB.Where("id = ?", e.ID).First(e).Error
	return e, nil
}

// SaveEntry 新增/更新条目（改动即触发版本号 +1）
func SaveEntry(userID, bookID, entryID uint, kind, key, content string, sortOrder int) (*WorldEntry, error) {
	var b WorldBook
	if err := models.DB.Where("id = ? AND user_id = ? AND status = ?", bookID, userID, "active").First(&b).Error; err != nil {
		return nil, errors.New("世界书不存在或无权编辑")
	}
	key = strings.TrimSpace(security.SanitizeUserContent(key))
	content = strings.TrimSpace(security.SanitizeUserContent(content))
	if key == "" || content == "" {
		return nil, errors.New("条目名与内容不能为空")
	}
	if len([]rune(content)) > MaxEntryRunes {
		return nil, errors.New("条目内容过长")
	}
	content, _ = security.ApplyBadWords(content, "oc")
	kind = NormalizeEntryKind(kind)

	if entryID > 0 {
		var e WorldEntry
		if err := models.DB.Where("id = ? AND book_id = ?", entryID, bookID).First(&e).Error; err != nil {
			return nil, errors.New("条目不存在")
		}
		e.Kind, e.Key, e.Content, e.SortOrder = kind, key, content, sortOrder
		e.UpdatedAt = time.Now()
		if err := models.DB.Save(&e).Error; err != nil {
			return nil, err
		}
		bumpBookVersion(bookID)
		return &e, nil
	}
	e := &WorldEntry{
		BookID: bookID, Kind: kind, Key: key, Content: content,
		SortOrder: sortOrder, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := models.DB.Create(e).Error; err != nil {
		return nil, err
	}
	bumpBookVersion(bookID)
	return e, nil
}

// DeleteEntry 删除条目
func DeleteEntry(userID, bookID, entryID uint) error {
	var b WorldBook
	if err := models.DB.Where("id = ? AND user_id = ?", bookID, userID).First(&b).Error; err != nil {
		return errors.New("世界书不存在或无权编辑")
	}
	if err := models.DB.Where("id = ? AND book_id = ?", entryID, bookID).Delete(&WorldEntry{}).Error; err != nil {
		return err
	}
	bumpBookVersion(bookID)
	return nil
}

func bumpBookVersion(bookID uint) {
	_ = models.DB.Model(&WorldBook{}).Where("id = ?", bookID).
		Updates(map[string]interface{}{"version": models.DB.Raw("version + 1"), "updated_at": time.Now()}).Error
}

// EntryKindLabel 条目分类展示名
func EntryKindLabel(k string) string {
	switch NormalizeEntryKind(k) {
	case EntryFaction:
		return "势力"
	case EntryPlace:
		return "地点"
	case EntryRule:
		return "规则"
	case EntryEvent:
		return "事件"
	default:
		return "世界观"
	}
}
