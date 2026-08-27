package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"nvs-server/config"
	"nvs-server/models"
	"nvs-server/security"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	"github.com/microcosm-cc/bluemonday"
)

func CreateChapter(c *gin.Context) {
	userID := currentUserID(c)
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

	// XSS 防护：移除内联 HTML 标签（保留 Markdown 语法与文本内容）
	raw := security.SanitizeUserContent(req.Content)
	cleaned, badHits := processBadWords(raw, "chapter")
	req.Content = cleaned

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
		ContentPath:      filepath.ToSlash(contentPath), // 存库统一正斜杠，跨平台可移植（Windows/Linux 均可读）
		ContentHash:      contentHash,
		ContentSignature: contentSignature,
		WordCount:        wordCount,
		Status:           "published",
	}

	if err := models.CreateChapter(chapter); err != nil {
		utils.InternalError(c, "创建章节失败")
		return
	}

	// 违禁词 mark 动作命中 → 写入待审标记记录
	flagBadWords(badHits, "chapter", chapter.ID, userID, raw, chapter.Title)

	models.UpdateNovelStats(uint(novelID))
	updateIndexJSON(uint(userID), uint(novelID))
	generateChecksums(uint(userID), uint(novelID))

	utils.Success(c, chapter)
}

func UpdateChapter(c *gin.Context) {
	userID := currentUserID(c)
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

	// XSS 防护：移除内联 HTML 标签（保留 Markdown 语法与文本内容）
	var badHits []security.BadWordHit
	var raw string
	if req.Content != "" {
		raw = security.SanitizeUserContent(req.Content)
		var cleaned string
		cleaned, badHits = processBadWords(raw, "chapter")
		req.Content = cleaned
	}

	if req.Title != "" {
		chapter.Title = bluemonday.StrictPolicy().Sanitize(req.Title)
	}
	if req.Content != "" {
		// 直接存储原始 Markdown 内容
		if err := os.WriteFile(utils.ResolveContentPath(chapter.ContentPath), []byte(req.Content), 0644); err != nil {
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

	// 违禁词 mark 动作命中 → 写入待审标记记录
	flagBadWords(badHits, "chapter", chapter.ID, userID, raw, chapter.Title)

	models.UpdateNovelStats(uint(novelID))
	updateIndexJSON(uint(userID), uint(novelID))
	generateChecksums(uint(userID), uint(novelID))

	utils.Success(c, chapter)
}

func DeleteChapter(c *gin.Context) {
	userID := currentUserID(c)
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

	// 合规留存：软删除被删章节，正文文件移入 .deleted/ 目录归档保留（可追溯）；
	// 随后将后续章节编号前移补齐（文件同步重命名），保证章节编号始终连续。
	authorDir := filepath.Join(config.NovelDataDir, "authors", fmt.Sprintf("%d", userID), fmt.Sprintf("%d", novelID))
	deletedDir := filepath.Join(authorDir, ".deleted")
	os.MkdirAll(deletedDir, 0755)
	deletedPath := filepath.Join(deletedDir, fmt.Sprintf("%d_%d.html", chapterNum, time.Now().Unix()))
	if chapter.ContentPath != "" {
		if err := os.Rename(utils.ResolveContentPath(chapter.ContentPath), deletedPath); err == nil {
			models.DB.Model(&models.Chapter{}).Where("id = ?", chapter.ID).Update("content_path", filepath.ToSlash(deletedPath))
		}
	}

	// 作者自删：清除封禁原因标记，避免误显示 176 号令封禁横幅（ban_reason 非空才视为封禁）
	_ = models.DB.Model(&models.Chapter{}).Where("id = ?", chapter.ID).Update("ban_reason", "")

	if err := models.DeleteChapter(chapter.ID); err != nil {
		utils.InternalError(c, "删除章节失败")
		return
	}

	// 标记为「用户自删」（区别于管理员封禁：deleted_by='admin'）
	_ = models.DB.Unscoped().Model(&models.Chapter{}).Where("id = ?", chapter.ID).Update("deleted_by", "user")

	// 编号补齐：删除章节后，后续章节 chapter_number 前移重排（正文文件同步重命名），
	// 保证「第N章」连续无空洞（章节管理/阅读器均按新编号展示）。
	renumberChapters(uint(novelID), chapterNum, authorDir)

	models.UpdateNovelStats(uint(novelID))
	updateIndexJSON(uint(userID), uint(novelID))
	generateChecksums(uint(userID), uint(novelID))

	utils.Success(c, gin.H{"message": "章节已删除（软删除留证，编号已自动补齐）"})
}

// renumberChapters 删除章节后编号补齐：将 chapter_number > deletedNum 的剩余章节
// 依次前移一位（文件同步重命名 + 数据库 chapter_number/content_path 更新）。
// 遍历按章节号升序（GetChaptersByNovel），目标号 = 源号-1 且目标已被释放，从小到大
// 执行重命名不会发生文件覆盖。重命名失败仅记录日志并跳过（保持数据尽力一致）。
func renumberChapters(novelID uint, deletedNum int, dir string) {
	chapters, err := models.GetChaptersByNovel(novelID)
	if err != nil {
		return
	}

	for _, ch := range chapters {
		if ch.ChapterNumber > deletedNum {
			oldPath := utils.ResolveContentPath(ch.ContentPath)
			newNum := ch.ChapterNumber - 1
			newPath := filepath.Join(dir, fmt.Sprintf("%d.html", newNum))

			// 重命名文件（目标号对应文件已被软删除移走，不会冲突）
			if err := os.Rename(oldPath, newPath); err != nil {
				log.Printf("[NVS] 章节编号补齐：文件重命名失败 %s → %s (%v)", oldPath, newPath, err)
				continue
			}

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
		contentPath := utils.ResolveContentPath(ch.ContentPath)
		fileInfo, err := os.Stat(contentPath)
		if err != nil {
			continue
		}
		content, err := os.ReadFile(contentPath)
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