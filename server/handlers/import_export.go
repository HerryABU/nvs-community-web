package handlers

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"nvs-server/config"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// POST /api/novels/import/preview — 预览导入（不创建）
func ImportPreview(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		utils.BadRequest(c, "请上传文件")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		utils.InternalError(c, "读取文件失败")
		return
	}

	content := string(data)
	splitRule := c.PostForm("split_rule")

	var chapters []parsedChapter
	if splitRule != "" {
		chapters = parseChaptersWithRule(content, splitRule)
	} else {
		chapters = parseChapters(content)
	}

	// 返回预览数据
	preview := make([]gin.H, 0, len(chapters))
	for i, ch := range chapters {
		previewText := ch.content
		if len([]rune(previewText)) > 200 {
			previewText = string([]rune(previewText)[:200]) + "..."
		}
		preview = append(preview, gin.H{
			"num":     i + 1,
			"title":   ch.title,
			"preview": previewText,
			"words":   len([]rune(ch.content)),
		})
	}

	utils.Success(c, gin.H{
		"total":    len(chapters),
		"chapters": preview,
	})
}

// POST /api/novels/import — 导入 TXT/Markdown（支持追加到已有小说）
func ImportNovel(c *gin.Context) {
	userID := c.GetUint("userID")

	title := c.PostForm("title")
	category := c.PostForm("category")
	categoriesStr := c.PostForm("categories")
	novelIDStr := c.PostForm("novel_id")

	if title == "" {
		title = "导入作品"
	}
	if category == "" {
		category = "其他"
	}

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		utils.BadRequest(c, "请上传文件")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		utils.InternalError(c, "读取文件失败")
		return
	}

	content := string(data)
	chapters := parseChapters(content)

	// 如果用户是 reader，自动升级为 author 并生成签名密钥
	var user models.User
	if err := models.DB.First(&user, userID).Error; err == nil && user.Role == "reader" {
		signingKey, _ := utils.GenerateSigningKey()
		updates := map[string]interface{}{"role": "author"}
		if signingKey != "" {
			updates["signing_key"] = signingKey
		}
		models.DB.Model(&user).Updates(updates)
	}

	var novel *models.Novel
	var isAppend bool

	// 如果传入了 novel_id，追加到已有小说
	if novelIDStr != "" {
		nid, parseErr := strconv.ParseUint(novelIDStr, 10, 64)
		if parseErr == nil {
			existingNovel, err := models.GetNovelByID(uint(nid))
			if err == nil && existingNovel.AuthorID == userID {
				novel = existingNovel
				isAppend = true
			}
		}
	}

	if !isAppend {
		// 创建新作品
		novel = &models.Novel{
			AuthorID: userID, Title: title, Category: category,
			Status: "draft",
		}
		if err := models.CreateNovel(novel); err != nil {
			utils.InternalError(c, "创建作品失败")
			return
		}

		// 保存多分类
		var cats []string
		if categoriesStr != "" {
			json.Unmarshal([]byte(categoriesStr), &cats)
		}
		saveNovelCategories(novel.ID, cats, category)
	}

	// 获取作者的签名密钥
	authorUser, _ := models.GetUserByID(userID)
	var signingKey string
	if authorUser != nil {
		signingKey = authorUser.SigningKey
	}

	// 创建/使用作者目录
	authorDir := filepath.Join(config.NovelDataDir, "authors", fmt.Sprintf("%d", userID), fmt.Sprintf("%d", novel.ID))
	os.MkdirAll(authorDir, 0755)

	// 获取已有章节的最大编号（追加模式）
	startNum := 1
	if isAppend {
		existingChapters, _ := models.GetChaptersByNovel(novel.ID)
		if len(existingChapters) > 0 {
			maxNum := 0
			for _, ch := range existingChapters {
				if ch.ChapterNumber > maxNum {
					maxNum = ch.ChapterNumber
				}
			}
			startNum = maxNum + 1
		}
	}

	// 写入各章节（保留原始 Markdown，前端 v-md-preview 渲染）
	createdChapters := make([]gin.H, 0)
	for i, ch := range chapters {
		num := startNum + i
		contentPath := filepath.Join(authorDir, fmt.Sprintf("%d.html", num))

		// 直接存储原始 Markdown 内容，不做 HTML 转义
		rawContent := ch.content
		if err := os.WriteFile(contentPath, []byte(rawContent), 0644); err != nil {
			continue
		}

		contentHash := computeSHA256(rawContent)
		contentSignature := ""
		if signingKey != "" {
			contentSignature = utils.SignContent(rawContent, signingKey)
		}

		chapter := &models.Chapter{
			NovelID:          novel.ID,
			ChapterNumber:    num,
			Title:            ch.title,
			ContentPath:      contentPath,
			ContentHash:      contentHash,
			ContentSignature: contentSignature,
			WordCount:        len([]rune(ch.content)),
			Status:           "published",
		}
		if err := models.CreateChapter(chapter); err == nil {
			createdChapters = append(createdChapters, gin.H{"num": num, "title": ch.title})
		}
	}

	models.UpdateNovelStats(novel.ID)

	utils.Success(c, gin.H{
		"novel_id":       novel.ID,
		"title":          title,
		"chapters_count": len(createdChapters),
		"chapters":       createdChapters,
	})
}

type parsedChapter struct {
	title   string
	content string
}

func parseChapters(content string) []parsedChapter {
	var chapters []parsedChapter

	// 尝试 `第X章` 模式
	chapterRe := regexp.MustCompile(`(?m)^第[一二三四五六七八九十百千0-9]+章\s*[^\n]*`)
	locs := chapterRe.FindAllStringIndex(content, -1)

	if len(locs) >= 2 {
		for i := 0; i < len(locs); i++ {
			title := strings.TrimSpace(content[locs[i][0]:locs[i][1]])
			var body string
			if i+1 < len(locs) {
				body = strings.TrimSpace(content[locs[i][1]:locs[i+1][0]])
			} else {
				body = strings.TrimSpace(content[locs[i][1]:])
			}
			chapters = append(chapters, parsedChapter{title: title, content: body})
		}
		return chapters
	}

	// 尝试 HTML 模式（检测 <h1>~<h6> 标签）
	htmlTitleRe := regexp.MustCompile(`(?i)<h([1-6])[^>]*>([^<]*)</h[1-6]>`)
	htmlLocs := htmlTitleRe.FindAllStringSubmatchIndex(content, -1)
	if len(htmlLocs) >= 1 {
		for i := 0; i < len(htmlLocs); i++ {
			title := strings.TrimSpace(content[htmlLocs[i][4]:htmlLocs[i][5]])
			var body string
			if i+1 < len(htmlLocs) {
				body = strings.TrimSpace(content[htmlLocs[i][1]:htmlLocs[i+1][0]])
			} else {
				body = strings.TrimSpace(content[htmlLocs[i][1]:])
			}
			if title == "" {
				title = fmt.Sprintf("第%d部分", len(chapters)+1)
			}
			chapters = append(chapters, parsedChapter{title: title, content: body})
		}
		return chapters
	}

	// 尝试 `# ` Markdown 标题
	mdRe := regexp.MustCompile(`(?m)^#{1,3}\s+([^\n]+)`)
	mdLocs := mdRe.FindAllStringSubmatchIndex(content, -1)
	if len(mdLocs) >= 1 {
		for i := 0; i < len(mdLocs); i++ {
			title := strings.TrimSpace(content[mdLocs[i][2]:mdLocs[i][3]])
			var body string
			if i+1 < len(mdLocs) {
				body = strings.TrimSpace(content[mdLocs[i][1]:mdLocs[i+1][0]])
			} else {
				body = strings.TrimSpace(content[mdLocs[i][1]:])
			}
			chapters = append(chapters, parsedChapter{title: title, content: body})
		}
		return chapters
	}

	// 无章节标记，整篇作为一章
	chapters = append(chapters, parsedChapter{
		title:   "正文",
		content: strings.TrimSpace(content),
	})
	return chapters
}

// parseChaptersWithRule 使用用户自定义正则分割章节
func parseChaptersWithRule(content string, rule string) []parsedChapter {
	re, err := regexp.Compile(rule)
	if err != nil {
		return parseChapters(content)
	}

	locs := re.FindAllStringIndex(content, -1)
	if len(locs) < 1 {
		return parseChapters(content)
	}

	var chapters []parsedChapter
	for i := 0; i < len(locs); i++ {
		// 标题 = 匹配行整行（从匹配位置到行尾），清理 markdown 标记
		lineEnd := strings.Index(content[locs[i][0]:], "\n")
		rawTitle := ""
		if lineEnd >= 0 {
			rawTitle = strings.TrimSpace(content[locs[i][0] : locs[i][0]+lineEnd])
		} else {
			rawTitle = strings.TrimSpace(content[locs[i][0]:])
		}

		// 清理标题：去掉开头的 #、*、空格等 markdown 标记
		title := cleanTitle(rawTitle)

		// 如果标题为空（如 --- 分割线），从分割点后取第一行非空行作为标题
		if title == "" {
			after := content[locs[i][1]:]
			nextLineEnd := strings.Index(after, "\n")
			if nextLineEnd >= 0 {
				nextLine := strings.TrimSpace(after[:nextLineEnd])
				title = cleanTitle(nextLine)
			}
			if title == "" {
				title = fmt.Sprintf("第%d部分", len(chapters)+1)
			}
		}

		// body 从行尾到下一个分割点
		bodyStart := locs[i][0]
		if lineEnd >= 0 {
			bodyStart = locs[i][0] + lineEnd + 1
		} else {
			bodyStart = locs[i][1]
		}

		var body string
		if i+1 < len(locs) {
			body = strings.TrimSpace(content[bodyStart:locs[i+1][0]])
		} else {
			body = strings.TrimSpace(content[bodyStart:])
		}

		// 过滤空章节
		if body == "" && title == "" {
			continue
		}
		if title == "" {
			title = fmt.Sprintf("第%d部分", len(chapters)+1)
		}
		chapters = append(chapters, parsedChapter{title: title, content: body})
	}

	if len(chapters) == 0 {
		chapters = append(chapters, parsedChapter{
			title:   "正文",
			content: strings.TrimSpace(content),
		})
	}

	return chapters
}

// cleanTitle 清理标题中的 markdown 标记（# * - 等前缀）
func cleanTitle(raw string) string {
	// 去掉开头的 # 号
	cleaned := regexp.MustCompile(`^#{1,6}\s*`).ReplaceAllString(raw, "")
	// 去掉 ** 和 * 包裹（如 *第X章*）
	cleaned = regexp.MustCompile(`^\*{1,2}(.+?)\*{1,2}$`).ReplaceAllString(cleaned, "$1")
	// 去掉行首行尾的 - 和多余空格
	cleaned = strings.Trim(cleaned, " -–—")
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return raw
	}
	return cleaned
}

// POST /api/novels/:id/export/epub — 导出 EPUB
func ExportEPUB(c *gin.Context) {
	userID := c.GetUint("userID")
	novelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	novel, err := models.GetNovelByID(uint(novelID))
	if err != nil || novel.AuthorID != userID {
		utils.NotFound(c, "作品不存在")
		return
	}

	chapters, err := models.GetChaptersByNovel(uint(novelID))
	if err != nil {
		utils.InternalError(c, "获取章节失败")
		return
	}

	tmpDir := filepath.Join(config.UploadDir, "exports")
	os.MkdirAll(tmpDir, 0755)

	epubPath := filepath.Join(tmpDir, fmt.Sprintf("%s.epub", novel.Title))
	f, _ := os.Create(epubPath)
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	// mimetype（必须无压缩）
	w, _ := zw.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	w.Write([]byte("application/epub+zip"))

	// container.xml
	w, _ = zw.Create("META-INF/container.xml")
	w.Write([]byte(`<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`))

	// content.opf
	opf := fmt.Sprintf(`<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="book-id">
  <metadata><dc:title xmlns:dc="http://purl.org/dc/elements/1.1/">%s</dc:title></metadata>
  <manifest><item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>`, html.EscapeString(novel.Title))
	for i := range chapters {
		opf += fmt.Sprintf(`<item id="ch%d" href="ch%d.xhtml" media-type="application/xhtml+xml"/>`, i+1, i+1)
	}
	opf += `</manifest><spine>`
	for i := range chapters {
		opf += fmt.Sprintf(`<itemref idref="ch%d"/>`, i+1)
	}
	opf += `</spine></package>`

	w, _ = zw.Create("OEBPS/content.opf")
	w.Write([]byte(opf))

	// 章节 xhtml
	for i, ch := range chapters {
		content, _ := os.ReadFile(ch.ContentPath)
		xhtml := fmt.Sprintf(`<?xml version="1.0"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml">
<head><title>%s</title></head>
<body><h1>%s</h1>%s</body></html>`, html.EscapeString(ch.Title), html.EscapeString(ch.Title), string(content))
		w, _ = zw.Create(fmt.Sprintf("OEBPS/ch%d.xhtml", i+1))
		w.Write([]byte(xhtml))
	}

	c.File(epubPath)
}

// POST /api/novels/:id/export/txt — 导出纯文本
func ExportTXT(c *gin.Context) {
	userID := c.GetUint("userID")
	novelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	novel, err := models.GetNovelByID(uint(novelID))
	if err != nil || novel.AuthorID != userID {
		utils.NotFound(c, "作品不存在")
		return
	}

	chapters, err := models.GetChaptersByNovel(uint(novelID))
	if err != nil {
		utils.InternalError(c, "获取章节失败")
		return
	}

	var txt strings.Builder
	txt.WriteString(fmt.Sprintf("%s\n\n作者: %d | 分类: %s\n\n%s\n\n",
		novel.Title, novel.AuthorID, novel.Category,
		strings.Repeat("=", 50)))

	for _, ch := range chapters {
		content, _ := os.ReadFile(ch.ContentPath)
		// 保留原始内容（章节存储为 Markdown），转换为纯文本
		text := string(content)
		// 简单清理 Markdown 标记
		text = cleanMarkdownForTxt(text)
		txt.WriteString(fmt.Sprintf("\n第%s章 %s\n\n%s\n\n%s\n\n",
			formatChapterNumber(ch.ChapterNumber), ch.Title,
			text,
			strings.Repeat("-", 40)))
	}

	tmpDir := filepath.Join(config.UploadDir, "exports")
	os.MkdirAll(tmpDir, 0755)
	txtPath := filepath.Join(tmpDir, fmt.Sprintf("%s.txt", novel.Title))
	os.WriteFile(txtPath, []byte(txt.String()), 0644)

	c.FileAttachment(txtPath, fmt.Sprintf("%s.txt", novel.Title))
}

// POST /api/novels/:id/export/markdown — 导出合并 Markdown
func ExportMarkdown(c *gin.Context) {
	userID := c.GetUint("userID")
	novelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	novel, err := models.GetNovelByID(uint(novelID))
	if err != nil || novel.AuthorID != userID {
		utils.NotFound(c, "作品不存在")
		return
	}

	chapters, err := models.GetChaptersByNovel(uint(novelID))
	if err != nil {
		utils.InternalError(c, "获取章节失败")
		return
	}

	var md strings.Builder
	md.WriteString(fmt.Sprintf("# %s\n\n> 作者ID: %d | 分类: %s | 共 %d 章\n\n",
		novel.Title, novel.AuthorID, novel.Category, len(chapters)))

	for _, ch := range chapters {
		content, _ := os.ReadFile(ch.ContentPath)
		// 保留原始 Markdown 内容（不做 HTML→Markdown 转换）
		md.WriteString(fmt.Sprintf("## 第%d章 %s\n\n%s\n\n", ch.ChapterNumber, ch.Title, string(content)))
	}

	tmpDir := filepath.Join(config.UploadDir, "exports")
	os.MkdirAll(tmpDir, 0755)
	mdPath := filepath.Join(tmpDir, fmt.Sprintf("%s.md", novel.Title))
	os.WriteFile(mdPath, []byte(md.String()), 0644)

	c.FileAttachment(mdPath, fmt.Sprintf("%s.md", novel.Title))
}

// formatChapterNumber 将数字转为中文章节号（简化版）
func formatChapterNumber(num int) string {
	if num <= 0 {
		return "零"
	}
	// 对于小说，直接返回数字
	return fmt.Sprintf("%d", num)
}

// cleanMarkdownForTxt 将 Markdown 转换为纯文本
func cleanMarkdownForTxt(md string) string {
	// 去掉 HTML 标签（章节可能包含 HTML）
	re := regexp.MustCompile(`<[^>]*>`)
	result := re.ReplaceAllString(md, "")

	// 去掉 Markdown 标题标记
	result = regexp.MustCompile(`(?m)^#{1,6}\s+`).ReplaceAllString(result, "")

	// 去掉粗体/斜体标记
	result = regexp.MustCompile(`\*{1,3}([^*]+)\*{1,3}`).ReplaceAllString(result, "$1")
	result = regexp.MustCompile(`_{1,3}([^_]+)_{1,3}`).ReplaceAllString(result, "$1")

	// 去掉 ==高亮== 标记
	result = regexp.MustCompile(`==([^=]+)==`).ReplaceAllString(result, "$1")

	// 去掉链接，保留文本
	result = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`).ReplaceAllString(result, "$1")

	// 去掉图片
	result = regexp.MustCompile(`!\[([^\]]*)\]\([^)]+\)`).ReplaceAllString(result, "$1")

	// 去掉行内代码标记
	result = regexp.MustCompile("`([^`]+)`").ReplaceAllString(result, "$1")

	return strings.TrimSpace(result)
}

func stripTags(html string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return strings.TrimSpace(re.ReplaceAllString(html, ""))
}
