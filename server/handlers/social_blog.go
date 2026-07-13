package handlers

import (
	"strconv"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	"github.com/microcosm-cc/bluemonday"
)

// ============ 作者博客 ============

type CreateBlogInput struct {
	Title   string `json:"title" binding:"required,max=256"`
	Content string `json:"content" binding:"required"`
	Summary string `json:"summary"`
}

type UpdateBlogInput struct {
	Title    string `json:"title"`
	Content  string `json:"content"`
	Summary  string `json:"summary"`
	IsPinned *bool  `json:"is_pinned"`
}

// POST /api/blogs — 创建博客
func CreateBlog(c *gin.Context) {
	userID := c.GetUint("userID")
	var req CreateBlogInput
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}

	sanitizer := bluemonday.UGCPolicy()
	blog := &models.AuthorBlog{
		AuthorID: userID,
		Title:    sanitizer.Sanitize(req.Title),
		Content:  sanitizer.Sanitize(req.Content),
		Summary:  sanitizer.Sanitize(req.Summary),
	}
	if err := models.CreateBlog(blog); err != nil {
		utils.InternalError(c, "创建失败")
		return
	}
	utils.Success(c, blog)
}

// GET /api/blogs — 公开博客列表
func ListPublicBlogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "12"))

	blogs, total, err := models.ListAllBlogs(page, pageSize)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{"list": blogs, "total": total})
}

// GET /api/blogs/:id — 获取博客详情
func GetBlog(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	blog, err := models.GetBlogByID(uint(id))
	if err != nil {
		utils.NotFound(c, "博客不存在")
		return
	}
	models.IncrementBlogView(uint(id))
	if blog.Author != nil {
		blog.Author.Email = ""
	}
	utils.Success(c, blog)
}

// GET /api/author/:id/blogs — 获取某作者的博客列表
func ListAuthorBlogs(c *gin.Context) {
	authorID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))

	blogs, total, err := models.GetBlogsByAuthor(uint(authorID), page, 10)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	for i := range blogs {
		if blogs[i].Author != nil {
			blogs[i].Author.Email = ""
		}
	}
	utils.Success(c, gin.H{"list": blogs, "total": total})
}

// PUT /api/blogs/:id — 更新博客
func UpdateBlog(c *gin.Context) {
	userID := c.GetUint("userID")
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	blog, err := models.GetBlogByID(uint(id))
	if err != nil || blog.AuthorID != userID {
		utils.Forbidden(c, "无权编辑此博客")
		return
	}

	var req UpdateBlogInput
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}

	sanitizer := bluemonday.UGCPolicy()
	if req.Title != "" {
		blog.Title = sanitizer.Sanitize(req.Title)
	}
	if req.Content != "" {
		blog.Content = sanitizer.Sanitize(req.Content)
	}
	if req.Summary != "" {
		blog.Summary = sanitizer.Sanitize(req.Summary)
	}
	if req.IsPinned != nil {
		blog.IsPinned = *req.IsPinned
	}

	if err := models.UpdateBlog(blog); err != nil {
		utils.InternalError(c, "更新失败")
		return
	}
	utils.Success(c, blog)
}

// DELETE /api/blogs/:id — 删除博客
func DeleteBlog(c *gin.Context) {
	userID := c.GetUint("userID")
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	blog, err := models.GetBlogByID(uint(id))
	if err != nil || blog.AuthorID != userID {
		utils.Forbidden(c, "无权删除此博客")
		return
	}

	if err := models.DeleteBlog(uint(id)); err != nil {
		utils.InternalError(c, "删除失败")
		return
	}
	utils.Success(c, gin.H{"message": "已删除"})
}
