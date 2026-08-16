package handlers

import (
	"strconv"
	"strings"
	"time"

	"nvs-server/models"
	"nvs-server/security"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	"github.com/microcosm-cc/bluemonday"
)


// GET /api/admin/users — 用户列表
func ListUsers(c *gin.Context) {
	if !ensureAdmin(c) { return }
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	search := c.Query("search")

	query := models.DB.Model(&models.User{})
	if search != "" {
		like := "%" + search + "%"
		query = query.Where("username LIKE ? OR nickname LIKE ? OR email LIKE ?", like, like, like)
	}

	var total int64
	query.Count(&total)

	var users []models.User
	query.Offset((page - 1) * 20).Limit(20).Order("id DESC").Find(&users)
	utils.Success(c, gin.H{"list": users, "total": total})
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

// DELETE /api/admin/users/:id — 删除用户
func DeleteUser(c *gin.Context) {
	if !ensureAdmin(c) { return }
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	// 不能删除自己
	adminID := c.GetUint("userID")
	if uint(id) == adminID {
		utils.BadRequest(c, "不能删除自己的账户")
		return
	}

	models.DB.Delete(&models.User{}, id)
	utils.Success(c, gin.H{"message": "用户已删除"})
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
	utils.Success(c, gin.H{"message": "已解除禁言"})
}