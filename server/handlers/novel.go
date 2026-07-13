package handlers

import (
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
	"gorm.io/gorm"
)

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