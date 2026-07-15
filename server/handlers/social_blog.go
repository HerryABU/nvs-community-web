package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"nvs-server/config"
	"nvs-server/models"
	"nvs-server/security"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ============ 作者博客 ============

// 博客文件存储根目录：data/blogs
func blogDataDir() string {
	return filepath.Join(filepath.Dir(config.NovelDataDir), "blogs")
}

func computeBlogHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

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

	// 净化内容
	content := security.SanitizeUserContent(req.Content)
	title := security.SanitizePlainText(req.Title)
	summary := security.SanitizePlainText(req.Summary)

	// 先创建数据库记录获取 ID
	blog := &models.AuthorBlog{
		AuthorID: userID,
		Title:    title,
		Summary:  summary,
	}
	if err := models.CreateBlog(blog); err != nil {
		utils.InternalError(c, "创建失败")
		return
	}

	// 写入文件系统
	authorDir := filepath.Join(blogDataDir(), fmt.Sprintf("%d", userID))
	os.MkdirAll(authorDir, 0755)
	contentPath := filepath.Join(authorDir, fmt.Sprintf("%d.md", blog.ID))
	if err := os.WriteFile(contentPath, []byte(content), 0644); err != nil {
		utils.InternalError(c, "写入文件失败")
		return
	}

	// 更新路径和哈希到数据库
	contentHash := computeBlogHash(content)
	models.DB.Model(blog).Updates(map[string]interface{}{
		"content_path": contentPath,
		"content_hash": contentHash,
	})
	blog.ContentPath = contentPath
	blog.ContentHash = contentHash
	blog.Content = content

	utils.Success(c, blog)
}

// 辅助：从文件系统加载博客内容
func loadBlogContent(blog *models.AuthorBlog) {
	if blog.ContentPath == "" {
		return
	}
	data, err := os.ReadFile(blog.ContentPath)
	if err != nil {
		return
	}
	blog.Content = string(data)
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
	loadBlogContent(blog)
	models.IncrementBlogView(uint(id))
	if blog.Author != nil {
		blog.Author.Email = ""
	}
	utils.Success(c, blog)
}

// GET /api/author/:id/blogs/:blogId — 获取某作者的某篇博客详情（独立 URL）
func GetAuthorBlog(c *gin.Context) {
	authorID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || authorID == 0 {
		utils.BadRequest(c, "无效的作者ID")
		return
	}
	blogID, err := strconv.ParseUint(c.Param("blogId"), 10, 64)
	if err != nil || blogID == 0 {
		utils.BadRequest(c, "无效的博客ID")
		return
	}

	blog, err := models.GetBlogByID(uint(blogID))
	if err != nil || blog.AuthorID != uint(authorID) {
		utils.NotFound(c, "博客不存在")
		return
	}

	loadBlogContent(blog)
	models.IncrementBlogView(uint(blogID))
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

	updates := map[string]interface{}{}
	if req.Title != "" {
		blog.Title = security.SanitizePlainText(req.Title)
		updates["title"] = blog.Title
	}
	if req.Content != "" {
		content := security.SanitizeUserContent(req.Content)
		if blog.ContentPath != "" {
			os.WriteFile(blog.ContentPath, []byte(content), 0644)
		}
		contentHash := computeBlogHash(content)
		blog.ContentHash = contentHash
		updates["content_hash"] = contentHash
	}
	if req.Summary != "" {
		blog.Summary = security.SanitizePlainText(req.Summary)
		updates["summary"] = blog.Summary
	}
	if req.IsPinned != nil {
		blog.IsPinned = *req.IsPinned
		updates["is_pinned"] = *req.IsPinned
	}

	if len(updates) > 0 {
		if err := models.DB.Model(blog).Updates(updates).Error; err != nil {
			utils.InternalError(c, "更新失败")
			return
		}
	}

	loadBlogContent(blog)
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

	// 删除文件
	if blog.ContentPath != "" {
		os.Remove(blog.ContentPath)
	}

	if err := models.DeleteBlog(uint(id)); err != nil {
		utils.InternalError(c, "删除失败")
		return
	}
	utils.Success(c, gin.H{"message": "已删除"})
}
