package oc

// ==================== 权限判定 ====================
// OC 是群组的玩法层，治理沿用群组：群主/管理员管配置与 AI 角色；
// 成员只能用自己的角色卡发言；一键发布限群主/管理员/流发起人。

import (
	"nvs-server/groups"
	"nvs-server/models"
)

// CanManageGroup 是否可管理该群的 OC 配置（群主/管理员/平台）
func CanManageGroup(groupID, userID uint, isPlatform bool) bool {
	if groupID == 0 || userID == 0 {
		return false
	}
	if isPlatform {
		return true
	}
	m, err := groups.GetMembership(groupID, userID)
	if err != nil || m == nil || m.Status != groups.GroupMemberActive {
		return false
	}
	return m.Role == groups.GroupRoleOwner || m.Role == groups.GroupRoleAdmin
}

// CanOperateFlow 是否可推进/暂停/发布该对话流（群管理 或 流发起人 或 平台）
func CanOperateFlow(f *Flow, userID uint, isPlatform bool) bool {
	if f == nil || userID == 0 {
		return false
	}
	if isPlatform || f.StarterID == userID {
		return true
	}
	return CanManageGroup(f.GroupID, userID, false)
}

// IsGroupMember OC 玩法要求群成员身份（私密群不对外开放）
func IsGroupMember(groupID, userID uint) bool {
	return groups.IsGroupMemberActive(groupID, userID)
}

// GroupMode 群组当前 OC 模式（未启用返回空串）
func GroupMode(groupID uint) string {
	set := GetGroupSetting(groupID)
	if !set.Enabled {
		return ""
	}
	return set.Mode
}

// GroupOwnerID 群主 ID（用于账单归属与通知）
func GroupOwnerID(groupID uint) uint {
	var g groups.Group
	if err := models.DB.Where("id = ?", groupID).First(&g).Error; err != nil {
		return 0
	}
	return g.OwnerID
}
