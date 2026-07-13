package handlers

import (
	"os"
	"strconv"

	"nvs-server/markdown"
	"nvs-server/models"
	"nvs-server/security"
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
			signatureVerified = security.VerifySignature(string(content), chapter.ContentSignature, author.SigningKey)
		}
	}

	chapter.Content = string(content)
	// 后端渲染 Markdown → HTML
	htmlContent := markdown.RenderMarkdown(string(content))

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
			signatureVerified = security.VerifySignature(string(content), chapter.ContentSignature, author.SigningKey)
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