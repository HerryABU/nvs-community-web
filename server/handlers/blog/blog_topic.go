package blog

import (
	"nvs-server/utils/infra"
	"strconv"
	"strings"

	"nvs-server/handlers/hctx"
	"nvs-server/models"

	"github.com/gin-gonic/gin"
	blogm "nvs-server/models/blog"
)

// ─────────────────────────────────────────────────────────────
// 博客话题（#话题）
//   GET  /api/topics          全站话题列表（按文章数降序）
//   GET  /api/topics/:id      话题详情 + 文章列表
//   POST /api/topics          创建话题（登录；同名去重）
//   GET  /api/topics/:id/blogs 话题下博客列表（分页）
// ─────────────────────────────────────────────────────────────

// GET /api/topics — 全站话题列表
func ListTopics(c *gin.Context) {
	infra.Success(c, blogm.ListBlogTopics(60))
}

// POST /api/topics — 创建话题 { name, description? }（登录；同名返回已存在）
func CreateTopic(c *gin.Context) {
	if hctx.CurrentUserID(c) == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	var req struct {
		Name        string `json:"name" binding:"required,max=48"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "请填写话题名称")
		return
	}
	name := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(req.Name), "#"))
	if name == "" {
		infra.BadRequest(c, "请填写话题名称")
		return
	}
	if existing, err := blogm.GetBlogTopicByName(name); err == nil && existing != nil {
		infra.Success(c, existing)
		return
	}
	tp := &blogm.BlogTopic{
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		CreatedBy:   hctx.CurrentUserID(c),
	}
	if err := models.DB.Create(tp).Error; err != nil {
		infra.InternalError(c, "创建话题失败")
		return
	}
	infra.Success(c, tp)
}

// GET /api/topics/:id — 话题详情
func GetTopic(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	tp, err := blogm.GetBlogTopicByID(uint(id))
	if err != nil {
		infra.NotFound(c, "话题不存在")
		return
	}
	// 文章数
	var cnt int64
	models.DB.Model(&blogm.AuthorBlog{}).
		Joins("JOIN blog_topic_links ON blog_topic_links.blog_id = author_blogs.id").
		Where("blog_topic_links.topic_id = ? AND author_blogs.deleted_at IS NULL", tp.ID).
		Count(&cnt)
	infra.Success(c, gin.H{
		"topic":      tp,
		"blog_count": cnt,
	})
}

// GET /api/topics/:id/blogs — 话题下博客列表（分页）
func ListTopicBlogs(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "12"))
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 12
	}
	var blogs []blogm.AuthorBlog
	var total int64
	query := models.DB.Model(&blogm.AuthorBlog{}).
		Joins("JOIN blog_topic_links ON blog_topic_links.blog_id = author_blogs.id").
		Where("blog_topic_links.topic_id = ?", id)
	query.Count(&total)
	err := query.Preload("Author").
		Order("author_blogs.is_pinned DESC, author_blogs.created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&blogs).Error
	if err != nil {
		infra.InternalError(c, "查询失败")
		return
	}
	FillBlogCategories(blogs)
	blogm.FillBlogTags(blogs)
	blogm.FillBlogTopics(blogs)
	for i := range blogs {
		if blogs[i].Author != nil {
			blogs[i].Author.Email = ""
		}
	}
	infra.Success(c, gin.H{"list": blogs, "total": total})
}
