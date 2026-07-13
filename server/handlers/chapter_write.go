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
	"nvs-server/security"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	"github.com/microcosm-cc/bluemonday"
)

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
	// 直接存储原始 Markdown 内容，前端 Cherry Markdown 负责渲染
	if err := os.WriteFile(contentPath, []byte(req.Content), 0644); err != nil {
		utils.InternalError(c, "写入文件失败")
		return
	}

	// 计算 SHA256 哈希
	contentHash := computeSHA256(req.Content)

	// 用作者签名密钥生成 HMAC-SHA256 防篡改签名
	contentSignature := ""
	if author, err := models.GetUserByID(userID); err == nil && author.SigningKey != "" {
		contentSignature = security.SignContent(req.Content, author.SigningKey)
	}

	wordCount := len([]rune(stripHTML(req.Content)))

	chapter := &models.Chapter{
		NovelID:          uint(novelID),
		ChapterNumber:    nextNum,
		Title:            bluemonday.StrictPolicy().Sanitize(req.Title),
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
		chapter.Title = bluemonday.StrictPolicy().Sanitize(req.Title)
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
			chapter.ContentSignature = security.SignContent(req.Content, author.SigningKey)
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
		UpdatedAt: security.NowString(),
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
		"updated_at":  security.NowString(),
		"chapters":    entries,
	}

	dir := filepath.Join(config.NovelDataDir, "authors", fmt.Sprintf("%d", authorID), fmt.Sprintf("%d", novelID))
	os.MkdirAll(dir, 0755)
	jsonBytes, _ := json.MarshalIndent(checksums, "", "  ")
	os.WriteFile(filepath.Join(dir, ".checksums.json"), jsonBytes, 0644)
}