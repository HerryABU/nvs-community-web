package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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
	Title      string   `json:"title" binding:"required,max=256"`
	Content    string   `json:"content" binding:"required"`
	Summary    string   `json:"summary"`
	CategoryID uint     `json:"category_id"`
	Tags       []string `json:"tags"`   // 博客标签（#tag）
	Topics     []string `json:"topics"` // 博客话题（#话题）
	// AI 辅助生成标识（176 号令）：作者在编辑器用过 AI 且未全部改写时置 true
	AiAssisted bool   `json:"ai_assisted"`
	AiModel    string `json:"ai_model"`
}

type UpdateBlogInput struct {
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	Summary    string   `json:"summary"`
	IsPinned   *bool    `json:"is_pinned"`
	CategoryID *uint    `json:"category_id"`
	AiAssisted bool     `json:"ai_assisted"`
	AiModel    string   `json:"ai_model"`
	Tags       []string `json:"tags"`   // 博客标签（#tag），null=不修改
	Topics     []string `json:"topics"` // 博客话题（#话题），null=不修改
}

// POST /api/blogs/import/batch — 批量导入博客（multipart，每个文件一篇博客，标题取文件名去扩展名）
func ImportBlogs(c *gin.Context) {
	userID := currentUserID(c)

	form, err := c.MultipartForm()
	if err != nil {
		utils.BadRequest(c, "请上传文件")
		return
	}
	files := form.File["files"]
	if len(files) == 0 {
		utils.BadRequest(c, "请至少上传一个文件")
		return
	}
	// 目标分类（可选）：导入后自动归入该分类（CSDN 式：导入时可指定专栏）
	categoryID, _ := strconv.ParseUint(c.PostForm("category_id"), 10, 64)
	if categoryID > 0 {
		var cat models.BlogCategory
		if err := models.DB.First(&cat, categoryID).Error; err != nil {
			utils.BadRequest(c, "目标分类不存在")
			return
		}
	}

	authorDir := filepath.Join(blogDataDir(), fmt.Sprintf("%d", userID))
	os.MkdirAll(authorDir, 0755)

	created := make([]gin.H, 0, len(files))
	createdIDs := make([]uint, 0, len(files))
	skipped := 0
	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			skipped++
			continue
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			skipped++
			continue
		}
		content, err := readImportContent(data, fh.Filename)
		if err != nil {
			skipped++
			continue
		}
		content = strings.TrimSpace(content)
		if content == "" {
			skipped++
			continue
		}

		title := security.SanitizePlainText(stripExt(fh.Filename))
		if title == "" {
			title = "未命名博客"
		}
		content = security.SanitizeUserContent(content)
		raw := content
		content, badHits := processBadWords(raw, "blog")
		summary := ""
		if len(content) > 120 {
			runes := []rune(content)
			summary = string(runes[:120])
		}

		blog := &models.AuthorBlog{
			AuthorID: userID,
			Title:    title,
			Summary:  summary,
		}
		if err := models.CreateBlog(blog); err != nil {
			skipped++
			continue
		}
		// 违禁词 mark 动作命中 → 写入待审标记记录
		flagBadWords(badHits, "blog", blog.ID, userID, raw, title)
		contentPath := filepath.Join(authorDir, fmt.Sprintf("%d.md", blog.ID))
		if err := os.WriteFile(contentPath, []byte(content), 0644); err != nil {
			skipped++
			continue
		}
		contentHash := computeBlogHash(content)
		models.DB.Model(blog).Updates(map[string]interface{}{
			"content_path": filepath.ToSlash(contentPath),
			"content_hash": contentHash,
		})
		if categoryID > 0 {
			models.DB.Model(blog).Update("category_id", categoryID)
			blog.CategoryID = uint(categoryID)
		}
		created = append(created, gin.H{
			"id":      blog.ID,
			"title":   title,
			"summary": summary,
			"words":   len([]rune(content)),
		})
		createdIDs = append(createdIDs, blog.ID)
	}

	if len(created) == 0 {
		utils.BadRequest(c, "未能从上传文件中解析出任何博客内容")
		return
	}
	utils.Success(c, gin.H{
		"count":       len(created),
		"skipped":     skipped,
		"created":     created,
		"created_ids": createdIDs,
		"category_id": categoryID,
		"message":     fmt.Sprintf("成功导入 %d 篇博客", len(created)),
	})
}

// POST /api/blogs — 创建博客
func CreateBlog(c *gin.Context) {
	userID := currentUserID(c)
	// 未成年保护门禁（strict 模式下未成年人不可发布作品）
	if checkMinorPublishRestricted(c) {
		return
	}
	var req CreateBlogInput
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请填写博客标题和内容")
		return
	}

	// 净化内容（XSS + 违禁词库）
	raw := security.SanitizeUserContent(req.Content)
	content, badHits := processBadWords(raw, "blog")
	title := security.SanitizePlainText(req.Title)
	summary := security.SanitizePlainText(req.Summary)

	// 每日发文限额预检（§13.4；后台关闭时不生效）
	blogWords := len([]rune(content))
	if !enforceDailyPublish(c, models.PublishKindBlog, blogWords) {
		return
	}

	// 先创建数据库记录获取 ID
	blog := &models.AuthorBlog{
		AuthorID: userID,
		Title:    title,
		Summary:  summary,
	}
	if req.CategoryID > 0 {
		var cat models.BlogCategory
		if err := models.DB.First(&cat, req.CategoryID).Error; err == nil {
			blog.CategoryID = req.CategoryID
		}
	}
	if err := models.CreateBlog(blog); err != nil {
		utils.InternalError(c, "创建失败")
		return
	}
	// 发布流水记账（计入每日发文限额）
	models.RecordPublish(userID, models.PublishKindBlog, blog.ID, blogWords)

	// 违禁词 mark 动作命中 → 写入待审标记记录
	flagBadWords(badHits, "blog", blog.ID, userID, raw, blog.Title)

	// AI 生成内容标识（合规）
	if req.AiAssisted {
		_ = models.MarkAiContent(models.AiMarkBlog, blog.ID, req.AiModel)
	}

	// 写入文件系统
	authorDir := filepath.Join(blogDataDir(), fmt.Sprintf("%d", userID))
	os.MkdirAll(authorDir, 0755)
	contentPath := filepath.Join(authorDir, fmt.Sprintf("%d.md", blog.ID))
	if err := os.WriteFile(contentPath, []byte(content), 0644); err != nil {
		utils.InternalError(c, "写入文件失败")
		return
	}

	// 更新路径和哈希到数据库（存库统一正斜杠，跨平台可移植）
	contentHash := computeBlogHash(content)
	models.DB.Model(blog).Updates(map[string]interface{}{
		"content_path": filepath.ToSlash(contentPath),
		"content_hash": contentHash,
	})
	blog.ContentPath = filepath.ToSlash(contentPath)
	blog.ContentHash = contentHash
	blog.Content = content
	blog.Category = models.GetBlogCategoryName(blog.CategoryID)
	// 同步标签（去重/去#前缀）
	if len(req.Tags) > 0 {
		if err := models.SyncBlogTags(blog.ID, req.Tags); err == nil {
			blog.Tags = models.GetBlogTags(blog.ID)
		}
	}
	// 同步话题（#话题）
	if len(req.Topics) > 0 {
		if err := models.SyncBlogTopics(blog.ID, req.Topics); err == nil {
			blog.Topics = models.GetBlogTopics(blog.ID)
		}
	}

	utils.Success(c, blog)
}

// 辅助：从文件系统加载博客内容
func loadBlogContent(blog *models.AuthorBlog) {
	if blog.ContentPath == "" {
		return
	}
	data, err := os.ReadFile(utils.ResolveContentPath(blog.ContentPath))
	if err != nil {
		return
	}
	blog.Content = string(data)
}

// GET /api/blogs — 公开博客列表（支持分类过滤：category_id，传一级分类则包含其下二级）
func ListPublicBlogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "12"))
	categoryID, _ := strconv.ParseUint(c.DefaultQuery("category_id", "0"), 10, 64)
	tag := strings.TrimSpace(c.DefaultQuery("tag", ""))

	var blogs []models.AuthorBlog
	var total int64
	var err error
	if tag != "" {
		// 按标签筛选
		ids, e := models.GetBlogIDsByTag(tag)
		if e != nil {
			utils.InternalError(c, "查询失败")
			return
		}
		if len(ids) == 0 {
			utils.Success(c, gin.H{"list": []models.AuthorBlog{}, "total": 0})
			return
		}
		blogs, total, err = models.ListBlogsByIDs(ids, uint(categoryID), page, pageSize)
	} else {
		blogs, total, err = models.ListAllBlogs(uint(categoryID), page, pageSize)
	}
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	fillBlogCategories(blogs)
	models.FillBlogTags(blogs)
	models.FillBlogTopics(blogs)
	utils.Success(c, gin.H{"list": blogs, "total": total})
}

// GET /api/blog-tags — 全站标签云（按文章数降序）
func ListBlogTags(c *gin.Context) {
	cloud := models.GetBlogTagCloud(30)
	utils.Success(c, cloud)
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
	blog.Category = models.GetBlogCategoryName(blog.CategoryID)
	blog.Tags = models.GetBlogTags(blog.ID)
	blog.Topics = models.GetBlogTopics(blog.ID)
	if blog.Author != nil {
		blog.Author.Email = ""
	}
	utils.Success(c, blog)
}

// GET /api/author/:id/blogs/:blogId — 获取某作者的某篇博客详情（独立 URL）
// :id 兼容数字主键 id 与用户名（username）
func GetAuthorBlog(c *gin.Context) {
	author, err := ResolveUserKey(c.Param("id"))
	if err != nil || author == nil {
		utils.NotFound(c, "作者不存在")
		return
	}
	blogID, err := strconv.ParseUint(c.Param("blogId"), 10, 64)
	if err != nil || blogID == 0 {
		utils.BadRequest(c, "无效的博客ID")
		return
	}

	blog, err := models.GetBlogByID(uint(blogID))
	if err != nil || blog.AuthorID != author.ID {
		utils.NotFound(c, "博客不存在")
		return
	}

	loadBlogContent(blog)
	models.IncrementBlogView(uint(blogID))
	blog.Category = models.GetBlogCategoryName(blog.CategoryID)
	blog.Tags = models.GetBlogTags(blog.ID)
	blog.Topics = models.GetBlogTopics(blog.ID)
	if blog.Author != nil {
		blog.Author.Email = ""
	}
	utils.Success(c, blog)
}

// GET /api/author/:id/blogs — 获取某作者的博客列表（支持 category_id 筛选）
func ListAuthorBlogs(c *gin.Context) {
	// :id 兼容数字主键 id 与用户名（username）
	author, err := ResolveUserKey(c.Param("id"))
	if err != nil || author == nil {
		utils.NotFound(c, "作者不存在")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	// category_id：缺省=全部；"0"=未分类；>0=该分类
	// exclude_category_id：>0 时排除该分类（"新增到分类"视图）
	var cid *uint
	if catStr := c.Query("category_id"); catStr != "" {
		if v, err := strconv.ParseUint(catStr, 10, 64); err == nil {
			u := uint(v)
			cid = &u
		}
	}
	excludeID, _ := strconv.ParseUint(c.DefaultQuery("exclude_category_id", "0"), 10, 64)

	blogs, total, err := models.GetBlogsByAuthorFilter(author.ID, cid, uint(excludeID), page, 10)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	fillBlogCategories(blogs)
	for i := range blogs {
		if blogs[i].Author != nil {
			blogs[i].Author.Email = ""
		}
	}
	models.FillBlogTags(blogs)
	models.FillBlogTopics(blogs)
	utils.Success(c, gin.H{"list": blogs, "total": total})
}

// PUT /api/blogs/:id — 更新博客
func UpdateBlog(c *gin.Context) {
	userID := currentUserID(c)
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
		raw := content
		var badHits []security.BadWordHit
		content, badHits = processBadWords(raw, "blog")
		if blog.ContentPath != "" {
			os.WriteFile(utils.ResolveContentPath(blog.ContentPath), []byte(content), 0644)
		}
		contentHash := computeBlogHash(content)
		blog.ContentHash = contentHash
		updates["content_hash"] = contentHash
		// 违禁词 mark 动作命中 → 写入待审标记记录
		flagBadWords(badHits, "blog", blog.ID, userID, raw, blog.Title)
	}
	if req.Summary != "" {
		blog.Summary = security.SanitizePlainText(req.Summary)
		updates["summary"] = blog.Summary
	}
	if req.IsPinned != nil {
		blog.IsPinned = *req.IsPinned
		updates["is_pinned"] = *req.IsPinned
	}
	if req.CategoryID != nil {
		cid := *req.CategoryID
		if cid > 0 {
			var cat models.BlogCategory
			if err := models.DB.First(&cat, cid).Error; err != nil {
				utils.BadRequest(c, "分类不存在")
				return
			}
		}
		blog.CategoryID = cid
		updates["category_id"] = cid
	}
	// 标签：仅当请求显式携带 tags 字段时同步（null 表示不修改）
	if req.Tags != nil {
		if err := models.SyncBlogTags(blog.ID, req.Tags); err != nil {
			utils.InternalError(c, "标签保存失败")
			return
		}
	}
	// 话题：仅当请求显式携带 topics 字段时同步
	if req.Topics != nil {
		if err := models.SyncBlogTopics(blog.ID, req.Topics); err != nil {
			utils.InternalError(c, "话题保存失败")
			return
		}
	}

	if len(updates) > 0 {
		if err := models.DB.Model(blog).Updates(updates).Error; err != nil {
			utils.InternalError(c, "更新失败")
			return
		}
	}

	loadBlogContent(blog)
	blog.Category = models.GetBlogCategoryName(blog.CategoryID)
	blog.Tags = models.GetBlogTags(blog.ID)
	blog.Topics = models.GetBlogTopics(blog.ID)
	utils.Success(c, blog)
}

// DELETE /api/blogs/:id — 删除博客
func DeleteBlog(c *gin.Context) {
	userID := currentUserID(c)
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	blog, err := models.GetBlogByID(uint(id))
	if err != nil || blog.AuthorID != userID {
		utils.Forbidden(c, "无权删除此博客")
		return
	}

	// 合规留存：软删除，保留正文文件（不物理移除），审计可追溯原始内容
	if err := models.DeleteBlog(uint(id)); err != nil {
		utils.InternalError(c, "删除失败")
		return
	}

	// 标记为「用户自删」（区别于管理员封禁：deleted_by='admin'），回收站据此区分
	_ = models.DB.Unscoped().Model(&models.AuthorBlog{}).Where("id = ?", id).Update("deleted_by", "user")
	utils.Success(c, gin.H{"message": "已删除"})
}

// BatchSetBlogCategory POST /api/blogs/batch-category — 批量设置博客分类（CSDN 式批量操作）
// body: { ids: [1,2,3], category_id: 5 }（category_id=0 表示移出到未分类）
func BatchSetBlogCategory(c *gin.Context) {
	userID := currentUserID(c)
	var req struct {
		IDs        []uint `json:"ids" binding:"required"`
		CategoryID uint   `json:"category_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		utils.BadRequest(c, "请选择要操作的博客")
		return
	}
	if len(req.IDs) > 200 {
		utils.BadRequest(c, "单次最多操作 200 篇")
		return
	}
	// 校验归属：只能操作自己的博客
	var cnt int64
	models.DB.Model(&models.AuthorBlog{}).
		Where("id IN ? AND author_id = ?", req.IDs, userID).Count(&cnt)
	if cnt != int64(len(req.IDs)) {
		utils.Forbidden(c, "存在无权操作的博客")
		return
	}
	if req.CategoryID > 0 {
		var cat models.BlogCategory
		if err := models.DB.First(&cat, req.CategoryID).Error; err != nil {
			utils.BadRequest(c, "分类不存在")
			return
		}
	}
	if err := models.DB.Model(&models.AuthorBlog{}).
		Where("id IN ?", req.IDs).Update("category_id", req.CategoryID).Error; err != nil {
		utils.InternalError(c, "操作失败")
		return
	}
	utils.Success(c, gin.H{"updated": len(req.IDs), "category_id": req.CategoryID})
}
