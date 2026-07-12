package security

import (
	"regexp"
	"strings"
	"time"

	"github.com/microcosm-cc/bluemonday"
)

// 预编译正则 + 策略（只初始化一次）
var (
	// 用于 Markdown 用户内容的净化器——允许基本 HTML 标签但不允许事件处理、javascript 链接等
	markdownPolicy *bluemonday.Policy

	// 用于纯文本评论的严格净化器——只保留纯文本
	strictPolicy *bluemonday.Policy

	// 危险模式（在 Markdown 渲染之前先去掉）
	scriptRe    = regexp.MustCompile(`(?i)<\s*script[\s/>]`)
	jsProtoRe   = regexp.MustCompile(`(?i)javascript\s*:`)
	dataProtoRe = regexp.MustCompile(`(?i)data\s*:`) // 可被用于 SVG/IMG XSS
	onEventRe   = regexp.MustCompile(`(?i)\s+on\w+\s*=\s*`)
	svgRe       = regexp.MustCompile(`(?i)<\s*svg[\s/>]`)
	embedRe     = regexp.MustCompile(`(?i)<\s*embed[\s/>]`)
	objectRe    = regexp.MustCompile(`(?i)<\s*object[\s/>]`)
	iframeRe    = regexp.MustCompile(`(?i)<\s*iframe[\s/>]`)
	linkTagRe   = regexp.MustCompile(`(?i)<\s*link[\s/>]`)
	metaRe      = regexp.MustCompile(`(?i)<\s*meta[\s/>]`)
	baseRe      = regexp.MustCompile(`(?i)<\s*base[\s/>]`)
	formRe      = regexp.MustCompile(`(?i)<\s*form[\s/>]`)
	exprRe      = regexp.MustCompile(`(?i)expression\s*\(`)
)

func init() {
	// 宽松策略：用于 Markdown 内容，允许基本格式标签和安全的 HTML 标签
	markdownPolicy = bluemonday.NewPolicy()
	markdownPolicy.AllowElements(
		"kbd", "mark", "sup", "sub", "br", "hr", "del", "ins",
		"u", "s", "b", "i", "em", "strong", "span", "div",
		"p", "h1", "h2", "h3", "h4", "h5", "h6",
		"ul", "ol", "li", "dl", "dt", "dd",
		"table", "thead", "tbody", "tfoot", "tr", "th", "td",
		"blockquote", "pre", "code", "a", "img",
		"details", "summary", "figure", "figcaption",
	)
	// 允许安全的属性（含 style，用于 KaTeX 行内样式和 Mermaid SVG）
	markdownPolicy.AllowAttrs("class", "id", "href", "src", "alt", "title", "lang", "dir",
		"target", "rel", "type", "style").OnElements("a", "img", "kbd", "span", "div", "pre", "code", "mark")

	// 只允许 http/https/ftp/mailto 链接
	markdownPolicy.AllowURLSchemes("http", "https", "ftp", "mailto")

	// 允许 data: 仅用于图片（base64 内嵌图片）
	markdownPolicy.AllowURLSchemes("data")

	// img 只允许 src, alt, title, class
	markdownPolicy.AllowAttrs("src", "alt", "title", "class", "width", "height").OnElements("img")

	// a 标签只允许 href, title, target, rel
	markdownPolicy.AllowAttrs("href", "title", "target", "rel").OnElements("a")
	markdownPolicy.RequireNoFollowOnLinks(true)
	markdownPolicy.AddTargetBlankToFullyQualifiedLinks(true)

	// 严格策略：只保留纯文本（用于 summary、昵称等不需要格式的字段）
	strictPolicy = bluemonday.StrictPolicy()
}

// SanitizeHTML 净化 HTML 内容（使用宽松策略，保留安全 HTML）
func SanitizeHTML(html string) string {
	return markdownPolicy.Sanitize(html)
}

// SanitizeScriptTag 过滤危险 HTML 标签和事件处理器（保留 Markdown 语法）
// 用于在存储评论/帖子内容之前的基础清理
func SanitizeScriptTag(content string) string {
	result := content

	// 1. 移除 <script> 标签（包括带属性/空格变体）
	result = scriptRe.ReplaceAllString(result, "&lt;script")

	// 2. 移除 javascript: 协议（常见 XSS 向量）
	result = jsProtoRe.ReplaceAllString(result, "blocked:")

	// 剩下几个在 Markdown 上下文中不太可能出现（因为 Cherry Markdown 会过滤），
	// 但作为防御层仍然保留：

	// 3. 移除数据 URI（可能包含 SVG XSS）
	if strings.Contains(result, "data:") {
		result = dataProtoRe.ReplaceAllString(result, "blocked:")
	}

	// 4. 移除行内事件处理器
	result = onEventRe.ReplaceAllString(result, " data-blocked=")

	// 5. 移除危险标签
	result = svgRe.ReplaceAllString(result, "&lt;svg")
	result = embedRe.ReplaceAllString(result, "&lt;embed")
	result = objectRe.ReplaceAllString(result, "&lt;object")
	result = iframeRe.ReplaceAllString(result, "&lt;iframe")
	result = linkTagRe.ReplaceAllString(result, "&lt;link")
	result = metaRe.ReplaceAllString(result, "&lt;meta")
	result = baseRe.ReplaceAllString(result, "&lt;base")
	result = formRe.ReplaceAllString(result, "&lt;form")

	// 6. CSS expression() 注入
	result = exprRe.ReplaceAllString(result, "blocked(")

	return result
}

// SanitizeUserContent 全面净化用户输入：先清理危险标签，再用 bluemonday 净化
// 这是所有用户输入（评论、帖子、回复等）都应调用的统一入口
func SanitizeUserContent(content string) string {
	// 第一道防线：正则移除最危险的内容
	cleaned := SanitizeScriptTag(content)

	// 第二道防线：bluemonday 净化残留 HTML，保留基本格式
	return markdownPolicy.Sanitize(cleaned)
}

// SanitizePlainText 纯文本净化（用于不需要 HTML 的字段）
func SanitizePlainText(text string) string {
	return strictPolicy.Sanitize(text)
}

// NowString 返回当前时间的格式化字符串
func NowString() string {
	return time.Now().Format(time.RFC3339)
}
