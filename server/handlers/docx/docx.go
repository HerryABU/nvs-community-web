package docx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"regexp"
	"strings"
)

// ==================== Word (.docx) 解析与生成 ====================

// ExtractDocxText 从 .docx 提取纯文本，标题段落转为 Markdown 标题行（供章节分割复用）
func ExtractDocxText(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", errors.New("不是有效的 Word 文档")
	}

	var docFile *zip.File
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			docFile = f
			break
		}
	}
	if docFile == nil {
		return "", errors.New("Word 文档缺少 word/document.xml")
	}

	rc, err := docFile.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()

	dec := xml.NewDecoder(rc)
	var sb strings.Builder
	var paraText strings.Builder
	var paraStyle string
	var styleVal string
	inP := false
	inT := false

	flushPara := func() {
		text := strings.TrimSpace(paraText.String())
		paraText.Reset()
		if text == "" {
			return
		}
		if lvl := headingLevel(paraStyle); lvl > 0 {
			sb.WriteString(strings.Repeat("#", lvl) + " " + text + "\n\n")
		} else {
			sb.WriteString(text + "\n\n")
		}
		paraStyle = ""
	}

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				inP = true
				paraText.Reset()
				paraStyle = ""
			case "pStyle":
				styleVal = ""
				for _, a := range t.Attr {
					if a.Name.Local == "val" {
						styleVal = a.Value
					}
				}
			case "t":
				inT = true
			case "br":
				if inP {
					paraText.WriteString("\n")
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "p":
				if inP {
					flushPara()
				}
				inP = false
			case "pStyle":
				paraStyle = styleVal
			case "t":
				inT = false
			}
		case xml.CharData:
			if inP && inT {
				paraText.Write(t)
			}
		}
	}

	return sb.String(), nil
}

// headingLevel 根据 Word 段落样式返回标题级别（0=非标题）
func headingLevel(style string) int {
	s := strings.ToLower(strings.TrimSpace(style))
	if s == "" {
		return 0
	}
	// 英文：heading1..heading9 / heading 1
	for i := 1; i <= 9; i++ {
		if s == "heading"+Itoa(i) || s == "heading "+Itoa(i) {
			if i <= 6 {
				return i
			}
			return 6
		}
	}
	// 中文：标题1..标题9 / 标题 1
	for i := 1; i <= 9; i++ {
		if s == "标题"+Itoa(i) || s == "标题 "+Itoa(i) {
			if i <= 6 {
				return i
			}
			return 6
		}
	}
	// 中文 Word 默认标题样式 ID 为纯数字 1..9
	if n := s; n >= "1" && n <= "9" && len(n) == 1 {
		return int(n[0] - '0')
	}
	return 0
}

func Itoa(i int) string {
	return string(rune('0' + i))
}

// BuildDocx 生成 Word (.docx) 文档（标题 Heading1，章节标题 Heading2，正文为普通段落）
func BuildDocx(title string, chapters []ChapterForExport) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// [Content_Types].xml
	w, _ := zw.Create("[Content_Types].xml")
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`))

	// _rels/.rels
	w, _ = zw.Create("_rels/.rels")
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`))

	// word/styles.xml（定义 Heading1/Heading2，保证标题样式可用）
	w, _ = zw.Create("word/styles.xml")
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/></w:style>
  <w:style w:type="paragraph" w:styleId="Heading2"><w:name w:val="heading 2"/></w:style>
  <w:style w:type="paragraph" w:styleId="Heading3"><w:name w:val="heading 3"/></w:style>
</w:styles>`))

	// word/document.xml
	var body strings.Builder
	body.WriteString(`<w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:t>` + xmlEscape(title) + `</w:t></w:r></w:p>`)
	for _, ch := range chapters {
		body.WriteString(`<w:p><w:pPr><w:pStyle w:val="Heading2"/></w:pPr><w:r><w:t>` + xmlEscape(ch.Title) + `</w:t></w:r></w:p>`)
		text := CleanMarkdownForTxt(ch.Content)
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			body.WriteString(`<w:p><w:r><w:t>` + xmlEscape(line) + `</w:t></w:r></w:p>`)
		}
	}

	documentXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>` + body.String() + `<w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440"/></w:sectPr></w:body>
</w:document>`

	w, _ = zw.Create("word/document.xml")
	w.Write([]byte(documentXML))

	zw.Close()
	return buf.Bytes(), nil
}

// xmlEscape 转义 XML 文本
func xmlEscape(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			sb.WriteString("&amp;")
		case '<':
			sb.WriteString("&lt;")
		case '>':
			sb.WriteString("&gt;")
		case '"':
			sb.WriteString("&quot;")
		case '\'':
			sb.WriteString("&apos;")
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// ChapterForExport 导出用章节结构
type ChapterForExport struct {
	Title   string
	Content string
}

// CleanMarkdownForTxt 将 Markdown 转换为纯文本
func CleanMarkdownForTxt(md string) string {
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
