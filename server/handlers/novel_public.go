package handlers

import (
	"strconv"

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

	utils.Success(c, gin.H{
		"list":      novels,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GET /api/search/authors — 搜索作者（公开）
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
	utils.Success(c, gin.H{"list": users, "total": total})
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

	utils.Success(c, novel)
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