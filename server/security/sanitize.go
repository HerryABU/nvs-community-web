package security

import (
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/microcosm-cc/bluemonday"
)

// 预编译正则 + 策略（只初始化一次）
var (
	// 用于后端 goldmark 渲染后 HTML 的净化器——只允许 Markdown 生成的安全标签
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
	// 宽松策略：用于后端 goldmark 渲染后的 HTML，仅允许 Markdown 生成的安全标签。
	// 注意：不允许 div/span/style 等内联容器与样式，防止 UI 欺骗（如全屏覆盖钓鱼）。
	markdownPolicy = bluemonday.NewPolicy()
	markdownPolicy.AllowElements(
		"kbd", "mark", "sup", "sub", "br", "hr", "del", "ins",
		"u", "s", "b", "i", "em", "strong",
		"p", "h1", "h2", "h3", "h4", "h5", "h6",
		"ul", "ol", "li", "dl", "dt", "dd",
		"table", "thead", "tbody", "tfoot", "tr", "th", "td",
		"blockquote", "pre", "code", "a", "img",
		"details", "summary", "figure", "figcaption",
	)
	// 允许安全的属性（不含 style，防止内联样式注入）
	markdownPolicy.AllowAttrs("class", "id", "href", "src", "alt", "title", "lang", "dir",
		"target", "rel", "type").OnElements("a", "img", "kbd", "mark", "span", "pre", "code", "p")

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

// SanitizeUserContent 全面净化用户输入：白名单放行安全标签（颜色/折叠/跳转/按钮），
// 阻止任何 XSS。这是所有用户输入（评论、帖子、回复、章节等）都应调用的统一入口。
// 处理顺序：
//   1) 应用管理员配置的动态 XSS 正则规则；
//   2) 转义白名单外标签（script/iframe 等 → &lt;script&gt; 完整显示，不执行不残缺）；
//   3) 白名单标签属性过滤（移除事件处理器、危险协议、style 危险 CSS）。
// 注意：不使用 bluemonday 处理整体内容，避免误转义 Markdown 数学符号（$a < b$）。
func SanitizeUserContent(content string) string {
	content = ApplyDynamicXssRules(content)
	content = escapeUnsafeTags(content)
	return sanitizeSafeTagAttrs(content)
}

// SanitizePlainText 纯文本净化（用于不需要 HTML 的字段）
func SanitizePlainText(text string) string {
	return strictPolicy.Sanitize(text)
}

// ═══ HTML 标签转义（完整显示，不删除）═══

var (
	// htmlTagRe 匹配真正的 HTML 标签（<tag ...> 或 </tag>），用于转义
	htmlTagRe = regexp.MustCompile(`</?[a-zA-Z][^>]*>`)
	// codeBlockRe 匹配 Markdown 代码块（``` ```）
	codeBlockRe = regexp.MustCompile("(?s)```.*?```")
	// inlineCodeRe 匹配行内代码（` `）
	inlineCodeRe = regexp.MustCompile("`[^`\n]+`")
	// tagNameRe 提取标签名（<tag 或 </tag）
	tagNameRe = regexp.MustCompile(`(?i)^</?([a-zA-Z][a-zA-Z0-9]*)`)
)

// safeHTMLTags 允许渲染的 HTML 标签白名单（颜色/折叠/跳转/按钮 + Markdown 标准）。
// 白名单外的标签（script/iframe/object/svg/input 等）会被转义为纯文本完整显示。
var safeHTMLTags = map[string]bool{
	"span": true, "font": true, "details": true, "summary": true, "div": true,
	"a": true, "button": true, "img": true,
	"p": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"ul": true, "ol": true, "li": true, "dl": true, "dt": true, "dd": true,
	"table": true, "thead": true, "tbody": true, "tfoot": true, "tr": true, "th": true, "td": true,
	"blockquote": true, "pre": true, "code": true, "br": true, "hr": true,
	"b": true, "i": true, "u": true, "s": true, "strong": true, "em": true, "del": true, "ins": true,
	"mark": true, "sup": true, "sub": true, "kbd": true,
	"figure": true, "figcaption": true,
}

// EscapeHTMLTags 转义用户内容中的 HTML 标签，保留完整文本（不删除标签）。
// 只转义真正的 HTML 标签，不误伤数学符号（$a < b$）与自动链接；
// 代码块（``` ```）与行内代码（` `）会被保护，避免转义其中的内容。
func EscapeHTMLTags(content string) string {
	if content == "" {
		return content
	}
	// 1) 保护代码块与行内代码
	blocks := make([]string, 0, 8)
	content = codeBlockRe.ReplaceAllStringFunc(content, func(m string) string {
		blocks = append(blocks, m)
		return "\uE000" + strconv.Itoa(len(blocks)-1) + "\uE001"
	})
	content = inlineCodeRe.ReplaceAllStringFunc(content, func(m string) string {
		blocks = append(blocks, m)
		return "\uE000" + strconv.Itoa(len(blocks)-1) + "\uE001"
	})
	// 2) 转义内联 HTML 标签
	content = htmlTagRe.ReplaceAllStringFunc(content, func(tag string) string {
		return strings.ReplaceAll(strings.ReplaceAll(tag, "<", "&lt;"), ">", "&gt;")
	})
	// 3) 还原受保护块
	for i, b := range blocks {
		content = strings.ReplaceAll(content, "\uE000"+strconv.Itoa(i)+"\uE001", b)
	}
	return content
}

// escapeUnsafeTags 转义非白名单标签（script/iframe/object/svg/input 等），保留白名单标签。
// 白名单标签（颜色/折叠/跳转/按钮 + Markdown 标准）保留原样，后续由 bluemonday 过滤属性；
// 白名单外标签转义为 &lt;tag&gt; 纯文本完整显示（既不执行、也不残缺）。
func escapeUnsafeTags(content string) string {
	if content == "" {
		return content
	}
	// 保护代码块与行内代码
	blocks := make([]string, 0, 8)
	content = codeBlockRe.ReplaceAllStringFunc(content, func(m string) string {
		blocks = append(blocks, m)
		return "\uE000" + strconv.Itoa(len(blocks)-1) + "\uE001"
	})
	content = inlineCodeRe.ReplaceAllStringFunc(content, func(m string) string {
		blocks = append(blocks, m)
		return "\uE000" + strconv.Itoa(len(blocks)-1) + "\uE001"
	})
	// 转义非白名单标签，保留白名单标签
	content = htmlTagRe.ReplaceAllStringFunc(content, func(tag string) string {
		m := tagNameRe.FindStringSubmatch(tag)
		if m == nil {
			return tag
		}
		if safeHTMLTags[strings.ToLower(m[1])] {
			return tag // 白名单，保留
		}
		return strings.ReplaceAll(strings.ReplaceAll(tag, "<", "&lt;"), ">", "&gt;")
	})
	// 还原受保护块
	for i, b := range blocks {
		content = strings.ReplaceAll(content, "\uE000"+strconv.Itoa(i)+"\uE001", b)
	}
	return content
}

// ═══ 白名单标签属性过滤（自定义正则，避免误转义 Markdown 数学符号）═══

var (
	// onEventAttrRe 匹配事件属性 on*="..."（onclick/onerror/onload 等）
	onEventAttrRe = regexp.MustCompile(`(?i)\s+on[a-z0-9_-]*\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
	// urlAttrRe 匹配 href/src 属性
	urlAttrRe = regexp.MustCompile(`(?i)\s+(href|src)\s*=\s*("([^"]*)"|'([^']*)'|([^\s>]+))`)
	// styleAttrRe 匹配 style 属性
	styleAttrRe = regexp.MustCompile(`(?i)\s+style\s*=\s*("([^"]*)"|'([^']*)')`)
	// unsafeURLRe 危险协议（javascript/vbscript/data）
	unsafeURLRe = regexp.MustCompile(`(?i)^\s*(javascript|vbscript|data)\s*:`)
)

// sanitizeSafeTagAttrs 对白名单标签做属性过滤：
// 移除事件处理器（on*）、危险协议（javascript:/data:）、style 危险 CSS（仅留颜色）。
func sanitizeSafeTagAttrs(content string) string {
	if content == "" {
		return content
	}
	return htmlTagRe.ReplaceAllStringFunc(content, sanitizeTagAttrs)
}

func sanitizeTagAttrs(tag string) string {
	// 1) 移除事件属性
	tag = onEventAttrRe.ReplaceAllString(tag, "")
	// 2) 校验 href/src 协议，危险协议移除整个属性
	tag = urlAttrRe.ReplaceAllStringFunc(tag, func(m string) string {
		sub := urlAttrRe.FindStringSubmatch(m)
		if len(sub) < 6 {
			return m
		}
		value := sub[3]
		if value == "" {
			value = sub[4]
		}
		if value == "" {
			value = sub[5]
		}
		if unsafeURLRe.MatchString(value) {
			return "" // 危险协议，移除属性
		}
		return m
	})
	// 3) style 只保留颜色相关 CSS
	tag = styleAttrRe.ReplaceAllStringFunc(tag, func(m string) string {
		sub := styleAttrRe.FindStringSubmatch(m)
		if len(sub) < 4 {
			return ""
		}
		value := sub[2]
		if value == "" {
			value = sub[3]
		}
		cleaned := cleanStyle(value)
		if cleaned == "" {
			return "" // 无合法 CSS，移除 style
		}
		return ` style="` + cleaned + `"`
	})
	return tag
}

// cleanStyle 只保留 color / background-color 声明，其余 CSS（position/url/expression 等）移除
func cleanStyle(style string) string {
	parts := strings.Split(style, ";")
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		lower := strings.ToLower(p)
		// 只保留 color / background-color（值禁止 url()/expression）
		if strings.HasPrefix(lower, "color") || strings.HasPrefix(lower, "background-color") {
			if strings.Contains(lower, "url(") || strings.Contains(lower, "expression") {
				continue
			}
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, "; ")
}

// ═══ 动态 XSS 正则规则（管理员可在后台维护，无需改代码）═══

// DynamicXssRule 一条可配置的 XSS 防御规则（由 models/handler 注入）
type DynamicXssRule struct {
	ID          uint
	Pattern     string // 正则表达式（Go RE2 语法）
	Action      string // escape（转义，默认）/ replace（替换）/ delete（删除）
	Replacement string // 仅 replace 动作时使用的替换文本
}

type compiledXssRule struct {
	id      uint
	re      *regexp.Regexp
	action  string
	replace string
}

var (
	dynamicRulesMu sync.RWMutex
	dynamicRules   []compiledXssRule
)

// SetDynamicXssRules 设置动态 XSS 规则集（编译并缓存，无效正则自动跳过）
func SetDynamicXssRules(rules []DynamicXssRule) {
	compiled := make([]compiledXssRule, 0, len(rules))
	for _, r := range rules {
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			continue // 跳过无效正则，不影响其他规则
		}
		action := r.Action
		if action == "" {
			action = "escape"
		}
		compiled = append(compiled, compiledXssRule{id: r.ID, re: re, action: action, replace: r.Replacement})
	}
	dynamicRulesMu.Lock()
	dynamicRules = compiled
	dynamicRulesMu.Unlock()
}

// ApplyDynamicXssRules 应用管理员配置的 XSS 正则规则。
// 默认动作 escape：转义匹配片段中的 HTML 标签，保留完整文本（不缺胳膊少腿）；
// 可选动作 replace（替换为 Replacement）/ delete（删除匹配内容）。
// 正文超过 MaxScanBytes 时仅处理前 1MB，尾部原样拼回（资源耗尽防护，见 userpattern.go）。
func ApplyDynamicXssRules(content string) string {
	scanned, truncated := TruncateForScan(content)
	dynamicRulesMu.RLock()
	rules := dynamicRules
	dynamicRulesMu.RUnlock()
	if len(rules) == 0 || content == "" {
		return content
	}
	result := scanned
	for _, r := range rules {
		switch r.action {
		case "replace":
			result = r.re.ReplaceAllString(result, r.replace)
		case "delete":
			result = r.re.ReplaceAllString(result, "")
		default: // escape：转义匹配到的 HTML 标签，保留完整文本
			result = r.re.ReplaceAllStringFunc(result, func(match string) string {
				return htmlTagRe.ReplaceAllStringFunc(match, func(tag string) string {
					return strings.ReplaceAll(strings.ReplaceAll(tag, "<", "&lt;"), ">", "&gt;")
				})
			})
		}
	}
	if truncated {
		result += content[MaxScanBytes:]
	}
	return result
}

// NowString 返回当前时间的格式化字符串
func NowString() string {
	return time.Now().Format(time.RFC3339)
}
