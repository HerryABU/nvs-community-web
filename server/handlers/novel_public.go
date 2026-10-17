package handlers

import (
	"fmt"
	"strconv"
	"strings"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	"github.com/microcosm-cc/bluemonday"
	"gorm.io/gorm"
)


// 分类列表（从数据库动态读取，不再硬编码）
// 使用 models.GetCategories() 替代

type CreateNovelInput struct {
	Title           string   `json:"title" binding:"required,max=256"`
	Category        string   `json:"category" binding:"max=64"`
	Categories      []string `json:"categories"`
	Tags            string   `json:"tags" binding:"max=4096"`
	Summary         string   `json:"summary" binding:"max=65536"`
	PricePerChapter float64  `json:"price_per_chapter"`
	Status          string   `json:"status"`
	SourceType      string   `json:"source_type"`     // original / reprint
	CreationMethod  string   `json:"creation_method"` // human / ai / human_ai_assisted
}

type UpdateNovelInput struct {
	Title           string   `json:"title" binding:"max=256"`
	Category        string   `json:"category" binding:"max=64"`
	Categories      []string `json:"categories"`
	Tags            string   `json:"tags" binding:"max=4096"`
	Summary         string   `json:"summary" binding:"max=65536"`
	CoverURL        string   `json:"cover_url" binding:"max=512"`
	PricePerChapter float64  `json:"price_per_chapter"`
	Status          string   `json:"status"`
	SourceType      string   `json:"source_type"`     // original / reprint
	CreationMethod  string   `json:"creation_method"` // human / ai / human_ai_assisted
	WallEnabled     *bool    `json:"wall_enabled"`
	WallWarning     *string  `json:"wall_warning"`
}

var htmlSanitizer = bluemonday.UGCPolicy()

// GET /api/novels
func ListNovels(c *gin.Context) {
	// 联邦同步会携带 source_name/source_url 自报身份，此时记录入站连接（谁连了我）
	if c.Query("source_name") != "" || c.Query("source_url") != "" {
		recordInbound(c, "novels")
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	category := c.Query("category")
	search := c.Query("search")
	sortBy := c.DefaultQuery("sort_by", "featured")
	// 白名单校验 sort_by 值，防止 SQL 注入
	if sortBy != "featured" && sortBy != "created_at" && sortBy != "updated_at" {
		sortBy = "featured"
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	novels, total, err := models.GetNovelsSorted(category, search, sortBy, page, pageSize)
	if err != nil {
		utils.InternalError(c, "获取作品列表失败")
		return
	}

	// 搜索埋点（F9：搜索热词来源之一）
	if search != "" {
		models.RecordSearchQuery(search, currentUserID(c), "novels", int(total))
	}

	utils.Success(c, gin.H{
		"list":      novels,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GET /api/search/authors — 搜索作者（公开）
func SearchAuthors(c *gin.Context) {
	search := c.Query("search")
	if search == "" {
		utils.Success(c, gin.H{"list": []models.User{}, "total": 0})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	like := "%" + search + "%"
	var total int64
	query := models.DB.Model(&models.User{}).
		Where("role IN ('author','vip_author')").
		Where("(username LIKE ? OR nickname LIKE ?)", like, like)
	query.Count(&total)

	var users []models.User
	offset := (page - 1) * pageSize
	query.Offset(offset).Limit(pageSize).Order("nickname ASC").Find(&users)
	// 脱敏
	models.SanitizeUserEmails(users)
	// 搜索埋点（F9：搜索热词来源之一）
	models.RecordSearchQuery(search, currentUserID(c), "authors", int(total))
	utils.Success(c, gin.H{"list": users, "total": total})
}

// GET /api/search — 统一搜索（公开）：作品 / 作者 / 博客 / HTML 扩展 / 专栏 / 话题 / 论坛
func UnifiedSearch(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		utils.Success(c, gin.H{"q": "", "novels": []models.Novel{}, "authors": []models.User{}, "blogs": []models.AuthorBlog{}, "htmls": []models.UserHTML{}})
		return
	}
	like := "%" + q + "%"
	const limit = 10

	// 1) 作品：书名 / 简介 / tag / 作者名
	var novels []models.Novel
	models.DB.Model(&models.Novel{}).Where("status = ?", "published").
		Joins("LEFT JOIN users ON users.id = novels.author_id").
		Where("(novels.title LIKE ? OR novels.summary LIKE ? OR novels.tags LIKE ? OR users.nickname LIKE ? OR users.username LIKE ?)",
			like, like, like, like, like).
		Preload("Author").Order("novels.updated_at DESC").Limit(limit).Find(&novels)

	// 2) 作者：用户名 / 昵称
	var authors []models.User
	models.DB.Model(&models.User{}).Where("role IN ('author','vip_author')").
		Where("(username LIKE ? OR nickname LIKE ?)", like, like).
		Order("nickname ASC").Limit(limit).Find(&authors)
	models.SanitizeUserEmails(authors)

	// 3) 博客：标题 / 摘要（含关键词）
	var blogs []models.AuthorBlog
	models.DB.Model(&models.AuthorBlog{}).
		Where("(title LIKE ? OR summary LIKE ?)", like, like).
		Preload("Author").Order("created_at DESC").Limit(limit).Find(&blogs)
	for i := range blogs {
		if blogs[i].Author != nil {
			blogs[i].Author.Email = ""
		}
	}

	// 4) HTML 扩展应用：名称 / 描述 / 项目名（仅公开且已审查）
	var htmls []models.UserHTML
	models.DB.Model(&models.UserHTML{}).
		Where("(name LIKE ? OR description LIKE ? OR project_name LIKE ?) AND is_public = ? AND review_status = ?",
			like, like, like, true, "approved").
		Preload("User").Order("created_at DESC").Limit(limit).Find(&htmls)

	// 5) 专栏（博客分类）：分类名（仅已公开 approved 分类）
	type categoryHit struct {
		ID          uint   `json:"id"`
		Name        string `json:"name"`
		FullName    string `json:"full_name"`
		CreatorID   uint   `json:"created_by"`
		CreatorName string `json:"creator_name"`
		BlogCount   int64  `json:"blog_count"`
	}
	var cats []models.BlogCategory
	models.DB.Model(&models.BlogCategory{}).Where("status = ? AND name LIKE ?", "approved", like).
		Order("id ASC").Limit(limit).Find(&cats)
	categories := make([]categoryHit, 0, len(cats))
	for _, c := range cats {
		hit := categoryHit{
			ID: c.ID, Name: c.Name,
			FullName:  models.GetBlogCategoryName(c.ID),
			CreatorID: c.CreatedBy,
		}
		models.DB.Model(&models.AuthorBlog{}).Where("category_id = ?", c.ID).Count(&hit.BlogCount)
		if c.CreatedBy > 0 {
			if u, err := models.GetUserByID(c.CreatedBy); err == nil && u != nil {
				hit.CreatorName = u.Nickname
				if hit.CreatorName == "" {
					hit.CreatorName = u.Username
				}
			}
		}
		categories = append(categories, hit)
	}

	// 6) 话题：话题名 / 描述
	var topics []models.BlogTopic
	models.DB.Model(&models.BlogTopic{}).Where("name LIKE ? OR description LIKE ?", like, like).
		Order("id ASC").Limit(limit).Find(&topics)

	// 7) 论坛：名称 / 描述（公开论坛）
	var forums []models.Forum
	models.DB.Model(&models.Forum{}).
		Where("(name LIKE ? OR description LIKE ?) AND is_public = ?", like, like, true).
		Order("thread_count DESC").Limit(limit).Find(&forums)

	// 搜索埋点（F9：搜索热词主入口；粗略命中数 = 各分区结果条数之和）
	hitCount := len(novels) + len(authors) + len(blogs) + len(htmls) +
		len(categories) + len(topics) + len(forums)
	models.RecordSearchQuery(q, currentUserID(c), "unified", hitCount)

	utils.Success(c, gin.H{
		"q": q, "novels": novels, "authors": authors, "blogs": blogs, "htmls": htmls,
		"categories": categories, "topics": topics, "forums": forums,
	})
}

// GET /api/novels/:id
func GetNovel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "无效的作品 ID")
		return
	}

	novel, err := models.GetNovelByID(uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.NotFound(c, "作品不存在")
			return
		}
		utils.InternalError(c, "获取作品失败")
		return
	}

	// 🔒 访客查看门禁（3.1/3.2/3.3）：匿名且需登录/作者上锁时拒绝，登录后可正常查看与爬取
	isLogin := currentUserID(c) != 0
	access := ResolveNovelAccess(novel, isLogin, currentUserRole(c))
	if access.RequiresLogin && !isLogin {
		utils.Unauthorized(c, "该作品需登录后查看")
		return
	}
	if access.IsTrial {
		access.TrialLimit = ComputeTrialLimit(novel.TotalChapters)
	}

	utils.Success(c, NovelView{
		Novel:          *novel,
		RequiresLogin:  access.RequiresLogin,
		IsTrial:        access.IsTrial,
		TrialLimit:     access.TrialLimit,
	})
}

// GET /api/categories
func ListCategories(c *gin.Context) {
	utils.Success(c, models.GetCategories())
}

// GET /api/categories/stats — 每个分类的作品数 + 前3部预览
func ListCategoryStats(c *gin.Context) {
	cats := models.GetCategories()
	result := make([]gin.H, 0, len(cats))
	for _, cat := range cats {
		novels, total, err := models.GetNovelsSorted(cat, "", "featured", 1, 3)
		if err != nil {
			continue
		}
		result = append(result, gin.H{
			"name":        cat,
			"novel_count": total,
			"novels":      novels,
		})
	}
	utils.Success(c, result)
}

// RedirectNovelReader 小说阅读路径301重定向
// /novel/:id/read/:chapter → /author/:authorId/novel/:id/read/:chapter
func RedirectNovelReader(c *gin.Context) {
	novelIDStr := c.Param("id")
	chapter := c.Param("chapter")

	// 如果参数为空，尝试从路径直接解析
	if novelIDStr == "" || chapter == "" {
		// Gin 可能没有正确解析参数，回退到手动解析
		path := c.Request.URL.Path
		// /novel/{id}/read/{chapter}
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) >= 4 && parts[0] == "novel" && parts[2] == "read" {
			novelIDStr = parts[1]
			chapter = parts[3]
		}
	}

	authorID := uint(0)
	if nid, err := strconv.ParseUint(novelIDStr, 10, 64); err == nil && nid > 0 {
		if novel, err := models.GetNovelByID(uint(nid)); err == nil {
			authorID = novel.AuthorID
		}
	}

	target := fmt.Sprintf("/author/%d/novel/%s/read/%s", authorID, novelIDStr, chapter)
	c.Header("Location", target)
	c.Status(301)
	c.Abort()
}

// RedirectNovelDetail 作品详情路径301重定向
// /novel/:id → /author/:authorId/novel/:id
func RedirectNovelDetail(c *gin.Context) {
	novelIDStr := c.Param("id")
	authorID := uint(0)
	if nid, err := strconv.ParseUint(novelIDStr, 10, 64); err == nil && nid > 0 {
		if novel, err := models.GetNovelByID(uint(nid)); err == nil {
			authorID = novel.AuthorID
		}
	}
	target := fmt.Sprintf("/author/%d/novel/%s", authorID, novelIDStr)
	c.Header("Location", target)
	c.Status(301)
	c.Abort()
}
