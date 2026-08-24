package handlers

// 站点公告栏 API：
//   - 管理端：GET/POST /api/admin/announcements，PUT/DELETE /api/admin/announcements/:id
//   - 公开：GET /api/announcements（仅已发布，置顶优先）

import (
	"strconv"

	"nvs-server/middleware"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// AdminListAnnouncements 公告列表（管理端，含草稿）
// GET /api/admin/announcements
func AdminListAnnouncements(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var list []models.Announcement
	models.DB.Order("pinned DESC, created_at DESC").Find(&list)
	utils.Success(c, list)
}

// AdminCreateAnnouncement 发布公告
// POST /api/admin/announcements
func AdminCreateAnnouncement(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		Title     string `json:"title" binding:"required"`
		Content   string `json:"content"`
		Pinned    bool   `json:"pinned"`
		Published bool   `json:"published"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "公告标题不能为空")
		return
	}
	a := &models.Announcement{
		Title:     req.Title,
		Content:   req.Content,
		Pinned:    req.Pinned,
		Published: req.Published,
		CreatedBy: middleware.UserID(c),
	}
	if err := models.DB.Create(a).Error; err != nil {
		utils.InternalError(c, "发布失败")
		return
	}
	utils.Success(c, a)
}

// AdminUpdateAnnouncement 更新公告
// PUT /api/admin/announcements/:id
func AdminUpdateAnnouncement(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var a models.Announcement
	if err := models.DB.First(&a, id).Error; err != nil {
		utils.NotFound(c, "公告不存在")
		return
	}
	var req struct {
		Title     *string `json:"title"`
		Content   *string `json:"content"`
		Pinned    *bool   `json:"pinned"`
		Published *bool   `json:"published"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	updates := map[string]interface{}{}
	if req.Title != nil && *req.Title != "" {
		updates["title"] = *req.Title
	}
	if req.Content != nil {
		updates["content"] = *req.Content
	}
	if req.Pinned != nil {
		updates["pinned"] = *req.Pinned
	}
	if req.Published != nil {
		updates["published"] = *req.Published
	}
	if len(updates) > 0 {
		models.DB.Model(&a).Updates(updates)
	}
	models.DB.First(&a, id)
	utils.Success(c, a)
}

// AdminDeleteAnnouncement 删除公告（软删除）
// DELETE /api/admin/announcements/:id
func AdminDeleteAnnouncement(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	models.DB.Delete(&models.Announcement{}, id)
	utils.Success(c, gin.H{"message": "公告已删除"})
}

// ListAnnouncements 公开公告列表（已发布，置顶优先）
// GET /api/announcements
func ListAnnouncements(c *gin.Context) {
	utils.Success(c, models.ListPublishedAnnouncements(20))
}
