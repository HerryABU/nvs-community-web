package groups

import (
	"nvs-server/models"

	"crypto/rand"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ==================== 群组（半私密/私密协作空间） ====================
// 定位：在公开社区（论坛/博客）之外提供「群组」——有可见性、角色、成员上限、群内发帖与文件。
// 与既有体系的关系：
//   - 审核：群内帖子照样过违禁词库（processBadWords），私密不豁免平台审核；
//   - 额度：建群/人数/每日发帖走 GroupQuotaConfig + 按人覆盖（与论坛配额、发文额度同一套思路）；
//   - 未成年：MinorProtected 用户禁止建群、禁止加入 semi/private 群；
//   - 举报：复用 reports 表（target_type = group_post / group）。
//
// 状态语义：
//   status        active（正常）/ archived（归档只读）/ banned（平台封禁）
//   visibility    public（公开）/ semi（半公开：存在可见，内容仅成员）/ private（私密：仅成员）
//   join_mode     open（直接加入）/ apply（申请待审）/ invite（邀请码）

const (
	GroupVisibilityPublic  = "public"
	GroupVisibilitySemi    = "semi"
	GroupVisibilityPrivate = "private"

	GroupJoinOpen   = "open"
	GroupJoinApply  = "apply"
	GroupJoinInvite = "invite"

	GroupStatusActive   = "active"
	GroupStatusArchived = "archived"
	GroupStatusBanned   = "banned"

	GroupRoleOwner  = "owner"
	GroupRoleAdmin  = "admin"
	GroupRoleMember = "member"

	GroupMemberActive  = "active"
	GroupMemberPending = "pending"

	// 群组名/简介长度上限（按 rune 计）
	groupNameMax = 32
	groupDescMax = 500
)

// Group 群组
type Group struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	OwnerID     uint   `gorm:"not null;index" json:"owner_id"`
	GroupNo     string `gorm:"size:16;uniqueIndex;not null;default:''" json:"group_no"` // 群号（6~8 位数字，可直接搜索/加入）
	Name        string `gorm:"size:64;not null;index" json:"name"`
	Description string `gorm:"type:text" json:"description"`
	Category    string `gorm:"size:32;default:'';index" json:"category"`
	Visibility  string `gorm:"size:16;not null;default:public;index" json:"visibility"`
	JoinMode    string `gorm:"size:16;not null;default:open" json:"join_mode"`
	InviteCode  string `gorm:"size:16;default:''" json:"-"` // 邀请码（invite 模式；只对群主/管理员可见）
	CoverURL    string `gorm:"size:512;default:''" json:"cover_url"`
	MemberLimit int    `gorm:"not null;default:200" json:"member_limit"`      // 人数上限（建群时被全局配置钳制）
	PostLimit   int    `gorm:"not null;default:20" json:"post_limit_per_day"` // 每人每日群内发帖上限（0=不限）
	ReviewFirst bool   `gorm:"not null;default:false" json:"review_first"`    // 先审后发（true 时新帖 pending）
	Status      string `gorm:"size:16;not null;default:active;index" json:"status"`
	MemberCount int64  `gorm:"not null;default:0" json:"member_count"`
	PostCount   int64  `gorm:"not null;default:0" json:"post_count"`

	// 群公告（置顶展示 + 变更时通知成员）
	Announcement   string     `gorm:"type:text" json:"announcement"`
	AnnouncementBy uint       `gorm:"not null;default:0" json:"announcement_by"`
	AnnouncementAt *time.Time `json:"announcement_at"`

	// 拉人机制：成员是否也能邀请（群主/管理员始终可邀请）
	AllowMemberInvite bool `gorm:"not null;default:true" json:"allow_member_invite"`

	// AI 智能体接入：群内 @ai 时由绑定的智能体接管（未绑定则用平台默认）
	AiEnabled bool `gorm:"not null;default:false" json:"ai_enabled"`
	AiAgentID uint `gorm:"not null;default:0" json:"ai_agent_id"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// 运行时字段（不落库）
	OwnerName   string `gorm:"-" json:"owner_name,omitempty"`
	MyRole      string `gorm:"-" json:"my_role,omitempty"`     // 我在群内的角色（非成员为空）
	MyStatus    string `gorm:"-" json:"my_status,omitempty"`   // active/pending
	CanRead     bool   `gorm:"-" json:"can_read"`              // 当前用户能否读内容
	InviteCode2 string `gorm:"-" json:"invite_code,omitempty"` // 仅群主/管理员下发
}

func (Group) TableName() string { return "groups" }

// NormalizeGroupVisibility 归一化可见性（非法/空 → public）
func NormalizeGroupVisibility(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case GroupVisibilitySemi:
		return GroupVisibilitySemi
	case GroupVisibilityPrivate:
		return GroupVisibilityPrivate
	default:
		return GroupVisibilityPublic
	}
}

// NormalizeGroupJoinMode 归一化加入方式（非法/空 → open）
func NormalizeGroupJoinMode(m string) string {
	switch strings.ToLower(strings.TrimSpace(m)) {
	case GroupJoinApply:
		return GroupJoinApply
	case GroupJoinInvite:
		return GroupJoinInvite
	default:
		return GroupJoinOpen
	}
}

// GroupVisibilityLabel 可见性中文标签（审计/通知用）
func GroupVisibilityLabel(v string) string {
	switch v {
	case GroupVisibilitySemi:
		return "半公开"
	case GroupVisibilityPrivate:
		return "私密"
	default:
		return "公开"
	}
}

// GroupInput 建群/改设置入参
type GroupInput struct {
	Name        string
	Description string
	Category    string
	Visibility  string
	JoinMode    string
	CoverURL    string
	MemberLimit int
	PostLimit   int
	ReviewFirst bool
	// 群管理（AllowMemberInvite 用指针区分「未传」与「显式 false」，未传时默认 true）
	AllowMemberInvite *bool
	AiEnabled         bool
	AiAgentID         uint
}

// generateGroupNo 生成唯一群号（7 位数字，首位非 0）
func generateGroupNo() (string, error) {
	for i := 0; i < 30; i++ {
		b := make([]byte, 8)
		if _, err := rand.Read(b); err != nil {
			return "", fmt.Errorf("生成群号失败")
		}
		// 首位 1~9，其余 0~9
		no := strconv.Itoa(int(b[0]%9) + 1)
		for j := 1; j < 7; j++ {
			no += strconv.Itoa(int(b[j] % 10))
		}
		var n int64
		models.DB.Unscoped().Model(&Group{}).Where("group_no = ?", no).Count(&n)
		if n == 0 {
			return no, nil
		}
	}
	return "", fmt.Errorf("群号分配失败，请重试")
}

// GetGroupByNo 按群号取群（加入/搜索用）
func GetGroupByNo(no string) (*Group, error) {
	no = strings.TrimSpace(no)
	if no == "" {
		return nil, fmt.Errorf("群号不能为空")
	}
	var g Group
	if err := models.DB.Where("group_no = ?", no).First(&g).Error; err != nil {
		return nil, err
	}
	return &g, nil
}

// SetGroupAnnouncement 发布群公告（群主/管理员；平台管理员可强制）
func SetGroupAnnouncement(groupID, operatorID uint, operatorRole, content string) error {
	g, err := GetGroup(groupID)
	if err != nil {
		return fmt.Errorf("群组不存在")
	}
	role, ok := GroupRoleOf(g, operatorID)
	platform := IsGroupPlatformAdmin(operatorRole)
	if !ok && !platform {
		return fmt.Errorf("无权操作")
	}
	if role != GroupRoleOwner && role != GroupRoleAdmin && !platform {
		return fmt.Errorf("仅群主/群管理员可发布公告")
	}
	content = strings.TrimSpace(content)
	if len([]rune(content)) > 2000 {
		return fmt.Errorf("公告过长（上限 2000 字）")
	}
	now := time.Now()
	if content == "" {
		return models.DB.Model(&Group{}).Where("id = ?", groupID).Updates(map[string]interface{}{
			"announcement": "", "announcement_by": operatorID, "announcement_at": now, "updated_at": now,
		}).Error
	}
	if err := models.DB.Model(&Group{}).Where("id = ?", groupID).Updates(map[string]interface{}{
		"announcement": content, "announcement_by": operatorID,
		"announcement_at": now, "updated_at": now,
	}).Error; err != nil {
		return err
	}
	// 通知成员（最多 200 人，异步避免拖慢请求）
	go func() {
		defer func() { _ = recover() }()
		var members []GroupMember
		models.DB.Where("group_id = ? AND status = ?", groupID, GroupMemberActive).Limit(200).Find(&members)
		for _, m := range members {
			if m.UserID == operatorID {
				continue
			}
			_ = models.PushNotification(operatorID, m.UserID, "group",
				"群公告更新："+g.Name, truncateRunes(content, 80),
				fmt.Sprintf("/group/%d", groupID))
		}
	}()
	return nil
}

// IsGroupPlatformAdmin 平台管理员/警方（可越过群组权限）
func IsGroupPlatformAdmin(role string) bool {
	return role == models.RoleAdmin || role == models.RolePolice
}

// generateInviteCode 生成 8 位邀请码（大写字母+数字，剔除易混淆 0/O/1/I）
func generateInviteCode() string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano()%1e8, 10)
	}
	out := make([]byte, 8)
	for i, v := range b {
		out[i] = alphabet[int(v)%len(alphabet)]
	}
	return string(out)
}

// CreateGroup 建群：额度 + 未成年 + 名称校验，创建者自动成为 owner 成员
func CreateGroup(ownerID uint, user *models.User, in GroupInput) (*Group, error) {
	if ownerID == 0 || user == nil {
		return nil, fmt.Errorf("请先登录")
	}
	// 未成年保护：禁止建群（需求 §3 / §10）
	if user.MinorProtected {
		return nil, fmt.Errorf("未成年人账号暂不支持创建群组")
	}
	name := strings.TrimSpace(in.Name)
	if len([]rune(name)) < 2 || len([]rune(name)) > groupNameMax {
		return nil, fmt.Errorf("群组名需 2~%d 个字符", groupNameMax)
	}
	desc := strings.TrimSpace(in.Description)
	if len([]rune(desc)) > groupDescMax {
		return nil, fmt.Errorf("群组简介不能超过 %d 个字符", groupDescMax)
	}

	cfg := GetGroupQuotaConfig()
	maxGroups, memberCap, dailyPosts := EffectiveGroupQuota(ownerID, user.Role)
	if maxGroups == 0 {
		return nil, fmt.Errorf("你当前没有创建群组的额度（可联系站长调整）")
	}
	if maxGroups > 0 {
		var n int64
		models.DB.Model(&Group{}).Where("owner_id = ?", ownerID).Count(&n)
		if int(n) >= maxGroups {
			return nil, fmt.Errorf("最多创建 %d 个群组（当前 %d 个）", maxGroups, n)
		}
	}

	limit := in.MemberLimit
	if limit <= 0 {
		limit = cfg.MemberLimit
	}
	if memberCap > 0 && limit > memberCap {
		limit = memberCap
	}
	if limit < 2 {
		limit = 2
	}
	allowInvite := true
	if in.AllowMemberInvite != nil {
		allowInvite = *in.AllowMemberInvite
	}
	postLimit := in.PostLimit
	if postLimit < 0 {
		postLimit = 0
	}
	if postLimit == 0 {
		postLimit = dailyPosts
	}

	g := &Group{
		OwnerID: ownerID, Name: name, Description: desc,
		Category:    strings.TrimSpace(in.Category),
		Visibility:  NormalizeGroupVisibility(in.Visibility),
		JoinMode:    NormalizeGroupJoinMode(in.JoinMode),
		CoverURL:    strings.TrimSpace(in.CoverURL),
		MemberLimit: limit, PostLimit: postLimit, ReviewFirst: in.ReviewFirst,
		AllowMemberInvite: allowInvite, AiEnabled: in.AiEnabled, AiAgentID: in.AiAgentID,
		Status: GroupStatusActive,
		// 私密/半公开默认先审后发：线下流量小、审核成本可控，也更贴合「私密空间」预期
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	no, nerr := generateGroupNo()
	if nerr != nil {
		return nil, nerr
	}
	g.GroupNo = no
	if g.JoinMode == GroupJoinInvite {
		g.InviteCode = generateInviteCode()
	}
	if g.Visibility != GroupVisibilityPublic {
		g.ReviewFirst = true
	}
	if err := models.DB.Create(g).Error; err != nil {
		return nil, fmt.Errorf("创建群组失败")
	}
	// owner 成员记录（用 map 精确写，避免零值被默认值覆盖）
	if err := models.DB.Create(&GroupMember{
		GroupID: g.ID, UserID: ownerID, Role: GroupRoleOwner,
		Status: GroupMemberActive, JoinedAt: time.Now(), UpdatedAt: time.Now(),
	}).Error; err != nil {
		return nil, fmt.Errorf("创建群组失败（成员记录）")
	}
	models.DB.Model(&Group{}).Where("id = ?", g.ID).Updates(map[string]interface{}{
		"member_count": 1,
		// 布尔零值必须精确写回，否则被列默认值（allow_member_invite 默认 true）覆盖
		"allow_member_invite": allowInvite,
		"ai_enabled":          in.AiEnabled,
		"ai_agent_id":         in.AiAgentID,
		"group_no":            no,
	})
	g.AllowMemberInvite = allowInvite
	g.AiEnabled = in.AiEnabled
	g.AiAgentID = in.AiAgentID
	g.MemberCount = 1
	return g, nil
}

// GetGroup 读取群组（含软删排除）
func GetGroup(id uint) (*Group, error) {
	var g Group
	if err := models.DB.Where("id = ?", id).First(&g).Error; err != nil {
		return nil, err
	}
	return &g, nil
}

// GetGroupUnscoped 读取群组（含软删除，供警方/管理端取证）
func GetGroupUnscoped(id uint) (*Group, error) {
	var g Group
	if err := models.DB.Unscoped().Where("id = ?", id).First(&g).Error; err != nil {
		return nil, err
	}
	return &g, nil
}

// UpdateGroupSettings 群设置修改（群主/群管理员；平台管理员亦可）
func UpdateGroupSettings(groupID, operatorID uint, operatorRole string, in GroupInput) error {
	g, err := GetGroup(groupID)
	if err != nil {
		return fmt.Errorf("群组不存在")
	}
	role, ok := GroupRoleOf(g, operatorID)
	if !ok && !IsGroupPlatformAdmin(operatorRole) {
		return fmt.Errorf("无权修改该群组")
	}
	if role != GroupRoleOwner && role != GroupRoleAdmin && !IsGroupPlatformAdmin(operatorRole) {
		return fmt.Errorf("仅群主/群管理员可修改设置")
	}
	name := strings.TrimSpace(in.Name)
	if name != "" {
		if len([]rune(name)) < 2 || len([]rune(name)) > groupNameMax {
			return fmt.Errorf("群组名需 2~%d 个字符", groupNameMax)
		}
	}
	desc := strings.TrimSpace(in.Description)
	if len([]rune(desc)) > groupDescMax {
		return fmt.Errorf("群组简介不能超过 %d 个字符", groupDescMax)
	}
	cfg := GetGroupQuotaConfig()
	limit := in.MemberLimit
	if limit <= 0 {
		limit = g.MemberLimit
	}
	if cfg.MemberLimit > 0 && limit > cfg.MemberLimit {
		limit = cfg.MemberLimit
	}
	if limit < int(g.MemberCount) {
		limit = int(g.MemberCount) // 不得把上限设到低于现有人数
	}
	fields := map[string]interface{}{
		"description": desc,
		"category":    strings.TrimSpace(in.Category),
		"cover_url":   strings.TrimSpace(in.CoverURL),
		"updated_at":  time.Now(),
	}
	if name != "" {
		fields["name"] = name
	}
	if strings.TrimSpace(in.Visibility) != "" {
		v := NormalizeGroupVisibility(in.Visibility)
		fields["visibility"] = v
		// 转私密/半公开时默认开启先审后发（可后续手动关闭）
		if v != GroupVisibilityPublic && !g.ReviewFirst {
			fields["review_first"] = true
		}
	}
	if strings.TrimSpace(in.JoinMode) != "" {
		jm := NormalizeGroupJoinMode(in.JoinMode)
		fields["join_mode"] = jm
		if jm == GroupJoinInvite && g.InviteCode == "" {
			fields["invite_code"] = generateInviteCode()
		}
	}
	fields["member_limit"] = limit
	fields["post_limit_per_day"] = in.PostLimit
	fields["review_first"] = in.ReviewFirst
	allowInvite := g.AllowMemberInvite
	if in.AllowMemberInvite != nil {
		allowInvite = *in.AllowMemberInvite
	}
	fields["allow_member_invite"] = allowInvite
	fields["ai_enabled"] = in.AiEnabled
	fields["ai_agent_id"] = in.AiAgentID
	return models.DB.Model(&Group{}).Where("id = ?", groupID).Updates(fields).Error
}

// RefreshGroupCounts 重算成员数与帖子数（写操作后调用，避免累计漂移）
func RefreshGroupCounts(groupID uint) {
	var members, posts int64
	models.DB.Model(&GroupMember{}).Where("group_id = ? AND status = ?", groupID, GroupMemberActive).Count(&members)
	models.DB.Model(&GroupPost{}).Where("group_id = ? AND status = ?", groupID, GroupPostPublished).Count(&posts)
	models.DB.Model(&Group{}).Where("id = ?", groupID).
		Updates(map[string]interface{}{"member_count": members, "post_count": posts})
}

// GroupDiscoverItem 发现页列表项
type GroupDiscoverItem struct {
	Group
	IsMember bool `json:"is_member"`
}

// ListDiscoverGroups 发现页：公开 + 半公开（私密群不出现在发现页）
func ListDiscoverGroups(userID uint, category, keyword string, page, size int) ([]GroupDiscoverItem, int64, error) {
	q := models.DB.Model(&Group{}).Where("status = ? AND visibility IN ?", GroupStatusActive,
		[]string{GroupVisibilityPublic, GroupVisibilitySemi})
	if c := strings.TrimSpace(category); c != "" {
		q = q.Where("category = ?", c)
	}
	if kw := strings.TrimSpace(keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("name LIKE ? OR description LIKE ?", like, like)
	}
	var total int64
	q.Count(&total)

	var list []Group
	if err := q.Order("member_count DESC, id DESC").
		Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	items := make([]GroupDiscoverItem, 0, len(list))
	roles := GroupRolesOf(userID, groupIDsOf(list))
	for i := range list {
		it := GroupDiscoverItem{Group: list[i]}
		if r, ok := roles[list[i].ID]; ok {
			it.IsMember = true
			it.Group.MyRole = r.Role
			it.Group.MyStatus = r.Status
		}
		it.CanRead = CanReadGroupContent(&list[i], it.Group.MyRole, it.Group.MyStatus)
		if it.Group.MyRole == GroupRoleOwner || it.Group.MyRole == GroupRoleAdmin {
			it.InviteCode2 = list[i].InviteCode
		}
		it.InviteCode = ""
		items = append(items, it)
	}
	return items, total, nil
}

// ListMyGroups 我的群组（含待审申请）
func ListMyGroups(userID uint) ([]GroupDiscoverItem, error) {
	var members []GroupMember
	if err := models.DB.Where("user_id = ?", userID).Order("joined_at DESC").Limit(200).Find(&members).Error; err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return []GroupDiscoverItem{}, nil
	}
	ids := make([]uint, 0, len(members))
	roleOf := map[uint]GroupMember{}
	for _, m := range members {
		ids = append(ids, m.GroupID)
		roleOf[m.GroupID] = m
	}
	var groups []Group
	models.DB.Where("id IN ?", ids).Find(&groups)
	byID := map[uint]Group{}
	for _, g := range groups {
		byID[g.ID] = g
	}
	out := make([]GroupDiscoverItem, 0, len(groups))
	for _, id := range ids {
		g, ok := byID[id]
		if !ok {
			continue
		}
		m := roleOf[id]
		it := GroupDiscoverItem{Group: g, IsMember: m.Status == GroupMemberActive}
		it.Group.MyRole = m.Role
		it.Group.MyStatus = m.Status
		it.CanRead = CanReadGroupContent(&g, m.Role, m.Status)
		if m.Role == GroupRoleOwner || m.Role == GroupRoleAdmin {
			it.InviteCode2 = g.InviteCode
		}
		it.InviteCode = ""
		out = append(out, it)
	}
	return out, nil
}

// CanReadGroupContent 读内容判定：公开群所有人；半公开/私密仅正常成员；封禁群仅平台管理员
func CanReadGroupContent(g *Group, role, memberStatus string) bool {
	if g == nil {
		return false
	}
	if g.Status == GroupStatusBanned {
		return false
	}
	if g.Visibility == GroupVisibilityPublic {
		return true
	}
	return memberStatus == GroupMemberActive && role != ""
}

// GroupCategoryList 群组分类（去重）
func GroupCategoryList() []string {
	var cats []string
	models.DB.Model(&Group{}).Where("status = ? AND category <> ''", GroupStatusActive).
		Distinct().Order("category ASC").Pluck("category", &cats)
	return cats
}

// SearchGroupNames 管理端/搜索用：按关键词取群名（限 20）
func SearchGroupNames(keyword string) []Group {
	var list []Group
	like := "%" + strings.TrimSpace(keyword) + "%"
	models.DB.Where("name LIKE ?", like).Order("id DESC").Limit(20).Find(&list)
	return list
}

func groupIDsOf(list []Group) []uint {
	ids := make([]uint, 0, len(list))
	for _, g := range list {
		ids = append(ids, g.ID)
	}
	return ids
}
