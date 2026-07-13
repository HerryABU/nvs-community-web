package handlers

import (
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// GET /api/admin/community — 社区动态仪表盘
func GetCommunityDashboard(c *gin.Context) {
	if !ensureAdmin(c) { return }

	// 最近注册用户
	var recentUsers []models.User
	models.DB.Order("created_at DESC").Limit(5).Find(&recentUsers)
	for i := range recentUsers {
		recentUsers[i].PasswordHash = ""
		recentUsers[i].Email = ""
	}

	// 最新作品
	var recentNovels []models.Novel
	models.DB.Preload("Author").Order("created_at DESC").Limit(5).Find(&recentNovels)

	// 最近评论
	var recentComments []struct {
		ID        uint   `json:"id"`
		Content   string `json:"content"`
		CreatedAt string `json:"created_at"`
		UserID    uint   `json:"user_id"`
		Username  string `json:"username"`
		NovelID   uint   `json:"novel_id"`
		NovelTitle string `json:"novel_title"`
	}
	models.DB.Raw(
		"SELECT c.id, c.content, c.created_at, c.user_id, u.nickname AS username, c.novel_id, n.title AS novel_title "+
			"FROM comments c JOIN users u ON u.id = c.user_id JOIN novels n ON n.id = c.novel_id "+
			"ORDER BY c.created_at DESC LIMIT 5",
	).Scan(&recentComments)

	// 最近论坛帖子
	var recentThreads []struct {
		ID        uint   `json:"id"`
		Title     string `json:"title"`
		CreatedAt string `json:"created_at"`
		UserID    uint   `json:"user_id"`
		Username  string `json:"username"`
		ForumID   uint   `json:"forum_id"`
		ForumName string `json:"forum_name"`
	}
	models.DB.Raw(
		"SELECT t.id, t.title, t.created_at, t.user_id, u.nickname AS username, t.forum_id, f.name AS forum_name "+
			"FROM threads t JOIN users u ON u.id = t.user_id JOIN forums f ON f.id = t.forum_id "+
			"ORDER BY t.created_at DESC LIMIT 5",
	).Scan(&recentThreads)

	utils.Success(c, gin.H{
		"recent_users":    recentUsers,
		"recent_novels":   recentNovels,
		"recent_comments": recentComments,
		"recent_threads":  recentThreads,
	})
}
