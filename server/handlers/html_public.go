package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ListPublicHTMLs GET /api/htmls/public
// 广场：所有公开的扩展HTML项目
func ListPublicHTMLs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 50 {
		size = 20
	}

	var total int64
	var htmls []models.UserHTML
	models.DB.Model(&models.UserHTML{}).Where("is_public = ? AND is_active = ?", true, true).Count(&total)
	models.DB.Where("is_public = ? AND is_active = ?", true, true).
		Preload("User").Order("updated_at DESC").
		Offset((page - 1) * size).Limit(size).Find(&htmls)

	utils.Success(c, gin.H{
		"items": htmls,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// GetAuthorPublicHTMLs GET /api/author/:id/htmls
// 作者专区：某作者的公开HTML项目
func GetAuthorPublicHTMLs(c *gin.Context) {
	authorID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var htmls []models.UserHTML
	models.DB.Where("user_id = ? AND is_public = ? AND is_active = ?", uint(authorID), true, true).
		Preload("User").Order("updated_at DESC").Find(&htmls)
	utils.Success(c, htmls)
}

// DownloadHTMLZip GET /api/userhtmls/:id/download
// 下载原始ZIP（需作者允许）
func DownloadHTMLZip(c *gin.Context) {
	htmlID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var html models.UserHTML
	if err := models.DB.First(&html, htmlID).Error; err != nil {
		utils.NotFound(c, "项目不存在")
		return
	}
	if !html.IsDownloadable {
		utils.Forbidden(c, "作者不允许下载此项目")
		return
	}

	zipPath := filepath.Join(html.ExtractDir, "_original.zip")
	if _, err := os.Stat(zipPath); os.IsNotExist(err) {
		utils.NotFound(c, "ZIP文件不存在")
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.zip\"", html.Name))
	c.File(zipPath)
}

// findThumbImage 从解压文件列表中查找首张图片路径
func findThumbImage(entries []utils.ZipEntry, baseDir string) string {
	imgExts := map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".svg": true}
	for _, e := range entries {
		ext := filepath.Ext(e.Name)
		if imgExts[ext] {
			return e.Name
		}
	}
	return ""
}
