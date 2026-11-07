package oc

// ==================== OC 组（阵容） ====================
// OC 组是用户个人的「OC 收藏/阵容」：把多张角色卡打包成一个可复用集合。
// 与「群组」彻底解耦——群组是多人社区场景，OC 组是纯个人创作资产；
// 群组可反向「召唤」某个 OC 组进群发帖（见 invoke.go），但 OC 组本身不依赖任何群组。
//
// #78.2 / 亚文化创作区：OC 组是亚文化创作区里「我的 OC 组」管理的核心实体，
// 兼容 furry 系（成员即角色卡，已带 oc_kind/species/species_traits/anthro_type）。

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"nvs-server/models"
)

// MaxOcGroupNameRunes OC 组名长度上限
const MaxOcGroupNameRunes = 40

// ─── OC 组 ───

// OcGroup OC 组/阵容（用户个人 OC 收藏集合）
//
// Visibility 与 Character 对齐：private=仅本人可见；public=公开库可被他人/群组引用。
// public 需走 Review（与角色卡一致：公开内容平台审核）。
type OcGroup struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	UserID      uint   `gorm:"not null;index" json:"user_id"`
	Name        string `gorm:"size:64;not null;index" json:"name"`
	Summary     string `gorm:"type:text" json:"summary"`
	CoverURL    string `gorm:"size:512;default:''" json:"cover_url"`
	Tags        string `gorm:"size:256;default:''" json:"tags"`
	MemberCount int    `gorm:"not null;default:0" json:"member_count"`
	Visibility  string `gorm:"size:16;not null;default:private;index" json:"visibility"` // private / public
	Review      string `gorm:"size:16;not null;default:approved;index" json:"review"`    // 私有默认 approved；公开需审核
	Status      string `gorm:"size:16;not null;default:active" json:"status"`            // active / deleted

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// 运行时
	OwnerName string `gorm:"-" json:"owner_name,omitempty"`
}

func (OcGroup) TableName() string { return "oc_groups" }

// ─── OC 组成员关系 ───

// OcGroupMember OC 组 ↔ 角色卡 绑定（多对多中间表）
//
// 成员即角色卡（已含 furry 字段），这里只存引用 + 排序，人设始终以角色卡为准。
// 不重复存储人设，避免快照与角色卡更新脱节。
type OcGroupMember struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	GroupID     uint      `gorm:"not null;index:idx_ocgm" json:"group_id"`
	CharacterID uint      `gorm:"not null;index:idx_ocgm" json:"character_id"`
	SortOrder   int       `gorm:"not null;default:0" json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`

	// 运行时
	Character *Character `gorm:"-" json:"character,omitempty"`
}

func (OcGroupMember) TableName() string { return "oc_group_members" }

// ─── 入参 ───

// OcGroupInput OC 组入参
type OcGroupInput struct {
	Name         string
	Summary      string
	CoverURL     string
	Tags         string
	Visibility   string
	CharacterIDs []uint // 创建/更新时一次性设置的成员角色卡 ID（可选）
}

func normalizeOcGroupInput(in *OcGroupInput) {
	in.Name = strings.TrimSpace(in.Name)
	in.Summary = strings.TrimSpace(in.Summary)
	in.CoverURL = strings.TrimSpace(in.CoverURL)
	in.Tags = strings.TrimSpace(in.Tags)
	in.Visibility = NormalizeVisibility(in.Visibility)
}

// ─── CRUD ───

// CreateOcGroup 创建 OC 组（公开需 pending 待审）
func CreateOcGroup(userID uint, in OcGroupInput) (*OcGroup, error) {
	normalizeOcGroupInput(&in)
	if in.Name == "" {
		return nil, errors.New("OC组名称不能为空")
	}
	if len([]rune(in.Name)) > MaxOcGroupNameRunes {
		return nil, errors.New("OC组名称过长")
	}
	review := ReviewApproved
	if in.Visibility == VisPublic {
		review = ReviewPending
	}
	g := &OcGroup{
		UserID:     userID,
		Name:       in.Name,
		Summary:    in.Summary,
		CoverURL:   in.CoverURL,
		Tags:       in.Tags,
		Visibility: in.Visibility,
		Review:     review,
		Status:     "active",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := models.DB.Create(g).Error; err != nil {
		return nil, err
	}
	// 创建时附带成员
	if len(in.CharacterIDs) > 0 {
		if err := addOcGroupMembers(g.ID, userID, in.CharacterIDs); err != nil {
			return g, err
		}
		recalcOcGroupMemberCount(g.ID)
	}
	return g, nil
}

// UpdateOcGroup 更新 OC 组元信息（转公开需重新审核）
func UpdateOcGroup(userID uint, id uint, in OcGroupInput) (*OcGroup, error) {
	var g OcGroup
	if err := models.DB.Where("id = ? AND user_id = ? AND status = ?", id, userID, "active").First(&g).Error; err != nil {
		return nil, errors.New("OC组不存在")
	}
	normalizeOcGroupInput(&in)
	if in.Name == "" {
		return nil, errors.New("OC组名称不能为空")
	}
	if in.Visibility == VisPublic && g.Visibility != VisPublic {
		g.Review = ReviewPending // 转公开需重新审核
	}
	g.Name = in.Name
	g.Summary = in.Summary
	g.CoverURL = in.CoverURL
	g.Tags = in.Tags
	g.Visibility = in.Visibility
	g.UpdatedAt = time.Now()
	if err := models.DB.Save(&g).Error; err != nil {
		return nil, err
	}
	return &g, nil
}

// DeleteOcGroup 软删除 OC 组（连带成员一并清退，留证）
func DeleteOcGroup(userID uint, id uint) error {
	var g OcGroup
	if err := models.DB.Where("id = ? AND user_id = ? AND status = ?", id, userID, "active").First(&g).Error; err != nil {
		return errors.New("OC组不存在")
	}
	return models.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&OcGroupMember{}).Where("group_id = ?", id).Delete(&OcGroupMember{}).Error; err != nil {
			return err
		}
		return tx.Model(&OcGroup{}).Where("id = ?", id).
			Updates(map[string]interface{}{"status": "deleted", "visibility": VisPrivate}).Error
	})
}

// GetOcGroup 读取（私有仅本人/管理员；公开需审核通过）
func GetOcGroup(id uint, viewerID uint, isAdmin bool) (*OcGroup, error) {
	var g OcGroup
	if err := models.DB.Where("id = ?", id).First(&g).Error; err != nil {
		return nil, errors.New("OC组不存在")
	}
	if g.Status != "active" && !isAdmin {
		return nil, errors.New("OC组已删除")
	}
	if g.Visibility == VisPublic {
		if g.Review != ReviewApproved && g.UserID != viewerID && !isAdmin {
			return nil, errors.New("OC组待审核")
		}
	} else if g.UserID != viewerID && !isAdmin {
		return nil, errors.New("OC组未公开")
	}
	fillOcGroupOwnerNames([]*OcGroup{&g})
	return &g, nil
}

// ListOcGroups 我的 OC 组 + 可选公开库
func ListOcGroups(userID uint, scope string, keyword string, page, pageSize int) ([]OcGroup, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	q := models.DB.Model(&OcGroup{}).Where("status = ?", "active")
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
	var list []OcGroup
	if err := q.Order("updated_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	ptrs := make([]*OcGroup, len(list))
	for i := range list {
		ptrs[i] = &list[i]
	}
	fillOcGroupOwnerNames(ptrs)
	return list, total, nil
}

func fillOcGroupOwnerNames(list []*OcGroup) {
	if len(list) == 0 {
		return
	}
	ids := map[uint]bool{}
	for _, g := range list {
		ids[g.UserID] = true
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
	for _, g := range list {
		g.OwnerName = nameOf[g.UserID]
	}
}

// ─── 成员管理 ───

// AddOcGroupMember 增加成员（去重；只接受本人拥有的 active 角色卡）
func AddOcGroupMember(groupID, userID, characterID uint) error {
	var g OcGroup
	if err := models.DB.Where("id = ? AND user_id = ? AND status = ?", groupID, userID, "active").First(&g).Error; err != nil {
		return errors.New("OC组不存在")
	}
	// 角色卡必须本人所有且 active
	var c Character
	if err := models.DB.Where("id = ? AND user_id = ? AND status = ?", characterID, userID, "active").First(&c).Error; err != nil {
		return errors.New("角色卡不可加入")
	}
	var cnt int64
	models.DB.Model(&OcGroupMember{}).Where("group_id = ? AND character_id = ?", groupID, characterID).Count(&cnt)
	if cnt > 0 {
		return nil // 已存在，幂等
	}
	// 排序：当前最大 +1
	var maxOrder int
	models.DB.Model(&OcGroupMember{}).Where("group_id = ?", groupID).Select("COALESCE(MAX(sort_order),0)").Scan(&maxOrder)
	m := &OcGroupMember{
		GroupID:     groupID,
		CharacterID: characterID,
		SortOrder:   maxOrder + 1,
		CreatedAt:   time.Now(),
	}
	if err := models.DB.Create(m).Error; err != nil {
		return err
	}
	recalcOcGroupMemberCount(groupID)
	return nil
}

// RemoveOcGroupMember 移除成员
func RemoveOcGroupMember(groupID, userID, characterID uint) error {
	var g OcGroup
	if err := models.DB.Where("id = ? AND user_id = ? AND status = ?", groupID, userID, "active").First(&g).Error; err != nil {
		return errors.New("OC组不存在")
	}
	if err := models.DB.Where("group_id = ? AND character_id = ?", groupID, characterID).Delete(&OcGroupMember{}).Error; err != nil {
		return err
	}
	recalcOcGroupMemberCount(groupID)
	return nil
}

// ReorderOcGroupMembers 调整成员排序（传入有序的 characterID 列表）
func ReorderOcGroupMembers(groupID, userID uint, characterIDs []uint) error {
	var g OcGroup
	if err := models.DB.Where("id = ? AND user_id = ? AND status = ?", groupID, userID, "active").First(&g).Error; err != nil {
		return errors.New("OC组不存在")
	}
	for i, cid := range characterIDs {
		models.DB.Model(&OcGroupMember{}).Where("group_id = ? AND character_id = ?", groupID, cid).
			Update("sort_order", i+1)
	}
	return nil
}

func addOcGroupMembers(groupID, userID uint, characterIDs []uint) error {
	for _, cid := range characterIDs {
		if err := AddOcGroupMember(groupID, userID, cid); err != nil {
			return err
		}
	}
	return nil
}

func recalcOcGroupMemberCount(groupID uint) {
	var cnt int64
	models.DB.Model(&OcGroupMember{}).Where("group_id = ?", groupID).Count(&cnt)
	models.DB.Model(&OcGroup{}).Where("id = ?", groupID).Update("member_count", cnt)
}

// ListOcGroupMembers 列出 OC 组成员，按排序返回，并注入角色卡（含 furry 字段）
func ListOcGroupMembers(groupID uint) ([]OcGroupMember, error) {
	var members []OcGroupMember
	if err := models.DB.Where("group_id = ?", groupID).Order("sort_order ASC, id ASC").Find(&members).Error; err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return members, nil
	}
	cids := make([]uint, 0, len(members))
	for _, m := range members {
		cids = append(cids, m.CharacterID)
	}
	var chars []Character
	models.DB.Where("id IN ? AND status = ?", cids, "active").Find(&chars)
	byID := map[uint]*Character{}
	for i := range chars {
		byID[chars[i].ID] = &chars[i]
	}
	for i := range members {
		if c, ok := byID[members[i].CharacterID]; ok {
			members[i].Character = c
		}
	}
	return members, nil
}

// GetOcGroupCharacters 直接取 OC 组全部角色卡（供群组召唤整组人设使用）
func GetOcGroupCharacters(groupID uint) ([]Character, error) {
	var members []OcGroupMember
	if err := models.DB.Where("group_id = ?", groupID).Order("sort_order ASC, id ASC").Find(&members).Error; err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return nil, nil
	}
	cids := make([]uint, 0, len(members))
	for _, m := range members {
		cids = append(cids, m.CharacterID)
	}
	var chars []Character
	models.DB.Where("id IN ? AND status = ?", cids, "active").Find(&chars)
	// 按成员排序重排（IN 查询不保序）
	order := map[uint]int{}
	for i, m := range members {
		order[m.CharacterID] = i
	}
	byID := map[uint]*Character{}
	for i := range chars {
		byID[chars[i].ID] = &chars[i]
	}
	out := make([]Character, 0, len(chars))
	for _, m := range members {
		if c, ok := byID[m.CharacterID]; ok {
			out = append(out, *c)
		}
	}
	_ = order
	return out, nil
}

// ReviewOcGroup 平台审核公开 OC 组（admin）
func ReviewOcGroup(id uint, approve bool, note string) error {
	_ = note
	review := ReviewRejected
	if approve {
		review = ReviewApproved
	}
	return models.DB.Model(&OcGroup{}).Where("id = ?", id).
		Updates(map[string]interface{}{"review": review, "updated_at": time.Now()}).Error
}
