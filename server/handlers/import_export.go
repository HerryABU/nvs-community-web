package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"nvs-server/config"
	"nvs-server/models"
	"nvs-server/security"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// readImportContent 读取导入文件内容：.docx 转 Markdown 文本，其余按纯文本/Markdown 处理
func readImportContent(data []byte, filename string) (string, error) {
	lower := strings.ToLower(filename)
	if strings.HasSuffix(lower, ".docx") {
		text, err := extractDocxText(data)
		if err != nil {
			return "", err
		}
		return text, nil
	}
	// 其余（.md/.txt/.html 等）按 UTF-8 文本读取
	return string(data), nil
}

// POST /api/novels/import/preview — 预览导入（不创建）
func ImportPreview(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
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

	filename := ""
	if header != nil {
		filename = header.Filename
	}
	content, err := readImportContent(data, filename)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

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
	userID := currentUserID(c)

	title := c.PostForm("title")
	category := c.PostForm("category")
	categoriesStr := c.PostForm("categories")
	novelIDStr := c.PostForm("novel_id")
	splitRule := c.PostForm("split_rule")

	if title == "" {
		title = "导入作品"
	}
	if category == "" {
		category = "其他"
	}

	file, header, err := c.Request.FormFile("file")
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

	filename := ""
	if header != nil {
		filename = header.Filename
	}
	content, err := readImportContent(data, filename)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	// 使用用户自定义的分割规则（与预览一致）；为空则自动识别
	var chapters []parsedChapter
	if splitRule != "" {
		chapters = parseChaptersWithRule(content, splitRule)
	} else {
		chapters = parseChapters(content)
	}

	// 如果用户是 reader，自动升级为 author 并生成签名密钥
	var user models.User
	if err := models.DB.First(&user, userID).Error; err == nil && user.Role == "reader" {
		signingKey, _ := security.GenerateSigningKey()
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

	// 获取已有章节的最大编号（追加模式；含软删除章节，避免串号）
	startNum := 1
	if isAppend {
		if maxNum, err := models.GetMaxChapterNumber(novel.ID); err == nil {
			startNum = maxNum + 1
		}
	}

	createdChapters := writeParsedChapters(novel, chapters, startNum, userID)

	utils.Success(c, gin.H{
		"novel_id":       novel.ID,
		"title":          title,
		"chapters_count": len(createdChapters),
		"chapters":       createdChapters,
	})
}

// writeParsedChapters 将解析后的章节写入文件系统与数据库
func writeParsedChapters(novel *models.Novel, chapters []parsedChapter, startNum int, userID uint) []gin.H {
	authorUser, _ := models.GetUserByID(userID)
	var signingKey string
	if authorUser != nil {
		signingKey = authorUser.SigningKey
	}

	authorDir := filepath.Join(config.NovelDataDir, "authors", fmt.Sprintf("%d", userID), fmt.Sprintf("%d", novel.ID))
	os.MkdirAll(authorDir, 0755)

	createdChapters := make([]gin.H, 0)
	for i, ch := range chapters {
		num := startNum + i
		contentPath := filepath.Join(authorDir, fmt.Sprintf("%d.html", num))

		// XSS 防护：移除内联 HTML 标签（保留 Markdown 语法与文本内容）
		rawContent := security.SanitizeUserContent(ch.content)
		if err := os.WriteFile(contentPath, []byte(rawContent), 0644); err != nil {
			continue
		}

		contentHash := computeSHA256(rawContent)
		contentSignature := ""
		if signingKey != "" {
			contentSignature = security.SignContent(rawContent, signingKey)
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
	return createdChapters
}

// ImportBatch POST /api/novels/import/batch — 批量导入多个 Word/Markdown 文件（合并为一部作品）
// form 参数：
//   title / category：作品标题与分类（仅新建作品时使用）
//   novel_id：可选，传入后追加到已有小说（作者本人），不再新建书本
//   mode：file(默认，一个文件一章) | rule(自定义正则拆分) | auto(自动识别标题拆分)
//   split_rule：mode=rule 时的正则表达式
//   title_prefix：on 时章节标题加「第N章 」前缀
func ImportBatch(c *gin.Context) {
	userID := currentUserID(c)

	title := c.PostForm("title")
	category := c.PostForm("category")
	novelIDStr := c.PostForm("novel_id")
	mode := c.PostForm("mode")
	splitRule := c.PostForm("split_rule")
	titlePrefix := c.PostForm("title_prefix") == "on"
	if title == "" {
		title = "批量导入作品"
	}
	if category == "" {
		category = "其他"
	}
	if mode == "" {
		mode = "file"
	}

	form, err := c.MultipartForm()
	if err != nil {
		utils.BadRequest(c, "请上传文件")
		return
	}
	files := form.File["files"]
	if len(files) == 0 {
		utils.BadRequest(c, "请至少上传一个文件")
		return
	}

	// 逐个文件解析为章节
	var allChapters []parsedChapter
	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			continue
		}
		content, err := readImportContent(data, fh.Filename)
		if err != nil {
			continue
		}
		content = strings.TrimSpace(content)
		if content == "" {
			continue
		}

		switch mode {
		case "rule":
			// 自定义正则拆分（正则匹配失败/无匹配则整篇一章）
			if strings.TrimSpace(splitRule) != "" {
				allChapters = append(allChapters, parseChaptersWithRule(content, splitRule)...)
			} else {
				allChapters = append(allChapters, parsedChapter{title: stripExt(fh.Filename), content: content})
			}
		case "auto":
			// 自动识别：按文件内「第X章 / HTML标题 / Markdown标题」拆分
			allChapters = append(allChapters, parseChapters(content)...)
		default: // file：一个文件一章，标题取文件名（去扩展名）
			allChapters = append(allChapters, parsedChapter{title: stripExt(fh.Filename), content: content})
		}
	}

	if len(allChapters) == 0 {
		utils.BadRequest(c, "未能从上传文件中解析出任何章节")
		return
	}

	// 可选：章节标题统一加「第N章 」前缀
	if titlePrefix {
		for i := range allChapters {
			allChapters[i].title = fmt.Sprintf("第%d章 %s", i+1, allChapters[i].title)
		}
	}

	// reader 自动升级为 author
	var user models.User
	if err := models.DB.First(&user, userID).Error; err == nil && user.Role == "reader" {
		signingKey, _ := security.GenerateSigningKey()
		updates := map[string]interface{}{"role": "author"}
		if signingKey != "" {
			updates["signing_key"] = signingKey
		}
		models.DB.Model(&user).Updates(updates)
	}

	var novel *models.Novel
	var isAppend bool

	// 传入 novel_id 且属于当前作者 → 追加到已有小说（不再新建书本）
	if novelIDStr != "" {
		if nid, parseErr := strconv.ParseUint(novelIDStr, 10, 64); parseErr == nil {
			existingNovel, err := models.GetNovelByID(uint(nid))
			if err == nil && existingNovel.AuthorID == userID {
				novel = existingNovel
				isAppend = true
			}
		}
	}

	if !isAppend {
		novel = &models.Novel{
			AuthorID: userID, Title: title, Category: category,
			Status: "draft",
		}
		if err := models.CreateNovel(novel); err != nil {
			utils.InternalError(c, "创建作品失败")
			return
		}
	}

	// 追加模式：从已有最大章节号后开始编号（含软删除章节，避免串号）
	startNum := 1
	if isAppend {
		if maxNum, err := models.GetMaxChapterNumber(novel.ID); err == nil {
			startNum = maxNum + 1
		}
	}

	createdChapters := writeParsedChapters(novel, allChapters, startNum, userID)

	utils.Success(c, gin.H{
		"novel_id":       novel.ID,
		"title":          novel.Title,
		"mode":           mode,
		"is_append":      isAppend,
		"files_count":    len(files),
		"chapters_count": len(createdChapters),
		"chapters":       createdChapters,
	})
}

// stripExt 去除文件扩展名（保留原始标题，如 "第一章 开端.md" → "第一章 开端"）
func stripExt(filename string) string {
	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	if ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	return strings.TrimSpace(base)
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
// 自动区分两种语义：匹配内容清理后为空 → 分割线规则；否则 → 标题前缀规则
func parseChaptersWithRule(content string, rule string) []parsedChapter {
	re, err := regexp.Compile(rule)
	if err != nil {
		return parseChapters(content)
	}

	locs := re.FindAllStringIndex(content, -1)
	if len(locs) < 1 {
		return parseChapters(content)
	}

	// 判断是"分割线"还是"标题"规则：看第一个匹配所在"整行"清理后是否为空
	// （如 --- 整行清理后为空 → 分割线；# 第一章 整行清理后为"第一章" → 标题）
	firstLineEnd := strings.Index(content[locs[0][0]:], "\n")
	firstLine := ""
	if firstLineEnd >= 0 {
		firstLine = strings.TrimSpace(content[locs[0][0] : locs[0][0]+firstLineEnd])
	} else {
		firstLine = strings.TrimSpace(content[locs[0][0]:])
	}
	if cleanTitle(firstLine) == "" {
		return splitByDivider(content, locs)
	}
	return splitByTitle(content, locs)
}

// splitByDivider 分割线规则（如 ---、***）：章节 = 分割线之间的内容
func splitByDivider(content string, locs [][]int) []parsedChapter {
	var chapters []parsedChapter
	start := 0
	for i := 0; i < len(locs); i++ {
		body := strings.TrimSpace(content[start:locs[i][0]])
		if body != "" {
			chapters = append(chapters, parsedChapter{
				title:   fmt.Sprintf("第%d部分", len(chapters)+1),
				content: body,
			})
		}
		start = locs[i][1]
	}
	// 最后一个分割线之后的内容
	last := strings.TrimSpace(content[start:])
	if last != "" {
		chapters = append(chapters, parsedChapter{
			title:   fmt.Sprintf("第%d部分", len(chapters)+1),
			content: last,
		})
	}
	if len(chapters) == 0 {
		chapters = append(chapters, parsedChapter{
			title:   "正文",
			content: strings.TrimSpace(content),
		})
	}
	return chapters
}

// splitByTitle 标题前缀规则（如 第X章、# 标题）：章节 = 标题 + 标题后到下一标题的内容
func splitByTitle(content string, locs [][]int) []parsedChapter {
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

		title := cleanTitle(rawTitle)

		// 如果标题为空，从分割点后取第一行非空行作为标题
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
	// 纯分隔符（如 ---）清理后为空，返回空字符串让调用方兜底，避免把分隔符当标题
	return cleaned
}