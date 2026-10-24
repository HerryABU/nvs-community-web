package groups

import (
	"nvs-server/models"

	"fmt"
	"strings"
	"time"
)

// ==================== 群组成员与角色 ====================
// 角色：owner（群主）/ admin（群管理员）/ member（成员）
// 状态：active（正常）/ pending（申请待审）
// 权限矩阵：
//   owner  ：改设置、任命/撤销管理员、踢人、禁言、解散群
//   admin  ：审核帖子、删帖、禁言普通成员
//   member ：发帖、回复、上传/下载群文件
// 平台管理员（admin/police）越过群内权限，但操作留审计。

// GroupMember 群成员
type GroupMember struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	GroupID   uint       `gorm:"not null;uniqueIndex:idx_group_member" json:"group_id"`
	UserID    uint       `gorm:"not null;uniqueIndex:idx_group_member;index:idx_group_member_user" json:"user_id"`
	Role      string     `gorm:"size:16;not null;default:member" json:"role"`
	Status    string     `gorm:"size:16;not null;default:active;index" json:"status"`
	MutedTill *time.Time `json:"muted_until"`
	Note      string     `gorm:"size:128;default:''" json:"note"` // 审核备注（申请理由/处理说明）
	JoinedAt  time.Time  `json:"joined_at"`
	UpdatedAt time.Time  `json:"updated_at"`

	// 运行时字段
	Username string `gorm:"-" json:"username,omitempty"`
	Nickname string `gorm:"-" json:"nickname,omitempty"`
	Avatar   string `gorm:"-" json:"avatar_url,omitempty"`
}

func (GroupMember) TableName() string { return "group_members" }

// GetMembership 取成员记录（不存在返回 nil, nil）
func GetMembership(groupID, userID uint) (*GroupMember, error) {
	if groupID == 0 || userID == 0 {
		return nil, nil
	}
	var m GroupMember
	err := models.DB.Where("group_id = ? AND user_id = ?", groupID, userID).First(&m).Error
	if err != nil {
		return nil, nil
	}
	return &m, nil
}

// GroupRoleOf 我在群内的角色（非成员返回空）
func GroupRoleOf(g *Group, userID uint) (string, bool) {
	m, err := GetMembership(g.ID, userID)
	if err != nil || m == nil || m.Status != GroupMemberActive {
		return "", false
	}
	return m.Role, true
}

// GroupRolesOf 批量取角色（列表页避免 N+1）
func GroupRolesOf(userID uint, groupIDs []uint) map[uint]GroupMember {
	out := map[uint]GroupMember{}
	if userID == 0 || len(groupIDs) == 0 {
		return out
	}
	var rows []GroupMember
	models.DB.Where("user_id = ? AND group_id IN ?", userID, groupIDs).Find(&rows)
	for _, r := range rows {
		out[r.GroupID] = r
	}
	return out
}

// IsGroupMemberActive 是否正常成员
func IsGroupMemberActive(groupID, userID uint) bool {
	m, _ := GetMembership(groupID, userID)
	return m != nil && m.Status == GroupMemberActive
}

// JoinGroup 加入群组：open 直接进；apply 待审；invite 校验邀请码
// 返回加入后的状态（active/pending）
func JoinGroup(g *Group, user *models.User, inviteCode string) (string, error) {
	if g == nil || user == nil {
		return "", fmt.Errorf("参数错误")
	}
	if g.Status == GroupStatusBanned {
		return "", fmt.Errorf("该群组已被平台封禁")
	}
	if g.Status == GroupStatusArchived {
		return "", fmt.Errorf("该群组已归档，不再接受新成员")
	}
	// 未成年保护：禁止加入半公开/私密群
	if user.MinorProtected && g.Visibility != GroupVisibilityPublic {
		return "", fmt.Errorf("未成年人账号暂不支持加入半公开/私密群组")
	}
	// 已在群内 / 已申请
	if m, _ := GetMembership(g.ID, user.ID); m != nil {
		if m.Status == GroupMemberActive {
			return GroupMemberActive, nil
		}
		// 待审状态允许用正确邀请码直接通过（invite 模式）
		if g.JoinMode != GroupJoinInvite || !strings.EqualFold(strings.TrimSpace(inviteCode), g.InviteCode) {
			return GroupMemberPending, fmt.Errorf("你已提交申请，等待群主审核")
		}
		models.DB.Model(&GroupMember{}).Where("id = ?", m.ID).Updates(map[string]interface{}{
			"status": GroupMemberActive, "joined_at": time.Now(), "updated_at": time.Now(),
		})
		RefreshGroupCounts(g.ID)
		return GroupMemberActive, nil
	}
	// 人数上限
	if g.MemberLimit > 0 && g.MemberCount >= int64(g.MemberLimit) {
		return "", fmt.Errorf("该群组人数已满（上限 %d）", g.MemberLimit)
	}

	switch g.JoinMode {
	case GroupJoinInvite:
		if !strings.EqualFold(strings.TrimSpace(inviteCode), g.InviteCode) || strings.TrimSpace(g.InviteCode) == "" {
			return "", fmt.Errorf("邀请码不正确")
		}
		if err := createMember(g.ID, user.ID, GroupRoleMember, GroupMemberActive, "邀请码加入"); err != nil {
			return "", err
		}
		return GroupMemberActive, nil
	case GroupJoinApply:
		if err := createMember(g.ID, user.ID, GroupRoleMember, GroupMemberPending, truncateRunes(strings.TrimSpace(inviteCode), 100)); err != nil {
			return "", err
		}
		return GroupMemberPending, nil
	default: // open
		if err := createMember(g.ID, user.ID, GroupRoleMember, GroupMemberActive, ""); err != nil {
			return "", err
		}
		return GroupMemberActive, nil
	}
}

func createMember(groupID, userID uint, role, status, note string) error {
	m := &GroupMember{
		GroupID: groupID, UserID: userID, Role: role, Status: status,
		Note: note, JoinedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := models.DB.Create(m).Error; err != nil {
		return fmt.Errorf("加入失败")
	}
	// status/role 用 map 精确写回（GORM struct 零值/默认值坑）
	return models.DB.Model(&GroupMember{}).Where("id = ?", m.ID).Updates(map[string]interface{}{
		"role": role, "status": status, "note": note,
	}).Error
}

// ApproveMember 审核通过加入申请（群主/管理员）
func ApproveMember(groupID, operatorID uint, operatorRole string, targetUserID uint, approve bool) error {
	g, err := GetGroup(groupID)
	if err != nil {
		return fmt.Errorf("群组不存在")
	}
	role, ok := GroupRoleOf(g, operatorID)
	if !ok && !IsGroupPlatformAdmin(operatorRole) {
		return fmt.Errorf("无权操作")
	}
	if role != GroupRoleOwner && role != GroupRoleAdmin && !IsGroupPlatformAdmin(operatorRole) {
		return fmt.Errorf("仅群主/群管理员可审核申请")
	}
	m, _ := GetMembership(groupID, targetUserID)
	if m == nil || m.Status != GroupMemberPending {
		return fmt.Errorf("该用户没有待审申请")
	}
	if !approve {
		return models.DB.Where("id = ?", m.ID).Delete(&GroupMember{}).Error
	}
	if g.MemberLimit > 0 && g.MemberCount >= int64(g.MemberLimit) {
		return fmt.Errorf("群成员已满（上限 %d）", g.MemberLimit)
	}
	if err := models.DB.Model(&GroupMember{}).Where("id = ?", m.ID).Updates(map[string]interface{}{
		"status": GroupMemberActive, "joined_at": time.Now(), "updated_at": time.Now(),
	}).Error; err != nil {
		return err
	}
	RefreshGroupCounts(groupID)
	return nil
}

// LeaveGroup 主动退群（群主需先转让或解散）
func LeaveGroup(groupID, userID uint) error {
	g, err := GetGroup(groupID)
	if err != nil {
		return fmt.Errorf("群组不存在")
	}
	m, _ := GetMembership(groupID, userID)
	if m == nil {
		return fmt.Errorf("你不在该群组")
	}
	if m.Role == GroupRoleOwner {
		return fmt.Errorf("群主需先转让群组或解散群组")
	}
	if err := models.DB.Where("id = ?", m.ID).Delete(&GroupMember{}).Error; err != nil {
		return err
	}
	_ = g
	RefreshGroupCounts(groupID)
	return nil
}

// RemoveMember 踢人（群主/管理员；管理员不能踢群主/其他管理员）
func RemoveMember(groupID, operatorID uint, operatorRole string, targetUserID uint) error {
	g, err := GetGroup(groupID)
	if err != nil {
		return fmt.Errorf("群组不存在")
	}
	opRole, ok := GroupRoleOf(g, operatorID)
	platform := IsGroupPlatformAdmin(operatorRole)
	if !ok && !platform {
		return fmt.Errorf("无权操作")
	}
	if opRole == GroupRoleMember && !platform {
		return fmt.Errorf("仅群主/群管理员可移除成员")
	}
	if targetUserID == operatorID {
		return LeaveGroup(groupID, targetUserID)
	}
	target, _ := GetMembership(groupID, targetUserID)
	if target == nil {
		return fmt.Errorf("该用户不在群组")
	}
	if target.Role == GroupRoleOwner {
		return fmt.Errorf("不能移除群主")
	}
	if !platform && opRole == GroupRoleAdmin && target.Role == GroupRoleAdmin {
		return fmt.Errorf("管理员不能移除其他管理员")
	}
	if err := models.DB.Where("id = ?", target.ID).Delete(&GroupMember{}).Error; err != nil {
		return err
	}
	RefreshGroupCounts(groupID)
	return nil
}

// SetMemberRole 任命/撤销管理员（仅群主）
func SetMemberRole(groupID, operatorID uint, operatorRole string, targetUserID uint, role string) error {
	g, err := GetGroup(groupID)
	if err != nil {
		return fmt.Errorf("群组不存在")
	}
	if role != GroupRoleAdmin && role != GroupRoleMember {
		return fmt.Errorf("只能设为管理员或普通成员")
	}
	opRole, ok := GroupRoleOf(g, operatorID)
	if !ok && !IsGroupPlatformAdmin(operatorRole) {
		return fmt.Errorf("无权操作")
	}
	if opRole != GroupRoleOwner && !IsGroupPlatformAdmin(operatorRole) {
		return fmt.Errorf("仅群主可任命管理员")
	}
	target, _ := GetMembership(groupID, targetUserID)
	if target == nil || target.Status != GroupMemberActive {
		return fmt.Errorf("该用户不是群成员")
	}
	if target.Role == GroupRoleOwner {
		return fmt.Errorf("不能修改群主角色")
	}
	return models.DB.Model(&GroupMember{}).Where("id = ?", target.ID).Updates(map[string]interface{}{
		"role": role, "updated_at": time.Now(),
	}).Error
}

// MuteMember 群内禁言（0 = 解除）
func MuteMember(groupID, operatorID uint, operatorRole string, targetUserID uint, minutes int) error {
	g, err := GetGroup(groupID)
	if err != nil {
		return fmt.Errorf("群组不存在")
	}
	opRole, ok := GroupRoleOf(g, operatorID)
	platform := IsGroupPlatformAdmin(operatorRole)
	if !ok && !platform {
		return fmt.Errorf("无权操作")
	}
	if opRole == GroupRoleMember && !platform {
		return fmt.Errorf("仅群主/群管理员可禁言")
	}
	if minutes < 0 {
		minutes = 0
	}
	if minutes > 60*24*30 {
		minutes = 60 * 24 * 30
	}
	target, _ := GetMembership(groupID, targetUserID)
	if target == nil {
		return fmt.Errorf("该用户不在群组")
	}
	if target.Role == GroupRoleOwner {
		return fmt.Errorf("不能禁言群主")
	}
	if !platform && opRole == GroupRoleAdmin && target.Role == GroupRoleAdmin {
		return fmt.Errorf("管理员不能禁言其他管理员")
	}
	var until *time.Time
	if minutes > 0 {
		t := time.Now().Add(time.Duration(minutes) * time.Minute)
		until = &t
	}
	return models.DB.Model(&GroupMember{}).Where("id = ?", target.ID).Updates(map[string]interface{}{
		"muted_till": until, "updated_at": time.Now(),
	}).Error
}

// IsMutedInGroup 群内是否处于禁言期
func IsMutedInGroup(groupID, userID uint) (bool, time.Time) {
	m, _ := GetMembership(groupID, userID)
	if m == nil || m.MutedTill == nil {
		return false, time.Time{}
	}
	return m.MutedTill.After(time.Now()), *m.MutedTill
}

// ListGroupMembers 成员列表（status 为空则 active+pending）
func ListGroupMembers(groupID uint, status string, page, size int) ([]GroupMember, int64, error) {
	q := models.DB.Model(&GroupMember{}).Where("group_id = ?", groupID)
	if s := strings.TrimSpace(status); s != "" {
		q = q.Where("status = ?", s)
	}
	var total int64
	q.Count(&total)
	var list []GroupMember
	if err := q.Order("FIELD(role,'owner','admin','member'), joined_at ASC").
		Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	fillMemberProfiles(list)
	return list, total, nil
}

// fillMemberProfiles 批量填充用户名/昵称/头像
func fillMemberProfiles(list []GroupMember) {
	if len(list) == 0 {
		return
	}
	ids := make([]uint, 0, len(list))
	for _, m := range list {
		ids = append(ids, m.UserID)
	}
	var users []models.User
	models.DB.Select("id, username, nickname, avatar_url").Where("id IN ?", ids).Find(&users)
	byID := map[uint]models.User{}
	for _, u := range users {
		byID[u.ID] = u
	}
	for i := range list {
		if u, ok := byID[list[i].UserID]; ok {
			list[i].Username = u.Username
			list[i].Nickname = u.Nickname
			list[i].Avatar = u.AvatarURL
		}
	}
}

// InviteMember 拉人入群：群主/管理员可邀请；普通成员需 Group.AllowMemberInvite 才可邀请。
// 被邀请者直接成为正常成员（不需要申请），但仍受人数上限与未成年限制。
func InviteMember(g *Group, operatorID uint, operatorRole, operatorUsername, targetUsername string) (*GroupMember, error) {
	if g == nil {
		return nil, fmt.Errorf("群组不存在")
	}
	if g.Status != GroupStatusActive {
		return nil, fmt.Errorf("该群组当前不可加入")
	}
	opRole, ok := GroupRoleOf(g, operatorID)
	platform := IsGroupPlatformAdmin(operatorRole)
	if !ok && !platform {
		return nil, fmt.Errorf("你不在该群组，无法邀请")
	}
	if opRole == GroupRoleMember && !g.AllowMemberInvite && !platform {
		return nil, fmt.Errorf("该群已关闭「成员邀请」，请联系群主")
	}

	target := strings.TrimSpace(targetUsername)
	if target == "" {
		return nil, fmt.Errorf("请填写要邀请的用户名或群号")
	}
	var user models.User
	if err := models.DB.Where("username = ?", target).First(&user).Error; err != nil {
		// 兼容按昵称精确匹配
		if err2 := models.DB.Where("nickname = ?", target).First(&user).Error; err2 != nil {
			return nil, fmt.Errorf("用户 %s 不存在", target)
		}
	}
	if user.ID == operatorID {
		return nil, fmt.Errorf("不能邀请自己")
	}
	if user.MinorProtected && g.Visibility != GroupVisibilityPublic {
		return nil, fmt.Errorf("未成年人账号不能加入半公开/私密群组")
	}
	if m, _ := GetMembership(g.ID, user.ID); m != nil {
		if m.Status == GroupMemberActive {
			return nil, fmt.Errorf("%s 已在群内", target)
		}
		// 待审申请 → 直接通过
		if err := models.DB.Model(&GroupMember{}).Where("id = ?", m.ID).Updates(map[string]interface{}{
			"status": GroupMemberActive, "joined_at": time.Now(), "updated_at": time.Now(),
			"note": "由 " + operatorUsername + " 邀请",
		}).Error; err != nil {
			return nil, fmt.Errorf("邀请失败")
		}
		RefreshGroupCounts(g.ID)
		m.Status = GroupMemberActive
		return m, nil
	}
	if g.MemberLimit > 0 && g.MemberCount >= int64(g.MemberLimit) {
		return nil, fmt.Errorf("群成员已满（上限 %d）", g.MemberLimit)
	}
	if err := createMember(g.ID, user.ID, GroupRoleMember, GroupMemberActive,
		"由 "+operatorUsername+" 邀请"); err != nil {
		return nil, err
	}
	RefreshGroupCounts(g.ID)
	m, _ := GetMembership(g.ID, user.ID)
	if m != nil {
		_ = models.PushNotification(operatorID, user.ID, "group",
			"你被邀请加入群组："+g.Name,
			operatorUsername+" 邀请你加入了群组，快去看看",
			fmt.Sprintf("/group/%d", g.ID))
	}
	return m, nil
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
