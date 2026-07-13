package handlers

import (
	"strconv"
	"strings"

	"nvs-server/models"
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