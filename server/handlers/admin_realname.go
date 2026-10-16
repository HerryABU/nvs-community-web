// admin_realname.go — 管理端实名记录查询（F10）。
// 姓名解密后仅在管理员响应中返回；列表默认脱敏，详情可查看明文（取证需要）。
package handlers

import (
	"strconv"
	"strings"
	"time"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// GET /api/admin/realname — 实名用户列表（分页 + 关键词）
// query: page, page_size, keyword(匹配用户名/昵称/身份证脱敏串)
func AdminListRealName(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	keyword := strings.TrimSpace(c.Query("keyword"))

	q := models.DB.Model(&models.User{}).Where("real_name_verified = ?", true)
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("username LIKE ? OR nickname LIKE ? OR id_card_masked LIKE ?", like, like, like)
	}

	var total int64
	q.Count(&total)

	var users []models.User
	if err := q.Order("real_name_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&users).Error; err != nil {
		utils.InternalError(c, "查询失败")
		return
	}

	rows := make([]gin.H, 0, len(users))
	for i := range users {
		u := &users[i]
		name := ""
		if dec, err := utils.DecryptField(u.RealName); err == nil {
			name = dec
		}
		age := 0
		if u.BirthDate != nil {
			age = utils.AgeAt(*u.BirthDate, time.Now())
		}
		rows = append(rows, gin.H{
			"user_id":          u.ID,
			"username":         u.Username,
			"nickname":         u.Nickname,
			"role":             u.Role,
			"phone":            u.Phone,
			"real_name":        name,
			"id_card_masked":   u.IDCardMasked,
			"minor_protected":  u.MinorProtected,
			"age":              age,
			"real_name_at":     u.RealNameAt,
		})
	}

	utils.Success(c, gin.H{
		"list":      rows,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}
