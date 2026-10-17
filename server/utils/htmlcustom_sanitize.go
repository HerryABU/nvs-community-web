// htmlcustom_sanitize.go — 自定义主页 HTML 片段消毒（alist 式「自定义主页」作者公开页专用）。
//
// 设计取舍（务必先读，再改）：
//   - 与 security.SanitizeUserContent（正文，会剥离 <style>/div 等布局标签，过于激进）不同，
//     自定义主页需要用户自由排布版式、写 CSS，故采用「保留结构、剥离脚本」的温和策略：
//       · 允许 <style> 内联样式 + 布局/排版/颜色等安全标签（div/section/table/img/a ...）；
//       · 剥离所有可执行/危险标签：script / iframe / object / embed / svg / link / meta /
//         base / form / input / button / textarea / select / audio / video / canvas ...；
//       · 剥离事件处理器（on*）、危险协议（javascript:/vbscript:/data:）、危险 CSS；
//       · <style> 块内 CSS 走 utils.SanitizeCSS 白名单消毒（布局/字体/动画/变换均可，禁 url() 外链）。
//   - 「消毒但不能太过」：保留观感（布局、颜色、字体、动画），只去危险执行通道。
//   - 信任模型：即便经本消毒，作者公开页内容仍可被技术用户构造钓鱼/误导版式，故叠加
//     站长/管理员审核队列（AuthorHomeCustomize.ReviewStatus），双重保险。
//   - 不做 innerHTML 注入：前端以容器 innerHTML 注入时，浏览器仍会执行残留内容，
//     因此消毒必须在本函数彻底完成（不依赖前端再处理）。
package utils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// MaxCustomHTMLLength 自定义主页 HTML 片段长度上限（256KB），防超长输入拖垮解析/入库。
const MaxCustomHTMLLength = 256 * 1024

var (
	// 抽取 <style>...</style>（大小写不敏感），内部 CSS 另行走 SanitizeCSS
	styleBlockRe = regexp.MustCompile(`(?i)<style[^>]*>([\s\S]*?)</style>`)
	// 整体移除 <script>...</script>（含内容），杜绝脚本执行（自定义主页禁止脚本）
	scriptBlockRe = regexp.MustCompile(`(?i)<script[^>]*>[\s\S]*?</script>`)
	// 还原占位符 \x00STYLE{n}\x00
	stylePlaceholderRe = regexp.MustCompile(`\x00STYLE(\d+)\x00`)
)

// 允许的标签（安全布局/排版/媒体标记；无脚本/表单/嵌入执行能力）。
// 注意：style 已被前置抽取，理论上不会走到这里；作为兜底，style 归入 danger 处理。
var customAllowedTags = map[string]bool{
	"div": true, "span": true, "section": true, "article": true, "header": true,
	"footer": true, "main": true, "aside": true, "nav": true, "p": true, "br": true,
	"hr": true, "details": true, "summary": true, "blockquote": true, "pre": true,
	"code": true, "figure": true, "figcaption": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"ul": true, "ol": true, "li": true, "dl": true, "dt": true, "dd": true,
	"table": true, "thead": true, "tbody": true, "tfoot": true, "tr": true,
	"th": true, "td": true, "caption": true, "colgroup": true, "col": true,
	"b": true, "i": true, "u": true, "s": true, "strong": true, "em": true,
	"small": true, "sub": true, "sup": true, "mark": true, "kbd": true, "q": true,
	"cite": true, "abbr": true, "time": true, "del": true, "ins": true,
	"a": true, "img": true, "picture": true, "ruby": true, "rt": true, "rp": true,
}

// 危险标签（整体移除标签，保留其文本子节点交给外层继续扫描；script 内容随标签一并丢弃）。
var customDangerTags = map[string]bool{
	"script": true, "iframe": true, "object": true, "embed": true, "svg": true,
	"link": true, "meta": true, "base": true, "form": true, "input": true,
	"button": true, "textarea": true, "select": true, "option": true, "label": true,
	"fieldset": true, "legend": true, "noscript": true, "template": true, "math": true,
	"audio": true, "video": true, "source": true, "track": true, "canvas": true,
	"map": true, "area": true, "frame": true, "frameset": true, "applet": true,
	"portal": true, "marquee": true, "blink": true, "style": true,
}

// 允许的（非 URL / 非 style）属性白名单。
var customAllowedAttrs = map[string]bool{
	"class": true, "id": true, "title": true, "lang": true, "dir": true,
	"width": true, "height": true, "align": true, "target": true, "rel": true,
	"type": true, "alt": true, "name": true, "value": true, "placeholder": true,
	"colspan": true, "rowspan": true, "scope": true, "controls": true,
	"autoplay": true, "loop": true, "muted": true, "preload": true,
	"cite": true, "datetime": true, "download": true, "hreflang": true,
	"loading": true, "referrerpolicy": true, "start": true, "reversed": true,
}

// SanitizeCustomHTML 对自定义主页 HTML 片段做「保留结构、剥离脚本」式消毒。
// 返回 (消毒后 HTML, 被移除项原因列表)；removed 恒非 nil，便于 JSON 序列化为 []。
func SanitizeCustomHTML(html string) (string, []string) {
	removed := []string{}
	if len(html) > MaxCustomHTMLLength {
		return "", append(removed, "自定义主页 HTML 超过 256KB 上限，已整体拒绝")
	}
	if html == "" {
		return "", removed
	}

	// 0) 整体移除 <script>...</script>（含内容），杜绝脚本执行（自定义主页禁止脚本）
	html = scriptBlockRe.ReplaceAllString(html, "")

	// 1) 抽取 <style> 块，消毒后待用，原位置替换为占位符
	var styles []string
	html = styleBlockRe.ReplaceAllStringFunc(html, func(m string) string {
		sub := styleBlockRe.FindStringSubmatch(m)
		if len(sub) < 2 {
			return ""
		}
		sanitized, rs := SanitizeCSS(sub[1])
		removed = append(removed, rs...)
		if sanitized == "" {
			removed = append(removed, "style 块无有效 CSS，已移除")
			return ""
		}
		idx := len(styles)
		styles = append(styles, "<style>\n"+sanitized+"\n</style>")
		return fmt.Sprintf("\x00STYLE%d\x00", idx)
	})

	// 2) 扫描并处理其余标签
	out := sanitizeCustomTags(html, &removed)

	// 3) 还原 style 占位符
	out = stylePlaceholderRe.ReplaceAllStringFunc(out, func(m string) string {
		sub := stylePlaceholderRe.FindStringSubmatch(m)
		if len(sub) < 2 {
			return ""
		}
		idx, err := strconv.Atoi(sub[1])
		if err != nil || idx < 0 || idx >= len(styles) {
			return ""
		}
		return styles[idx]
	})

	return out, removed
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f'
}

// scanTagEnd 从 '<'（下标 i）扫描到 '>'，字符串字面量内的 '>' 不生效；未闭合返回 -1。
func scanTagEnd(s string, i int) int {
	j := i + 1
	for j < len(s) {
		c := s[j]
		if c == '"' || c == '\'' {
			q := c
			j++
			for j < len(s) && s[j] != q {
				j++
			}
			if j < len(s) {
				j++ // 跳过闭引号
			}
			continue
		}
		if c == '>' {
			return j
		}
		j++
	}
	return -1
}

// sanitizeCustomTags 逐标签扫描：危险标签整体移除；允许标签消毒属性；未知标签移除标签保留子节点。
func sanitizeCustomTags(s string, removed *[]string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	n := len(s)
	for i < n {
		if s[i] != '<' {
			b.WriteByte(s[i])
			i++
			continue
		}
		end := scanTagEnd(s, i)
		if end < 0 {
			// 无闭合 '>'：剩余当作纯文本原样写出
			b.WriteString(s[i:])
			break
		}
		tag := s[i : end+1]
		b.WriteString(sanitizeOneCustomTag(tag, removed))
		i = end + 1
	}
	return b.String()
}

func sanitizeOneCustomTag(tag string, removed *[]string) string {
	if len(tag) < 3 {
		return ""
	}
	closing := false
	inner := tag[1 : len(tag)-1]
	if tag[1] == '/' {
		closing = true
		inner = tag[2 : len(tag)-1]
	}

	// 提取标签名（遇空白或自闭合斜杠停止）
	nameEnd := 0
	for nameEnd < len(inner) && !isSpace(inner[nameEnd]) {
		if inner[nameEnd] == '/' {
			break
		}
		nameEnd++
	}
	name := strings.ToLower(inner[:nameEnd])

	if closing {
		if customAllowedTags[name] {
			return tag
		}
		return "" // 移除未知/危险闭合标签
	}

	if name == "script" {
		*removed = append(*removed, "已移除 <script> 及其内容（自定义主页禁止脚本执行）")
		return ""
	}
	if !customAllowedTags[name] {
		if customDangerTags[name] {
			*removed = append(*removed, "已移除危险标签 <"+name+">")
		}
		return "" // 移除标签，保留子节点继续扫描
	}

	// 允许标签：消毒属性
	attrsPart := inner[nameEnd:]
	attrsPart = strings.TrimRight(attrsPart, " \t\n\r")
	if strings.HasSuffix(attrsPart, "/") {
		// 自闭合标记：仅影响 void 元素输出形态，这里统一输出为普通开始标签
		attrsPart = strings.TrimRight(attrsPart[:len(attrsPart)-1], " \t\n\r")
	}
	cleaned := sanitizeCustomAttrs(attrsPart, removed)
	if cleaned == "" {
		return "<" + name + ">"
	}
	return "<" + name + " " + cleaned + ">"
}

// sanitizeCustomAttrs 解析并消毒属性（字符串字面量感知），返回空格分隔的保留属性串。
func sanitizeCustomAttrs(s string, removed *[]string) string {
	var kept []string
	i := 0
	n := len(s)
	for i < n {
		for i < n && isSpace(s[i]) {
			i++
		}
		if i >= n {
			break
		}
		start := i
		for i < n && !isSpace(s[i]) && s[i] != '=' && s[i] != '>' {
			i++
		}
		name := s[start:i]
		nameLower := strings.ToLower(name)

		if i < n && s[i] == '=' {
			i++
			for i < n && isSpace(s[i]) {
				i++
			}
			var val string
			if i < n && (s[i] == '"' || s[i] == '\'') {
				q := s[i]
				j := i + 1
				if endq := strings.IndexByte(s[j:], q); endq >= 0 {
					val = s[j : j+endq]
					i = j + endq + 1
				} else {
					val = s[i+1:]
					i = n
				}
			} else {
				vs := i
				for i < n && !isSpace(s[i]) && s[i] != '>' {
					i++
				}
				val = s[vs:i]
			}
			if clean, ok := sanitizeCustomAttr(nameLower, val); ok {
				kept = append(kept, name+"=\""+clean+"\"")
			} else {
				*removed = append(*removed, "已移除属性 "+name+"（不安全）")
			}
		} else {
			// 无值属性（布尔属性）
			if nameLower == "checked" || nameLower == "disabled" ||
				nameLower == "selected" || nameLower == "open" ||
				nameLower == "controls" || nameLower == "autoplay" ||
				nameLower == "loop" || nameLower == "muted" || nameLower == "reversed" {
				kept = append(kept, name)
			} else {
				*removed = append(*removed, "已移除属性 "+name+"（不允许）")
			}
		}
	}
	return strings.Join(kept, " ")
}

// sanitizeCustomAttr 单属性决策：返回 (消毒后值, 是否保留)。
func sanitizeCustomAttr(name, val string) (string, bool) {
	// 事件处理器：on* 一律拒绝
	if len(name) > 2 && name[0] == 'o' && name[1] == 'n' && isLetter(name[2]) {
		return "", false
	}
	switch name {
	case "style":
		clean := sanitizeInlineStyle(val)
		if clean == "" {
			return "", false
		}
		return clean, true
	case "href", "src", "poster", "cite", "background", "formaction":
		if !safeCustomURL(val) {
			return "", false
		}
		return val, true
	}
	if customAllowedAttrs[name] {
		return val, true
	}
	return "", false
}

func isLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// safeCustomURL 仅允许安全协议 / 相对路径 / data:image；拒绝 javascript:/vbscript:/data:(非图片)。
func safeCustomURL(val string) bool {
	v := strings.TrimSpace(strings.ToLower(val))
	if v == "" || v == "#" {
		return true
	}
	if strings.HasPrefix(v, "data:") {
		return strings.HasPrefix(v, "data:image/")
	}
	if strings.HasPrefix(v, "javascript:") || strings.HasPrefix(v, "vbscript:") || strings.HasPrefix(v, "about:") {
		return false
	}
	if strings.Contains(v, ":") {
		for _, p := range []string{"http:", "https:", "ftp:", "mailto:", "tel:"} {
			if strings.HasPrefix(v, p) {
				return true
			}
		}
		return false
	}
	return true // 相对路径
}

// sanitizeInlineStyle 消毒行内 style：复用 SanitizeCSS 的属性白名单与值黑名单。
func sanitizeInlineStyle(style string) string {
	parts := strings.Split(style, ";")
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		colon := strings.Index(p, ":")
		if colon <= 0 {
			continue
		}
		prop := strings.TrimSpace(p[:colon])
		val := strings.TrimSpace(p[colon+1:])
		if !allowedProperty(strings.ToLower(prop)) {
			continue
		}
		if val == "" {
			continue
		}
		if bad, _ := dangerousValue(val); bad {
			continue
		}
		kept = append(kept, prop+": "+val)
	}
	return strings.Join(kept, "; ")
}
