// home_latest.go — 首页「最新帖」聚合（F15 论坛首页风格区块用）。
// 只返回公开可访问论坛的帖子：排除加密论坛（password_hash 非空）、
// 待审/驳回的自建论坛（review_status != approved）；
// 系统论坛（非 custom 类型）默认在大厅按类型公开列出，故不受 is_public 约束。
package handlers

import (
	"strconv"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ListLatestThreads GET /api/threads/latest?limit=8 — 全站最新帖（公开论坛）
func ListLatestThreads(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "8"))
	if limit < 1 || limit > 20 {
		limit = 8
	}

	var threads []models.Thread
	err := models.DB.
		Joins("JOIN forums ON forums.id = threads.forum_id").
		Where("forums.password_hash = ?", "").
		Where("forums.review_status = ?", "approved").
		Where("forums.type != ? OR forums.is_public = ?", "custom", true).
		Where("threads.deleted_at IS NULL").
		Where("forums.deleted_at IS NULL").
		Preload("User").
		Preload("Forum").
		Order("threads.created_at DESC").
		Limit(limit).
		Find(&threads).Error
	if err != nil {
		utils.InternalError(c, "获取最新帖失败")
		return
	}

	// 精简 User 字段，避免把邮箱等敏感信息带回公开页
	type liteUser struct {
		ID        uint   `json:"id"`
		Username  string `json:"username"`
		Nickname  string `json:"nickname"`
		AvatarURL string `json:"avatar_url"`
	}
	items := make([]gin.H, 0, len(threads))
	for _, t := range threads {
		item := gin.H{
			"id":         t.ID,
			"forum_id":   t.ForumID,
			"title":      t.Title,
			"post_count": t.PostCount,
			"view_count": t.ViewCount,
			"is_pinned":  t.IsPinned,
			"created_at": t.CreatedAt,
		}
		if t.User != nil {
			item["user"] = liteUser{ID: t.User.ID, Username: t.User.Username, Nickname: t.User.Nickname, AvatarURL: t.User.AvatarURL}
		}
		if t.Forum != nil {
			item["forum"] = gin.H{"id": t.Forum.ID, "name": t.Forum.Name}
		}
		items = append(items, item)
	}
	utils.Success(c, gin.H{"items": items, "total": len(items)})
}
