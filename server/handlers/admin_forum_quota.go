package handlers

// 自建论坛额度管控（管理员）：
//   - 全局默认：每人最多可创建几个自建论坛（0 = 不限制）
//   - 按人覆盖：对特定用户单独放开或削减（0 = 直接剥夺其建版权利）
//
// 路由前缀刻意用 /admin/forum-quota 而非 /admin/forums/quota：
// gin 的路由树不允许静态段与同级通配段共存，/forums/:id 已占用通配，
// 故单独开一棵子树，避免注册时 panic。

import (
	"strconv"
	"strings"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// GetForumQuotaConfig GET /api/admin/forum-quota —— 全局默认额度
func GetForumQuotaConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	cfg := models.LoadForumQuotaConfig()
	quotas, err := models.ListUserForumQuotas()
	if err != nil {
		quotas = []models.UserForumQuota{}
	}
	utils.Success(c, gin.H{"config": cfg, "overrides": quotas})
}

// UpdateForumQuotaConfig PUT /api/admin/forum-quota —— 保存全局默认额度（立即生效）
func UpdateForumQuotaConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		MaxPerUser int `json:"max_per_user"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if req.MaxPerUser < 0 || req.MaxPerUser > 1000 {
		utils.BadRequest(c, "每人上限需在 0 ~ 1000 之间（0 = 不限制）")
		return
	}
	if err := models.SaveForumQuotaConfig(models.ForumQuotaConfig{MaxPerUser: req.MaxPerUser}); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.Success(c, gin.H{"message": "已保存（立即生效）"})
}

// SetUserForumQuota PUT /api/admin/forum-quota/users/:id —— 为特定用户单独设置建版额度
// max_forums：-1 = 跟随全局；0 = 禁止创建；>0 = 该用户专属上限
func SetUserForumQuota(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	uid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if uid == 0 {
		utils.BadRequest(c, "用户 ID 无效")
		return
	}
	if _, err := models.GetUserByID(uint(uid)); err != nil {
		utils.NotFound(c, "用户不存在")
		return
	}
	var req struct {
		MaxForums *int   `json:"max_forums"`
		Note      string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if req.MaxForums == nil {
		utils.BadRequest(c, "请填写 max_forums（-1=跟随全局，0=禁止创建）")
		return
	}
	if *req.MaxForums < -1 || *req.MaxForums > 1000 {
		utils.BadRequest(c, "额度需在 -1 ~ 1000 之间（-1=跟随全局，0=禁止创建）")
		return
	}
	q := models.GetUserForumQuota(uint(uid))
	if q == nil {
		q = &models.UserForumQuota{UserID: uint(uid)}
	}
	q.MaxForums = *req.MaxForums
	q.Note = strings.TrimSpace(req.Note)
	if err := models.SaveUserForumQuota(q); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.Success(c, gin.H{"message": "已保存（立即生效）", "quota": models.ForumQuotaFor(uint(uid))})
}

// DeleteUserForumQuota DELETE /api/admin/forum-quota/users/:id —— 撤销按人设置（恢复跟随全局）
func DeleteUserForumQuota(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	uid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	models.DB.Where("user_id = ?", uid).Delete(&models.UserForumQuota{})
	utils.Success(c, gin.H{"message": "已撤销该用户的额度设置（恢复跟随全局）"})
}

// ListUserForumQuotas GET /api/admin/forum-quota/users —— 所有被单独设置过的用户
func ListUserForumQuotas(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	list, err := models.ListUserForumQuotas()
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, list)
}
