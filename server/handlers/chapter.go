package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"nvs-server/config"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

func GetChapters(c *gin.Context) {
	novelID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "无效的作品ID")
		return
	}
	chapters, err := models.GetChaptersByNovel(uint(novelID))
	if err != nil {
		utils.InternalError(c, "获取章节列表失败")
		return
	}
	utils.Success(c, chapters)
}

func GetChapterContent(c *gin.Context) {
	novelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	chapterNum, _ := strconv.Atoi(c.Param("num"))

	chapter, err := models.GetChapterByNovelAndNumber(uint(novelID), chapterNum)
	if err != nil {
		utils.NotFound(c, "章节不存在")
		return
	}

	content, err := os.ReadFile(chapter.ContentPath)
	if err != nil {
		utils.InternalError(c, "读取章节内容失败")
		return
	}

	// 重新计算哈希并校验
	currentHash := computeSHA256(string(content))
	var fileSize int64
	var modTime int64
	if fi, err := os.Stat(chapter.ContentPath); err == nil {
		fileSize = fi.Size()
		modTime = fi.ModTime().Unix()
	}

	// 签名验证
	signatureVerified := false
	if novel, _ := models.GetNovelByID(chapter.NovelID); novel != nil {
		if author, _ := models.GetUserByID(novel.AuthorID); author != nil && author.SigningKey != "" {
			signatureVerified = utils.VerifySignature(string(content), chapter.ContentSignature, author.SigningKey)
		}
	}

	chapter.Content = string(content)
	// 后端渲染 Markdown → HTML
	htmlContent := utils.RenderMarkdown(string(content))

	utils.Success(c, gin.H{
		"chapter":            chapter,
		"html_content":       htmlContent,
		"hash":               currentHash,
		"hash_match":         currentHash == chapter.ContentHash,
		"signature_verified": signatureVerified,
		"file_size":          fileSize,
		"modified_at":        modTime,
	})
}

func CreateChapter(c *gin.Context) {
	userID := c.GetUint("userID")
	novelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	novel, err := models.GetNovelByID(uint(novelID))
	if err != nil || novel.AuthorID != userID {
		utils.Forbidden(c, "无权操作此作品")
		return
	}

	var req struct {
		Title   string `json:"title" binding:"required"`
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请填写章节标题和内容")
		return
	}

	nextNum, _ := models.GetMaxChapterNumber(uint(novelID))
	nextNum++

	authorDir := filepath.Join(config.NovelDataDir, "authors", fmt.Sprintf("%d", userID), fmt.Sprintf("%d", novelID))
	if err := os.MkdirAll(authorDir, 0755); err != nil {
		utils.InternalError(c, "创建目录失败")
		return
	}

	contentPath := filepath.Join(authorDir, fmt.Sprintf("%d.html", nextNum))
	// 直接存储原始 Markdown 内容，前端 v-md-preview 负责渲染
	if err := os.WriteFile(contentPath, []byte(req.Content), 0644); err != nil {
		utils.InternalError(c, "写入文件失败")
		return
	}

	// 计算 SHA256 哈希
	contentHash := computeSHA256(req.Content)

	// 用作者签名密钥生成 HMAC-SHA256 防篡改签名
	contentSignature := ""
	if author, err := models.GetUserByID(userID); err == nil && author.SigningKey != "" {
		contentSignature = utils.SignContent(req.Content, author.SigningKey)
	}

	wordCount := len([]rune(stripHTML(req.Content)))

	chapter := &models.Chapter{
		NovelID:          uint(novelID),
		ChapterNumber:    nextNum,
		Title:            req.Title,
		ContentPath:      contentPath,
		ContentHash:      contentHash,
		ContentSignature: contentSignature,
		WordCount:        wordCount,
		Status:           "published",
	}

	if err := models.CreateChapter(chapter); err != nil {
		utils.InternalError(c, "创建章节失败")
		return
	}

	models.UpdateNovelStats(uint(novelID))
	updateIndexJSON(uint(userID), uint(novelID))
	generateChecksums(uint(userID), uint(novelID))

	utils.Success(c, chapter)
}

func UpdateChapter(c *gin.Context) {
	userID := c.GetUint("userID")
	novelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	chapterNum, _ := strconv.Atoi(c.Param("num"))

	novel, err := models.GetNovelByID(uint(novelID))
	if err != nil || novel.AuthorID != userID {
		utils.Forbidden(c, "无权操作此作品")
		return
	}

	chapter, err := models.GetChapterByNovelAndNumber(uint(novelID), chapterNum)
	if err != nil {
		utils.NotFound(c, "章节不存在")
		return
	}

	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误")
		return
	}

	if req.Title != "" {
		chapter.Title = req.Title
	}
	if req.Content != "" {
		// 直接存储原始 Markdown 内容
		if err := os.WriteFile(chapter.ContentPath, []byte(req.Content), 0644); err != nil {
			utils.InternalError(c, "写入文件失败")
			return
		}
		chapter.ContentHash = computeSHA256(req.Content)
		// 用作者签名密钥生成 HMAC-SHA256 防篡改签名
		if author, err := models.GetUserByID(userID); err == nil && author.SigningKey != "" {
			chapter.ContentSignature = utils.SignContent(req.Content, author.SigningKey)
		}
		chapter.WordCount = len([]rune(stripHTML(req.Content)))
	}

	if err := models.UpdateChapter(chapter); err != nil {
		utils.InternalError(c, "更新章节失败")
		return
	}

	models.UpdateNovelStats(uint(novelID))
	updateIndexJSON(uint(userID), uint(novelID))
	generateChecksums(uint(userID), uint(novelID))

	utils.Success(c, chapter)
}

func DeleteChapter(c *gin.Context) {
	userID := c.GetUint("userID")
	novelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	chapterNum, _ := strconv.Atoi(c.Param("num"))

	novel, err := models.GetNovelByID(uint(novelID))
	if err != nil || novel.AuthorID != userID {
		utils.Forbidden(c, "无权操作此作品")
		return
	}

	chapter, err := models.GetChapterByNovelAndNumber(uint(novelID), chapterNum)
	if err != nil {
		utils.NotFound(c, "章节不存在")
		return
	}

	os.Remove(chapter.ContentPath)

	if err := models.DeleteChapter(chapter.ID); err != nil {
		utils.InternalError(c, "删除章节失败")
		return
	}

	// 自动重排：将后续章节编号前移
	authorDir := filepath.Join(config.NovelDataDir, "authors", fmt.Sprintf("%d", userID), fmt.Sprintf("%d", novelID))
	renumberChapters(uint(novelID), chapterNum, authorDir)

	models.UpdateNovelStats(uint(novelID))
	updateIndexJSON(uint(userID), uint(novelID))
	generateChecksums(uint(userID), uint(novelID))

	utils.Success(c, gin.H{"message": "章节已删除，编号已自动重排"})
}

// renumberChapters 将章节编号 >= deletedNum 的章节前移一位，并重命名文件
func renumberChapters(novelID uint, deletedNum int, dir string) {
	chapters, err := models.GetChaptersByNovel(novelID)
	if err != nil {
		return
	}

	for _, ch := range chapters {
		if ch.ChapterNumber > deletedNum {
			oldPath := ch.ContentPath
			newNum := ch.ChapterNumber - 1
			newPath := filepath.Join(dir, fmt.Sprintf("%d.html", newNum))

			// 重命名文件
			os.Rename(oldPath, newPath)

			// 更新数据库
			models.DB.Model(&models.Chapter{}).Where("id = ?", ch.ID).Updates(map[string]interface{}{
				"chapter_number": newNum,
				"content_path":   newPath,
			})
		}
	}
}

func updateIndexJSON(authorID, novelID uint) {
	chapters, err := models.GetChaptersByNovel(novelID)
	if err != nil {
		return
	}

	type IndexEntry struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Words  int    `json:"words"`
	}
	type IndexData struct {
		NovelID   uint         `json:"novel_id"`
		Chapters  []IndexEntry `json:"chapters"`
		UpdatedAt string       `json:"updated_at"`
	}

	entries := make([]IndexEntry, len(chapters))
	for i, ch := range chapters {
		entries[i] = IndexEntry{
			Number: ch.ChapterNumber,
			Title:  ch.Title,
			Words:  ch.WordCount,
		}
	}

	dir := filepath.Join(config.NovelDataDir, "authors", fmt.Sprintf("%d", authorID), fmt.Sprintf("%d", novelID))
	indexPath := filepath.Join(dir, "index.json")

	data := IndexData{
		NovelID:   novelID,
		Chapters:  entries,
		UpdatedAt: utils.NowString(),
	}

	jsonBytes, _ := json.MarshalIndent(data, "", "  ")
	os.WriteFile(indexPath, jsonBytes, 0644)
}

func stripHTML(html string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return regexp.MustCompile(`\s+`).ReplaceAllString(re.ReplaceAllString(html, ""), " ")
}

// computeSHA256 计算内容的 SHA256 哈希
func computeSHA256(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

// generateChecksums 为指定小说的所有章节生成 .checksums.json
func generateChecksums(authorID, novelID uint) {
	chapters, err := models.GetChaptersByNovel(novelID)
	if err != nil {
		return
	}

	type ChecksumEntry struct {
		ChapterNumber int    `json:"chapter_number"`
		Title         string `json:"title"`
		Hash          string `json:"hash"`
		FileSize      int64  `json:"file_size"`
		ModifiedAt    int64  `json:"modified_at"`
	}

	entries := make([]ChecksumEntry, 0, len(chapters))
	for _, ch := range chapters {
		fileInfo, err := os.Stat(ch.ContentPath)
		if err != nil {
			continue
		}
		content, err := os.ReadFile(ch.ContentPath)
		if err != nil {
			continue
		}
		hash := computeSHA256(string(content))
		entries = append(entries, ChecksumEntry{
			ChapterNumber: ch.ChapterNumber,
			Title:         ch.Title,
			Hash:          hash,
			FileSize:      fileInfo.Size(),
			ModifiedAt:    fileInfo.ModTime().Unix(),
		})
	}

	checksums := map[string]interface{}{
		"novel_id":    novelID,
		"updated_at":  utils.NowString(),
		"chapters":    entries,
	}

	dir := filepath.Join(config.NovelDataDir, "authors", fmt.Sprintf("%d", authorID), fmt.Sprintf("%d", novelID))
	os.MkdirAll(dir, 0755)
	jsonBytes, _ := json.MarshalIndent(checksums, "", "  ")
	os.WriteFile(filepath.Join(dir, ".checksums.json"), jsonBytes, 0644)
}

// VerifyChapter GET /api/novels/:id/chapters/:num/verify
// 返回章节的实时校验信息，供读者客户端验证内容完整性
func VerifyChapter(c *gin.Context) {
	novelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	chapterNum, _ := strconv.Atoi(c.Param("num"))

	chapter, err := models.GetChapterByNovelAndNumber(uint(novelID), chapterNum)
	if err != nil {
		utils.NotFound(c, "章节不存在")
		return
	}

	content, err := os.ReadFile(chapter.ContentPath)
	if err != nil {
		utils.InternalError(c, "读取文件失败")
		return
	}

	fileInfo, err := os.Stat(chapter.ContentPath)
	if err != nil {
		utils.InternalError(c, "获取文件信息失败")
		return
	}

	currentHash := computeSHA256(string(content))
	dbHash := chapter.ContentHash

	// 签名验证：用作者密钥验证 HMAC 签名
	signatureVerified := false
	if novel, _ := models.GetNovelByID(chapter.NovelID); novel != nil {
		if author, _ := models.GetUserByID(novel.AuthorID); author != nil && author.SigningKey != "" {
			signatureVerified = utils.VerifySignature(string(content), chapter.ContentSignature, author.SigningKey)
		}
	}

	hashOk := currentHash == dbHash
	msg := "内容完整，未被篡改"
	if !hashOk {
		msg = "警告：内容已被修改！"
	}
	if hashOk && chapter.ContentSignature != "" && !signatureVerified {
		msg = "警告：哈希匹配但作者签名无效，内容可能被站长替换！"
	}

	utils.Success(c, gin.H{
		"novel_id":           chapter.NovelID,
		"chapter_number":     chapter.ChapterNumber,
		"title":              chapter.Title,
		"hash_db":            dbHash,
		"hash_current":       currentHash,
		"hash_verified":      hashOk,
		"signature_verified": signatureVerified,
		"file_size":          fileInfo.Size(),
		"modified_at":        fileInfo.ModTime().Unix(),
		"message":            msg,
	})
}
