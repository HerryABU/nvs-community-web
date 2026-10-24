package oc

// ==================== 角色卡 ====================
// 角色卡是「用户生成的设定数据」：本身要过净化与违禁词；
// 进入 AI 提示词后会左右 AI 输出，所以**设定数据本身也要审**，
// 且「角色设定」不能成为绕过审核的理由（AI 输出仍独立过审）。

import (
	"errors"
	"strings"
	"time"

	"nvs-server/models"
	"nvs-server/security"
)

// CharacterInput 角色卡入参（字段划分对齐酒馆 V2 + 小团子养成）
type CharacterInput struct {
	Name       string
	AvatarURL  string
	Persona    string
	Appearance string
	Speech     string
	Tags       string
	Visibility string

	Scenario     string
	FirstMessage string
	MesExample   string
	SystemPrompt string
	PostHistory  string
	CreatorNotes string
	Greetings    string // JSON 数组字符串（备用开场）
	BookID       uint   // 角色自带世界书

	Mood     int
	Affinity int
	Energy   int
	EmoteMap string // JSON：{"笑":"立绘URL"}
}

func normalizeCharacterInput(in *CharacterInput) {
	in.Name = strings.TrimSpace(in.Name)
	in.Persona = strings.TrimSpace(in.Persona)
	in.Appearance = strings.TrimSpace(in.Appearance)
	in.Speech = strings.TrimSpace(in.Speech)
	in.Tags = strings.TrimSpace(in.Tags)
	in.AvatarURL = strings.TrimSpace(in.AvatarURL)
	in.Visibility = NormalizeVisibility(in.Visibility)
	in.Scenario = strings.TrimSpace(in.Scenario)
	in.FirstMessage = strings.TrimSpace(in.FirstMessage)
	in.MesExample = strings.TrimSpace(in.MesExample)
	in.SystemPrompt = strings.TrimSpace(in.SystemPrompt)
	in.PostHistory = strings.TrimSpace(in.PostHistory)
	in.CreatorNotes = strings.TrimSpace(in.CreatorNotes)
	in.Greetings = strings.TrimSpace(in.Greetings)
	in.EmoteMap = strings.TrimSpace(in.EmoteMap)
	in.Mood = clamp01(in.Mood, 50)
	in.Affinity = clamp01(in.Affinity, 0)
	in.Energy = clamp01(in.Energy, 100)
}

func clamp01(v, def int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	if v == 0 && def != 0 {
		return def
	}
	return v
}

// sanitizeCharacter 净化 + 违禁词扫描（设定数据本身也要审）
func sanitizeCharacter(in *CharacterInput) []security.BadWordHit {
	in.Name = security.SanitizeUserContent(in.Name)
	in.Persona = security.SanitizeUserContent(in.Persona)
	in.Appearance = security.SanitizeUserContent(in.Appearance)
	in.Speech = security.SanitizeUserContent(in.Speech)
	in.Tags = security.SanitizeUserContent(in.Tags)
	in.Scenario = security.SanitizeUserContent(in.Scenario)
	in.FirstMessage = security.SanitizeUserContent(in.FirstMessage)
	in.MesExample = security.SanitizeUserContent(in.MesExample)
	in.SystemPrompt = security.SanitizeUserContent(in.SystemPrompt)
	in.PostHistory = security.SanitizeUserContent(in.PostHistory)
	in.CreatorNotes = security.SanitizeUserContent(in.CreatorNotes)
	var hits []security.BadWordHit
	// scope 用 oc，便于后台单独配置 OC 词库
	in.Name, _ = security.ApplyBadWords(in.Name, "oc")
	var h []security.BadWordHit
	in.Persona, h = security.ApplyBadWords(in.Persona, "oc")
	hits = append(hits, h...)
	in.Appearance, h = security.ApplyBadWords(in.Appearance, "oc")
	hits = append(hits, h...)
	in.Speech, h = security.ApplyBadWords(in.Speech, "oc")
	hits = append(hits, h...)
	in.Scenario, h = security.ApplyBadWords(in.Scenario, "oc")
	hits = append(hits, h...)
	in.MesExample, h = security.ApplyBadWords(in.MesExample, "oc")
	hits = append(hits, h...)
	in.SystemPrompt, h = security.ApplyBadWords(in.SystemPrompt, "oc")
	hits = append(hits, h...)
	return hits
}

// CreateCharacter 创建角色卡（公开需 pending 待审）
func CreateCharacter(userID uint, in CharacterInput) (*Character, error) {
	normalizeCharacterInput(&in)
	if in.Name == "" {
		return nil, errors.New("角色名不能为空")
	}
	if len([]rune(in.Name)) > MaxNameRunes {
		return nil, errors.New("角色名过长")
	}
	sanitizeCharacter(&in)
	if len([]rune(in.Persona)) > MaxPersonaRunes {
		return nil, errors.New("人设内容过长")
	}
	review := ReviewApproved
	if in.Visibility == VisPublic {
		review = ReviewPending
	}
	c := &Character{
		UserID: userID, Name: in.Name, AvatarURL: in.AvatarURL,
		Persona: in.Persona, Appearance: in.Appearance, Speech: in.Speech, Tags: in.Tags,
		Scenario: in.Scenario, FirstMessage: in.FirstMessage, MesExample: in.MesExample,
		SystemPrompt: in.SystemPrompt, PostHistory: in.PostHistory,
		CreatorNotes: in.CreatorNotes, Greetings: in.Greetings, BookID: in.BookID,
		Mood: in.Mood, Affinity: in.Affinity, Energy: in.Energy, EmoteMap: in.EmoteMap,
		Visibility: in.Visibility, Review: review, Status: "active",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := models.DB.Create(c).Error; err != nil {
		return nil, err
	}
	return c, nil
}

// UpdateCharacter 更新角色卡（改为公开时回到 pending）
func UpdateCharacter(userID uint, id uint, in CharacterInput) (*Character, error) {
	var c Character
	if err := models.DB.Where("id = ? AND user_id = ? AND status = ?", id, userID, "active").First(&c).Error; err != nil {
		return nil, errors.New("角色卡不存在")
	}
	normalizeCharacterInput(&in)
	sanitizeCharacter(&in)
	c.Name = in.Name
	c.AvatarURL = in.AvatarURL
	c.Persona = in.Persona
	c.Appearance = in.Appearance
	c.Speech = in.Speech
	c.Tags = in.Tags
	c.Scenario = in.Scenario
	c.FirstMessage = in.FirstMessage
	c.MesExample = in.MesExample
	c.SystemPrompt = in.SystemPrompt
	c.PostHistory = in.PostHistory
	c.CreatorNotes = in.CreatorNotes
	c.Greetings = in.Greetings
	c.BookID = in.BookID
	c.Mood = in.Mood
	c.Affinity = in.Affinity
	c.Energy = in.Energy
	c.EmoteMap = in.EmoteMap
	if in.Visibility == VisPublic && c.Visibility != VisPublic {
		c.Review = ReviewPending // 转公开需重新审核
	}
	c.Visibility = in.Visibility
	c.UpdatedAt = time.Now()
	if err := models.DB.Save(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

// DeleteCharacter 软删除（留证，供举报/版权处理追溯）
func DeleteCharacter(userID uint, id uint) error {
	return models.DB.Model(&Character{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]interface{}{"status": "deleted", "visibility": VisPrivate}).Error
}

// GetCharacter 读取（私有仅本人可见；公开需审核通过）
func GetCharacter(id uint, viewerID uint, isAdmin bool) (*Character, error) {
	var c Character
	if err := models.DB.Where("id = ?", id).First(&c).Error; err != nil {
		return nil, errors.New("角色卡不存在")
	}
	if c.Status != "active" && !isAdmin {
		return nil, errors.New("角色卡已删除")
	}
	if c.Visibility == VisPublic {
		if c.Review != ReviewApproved && c.UserID != viewerID && !isAdmin {
			return nil, errors.New("角色卡待审核")
		}
	} else if c.UserID != viewerID && !isAdmin {
		return nil, errors.New("角色卡未公开")
	}
	fillAuthorNames([]*Character{&c})
	return &c, nil
}

// ListCharacters 我的角色卡 + 可选公开库
func ListCharacters(userID uint, scope string, keyword string, page, pageSize int) ([]Character, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	q := models.DB.Model(&Character{}).Where("status = ?", "active")
	switch scope {
	case "public":
		q = q.Where("visibility = ? AND review = ?", VisPublic, ReviewApproved)
	case "mine":
		q = q.Where("user_id = ?", userID)
	default: // usable：我的 + 已公开过审的
		q = q.Where("user_id = ? OR (visibility = ? AND review = ?)", userID, VisPublic, ReviewApproved)
	}
	if kw := strings.TrimSpace(keyword); kw != "" {
		q = q.Where("name LIKE ?", "%"+kw+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []Character
	if err := q.Order("updated_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	ptrs := make([]*Character, len(list))
	for i := range list {
		ptrs[i] = &list[i]
	}
	fillAuthorNames(ptrs)
	return list, total, nil
}

func fillAuthorNames(list []*Character) {
	if len(list) == 0 {
		return
	}
	ids := map[uint]bool{}
	for _, c := range list {
		ids[c.UserID] = true
	}
	keys := make([]uint, 0, len(ids))
	for id := range ids {
		keys = append(keys, id)
	}
	var users []models.User
	models.DB.Select("id, username, nickname").Where("id IN ?", keys).Find(&users)
	nameOf := map[uint]string{}
	for _, u := range users {
		if u.Nickname != "" {
			nameOf[u.ID] = u.Nickname
		} else {
			nameOf[u.ID] = u.Username
		}
	}
	for _, c := range list {
		c.AuthorName = nameOf[c.UserID]
	}
}

// ReviewCharacter 平台审核公开角色卡（admin）
func ReviewCharacter(id uint, approve bool, note string) error {
	review := ReviewRejected
	if approve {
		review = ReviewApproved
	}
	return models.DB.Model(&Character{}).Where("id = ?", id).
		Updates(map[string]interface{}{"review": review, "updated_at": time.Now()}).Error
}
