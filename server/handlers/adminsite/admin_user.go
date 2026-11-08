package adminsite

import (
	"fmt"
	"nvs-server/handlers/admincommon"
	"nvs-server/handlers/admingov"
	"nvs-server/utils/infra"
	"strconv"
	"strings"
	"time"

	"nvs-server/handlers/emergency"
	"nvs-server/handlers/hctx"
	"nvs-server/models"
	"nvs-server/security"

	"github.com/gin-gonic/gin"
	"github.com/microcosm-cc/bluemonday"
	apikey "nvs-server/models/apikey"
	siteops "nvs-server/models/siteops"
	social "nvs-server/models/social"
)

// GET /api/admin/users — 用户列表（支持关键词搜索 + 角色分类 + 状态筛选）
// query: page, search(用户名/昵称/邮箱), role(角色分类), status(active/muted/banned)
func ListUsers(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	search := c.Query("search")
	role := c.Query("role")
	status := c.Query("status") // active / muted / banned(软删除)

	// banned 用户是软删除记录，需 Unscoped 才能查到；其余状态默认排除软删除
	db := models.DB.Model(&models.User{})
	if status == "banned" {
		db = db.Unscoped()
	}

	if search != "" {
		like := "%" + search + "%"
		db = db.Where("username LIKE ? OR nickname LIKE ? OR email LIKE ?", like, like, like)
	}
	if role == "super" {
		// 「超级管理员」筛选：唯一超管（显式 super_* 或回退的首位 admin）
		db = db.Where("id = ?", admincommon.SuperAdminID())
	} else if role != "" {
		db = db.Where("role = ?", role)
	}
	switch status {
	case "muted":
		db = db.Where("muted_until IS NOT NULL AND muted_until > ?", time.Now())
	case "active":
		db = db.Where("(muted_until IS NULL OR muted_until <= ?) AND deleted_at IS NULL", time.Now())
	case "banned":
		db = db.Where("deleted_at IS NOT NULL")
	}

	var total int64
	db.Count(&total)

	var users []models.User
	db.Offset((page - 1) * 20).Limit(20).Order("id DESC").Find(&users)
	// 回填「是否超级管理员」（非持久化字段），供前端区分命名
	superID := admincommon.SuperAdminID()
	for i := range users {
		users[i].IsSuper = social.IsSuperRole(users[i].Role) || users[i].ID == superID
	}
	infra.Success(c, gin.H{"list": users, "total": total})
}

// GET /api/admin/users/audit — 用户行为审计日志（176号令合规查询，可按 user_id/action 筛选）
func ListUserAuditLogs(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	userID, _ := strconv.ParseUint(c.Query("user_id"), 10, 64)
	action := c.Query("action")
	logs, err := apikey.ListUserAuditLogs(uint(userID), action, 500)
	if err != nil {
		infra.InternalError(c, "获取审计日志失败")
		return
	}
	infra.Success(c, logs)
}

// PUT /api/admin/users/:id — 修改用户（支持 role、nickname、email）
func UpdateUser(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Role     *string `json:"role"`
		Nickname *string `json:"nickname"`
		Email    *string `json:"email"`
	}
	c.ShouldBindJSON(&req)

	// 禁止通过通用编辑修改自己的角色（身份）——防止站长自我降级或自我提权。
	// 仅拦截 role 字段；昵称、邮箱等资料仍可正常修改。
	if req.Role != nil && uint(id) == hctx.CurrentUserID(c) {
		infra.Forbidden(c, "不能修改自己的角色（身份）")
		return
	}

	updates := map[string]interface{}{}
	if req.Role != nil {
		// police 角色必须通过专用提权接口设置（留审计），不允许通用编辑直接改
		if *req.Role == social.RolePolice {
			infra.BadRequest(c, "警方角色需通过「提权为警方」专用接口设置")
			return
		}
		if !social.ValidPromotableRoles()[*req.Role] {
			infra.BadRequest(c, "无效的角色: "+*req.Role)
			return
		}
		// 管理端角色（admin / manager）仅超级管理员可指派；
		// 普通管理员不得把他人提升为管理员，避免越权。
		if social.IsElevated(*req.Role) && !admincommon.IsSuperAdmin(c) {
			infra.Forbidden(c, "仅超级管理员（站长）可指派管理端角色")
			return
		}
		updates["role"] = *req.Role
	}
	if req.Nickname != nil {
		updates["nickname"] = bluemonday.UGCPolicy().Sanitize(*req.Nickname)
	}
	if req.Email != nil {
		updates["email"] = strings.TrimSpace(*req.Email)
	}
	if len(updates) > 0 {
		models.DB.Model(&models.User{}).Where("id = ?", id).Updates(updates)
	}
	infra.Success(c, gin.H{"message": "更新成功"})
}

// DELETE /api/admin/users/:id — 删除/封禁用户（软删除 + 留证）
func DeleteUser(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	// 不能删除自己
	adminID := hctx.CurrentUserID(c)
	if uint(id) == adminID {
		infra.BadRequest(c, "不能删除自己的账户")
		return
	}

	if err := admingov.SoftDeleteContent(c, "user", uint(id), "管理员封禁/删除用户", "admin", "ban"); err != nil {
		if err == admingov.ErrContentNotFound {
			infra.NotFound(c, "用户不存在")
		} else {
			infra.InternalError(c, err.Error())
		}
		return
	}
	infra.Success(c, gin.H{"message": "用户已封禁（软删除并留证）"})
}

// POST /api/admin/users/:id/mute — 全局禁言用户
func MuteUser(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	var req struct {
		DurationHours int    `json:"duration_hours"` // 0=永久
		Reason        string `json:"reason"`
	}
	c.ShouldBindJSON(&req)

	var until *time.Time
	if req.DurationHours > 0 {
		t := time.Now().Add(time.Duration(req.DurationHours) * time.Hour)
		until = &t
	}

	models.DB.Model(&models.User{}).Where("id = ?", id).Updates(map[string]interface{}{
		"muted_until": until,
		"mute_reason": security.SanitizePlainText(req.Reason),
	})
	// 合规审计：禁言用户
	duration := "永久"
	if req.DurationHours > 0 {
		duration = fmt.Sprintf("%d 小时", req.DurationHours)
	}
	emergency.AuditEmergency(c, siteops.AuditUserMute, "user", uint(id),
		fmt.Sprintf("管理员禁言用户ID %d，时长 %s，原因：%s",
			id, duration, security.SanitizePlainText(req.Reason)))
	infra.Success(c, gin.H{"message": "已禁言"})
}

// POST /api/admin/users/:id/unmute — 解除全局禁言
func UnmuteUser(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	models.DB.Model(&models.User{}).Where("id = ?", id).Updates(map[string]interface{}{
		"muted_until": nil,
		"mute_reason": "",
	})
	// 合规审计：解除禁言
	emergency.AuditEmergency(c, siteops.AuditUserUnmute, "user", uint(id),
		fmt.Sprintf("管理员解除禁言用户ID %d", id))
	infra.Success(c, gin.H{"message": "已解除禁言"})
}

// POST /api/admin/users/:id/promote-police — 管理员提权为警方取管台账号
// 警方角色不可自注册，只能由管理员在此显式提权（留审计痕迹），防止越权/漏洞提权。
func AdminPromoteToPolice(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if uint(id) == hctx.CurrentUserID(c) {
		infra.BadRequest(c, "不能对自己提权")
		return
	}
	u, err := models.GetUserByID(uint(id))
	if err != nil {
		infra.NotFound(c, "用户不存在")
		return
	}
	if u.Role == social.RoleAdmin {
		infra.BadRequest(c, "管理员无需提权为警方")
		return
	}
	operator := ""
	if admin, aerr := models.GetUserByID(hctx.CurrentUserID(c)); aerr == nil && admin != nil {
		operator = admin.Username
	}
	now := time.Now()
	if err := models.DB.Model(&models.User{}).Where("id = ?", id).Updates(map[string]interface{}{
		"role":              social.RolePolice,
		"police_granted_by": operator,
		"police_granted_at": &now,
	}).Error; err != nil {
		infra.InternalError(c, "提权失败")
		return
	}
	emergency.AuditEmergency(c, siteops.AuditUserPromotePolice, "user", uint(id),
		fmt.Sprintf("管理员 %s 将用户 %s(ID %d) 提权为警方取管台账号", operator, u.Username, id))
	infra.Success(c, gin.H{
		"message":           "已提权为警方取管台账号",
		"role":              social.RolePolice,
		"police_granted_by": operator,
		"police_granted_at": now,
	})
}

// POST /api/admin/users/:id/revoke-police — 撤销警方取管台权限（回退为读者）
func AdminRevokePolice(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	u, err := models.GetUserByID(uint(id))
	if err != nil {
		infra.NotFound(c, "用户不存在")
		return
	}
	if u.Role != social.RolePolice {
		infra.BadRequest(c, "该用户不是警方账号")
		return
	}
	operator := ""
	if admin, aerr := models.GetUserByID(hctx.CurrentUserID(c)); aerr == nil && admin != nil {
		operator = admin.Username
	}
	if err := models.DB.Model(&models.User{}).Where("id = ?", id).Updates(map[string]interface{}{
		"role":              social.RoleReader,
		"police_granted_by": "",
		"police_granted_at": nil,
	}).Error; err != nil {
		infra.InternalError(c, "撤销失败")
		return
	}
	emergency.AuditEmergency(c, siteops.AuditUserRevokePolice, "user", uint(id),
		fmt.Sprintf("管理员 %s 撤销用户 %s(ID %d) 的警方取管台权限", operator, u.Username, id))
	infra.Success(c, gin.H{"message": "已撤销警方权限", "role": social.RoleReader})
}
