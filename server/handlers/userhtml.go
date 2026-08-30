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

// ==================== 预检（解压前，纯内存） ====================

// zipPrecheck 结果
type zipPrecheckResult struct {
	Blocked   bool
	BlockInfo string // 阻断原因（文件名 + 命中规则）
	Threats   []security.Threat
	Warnings  []security.Threat
	FileCount int
	TotalSize int64
}

// precheckZip 在解压前对 ZIP 内每个条目做安全扫描（全程内存，不落盘）。
// 命中阻断级（critical/high）→ Blocked=true，调用方据此走「封存不解压」。
// 单个条目读取上限 8MB，避免预检阶段被超大文件拖死。
func precheckZip(zr *zip.Reader) zipPrecheckResult {
	var out zipPrecheckResult
	const maxScanBytes = 8 * 1024 * 1024

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		out.FileCount++
		out.TotalSize += int64(f.UncompressedSize64)

		rc, err := f.Open()
		if err != nil {
			continue
		}
		buf := make([]byte, maxScanBytes)
		n, _ := io.ReadFull(rc, buf)
		rc.Close()
		if n <= 0 {
			continue
		}
		blocked, threats, warnings := security.ClassifyScan(string(buf[:n]), f.Name)
		if blocked {
			out.Blocked = true
			out.Threats = append(out.Threats, threats...)
			if out.BlockInfo == "" && len(threats) > 0 {
				out.BlockInfo = fmt.Sprintf("%s 命中 %s（%s）", f.Name, threats[0].Pattern, threats[0].Severity)
			}
		}
		out.Warnings = append(out.Warnings, warnings...)
	}
	return out
}

// quarantineZip 把原始 ZIP 原样封存到隔离目录（不解压），返回封存路径。
// 目录独立于正常解压目录，避免与已发布扩展混放造成误读。
func quarantineZip(userID uint, safeName string, timestamp int64, zipData []byte) (string, error) {
	dir := filepath.Join(config.UserHTMLDir, "_quarantine", fmt.Sprintf("%d", userID))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("%s_%d.zip", safeName, timestamp))
	if err := os.WriteFile(path, zipData, 0644); err != nil {
		return "", err
	}
	return path, nil
}

// UploadZipHTML POST /api/userhtmls/upload
// 接收ZIP文件 → 解压前纯内存预检 → 预检未通过原样封存（不解压，等管理员处置）→ 通过才解压
func UploadZipHTML(c *gin.Context) {
	userID := currentUserID(c)

	// 接收多部分表单
	name := c.PostForm("name")
	description := c.PostForm("description")
	entryFile := c.PostForm("entry_file")
	allowWasm := c.PostForm("allow_wasm") == "true"
	isPublic := c.PostForm("is_public") == "true"
	isDownloadable := c.PostForm("is_downloadable") == "true"
	runMode := c.PostForm("run_mode")
	novelIDStr := c.PostForm("novel_id")

	if name == "" {
		utils.BadRequest(c, "扩展名称不能为空")
		return
	}
	// run_mode 校验：默认 sandbox
	if runMode != "inline" {
		runMode = "sandbox"
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

	userDir := filepath.Join(config.UserHTMLDir, fmt.Sprintf("%d", userID))
	timestamp := time.Now().UnixMilli()
	safeName := sanitizeFilename(name)
	extractDir := filepath.Join(userDir, fmt.Sprintf("%s_%d", safeName, timestamp))

	// ====== 第一步：ZIP 炸弹检测（纯内存，不落盘） ======
	if ok, msg := utils.ValidateZipBomb(zipReader, nil); !ok {
		utils.BadRequest(c, msg)
		return
	}

	// ====== 第二步：解压前安全预检（纯内存） ======
	// 预检未通过 → 原样封存 ZIP，不解压、不落盘任何扩展文件，等管理员处置。
	pre := precheckZip(zipReader)

	// 构建novelID指针
	var novelID *uint
	if novelIDStr != "" {
		if id, err := strconv.ParseUint(novelIDStr, 10, 64); err == nil {
			nid := uint(id)
			novelID = &nid
		}
	}

	if pre.Blocked {
		qPath, err := quarantineZip(userID, safeName, timestamp, zipData)
		if err != nil {
			utils.InternalError(c, "封存失败，上传已中止")
			return
		}
		blockReason := pre.BlockInfo
		if blockReason == "" {
			blockReason = "内容安全预检命中高危代码"
		}
		item := &models.UserHTML{
			UserID:          userID,
			NovelID:         novelID,
			Name:            name,
			Description:     description,
			ExtractDir:      "", // 未解压
			EntryFile:       "",
			FileCount:       pre.FileCount,
			TotalSize:       int64(len(zipData)),
			FilePath:        "",
			ZipPath:         qPath,
			AllowWasm:       allowWasm,
			RunMode:         runMode,
			ReviewStatus:    "quarantine",
			ScanWarnings:    security.ThreatsToJSON(append(pre.Warnings, pre.Threats...)),
			IsExtracted:     false,
			QuarantinePath:  qPath,
			ScanBlocked:     true,
			ScanBlockReason: blockReason,
		}
		if err := models.DB.Create(item).Error; err != nil {
			utils.InternalError(c, "创建封存记录失败")
			return
		}
		// ⚠️ 封存态的关键字段全是零值（is_extracted=false / is_active=false / is_public=false /
		// is_downloadable=false），而 GORM 对「带 default 标签的列」在 INSERT 时会直接跳过、
		// 由数据库填默认值——哪怕用 map 显式传 false 也一样。这会导致封存件被误标成
		// 「已解压 + 已启用」，放行时跳过补解压、详情页也走错分支。
		// 因此建记录后再用 Updates(map) 强制写回（Updates 不受默认值影响，按列精确写）。
		models.DB.Model(&models.UserHTML{}).Where("id = ?", item.ID).Updates(map[string]interface{}{
			"is_extracted":    false,
			"is_active":       false,
			"is_public":       false,
			"is_downloadable": false,
		})
		item.IsExtracted = false
		item.IsActive = false
		item.IsPublic = false
		item.IsDownloadable = false
		// 通知全部管理员：有待处置的封存件
		notifyAdminsHTMLQuarantine(item)
		utils.Success(c, gin.H{
			"html":        item,
			"quarantined": true,
			"message":     "安全预检未通过，ZIP 已原样封存、未解压，等待管理员审核",
		})
		return
	}

	// ====== 第三步：预检通过 → 解压 ======
	os.MkdirAll(extractDir, 0755)
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

	// 中/低危告警 → 已解压但转人工复核；无告警 → 直接通过
	reviewStatus := "approved"
	scanWarnings := ""
	if len(pre.Warnings) > 0 {
		reviewStatus = "pending"
		scanWarnings = security.ThreatsToJSON(pre.Warnings)
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

	// 🔒 安全审计 S2：entry_file 必须为解压目录内的相对路径（防路径穿越 LFI）
	entryFile = strings.TrimSpace(strings.ReplaceAll(entryFile, "\\", "/"))
	entryFile = strings.TrimPrefix(entryFile, "/")
	if entryFile == "" || strings.HasPrefix(entryFile, "..") || strings.Contains(entryFile, "../") {
		os.RemoveAll(extractDir)
		utils.BadRequest(c, "入口文件路径非法：仅允许解压目录内的相对路径")
		return
	}

	// 待人工复核期间一律不公开，管理员审核通过后才可上广场
	if reviewStatus == "pending" {
		isPublic = false
		isDownloadable = false
	}

	htmlItem := &models.UserHTML{
		UserID:         userID,
		NovelID:        novelID,
		Name:           name,
		Description:    description,
		ExtractDir:     extractDir,
		EntryFile:      entryFile,
		FileCount:      len(entries),
		TotalSize:      totalSize,
		FilePath:       extractDir, // 旧字段兼容
		ZipPath:        zipPath,
		AllowWasm:      allowWasm,
		IsPublic:       isPublic,
		IsDownloadable: isDownloadable,
		ThumbURL:       findThumbPath(entries, extractDir, userID, safeName, timestamp),
		RunMode:        runMode,
		ReviewStatus:   reviewStatus,
		ScanWarnings:   scanWarnings,
		IsExtracted:    true,
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
	userID := currentUserID(c)
	var htmls []models.UserHTML
	models.DB.Where("user_id = ?", userID).Order("updated_at DESC").Find(&htmls)
	utils.Success(c, htmls)
}

// GetHTML GET /api/userhtmls/:id
func GetHTML(c *gin.Context) {
	userID := currentUserID(c)
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
	userID := currentUserID(c)
	htmlID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	var htmlItem models.UserHTML
	if err := models.DB.Where("id = ? AND user_id = ?", htmlID, userID).First(&htmlItem).Error; err != nil {
		utils.NotFound(c, "扩展不存在")
		return
	}

	var req struct {
		Name           *string `json:"name"`
		Description    *string `json:"description"`
		NovelID        *uint   `json:"novel_id"`
		EntryFile      *string `json:"entry_file"`
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
	// 🔒 安全审计 S2：entry_file 必须为解压目录内的相对路径（防路径穿越 LFI——
	// 旧实现可直接把 entry_file 改为 ../../.env 读取任意文件）
	if req.EntryFile != nil {
		entry := strings.TrimSpace(*req.EntryFile)
		entry = strings.ReplaceAll(entry, "\\", "/")
		entry = strings.TrimPrefix(entry, "/")
		if entry == "" || strings.HasPrefix(entry, "..") || strings.Contains(entry, "../") {
			utils.BadRequest(c, "入口文件路径非法：仅允许解压目录内的相对路径")
			return
		}
		htmlItem.EntryFile = entry
	}
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
	userID := currentUserID(c)
	htmlID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	var htmlItem models.UserHTML
	if err := models.DB.Where("id = ? AND user_id = ?", htmlID, userID).First(&htmlItem).Error; err != nil {
		utils.NotFound(c, "扩展不存在")
		return
	}

	// 合规留存：软删除，保留 ZIP 与解压目录（不物理移除），可追溯原始内容
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
