package handlers

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"nvs-server/config"
	"nvs-server/models"
	"nvs-server/security"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== UserHTML 扩展HTML（ZIP上传） ====================

// UploadZipHTML POST /api/userhtmls/upload
// 接收ZIP文件，安全检测，解压到文件系统
func UploadZipHTML(c *gin.Context) {
	userID := c.GetUint("user_id")

	// 接收多部分表单
	name := c.PostForm("name")
	description := c.PostForm("description")
	entryFile := c.PostForm("entry_file")
	allowWasm := c.PostForm("allow_wasm") == "true"
	isPublic := c.PostForm("is_public") == "true"
	isDownloadable := c.PostForm("is_downloadable") == "true"
	novelIDStr := c.PostForm("novel_id")

	if name == "" {
		utils.BadRequest(c, "扩展名称不能为空")
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		utils.BadRequest(c, "请上传ZIP文件")
		return
	}
	defer file.Close()

	// 检查是否为ZIP文件
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
		utils.BadRequest(c, "仅支持 .zip 文件")
		return
	}

	// 限制上传大小（最大20MB）
	if header.Size > 20*1024*1024 {
		utils.BadRequest(c, "ZIP文件不能超过 20MB")
		return
	}

	// 读取ZIP到内存
	zipData, err := io.ReadAll(file)
	if err != nil {
		utils.InternalError(c, "读取ZIP文件失败")
		return
	}

	// 解析ZIP
	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		utils.BadRequest(c, "无效的ZIP文件: "+err.Error())
		return
	}

	// 安全检测 + 解压
	userDir := filepath.Join(config.UserHTMLDir, fmt.Sprintf("%d", userID))
	timestamp := time.Now().UnixMilli()
	safeName := sanitizeFilename(name)
	extractDir := filepath.Join(userDir, fmt.Sprintf("%s_%d", safeName, timestamp))
	os.MkdirAll(extractDir, 0755)

	// 安全解压（含ZIP炸弹检测）
	entries, err := utils.ExtractZipSafe(zipReader, extractDir, nil)
	if err != nil {
		os.RemoveAll(extractDir) // 清理
		utils.BadRequest(c, err.Error())
		return
	}

	if len(entries) == 0 {
		os.RemoveAll(extractDir)
		utils.BadRequest(c, "ZIP文件中没有有效文件")
		return
	}

	// 入口HTML：优先用户指定，否则自动查找
	if entryFile == "" {
		entryFile = findEntryHTML(entries)
	} else {
		// 验证指定的入口文件存在
		if !hasEntry(entries, entryFile) {
			os.RemoveAll(extractDir)
			utils.BadRequest(c, "指定入口不存在: "+entryFile)
			return
		}
	}

	// 🔍 内容安全扫描：逐文件检测恶意代码（提权/木马/僵尸/窃取）
	for _, entry := range entries {
		content, err := os.ReadFile(entry.Path)
		if err != nil {
			continue
		}
		passed, reason := security.ScanContentStrict(string(content), entry.Name)
		if !passed {
			os.RemoveAll(extractDir) // 清理恶意内容
			utils.BadRequest(c, reason)
			return
		}
	}

	// 保存原始ZIP
	zipPath := filepath.Join(extractDir, "_original.zip")
	os.WriteFile(zipPath, zipData, 0644)

	// 🔒 安全加固：锁定解压目录，禁止创建新脚本文件
	// 锁定文件为只读（Windows: ReadOnly, Unix: 0444/0555）
	if err := security.LockDirectory(extractDir); err != nil {
		// 锁定失败记录日志但不中断（尽力而为）
		c.Writer.Header().Set("X-NVS-Sandbox-Lock", "partial")
	} else {
		security.MarkSandboxLocked(extractDir)
		c.Writer.Header().Set("X-NVS-Sandbox-Lock", "locked")
	}

	// 计算总大小
	var totalSize int64
	for _, e := range entries {
		totalSize += e.Size
	}

	// 构建novelID指针
	var novelID *uint
	if novelIDStr != "" {
		if id, err := strconv.ParseUint(novelIDStr, 10, 64); err == nil {
			nid := uint(id)
			novelID = &nid
		}
	}

	htmlItem := &models.UserHTML{
		UserID:      userID,
		NovelID:     novelID,
		Name:        name,
		Description: description,
		ExtractDir:  extractDir,
		EntryFile:   entryFile,
		FileCount:   len(entries),
		TotalSize:   totalSize,
		FilePath:    extractDir, // 旧字段兼容
		ZipPath:        zipPath,
		AllowWasm:      allowWasm,
		IsPublic:       isPublic,
		IsDownloadable: isDownloadable,
		ThumbURL:       findThumbPath(entries, extractDir, userID, safeName, timestamp),
	}
	if err := models.DB.Create(htmlItem).Error; err != nil {
		os.RemoveAll(extractDir)
		utils.InternalError(c, "创建记录失败")
		return
	}

	utils.Success(c, gin.H{
		"html":       htmlItem,
		"files":      entries,
		"entry_file": entryFile,
	})
}

// ListHTMLs GET /api/userhtmls
func ListHTMLs(c *gin.Context) {
	userID := c.GetUint("user_id")
	var htmls []models.UserHTML
	models.DB.Where("user_id = ?", userID).Order("updated_at DESC").Find(&htmls)
	utils.Success(c, htmls)
}

// GetHTML GET /api/userhtmls/:id
func GetHTML(c *gin.Context) {
	userID := c.GetUint("user_id")
	htmlID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	var htmlItem models.UserHTML
	if err := models.DB.Where("id = ? AND user_id = ?", htmlID, userID).First(&htmlItem).Error; err != nil {
		utils.NotFound(c, "扩展不存在")
		return
	}

	// 列出解压目录中的文件
	files := listExtractedFiles(htmlItem.ExtractDir)

	utils.Success(c, gin.H{
		"html":  htmlItem,
		"files": files,
	})
}

// UpdateHTML PUT /api/userhtmls/:id
func UpdateHTML(c *gin.Context) {
	userID := c.GetUint("user_id")
	htmlID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	var htmlItem models.UserHTML
	if err := models.DB.Where("id = ? AND user_id = ?", htmlID, userID).First(&htmlItem).Error; err != nil {
		utils.NotFound(c, "扩展不存在")
		return
	}

	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		NovelID     *uint   `json:"novel_id"`
		EntryFile   *string `json:"entry_file"`
		IsActive       *bool   `json:"is_active"`
		AllowWasm      *bool   `json:"allow_wasm"`
		IsPublic       *bool   `json:"is_public"`
		IsDownloadable *bool   `json:"is_downloadable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	applyIfNotNil(req.Name, &htmlItem.Name)
	applyIfNotNil(req.Description, &htmlItem.Description)
	if req.NovelID != nil {
		htmlItem.NovelID = req.NovelID
	}
	applyIfNotNil(req.EntryFile, &htmlItem.EntryFile)
	applyIfNotNil(req.IsActive, &htmlItem.IsActive)
	applyIfNotNil(req.AllowWasm, &htmlItem.AllowWasm)
	applyIfNotNil(req.IsPublic, &htmlItem.IsPublic)
	applyIfNotNil(req.IsDownloadable, &htmlItem.IsDownloadable)

	if err := models.DB.Save(&htmlItem).Error; err != nil {
		utils.InternalError(c, "更新失败")
		return
	}
	utils.Success(c, htmlItem)
}

// DeleteHTML DELETE /api/userhtmls/:id
func DeleteHTML(c *gin.Context) {
	userID := c.GetUint("user_id")
	htmlID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	var htmlItem models.UserHTML
	if err := models.DB.Where("id = ? AND user_id = ?", htmlID, userID).First(&htmlItem).Error; err != nil {
		utils.NotFound(c, "扩展不存在")
		return
	}

	// 删除整个解压目录
	os.RemoveAll(htmlItem.ExtractDir)
	models.DB.Delete(&htmlItem)
	utils.Success(c, gin.H{"message": "扩展已删除"})
}

// GetNovelHTMLs GET /api/novels/:id/htmls
func GetNovelHTMLs(c *gin.Context) {
	novelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var htmls []models.UserHTML
	models.DB.Where("novel_id = ? AND is_active = ?", uint(novelID), true).
		Order("updated_at DESC").Find(&htmls)
	utils.Success(c, htmls)
}

// ==================== 辅助 ====================

func findEntryHTML(entries []utils.ZipEntry) string {
	// 优先 index.html，其次 *.html，否则第一个.html
	if len(entries) == 0 {
		return ""
	}
	for _, e := range entries {
		base := strings.ToLower(filepath.Base(e.Name))
		if base == "index.html" || base == "index.htm" {
			return e.Name
		}
	}
	for _, e := range entries {
		if strings.HasSuffix(strings.ToLower(e.Name), ".html") ||
			strings.HasSuffix(strings.ToLower(e.Name), ".htm") {
			return e.Name
		}
	}
	return entries[0].Name
}

func listExtractedFiles(dir string) []utils.ZipEntry {
	var result []utils.ZipEntry
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		rel = filepath.ToSlash(rel)
		// 跳过原始ZIP
		if rel == "_original.zip" {
			return nil
		}
		result = append(result, utils.ZipEntry{
			Name: rel,
			Size: info.Size(),
			Path: path,
		})
		return nil
	})
	return result
}

func hasEntry(entries []utils.ZipEntry, name string) bool {
	for _, e := range entries {
		if e.Name == name {
			return true
		}
	}
	return false
}

func findThumbPath(entries []utils.ZipEntry, extractDir string, userID uint, safeName string, timestamp int64) string {
	name := findThumbImage(entries, extractDir)
	if name == "" {
		return ""
	}
	return fmt.Sprintf("/sandbox/userhtml/%d/%s_%d/%s", userID, safeName, timestamp, name)
}

var _ = bytes.NewReader