package admincommon

// 管理端角色分层（用户明确要求）：
//
//   - **超级管理员（站长）**：全站唯一一位，可增删改查，包括删除规则。
//   - **管理员**：可以有任意多位，但**只能新增 / 编辑规则，不能删除**。
//     需要删除时提交申请，由超级管理员批准；申请与批准都会通知超级管理员。
//
// 兼容性：既有站点只有 role=admin 这一类账号，不能靠新增角色把老站长锁在门外。
// 因此「超级管理员」= role=admin 且 ID 最小的那一位（即建站时创建的第一位管理员），
// 或显式标记为 role=super_admin 的账号。其余 role=admin 自动视为「管理员」。
// 这样老站点零迁移即可获得分层，且天然保证「唯一一位」。

import (
	"fmt"

	"nvs-server/handlers/hctx"
	"nvs-server/models"
	"nvs-server/utils/infra"

	"github.com/gin-gonic/gin"
)

const (
	RoleSuperAdmin = "super_admin"
	RoleAdmin      = "admin"
	RoleManager    = "manager"
	RolePolice     = "police"
)

// IsElevated 是否是管理端账号（超级管理员 / 管理员；警方走自己的台，不算）。
func IsElevated(role string) bool {
	return role == RoleSuperAdmin || role == RoleAdmin || role == RoleManager
}

// SuperAdminID 返回唯一超级管理员的用户 ID。
// 优先取显式 role=super_admin 中 ID 最小者；没有则退回落库的 role=admin 中 ID 最小者。
func SuperAdminID() uint {
	var u models.User
	if err := models.DB.Where("role = ?", RoleSuperAdmin).Order("id ASC").First(&u).Error; err == nil && u.ID > 0 {
		return u.ID
	}
	if err := models.DB.Where("role = ?", RoleAdmin).Order("id ASC").First(&u).Error; err == nil && u.ID > 0 {
		return u.ID
	}
	return 0
}

// IsSuperAdmin 当前登录者是否为唯一超级管理员（站长）。
func IsSuperAdmin(c *gin.Context) bool {
	uid := hctx.CurrentUserID(c)
	if uid == 0 {
		return false
	}
	return uid == SuperAdminID()
}

// EnsureElevated 管理端基础鉴权（超级管理员 + 管理员都可通过）。
func EnsureElevated(c *gin.Context) bool {
	uid := hctx.CurrentUserID(c)
	u, _ := models.GetUserByID(uid)
	if u == nil || !IsElevated(u.Role) {
		infra.Forbidden(c, "需要管理员权限")
		c.Abort()
		return false
	}
	return true
}

// EnsureSuperAdmin 仅超级管理员可通过（删除、角色调整等破坏性操作）。
func EnsureSuperAdmin(c *gin.Context) bool {
	if !IsSuperAdmin(c) {
		infra.Forbidden(c, "仅超级管理员（站长）可执行此操作")
		c.Abort()
		return false
	}
	return true
}

// CanDelete 当前账号是否可直接删除。普通管理员返回 false。
func CanDelete(c *gin.Context) bool { return IsSuperAdmin(c) }

// NotifySuperAdmin 向唯一超级管理员推送站内通知（best-effort）。
// 普通管理员提交删除申请、以及任何需要站长知悉的动作都走这里。
func NotifySuperAdmin(actorID uint, notifType, title, body, link string) {
	sid := SuperAdminID()
	if sid == 0 {
		return
	}
	_ = models.PushNotification(actorID, sid, notifType, title, body, link)
}

// DeleteRequestHint 普通管理员被拦下时给前端的引导文案（修改/删除统一口径）。
func DeleteRequestHint() string {
	return "仅超级管理员（站长）可直接修改或删除；已提交修改/删除申请并通知站长，等待批准后方可生效。"
}

// RoleLabel 角色显示名（前端与通知文案统一口径）。
func RoleLabel(role string) string {
	switch role {
	case RoleSuperAdmin:
		return "超级管理员（站长）"
	case RoleAdmin, RoleManager:
		return "管理员"
	case RolePolice:
		return "警方"
	default:
		return role
	}
}

// DescribeActor 通知里描述操作者。
func DescribeActor(c *gin.Context) (uint, string) {
	uid := hctx.CurrentUserID(c)
	u, _ := models.GetUserByID(uid)
	name := fmt.Sprintf("用户 #%d", uid)
	if u != nil {
		name = u.Username
	}
	return uid, name
}
