package handlers

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"nvs-server/config"
	"nvs-server/models"
	"nvs-server/security"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	"github.com/microcosm-cc/bluemonday"
	"gorm.io/gorm"
)

// 分类列表（从数据库动态读取，不再硬编码）
// 使用 models.GetCategories() 替代

type CreateNovelInput struct {
	Title           string   `json:"title" binding:"required,max=256"`
	Category        string   `json:"category"`
	Categories      []string `json:"categories"`
	Tags            string   `json:"tags"`
	Summary         string   `json:"summary"`
	PricePerChapter float64  `json:"price_per_chapter"`
	Status          string   `json:"status"`
	SourceType      string   `json:"source_type"`     // original / reprint
	CreationMethod  string   `json:"creation_method"` // human / ai / human_ai_assisted
}

type UpdateNovelInput struct {
	Title           string   `json:"title"`
	Category        string   `json:"category"`
	Categories      []string `json:"categories"`
	Tags            string   `json:"tags"`
	Summary         string   `json:"summary"`
	CoverURL        string   `json:"cover_url"`
	PricePerChapter float64  `json:"price_per_chapter"`
	Status          string   `json:"status"`
	SourceType      string   `json:"source_type"`     // original / reprint
	CreationMethod  string   `json:"creation_method"` // human / ai / human_ai_assisted
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

// POST /api/novels
func CreateNovel(c *gin.Context) {
	userID := c.GetUint("userID")
	userRole := c.GetString("userRole")

	// 如果用户是 reader，自动升级为 author（首次发布作品时）
	if userRole == "reader" {
		// 生成签名密钥
		signingKey, _ := security.GenerateSigningKey()
		updates := map[string]interface{}{"role": "author"}
		if signingKey != "" {
			updates["signing_key"] = signingKey
		}
		if err := models.DB.Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
			utils.InternalError(c, "升级作者角色失败")
			return
		}
		userRole = "author"
		c.Set("userRole", "author")
	}

	if userRole != "author" && userRole != "vip_author" && userRole != "admin" {
		utils.Forbidden(c, "只有作者才能发布作品")
		return
	}

	var input CreateNovelInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	// 净化 HTML
	input.Summary = htmlSanitizer.Sanitize(input.Summary)

	status := input.Status
	if status == "" {
		status = "draft"
	}

	tags := input.Tags
	if tags == "" {
		tags = "[]"
	}

	sourceType := input.SourceType
	if sourceType == "" {
		sourceType = "original"
	}
	creationMethod := input.CreationMethod
	if creationMethod == "" {
		creationMethod = "human"
	}

	novel := &models.Novel{
		AuthorID:        userID,
		Title:           input.Title,
		Category:        input.Category,
		Tags:            tags,
		Summary:         input.Summary,
		PricePerChapter: input.PricePerChapter,
		Status:          status,
		SourceType:      sourceType,
		CreationMethod:  creationMethod,
	}

	if err := models.CreateNovel(novel); err != nil {
		utils.InternalError(c, "创建作品失败")
		return
	}

	// 保存多分类
	saveNovelCategories(novel.ID, input.Categories, input.Category)
	// 如果只有单分类，确保向后兼容
	if input.Category == "" && len(input.Categories) > 0 {
		novel.Category = input.Categories[0]
		models.DB.Model(novel).Update("category", novel.Category)
	}
	// 填充返回用的分类名
	novel.CategoryNames = getCategoryNames(input.Categories, input.Category)

	// 创建作者目录
	authorDir := filepath.Join(config.NovelDataDir, "authors", fmt.Sprintf("%d", userID))
	novelDir := filepath.Join(authorDir, fmt.Sprintf("%d", novel.ID))
	os.MkdirAll(novelDir, 0755)

	// 创建 index.json
	indexPath := filepath.Join(novelDir, "index.json")
	indexData := map[string]interface{}{
		"novel_id":   novel.ID,
		"title":      novel.Title,
		"chapters":   []interface{}{},
	}
	indexBytes, _ := json.MarshalIndent(indexData, "", "  ")
	os.WriteFile(indexPath, indexBytes, 0644)

	utils.SuccessMessage(c, "创建成功", novel)
}

// PUT /api/novels/:id
func UpdateNovel(c *gin.Context) {
	userID := c.GetUint("userID")
	userRole := c.GetString("userRole")

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

	// 只有作者本人或管理员可以编辑
	if novel.AuthorID != userID && userRole != "admin" {
		utils.Forbidden(c, "无权编辑此作品")
		return
	}

	var input UpdateNovelInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	if input.Title != "" {
		novel.Title = input.Title
	}
	if input.Category != "" {
		novel.Category = input.Category
	}
	if input.Tags != "" {
		novel.Tags = input.Tags
	}
	if input.Summary != "" {
		novel.Summary = htmlSanitizer.Sanitize(input.Summary)
	}
	if input.CoverURL != "" {
		novel.CoverURL = input.CoverURL
	}
	if input.PricePerChapter >= 0 {
		novel.PricePerChapter = input.PricePerChapter
	}
	if input.Status != "" {
		novel.Status = input.Status
	}
	if input.SourceType != "" {
		novel.SourceType = input.SourceType
	}
	if input.CreationMethod != "" {
		novel.CreationMethod = input.CreationMethod
	}

	if err := models.UpdateNovel(novel); err != nil {
		utils.InternalError(c, "更新作品失败")
		return
	}

	// 更新多分类（只在显式传入时更新）
	if input.Categories != nil || input.Category != "" {
		saveNovelCategories(novel.ID, input.Categories, input.Category)
		if len(input.Categories) > 0 {
			models.DB.Model(novel).Update("category", input.Categories[0])
		} else if input.Category != "" {
			models.DB.Model(novel).Update("category", input.Category)
		}
		novel.CategoryNames = getCategoryNames(input.Categories, input.Category)
	} else {
		// 加载已有分类
		novel.CategoryNames = loadCategoryNames(novel.ID)
	}

	utils.SuccessMessage(c, "更新成功", novel)
}

// DELETE /api/novels/:id
func DeleteNovel(c *gin.Context) {
	userID := c.GetUint("userID")
	userRole := c.GetString("userRole")

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

	if novel.AuthorID != userID && userRole != "admin" {
		utils.Forbidden(c, "无权删除此作品")
		return
	}

	if err := models.DeleteNovel(uint(id)); err != nil {
		utils.InternalError(c, "删除失败")
		return
	}

	// 清理文件
	novelDir := filepath.Join(config.NovelDataDir, "authors", fmt.Sprintf("%d", novel.AuthorID), fmt.Sprintf("%d", id))
	os.RemoveAll(novelDir)

	utils.SuccessMessage(c, "删除成功", nil)
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

// POST /api/novels/:id/export
func ExportNovel(c *gin.Context) {
	userID := c.GetUint("userID")
	userRole := c.GetString("userRole")

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

	if novel.AuthorID != userID && userRole != "admin" {
		utils.Forbidden(c, "无权导出此作品")
		return
	}

	chapters, err := models.GetChaptersByNovel(uint(id))
	if err != nil {
		utils.InternalError(c, "获取章节列表失败")
		return
	}

	// 创建临时 ZIP 文件
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("novel_%d_*.zip", id))
	if err != nil {
		utils.InternalError(c, "创建临时文件失败")
		return
	}
	defer os.Remove(tmpFile.Name())

	zipWriter := zip.NewWriter(tmpFile)

	// 添加作品信息
	novelJSON, _ := json.MarshalIndent(novel, "", "  ")
	w, _ := zipWriter.Create("novel_info.json")
	w.Write(novelJSON)

	// 添加各章节
	for _, ch := range chapters {
		// 读取章节内容文件
		if ch.ContentPath != "" {
			content, err := os.ReadFile(ch.ContentPath)
			if err == nil {
				fileName := fmt.Sprintf("chapter_%04d.html", ch.ChapterNumber)
				w, _ := zipWriter.Create(fileName)
				w.Write(content)
			}
		}
	}

	zipWriter.Close()
	tmpFile.Close()

	// 设置下载响应头
	fileName := fmt.Sprintf("%s.zip", novel.Title)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))
	c.Header("Content-Type", "application/zip")
	c.File(tmpFile.Name())
}

// novelOwnershipCheck 检查当前用户是否是作品的作者
func novelOwnershipCheck(c *gin.Context, novelID uint) (*models.Novel, bool) {
	userID := c.GetUint("userID")
	userRole := c.GetString("userRole")

	novel, err := models.GetNovelByID(novelID)
	if err != nil {
		return nil, false
	}

	if novel.AuthorID != userID && userRole != "admin" {
		return novel, false
	}

	return novel, true
}

// saveNovelCategories 保存作品的多分类记录
func saveNovelCategories(novelID uint, categories []string, fallbackCategory string) {
	// 先删除旧分类
	models.DB.Where("novel_id = ?", novelID).Delete(&models.NovelCategory{})

	// 合并 categories 和 fallbackCategory
	categorySet := make(map[string]bool)
	for _, cat := range categories {
		cat = strings.TrimSpace(cat)
		if cat != "" {
			categorySet[cat] = true
		}
	}
	if fallbackCategory != "" {
		categorySet[strings.TrimSpace(fallbackCategory)] = true
	}

	// 插入新分类
	for cat := range categorySet {
		models.DB.Create(&models.NovelCategory{
			NovelID:  novelID,
			Category: cat,
		})
	}
}

// getCategoryNames 从前端输入获取分类名列表
func getCategoryNames(categories []string, fallbackCategory string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, cat := range categories {
		cat = strings.TrimSpace(cat)
		if cat != "" && !seen[cat] {
			seen[cat] = true
			result = append(result, cat)
		}
	}
	if fallbackCategory != "" {
		fallbackCategory = strings.TrimSpace(fallbackCategory)
		if !seen[fallbackCategory] {
			result = append(result, fallbackCategory)
		}
	}
	if len(result) == 0 {
		result = append(result, "其他")
	}
	return result
}

// loadCategoryNames 从数据库加载作品的分类名列表
func loadCategoryNames(novelID uint) []string {
	var cats []models.NovelCategory
	models.DB.Where("novel_id = ?", novelID).Find(&cats)
	var result []string
	for _, c := range cats {
		result = append(result, c.Category)
	}
	if len(result) == 0 {
		result = append(result, "其他")
	}
	return result
}

