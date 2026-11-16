package home

// ==================== 首页「精选高级博客」推流 ====================
// 只推作者主动选择 advanced 主题的博客，供首页卡片块使用；公开只读。

import (
	"nvs-server/models"
	blogm "nvs-server/models/blog"
	"nvs-server/models/theme"
	"nvs-server/utils/infra"

	"github.com/gin-gonic/gin"
)

// GET /api/feature-themes
func ListFeatureThemes(c *gin.Context) {
	ids := theme.AdvancedThemeAuthors()
	if len(ids) == 0 {
		infra.Success(c, gin.H{"list": []any{}})
		return
	}
	var users []models.User
	if err := models.DB.Select("id, username, nickname, avatar_url, signature").
		Where("id IN ?", ids).Find(&users).Error; err != nil {
		infra.InternalError(c, "查询失败")
		return
	}

	type recentBlog struct {
		ID    uint   `json:"id"`
		Title string `json:"title"`
		Cover string `json:"cover_url"`
	}
	list := make([]gin.H, 0, len(users))
	for _, u := range users {
		name := u.Nickname
		if name == "" {
			name = u.Username
		}
		var count int64
		models.DB.Model(&blogm.AuthorBlog{}).Where("author_id = ?", u.ID).Count(&count)
		var recent []blogm.AuthorBlog
		models.DB.Select("id, title, cover_url").Where("author_id = ?", u.ID).
			Order("is_pinned DESC, created_at DESC").Limit(3).Find(&recent)
		posts := make([]recentBlog, 0, len(recent))
		for _, b := range recent {
			posts = append(posts, recentBlog{ID: b.ID, Title: b.Title, Cover: b.CoverURL})
		}
		list = append(list, gin.H{
			"user_id": u.ID, "username": u.Username, "nickname": name,
			"avatar_url": u.AvatarURL, "signature": u.Signature,
			"post_count": count, "recent": posts,
		})
	}
	infra.Success(c, gin.H{"list": list})
}
