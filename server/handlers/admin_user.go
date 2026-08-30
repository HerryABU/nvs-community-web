package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"nvs-server/models"
	"nvs-server/security"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	"github.com/microcosm-cc/bluemonday"
)


// GET /api/admin/users — 用户列表（支持关键词搜索 + 角色分类 + 状态筛选）
// query: page, search(用户名/昵称/邮箱), role(角色分类), status(active/muted/banned)
func ListUsers(c *gin.Context) {
	if !ensureAdmin(c) { return }
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
	if role != "" {
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
	utils.Success(c, gin.H{"list": users, "total": total})
}

// GET /api/admin/users/audit — 用户行为审计日志（176号令合规查询，可按 user_id/action 筛选）
func ListUserAuditLogs(c *gin.Context) {
	if !ensureAdmin(c) { return }
	userID, _ := strconv.ParseUint(c.Query("user_id"), 10, 64)
	action := c.Query("action")
	logs, err := models.ListUserAuditLogs(uint(userID), action, 500)
	if err != nil {
		utils.InternalError(c, "获取审计日志失败")
		return
	}
	utils.Success(c, logs)
}

// PUT /api/admin/users/:id — 修改用户（支持 role、nickname、email）
func UpdateUser(c *gin.Context) {
	if !ensureAdmin(c) { return }
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Role     *string `json:"role"`
		Nickname *string `json:"nickname"`
		Email    *string `json:"email"`
	}
	c.ShouldBindJSON(&req)

	updates := map[string]interface{}{}
	if req.Role != nil {
		// police 角色必须通过专用提权接口设置（留审计），不允许通用编辑直接改
		if *req.Role == models.RolePolice {
			utils.BadRequest(c, "警方角色需通过「提权为警方」专用接口设置")
			return
		}
		if !models.ValidPromotableRoles()[*req.Role] {
			utils.BadRequest(c, "无效的角色: "+*req.Role)
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
	utils.Success(c, gin.H{"message": "更新成功"})
}

// DELETE /api/admin/users/:id — 删除/封禁用户（软删除 + 留证）
func DeleteUser(c *gin.Context) {
	if !ensureAdmin(c) { return }
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	// 不能删除自己
	adminID := currentUserID(c)
	if uint(id) == adminID {
		utils.BadRequest(c, "不能删除自己的账户")
		return
	}

	if err := softDeleteContent(c, "user", uint(id), "管理员封禁/删除用户", "admin", "ban"); err != nil {
		if err == errContentNotFound {
			utils.NotFound(c, "用户不存在")
		} else {
			utils.InternalError(c, err.Error())
		}
		return
	}
	utils.Success(c, gin.H{"message": "用户已封禁（软删除并留证）"})
}

// POST /api/admin/users/:id/mute — 全局禁言用户
func MuteUser(c *gin.Context) {
	if !ensureAdmin(c) { return }
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
	auditEmergency(c, models.AuditUserMute, "user", uint(id),
		fmt.Sprintf("管理员禁言用户ID %d，时长 %s，原因：%s",
			id, duration, security.SanitizePlainText(req.Reason)))
	utils.Success(c, gin.H{"message": "已禁言"})
}

// POST /api/admin/users/:id/unmute — 解除全局禁言
func UnmuteUser(c *gin.Context) {
	if !ensureAdmin(c) { return }
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	models.DB.Model(&models.User{}).Where("id = ?", id).Updates(map[string]interface{}{
		"muted_until": nil,
		"mute_reason": "",
	})
	// 合规审计：解除禁言
	auditEmergency(c, models.AuditUserUnmute, "user", uint(id),
		fmt.Sprintf("管理员解除禁言用户ID %d", id))
	utils.Success(c, gin.H{"message": "已解除禁言"})
}

// POST /api/admin/users/:id/promote-police — 管理员提权为警方取管台账号
// 警方角色不可自注册，只能由管理员在此显式提权（留审计痕迹），防止越权/漏洞提权。
func AdminPromoteToPolice(c *gin.Context) {
	if !ensureAdmin(c) { return }
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if uint(id) == currentUserID(c) {
		utils.BadRequest(c, "不能对自己提权")
		return
	}
	u, err := models.GetUserByID(uint(id))
	if err != nil {
		utils.NotFound(c, "用户不存在")
		return
	}
	if u.Role == models.RoleAdmin {
		utils.BadRequest(c, "管理员无需提权为警方")
		return
	}
	operator := ""
	if admin, aerr := models.GetUserByID(currentUserID(c)); aerr == nil && admin != nil {
		operator = admin.Username
	}
	now := time.Now()
	if err := models.DB.Model(&models.User{}).Where("id = ?", id).Updates(map[string]interface{}{
		"role":               models.RolePolice,
		"police_granted_by": operator,
		"police_granted_at": &now,
	}).Error; err != nil {
		utils.InternalError(c, "提权失败")
		return
	}
	auditEmergency(c, models.AuditUserPromotePolice, "user", uint(id),
		fmt.Sprintf("管理员 %s 将用户 %s(ID %d) 提权为警方取管台账号", operator, u.Username, id))
	utils.Success(c, gin.H{
		"message":           "已提权为警方取管台账号",
		"role":              models.RolePolice,
		"police_granted_by": operator,
		"police_granted_at": now,
	})
}

// POST /api/admin/users/:id/revoke-police — 撤销警方取管台权限（回退为读者）
func AdminRevokePolice(c *gin.Context) {
	if !ensureAdmin(c) { return }
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	u, err := models.GetUserByID(uint(id))
	if err != nil {
		utils.NotFound(c, "用户不存在")
		return
	}
	if u.Role != models.RolePolice {
		utils.BadRequest(c, "该用户不是警方账号")
		return
	}
	operator := ""
	if admin, aerr := models.GetUserByID(currentUserID(c)); aerr == nil && admin != nil {
		operator = admin.Username
	}
	if err := models.DB.Model(&models.User{}).Where("id = ?", id).Updates(map[string]interface{}{
		"role":               models.RoleReader,
		"police_granted_by": "",
		"police_granted_at": nil,
	}).Error; err != nil {
		utils.InternalError(c, "撤销失败")
		return
	}
	auditEmergency(c, models.AuditUserRevokePolice, "user", uint(id),
		fmt.Sprintf("管理员 %s 撤销用户 %s(ID %d) 的警方取管台权限", operator, u.Username, id))
	utils.Success(c, gin.H{"message": "已撤销警方权限", "role": models.RoleReader})
}