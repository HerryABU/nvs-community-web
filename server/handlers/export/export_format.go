package export

import (
	"archive/zip"
	"fmt"
	"html"
	"nvs-server/utils/infra"
	"nvs-server/utils/sanitize"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"nvs-server/config"
	"nvs-server/handlers/docx"
	"nvs-server/handlers/hctx"
	"nvs-server/models"

	"github.com/gin-gonic/gin"
)

// POST /api/novels/:id/export/epub — 导出 EPUB
func ExportEPUB(c *gin.Context) {
	userID := hctx.CurrentUserID(c)
	novelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	novel, err := models.GetNovelByID(uint(novelID))
	if err != nil || novel.AuthorID != userID {
		infra.NotFound(c, "作品不存在")
		return
	}

	chapters, err := models.GetChaptersByNovel(uint(novelID))
	if err != nil {
		infra.InternalError(c, "获取章节失败")
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
		content, _ := os.ReadFile(sanitize.ResolveContentPath(ch.ContentPath))
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
	userID := hctx.CurrentUserID(c)
	novelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	novel, err := models.GetNovelByID(uint(novelID))
	if err != nil || novel.AuthorID != userID {
		infra.NotFound(c, "作品不存在")
		return
	}

	chapters, err := models.GetChaptersByNovel(uint(novelID))
	if err != nil {
		infra.InternalError(c, "获取章节失败")
		return
	}

	var txt strings.Builder
	txt.WriteString(fmt.Sprintf("%s\n\n作者: %d | 分类: %s\n\n%s\n\n",
		novel.Title, novel.AuthorID, novel.Category,
		strings.Repeat("=", 50)))

	for _, ch := range chapters {
		content, _ := os.ReadFile(sanitize.ResolveContentPath(ch.ContentPath))
		// 保留原始内容（章节存储为 Markdown），转换为纯文本
		text := string(content)
		// 简单清理 Markdown 标记
		text = docx.CleanMarkdownForTxt(text)
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

// POST /api/novels/:id/export/docx — 导出 Word (.docx)
func ExportDOCX(c *gin.Context) {
	userID := hctx.CurrentUserID(c)
	novelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	novel, err := models.GetNovelByID(uint(novelID))
	if err != nil || novel.AuthorID != userID {
		infra.NotFound(c, "作品不存在")
		return
	}

	chapters, err := models.GetChaptersByNovel(uint(novelID))
	if err != nil {
		infra.InternalError(c, "获取章节失败")
		return
	}

	exports := make([]docx.ChapterForExport, 0, len(chapters))
	for _, ch := range chapters {
		content, _ := os.ReadFile(sanitize.ResolveContentPath(ch.ContentPath))
		exports = append(exports, docx.ChapterForExport{
			Title:   fmt.Sprintf("第%d章 %s", ch.ChapterNumber, ch.Title),
			Content: string(content),
		})
	}

	data, err := docx.BuildDocx(novel.Title, exports)
	if err != nil {
		infra.InternalError(c, "生成 Word 文档失败")
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.docx\"", novel.Title))
	c.Data(200, "application/vnd.openxmlformats-officedocument.wordprocessingml.document", data)
}

// POST /api/novels/:id/export/markdown — 导出合并 Markdown
func ExportMarkdown(c *gin.Context) {
	userID := hctx.CurrentUserID(c)
	novelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	novel, err := models.GetNovelByID(uint(novelID))
	if err != nil || novel.AuthorID != userID {
		infra.NotFound(c, "作品不存在")
		return
	}

	chapters, err := models.GetChaptersByNovel(uint(novelID))
	if err != nil {
		infra.InternalError(c, "获取章节失败")
		return
	}

	var md strings.Builder
	md.WriteString(fmt.Sprintf("# %s\n\n> 作者ID: %d | 分类: %s | 共 %d 章\n\n",
		novel.Title, novel.AuthorID, novel.Category, len(chapters)))

	for _, ch := range chapters {
		content, _ := os.ReadFile(sanitize.ResolveContentPath(ch.ContentPath))
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

func stripTags(html string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return strings.TrimSpace(re.ReplaceAllString(html, ""))
}
