package utils

import (
	"regexp"
	"strings"
)

// RenderMarkdown 纯 Go 标准库实现的简易 Markdown→HTML 渲染器
// 支持：标题、粗体、斜体、代码块、段落、分割线
// 输出已通过 bluemonday 净化，可安全用于 v-html
func RenderMarkdown(md string) string {
	html := md

	// 代码块 ``` ... ```
	codeRe := regexp.MustCompile("(?s)```(\\w*)\\n?(.+?)```")
	html = codeRe.ReplaceAllStringFunc(html, func(m string) string {
		parts := codeRe.FindStringSubmatch(m)
		lang := parts[1]
		code := strings.TrimSpace(parts[2])
		// HTML 实体转义
		code = htmlEscape(code)
		if lang != "" {
			return "<pre><code class=\"language-" + lang + "\">" + code + "</code></pre>"
		}
		return "<pre><code>" + code + "</code></pre>"
	})

	// 行内代码 `code`
	inlineCodeRe := regexp.MustCompile("`([^`]+)`")
	html = inlineCodeRe.ReplaceAllString(html, "<code>$1</code>")

	// 粗体 **text** 和 __text__
	boldRe := regexp.MustCompile(`\*\*(.+?)\*\*`)
	html = boldRe.ReplaceAllString(html, "<strong>$1</strong>")
	bold2Re := regexp.MustCompile(`__(.+?)__`)
	html = bold2Re.ReplaceAllString(html, "<strong>$1</strong>")

	// 斜体 *text* 和 _text_
	italicRe := regexp.MustCompile(`\*(.+?)\*`)
	html = italicRe.ReplaceAllString(html, "<em>$1</em>")
	italic2Re := regexp.MustCompile(`_(.+?)_`)
	html = italic2Re.ReplaceAllString(html, "<em>$1</em>")

	// 分割线 --- / *** / ___
	hrRe := regexp.MustCompile(`(?m)^[-*_]{3,}\s*$`)
	html = hrRe.ReplaceAllString(html, "<hr>")

	// 标题 H1-H6
	hRe := regexp.MustCompile(`(?m)^#{1,6}\s+(.+)$`)
	html = hRe.ReplaceAllStringFunc(html, func(m string) string {
		level := 0
		for _, c := range m {
			if c == '#' {
				level++
			} else {
				break
			}
		}
		text := strings.TrimSpace(m[level:])
		if text == "" {
			return m
		}
		return strings.Repeat("<", 1) + "h" + string(rune('0'+level)) + ">" + text + "</h" + string(rune('0'+level)) + ">"
	})

	// 块引用 >
	lines := strings.Split(html, "\n")
	var result []string
	inBlockquote := false
	var blockquoteLines []string

	flushBlockquote := func() {
		if len(blockquoteLines) > 0 {
			result = append(result, "<blockquote><p>"+strings.Join(blockquoteLines, "<br>")+"</p></blockquote>")
			blockquoteLines = nil
		}
		inBlockquote = false
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "> ") || trimmed == ">" {
			if !inBlockquote {
				flushBlockquote()
				inBlockquote = true
			}
			content := strings.TrimPrefix(trimmed, "> ")
			if content == "" {
				content = strings.TrimPrefix(trimmed, ">")
			}
			blockquoteLines = append(blockquoteLines, content)
			continue
		}

		flushBlockquote()

		// 跳过已经是 HTML 标签的行
		if strings.HasPrefix(trimmed, "<") && !strings.HasPrefix(trimmed, "<p>") {
			result = append(result, line)
			continue
		}

		if trimmed == "" {
			result = append(result, "")
		} else {
			result = append(result, line)
		}
	}
	flushBlockquote()
	html = strings.Join(result, "\n")

	// 段落：连续非空行包裹为 <p>
	html = paragraphWrap(html)

	// 安全净化：移除任何残留的危险标签和事件处理器
	html = SanitizeHTML(html)

	return html
}

func paragraphWrap(html string) string {
	lines := strings.Split(html, "\n")
	var result []string
	var para []string

	flush := func() {
		if len(para) > 0 {
			result = append(result, "<p>"+strings.Join(para, "<br>\n")+"</p>")
			para = nil
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// 已是 HTML 块级元素，直接输出
		if strings.HasPrefix(trimmed, "<h") ||
			strings.HasPrefix(trimmed, "<pre") ||
			strings.HasPrefix(trimmed, "<hr") ||
			strings.HasPrefix(trimmed, "<blockquote") ||
			strings.HasPrefix(trimmed, "<ul") ||
			strings.HasPrefix(trimmed, "<ol") ||
			strings.HasPrefix(trimmed, "<li") ||
			strings.HasPrefix(trimmed, "</") {
			flush()
			result = append(result, trimmed)
			continue
		}
		if trimmed == "" {
			flush()
			result = append(result, "")
		} else {
			para = append(para, trimmed)
		}
	}
	flush()
	return strings.Join(result, "\n")
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
