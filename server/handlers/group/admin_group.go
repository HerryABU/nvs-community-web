package grouphandler

import (
	"nvs-server/groups"
	"nvs-server/middleware"
	"strconv"
	"strings"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== 群组管理端 ====================
// 站长视角：群列表（含封禁态）、封禁/解封、强制解散、额度配置（全局 + 按人）。
// 路由用 /admin/group-quota 独立子树：/admin/groups/:id 已占通配段，避免静态段冲突。

// AdminListGroups GET /api/admin/groups?status=&q=&page=
func AdminListGroups(c *gin.Context) {
	if !requirePlatformAdmin(c) {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	q := models.DB.Model(&groups.Group{})
	if s := strings.TrimSpace(c.Query("status")); s != "" {
		q = q.Where("status = ?", s)
	}
	if v := strings.TrimSpace(c.Query("visibility")); v != "" {
		q = q.Where("visibility = ?", v)
	}
	if kw := strings.TrimSpace(c.Query("q")); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("name LIKE ? OR group_no = ? OR description LIKE ?", like, kw, like)
	}
	var total int64
	q.Count(&total)
	var list []groups.Group
	if err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	// 补群主名
	if len(list) > 0 {
		ids := make([]uint, 0, len(list))
		for _, g := range list {
			ids = append(ids, g.OwnerID)
		}
		var users []models.User
		models.DB.Select("id, username, nickname").Where("id IN ?", ids).Find(&users)
		byID := map[uint]models.User{}
		for _, u := range users {
			byID[u.ID] = u
		}
		for i := range list {
			if u, ok := byID[list[i].OwnerID]; ok {
				if strings.TrimSpace(u.Nickname) != "" {
					list[i].OwnerName = u.Nickname
				} else {
					list[i].OwnerName = u.Username
				}
			}
		}
	}
	utils.Success(c, gin.H{"list": list, "total": total, "page": page, "size": size})
}

// AdminBanGroup POST /api/admin/groups/:id/ban { banned, reason }
func AdminBanGroup(c *gin.Context) {
	if !requirePlatformAdmin(c) {
		return
	}
	gid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	g, err := groups.GetGroup(uint(gid))
	if err != nil {
		utils.NotFound(c, "群组不存在")
		return
	}
	var req struct {
		Banned bool   `json:"banned"`
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	status := groups.GroupStatusActive
	if req.Banned {
		status = groups.GroupStatusBanned
	}
	if err := models.DB.Model(&groups.Group{}).Where("id = ?", g.ID).
		Updates(map[string]interface{}{"status": status}).Error; err != nil {
		utils.InternalError(c, "操作失败")
		return
	}
	// 通知群主（封禁原因只给群主看）
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "违反平台规范"
	}
	if req.Banned {
		_ = models.PushNotification(middleware.UserID(c), g.OwnerID, "group",
			"你的群组已被封禁："+g.Name, reason, "/group/"+strconv.FormatUint(uint64(g.ID), 10))
	}
	msg := "已解封"
	if req.Banned {
		msg = "已封禁"
	}
	utils.SuccessMessage(c, msg, nil)
}

// AdminDeleteGroup DELETE /api/admin/groups/:id —— 强制解散（软删除）
func AdminDeleteGroup(c *gin.Context) {
	if !requirePlatformAdmin(c) {
		return
	}
	gid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if _, err := groups.GetGroup(uint(gid)); err != nil {
		utils.NotFound(c, "群组不存在")
		return
	}
	if err := models.DB.Delete(&groups.Group{}, uint(gid)).Error; err != nil {
		utils.InternalError(c, "删除失败")
		return
	}
	utils.SuccessMessage(c, "已解散群组", nil)
}

// AdminGetGroupQuota GET /api/admin/group-quota —— 全局额度 + 按人覆盖
func AdminGetGroupQuota(c *gin.Context) {
	if !requirePlatformAdmin(c) {
		return
	}
	cfg := groups.GetGroupQuotaConfig()
	overrides, err := groups.ListUserGroupQuotas()
	if err != nil {
		overrides = []groups.UserGroupQuota{}
	}
	utils.Success(c, gin.H{"config": cfg, "overrides": overrides})
}

// AdminSetGroupQuota PUT /api/admin/group-quota { enabled, max_per_user, roles, member_limit, daily_posts }
func AdminSetGroupQuota(c *gin.Context) {
	if !requirePlatformAdmin(c) {
		return
	}
	var req groups.GroupQuotaConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := groups.SetGroupQuotaConfig(req); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.SuccessMessage(c, "已保存（立即生效）", groups.GetGroupQuotaConfig())
}

// AdminSetUserGroupQuota PUT /api/admin/group-quota/users/:id { max_groups, note }
// max_groups：-1 跟随角色/全局；0 禁止建群；>0 专属上限。传 delete=true 删除覆盖。
func AdminSetUserGroupQuota(c *gin.Context) {
	if !requirePlatformAdmin(c) {
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
		MaxGroups *int   `json:"max_groups"`
		Note      string `json:"note"`
		Delete    bool   `json:"delete"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if req.Delete {
		if err := groups.DeleteUserGroupQuota(uint(uid)); err != nil {
			utils.InternalError(c, "删除失败")
			return
		}
		utils.SuccessMessage(c, "已恢复跟随默认额度", nil)
		return
	}
	if req.MaxGroups == nil {
		utils.BadRequest(c, "缺少 max_groups")
		return
	}
	if err := groups.SetUserGroupQuota(uint(uid), *req.MaxGroups, req.Note); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.SuccessMessage(c, "已保存", nil)
}
