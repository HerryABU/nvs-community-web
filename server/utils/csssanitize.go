// csssanitize.go — 用户自定义主题 CSS 的白名单式消毒（F12）。
//
// 安全模型（alist 式自定义主题，XSS 是红线）：
//  1. 先剥离全部 CSS 注释再切分——注释是经典混淆载体（ex/**/pression、@imp/**/ort），
//     剥离后混淆 token 会拼回原形，随后的黑名单校验即可识别；
//  2. 结构化解析（规则块/声明块逐层切分，字符串感知），不是几个正则了事；
//  3. @ 规则白名单：仅保留 @media / @keyframes（含 -webkit- 等前缀变体），
//     其余（@import/@font-face/@supports/@charset/@namespace/@document…）一律移除；
//  4. 声明级双重校验：属性名白名单 + 值黑名单（url(/expression(/javascript:/
//     -moz-binding/progid:/反斜杠转义/＜ 一律拒绝）；
//  5. 被移除的每条规则记录原因返回给前端提示用户。
package utils

import (
	"strings"
	"unicode"
)

// MaxCSSLength 主题 CSS 长度上限（128KB）。超出直接整体拒绝，
// 防超长输入拖垮解析（解析为 O(n) 线性扫描，但入库体积必须受控）。
const MaxCSSLength = 128 * 1024

// SanitizeCSS 对用户提交的主题 CSS 做白名单式消毒。
// 返回消毒后的 CSS 与被移除的规则列表（removed 恒非 nil，便于 JSON 序列化为 []）。
func SanitizeCSS(css string) (string, []string) {
	removed := []string{}
	if len(css) > MaxCSSLength {
		return "", append(removed, "CSS 长度超过 128KB 上限 已整体移除")
	}
	s := &cssSanitizer{removed: removed}
	return s.sanitizeRuleList(stripCSSComments(css), false), s.removed
}

// cssSanitizer 消毒状态机（removed 累积被拒规则的原因）
type cssSanitizer struct{ removed []string }

// drop 记录一条被移除的规则（截断超长原因，避免回显爆炸）
func (s *cssSanitizer) drop(reason string) {
	reason = strings.Join(strings.Fields(reason), " ")
	if reason == "" {
		reason = "（空规则）"
	}
	if len(reason) > 80 {
		reason = reason[:80] + "…"
	}
	s.removed = append(s.removed, reason+" 已移除")
}

// ---------------- 注释剥离（第一道防线） ----------------

// stripCSSComments 剥离全部 /* */ 注释后再进入解析。
// 注意方向：先剥离会使 ex/**/pression 重新拼回 expression 从而被识别，
// 而不是把攻击串打散——这是防注释混淆的正确顺序。
func stripCSSComments(css string) string {
	if !strings.Contains(css, "/*") {
		return css
	}
	var b strings.Builder
	b.Grow(len(css))
	for i := 0; i < len(css); {
		if css[i] == '/' && i+1 < len(css) && css[i+1] == '*' {
			end := strings.Index(css[i+2:], "*/")
			if end < 0 {
				break // 未闭合注释：剩余内容整体丢弃
			}
			i += 2 + end + 2
			continue
		}
		b.WriteByte(css[i])
		i++
	}
	return b.String()
}

// ---------------- 结构化扫描原语（字符串感知） ----------------

func isWS(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f'
}

func skipWS(s string, i int) int {
	for i < len(s) && isWS(s[i]) {
		i++
	}
	return i
}

// skipString 返回字符串字面量（"..." / '...'）结束后的下标；未闭合则到文末。
func skipString(s string, i int) int {
	q := s[i]
	i++
	for i < len(s) {
		if s[i] == '\\' {
			i += 2
			continue
		}
		if s[i] == q {
			return i + 1
		}
		i++
	}
	return len(s)
}

// indexStructural 查找首个不在字符串内的目标字符；未找到返回 -1。
func indexStructural(s string, i int, targets string) int {
	for i < len(s) {
		if c := s[i]; c == '"' || c == '\'' {
			i = skipString(s, i)
			continue
		} else if strings.IndexByte(targets, c) >= 0 {
			return i
		}
		i++
	}
	return -1
}

// matchBrace 返回与 openIdx 处 '{' 配对的 '}' 下标；未闭合返回 -1。
func matchBrace(s string, openIdx int) int {
	depth := 0
	for i := openIdx; i < len(s); {
		if c := s[i]; c == '"' || c == '\'' {
			i = skipString(s, i)
			continue
		} else if c == '{' {
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				return i
			}
		}
		i++
	}
	return -1
}

// splitStructural 按分隔字节切分（字符串字面量内的分隔符不生效）。
func splitStructural(s string, sep byte) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); {
		if c := s[i]; c == '"' || c == '\'' {
			i = skipString(s, i)
			continue
		} else if c == sep {
			parts = append(parts, s[start:i])
			start = i + 1
		}
		i++
	}
	return append(parts, s[start:])
}

// ---------------- 规则列表层（顶层 / @media 内部 / @keyframes 内部） ----------------

// sanitizeRuleList 消毒一组规则（选择器{声明} 或 @ 规则）。
// keyframeCtx=true 时选择器仅允许 from/to/百分比（@keyframes 内部）。
func (s *cssSanitizer) sanitizeRuleList(body string, keyframeCtx bool) string {
	var out strings.Builder
	i := skipWS(body, 0)
	for i < len(body) {
		switch body[i] {
		case '}':
			i++ // 容忍孤立的右花括号
			continue
		case '@':
			block, next := s.sanitizeAtRule(body, i)
			if block != "" {
				out.WriteString(block)
				out.WriteByte('\n')
			}
			i = skipWS(body, next)
			continue
		}
		brace := indexStructural(body, i, "{")
		if brace < 0 {
			if rest := strings.TrimSpace(body[i:]); rest != "" {
				s.drop(rest + "（缺少声明块）")
			}
			break
		}
		end := matchBrace(body, brace)
		if end < 0 {
			s.drop(strings.TrimSpace(body[i:brace]) + " { …（规则块未闭合）")
			break
		}
		sel := strings.TrimSpace(body[i:brace])
		valid := validSelector(sel)
		if valid && keyframeCtx {
			valid = validKeyframeSelector(sel)
		}
		var decls string
		switch {
		case !valid:
			s.drop(sel + "（选择器非法）")
		default:
			decls = s.sanitizeDeclarations(body[brace+1 : end])
			if decls == "" {
				s.drop(sel + "（无有效声明）")
			}
		}
		if decls != "" {
			out.WriteString(sel + " {\n" + decls + "}\n")
		}
		i = skipWS(body, end+1)
	}
	return out.String()
}

// sanitizeAtRule 处理 @ 规则，返回 (保留的输出块, 下一个扫描位置)。
// 白名单仅 @media / @keyframes（含供应商前缀）；其余整条移除。
func (s *cssSanitizer) sanitizeAtRule(body string, i int) (string, int) {
	j := i + 1
	for j < len(body) && isNameByte(body[j]) {
		j++
	}
	name := strings.ToLower(body[i:j]) // @media / @import / @-webkit-keyframes …
	brace := indexStructural(body, j, "{")
	semi := indexStructural(body, j, ";")

	isKeyframes := strings.HasSuffix(name, "keyframes")
	if name != "@media" && !isKeyframes {
		// 白名单之外的 @ 规则：语句型（分号结尾，如 @import/@charset/@namespace）
		// 移除至分号；块型（如 @font-face/@supports/@document）整块移除。
		// 注意优先用更近的分号，避免误吞后面无关的规则块。
		if semi >= 0 && (brace < 0 || semi < brace) {
			s.drop(strings.TrimSpace(body[i:semi]))
			return "", semi + 1
		}
		if brace >= 0 {
			if end := matchBrace(body, brace); end >= 0 {
				s.drop(name)
				return "", end + 1
			}
			s.drop(name)
			return "", len(body)
		}
		s.drop(name)
		return "", len(body)
	}

	// @media / @keyframes：分号出现在块之前属于畸形输入，按语句型移除
	if semi >= 0 && semi < brace {
		s.drop(strings.TrimSpace(body[i:semi]))
		return "", semi + 1
	}
	// @media / @keyframes：必须有完整块，且前导条件不得含非法字符
	if brace < 0 {
		s.drop(name + "（规则块未闭合）")
		return "", len(body)
	}
	prelude := strings.TrimSpace(body[j:brace])
	end := matchBrace(body, brace)
	if end < 0 {
		s.drop(name + "（规则块未闭合）")
		return "", len(body)
	}
	if prelude == "" || !validAtPrelude(prelude) {
		s.drop(name + " " + prelude + "（条件非法）")
		return "", end + 1
	}
	inner := s.sanitizeRuleList(body[brace+1:end], isKeyframes)
	if inner == "" {
		s.drop(name + " " + prelude + "（内部无有效规则）")
		return "", end + 1
	}
	return name + " " + prelude + " {\n" + inner + "}\n", end + 1
}

// ---------------- 声明层 ----------------

// sanitizeDeclarations 消毒一个声明块，返回 "  prop: value;\n…" 形式（无有效声明返回 ""）。
func (s *cssSanitizer) sanitizeDeclarations(block string) string {
	var out strings.Builder
	for _, part := range splitStructural(block, ';') {
		decl := strings.TrimSpace(part)
		if decl == "" {
			continue
		}
		colon := indexStructural(decl, 0, ":")
		if colon <= 0 {
			s.drop(decl + "（缺少冒号）")
			continue
		}
		prop := strings.TrimSpace(decl[:colon])
		value := strings.TrimSpace(decl[colon+1:])
		if !allowedProperty(strings.ToLower(prop)) {
			s.drop(prop + ": " + value + "（属性不在白名单）")
			continue
		}
		if value == "" {
			s.drop(prop + "（缺少值）")
			continue
		}
		if bad, why := dangerousValue(value); bad {
			s.drop(prop + ": " + value + "（" + why + "）")
			continue
		}
		out.WriteString("  " + prop + ": " + value + ";\n")
	}
	return out.String()
}

// dangerousValue 值级黑名单：外链资源 / 脚本执行 / IE 行为 / 转义混淆。
func dangerousValue(v string) (bool, string) {
	low := strings.ToLower(v)
	switch {
	case strings.Contains(low, "url("):
		return true, "不允许 url() 外链资源"
	case strings.Contains(low, "expression("):
		return true, "不允许 expression()"
	case strings.Contains(low, "javascript:"), strings.Contains(low, "vbscript:"):
		return true, "不允许脚本协议"
	case strings.Contains(low, "-moz-binding"):
		return true, "不允许 -moz-binding"
	case strings.Contains(low, "progid:"):
		return true, "不允许 IE 滤镜 progid"
	case strings.Contains(v, "\\"):
		return true, "不允许反斜杠转义"
	case strings.Contains(v, "<"):
		return true, "不允许 < 字符"
	}
	return false, ""
}

// ---------------- 字符白名单校验 ----------------

func isNameByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '-'
}

// validSelector 选择器字符白名单：标识符/类/伪类/属性/组合符/百分比(keyframes)。
// 拒绝 < \ { } ; @ & 等（防注入与 CSS 转义混淆）。
func validSelector(sel string) bool {
	if sel == "" {
		return false
	}
	for _, r := range sel {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
		case strings.ContainsRune("-_.#:%*,>+~[]=\"' \t\n\r()", r):
		default:
			return false
		}
	}
	return true
}

// validKeyframeSelector @keyframes 内部仅允许 from / to / 百分比（可逗号组合）。
func validKeyframeSelector(sel string) bool {
	for _, r := range sel {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
		case strings.ContainsRune("%, \t\n\r", r):
		default:
			return false
		}
	}
	return true
}

// validAtPrelude @media 条件字符白名单（查询语法无需引号外的更多字符）。
func validAtPrelude(pre string) bool {
	for _, r := range pre {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
		case strings.ContainsRune(" \t\n\r()-:,.'%", r):
		default:
			return false
		}
	}
	return true
}

// ---------------- 属性白名单 ----------------

// allowedCSSProperties 常见布局/颜色/字体/动画属性白名单（不含 behavior/-moz-binding）。
// 供应商前缀（-webkit-/-moz-/-ms-/-o-）在 allowedProperty 中剥壳后查本表。
var allowedCSSProperties = map[string]bool{
	// 布局与盒模型
	"display": true, "position": true, "top": true, "right": true, "bottom": true, "left": true,
	"inset": true, "inset-block": true, "inset-inline": true,
	"inset-block-start": true, "inset-block-end": true, "inset-inline-start": true, "inset-inline-end": true,
	"z-index": true, "float": true, "clear": true,
	"width": true, "min-width": true, "max-width": true, "height": true, "min-height": true, "max-height": true,
	"block-size": true, "inline-size": true, "min-block-size": true, "min-inline-size": true, "max-block-size": true, "max-inline-size": true,
	"margin": true, "margin-top": true, "margin-right": true, "margin-bottom": true, "margin-left": true,
	"margin-block": true, "margin-block-start": true, "margin-block-end": true,
	"margin-inline": true, "margin-inline-start": true, "margin-inline-end": true,
	"padding": true, "padding-top": true, "padding-right": true, "padding-bottom": true, "padding-left": true,
	"padding-block": true, "padding-block-start": true, "padding-block-end": true,
	"padding-inline": true, "padding-inline-start": true, "padding-inline-end": true,
	"border": true, "border-width": true, "border-style": true, "border-color": true,
	"border-top": true, "border-right": true, "border-bottom": true, "border-left": true,
	"border-top-width": true, "border-right-width": true, "border-bottom-width": true, "border-left-width": true,
	"border-top-style": true, "border-right-style": true, "border-bottom-style": true, "border-left-style": true,
	"border-top-color": true, "border-right-color": true, "border-bottom-color": true, "border-left-color": true,
	"border-block": true, "border-inline": true, "border-block-width": true, "border-inline-width": true,
	"border-block-style": true, "border-inline-style": true, "border-block-color": true, "border-inline-color": true,
	"border-radius":          true,
	"border-top-left-radius": true, "border-top-right-radius": true,
	"border-bottom-left-radius": true, "border-bottom-right-radius": true,
	"border-start-start-radius": true, "border-start-end-radius": true,
	"border-end-start-radius": true, "border-end-end-radius": true,
	"border-collapse": true, "border-spacing": true,
	"outline": true, "outline-width": true, "outline-style": true, "outline-color": true, "outline-offset": true,
	"box-sizing": true, "box-shadow": true,
	"overflow": true, "overflow-x": true, "overflow-y": true, "overflow-wrap": true, "overflow-clip-margin": true,
	"visibility": true, "clip": true, "clip-path": true, "aspect-ratio": true,
	"gap": true, "row-gap": true, "column-gap": true,
	"flex": true, "flex-grow": true, "flex-shrink": true, "flex-basis": true,
	"flex-direction": true, "flex-wrap": true, "flex-flow": true,
	"justify-content": true, "justify-items": true, "justify-self": true,
	"align-items": true, "align-content": true, "align-self": true,
	"place-items": true, "place-content": true, "place-self": true,
	"order": true, "grid": true, "grid-template": true, "grid-template-columns": true,
	"grid-template-rows": true, "grid-template-areas": true, "grid-auto-columns": true,
	"grid-auto-rows": true, "grid-auto-flow": true, "grid-column": true, "grid-row": true,
	"grid-column-start": true, "grid-column-end": true, "grid-row-start": true, "grid-row-end": true,
	"columns": true, "column-width": true, "column-count": true, "column-rule": true,
	"column-span": true, "column-fill": true,
	"object-fit": true, "object-position": true, "resize": true, "cursor": true,
	"pointer-events": true, "user-select": true, "appearance": true,
	"container-type": true, "container-name": true,
	// 颜色与背景（外链资源一律被值级 url() 黑名单拦截）
	"color": true, "opacity": true, "background": true, "background-color": true,
	"background-position": true, "background-size": true, "background-repeat": true,
	"background-origin": true, "background-clip": true, "background-attachment": true,
	"background-blend-mode": true, "mix-blend-mode": true, "isolation": true,
	"filter": true, "backdrop-filter": true, "accent-color": true, "caret-color": true, "color-scheme": true,
	"mask": true, "mask-image": true, "mask-size": true, "mask-repeat": true,
	"mask-position": true, "mask-clip": true, "mask-origin": true, "mask-mode": true,
	"mask-composite": true, "mask-type": true,
	// 文字
	"font": true, "font-family": true, "font-size": true, "font-size-adjust": true,
	"font-weight": true, "font-style": true, "font-variant": true, "font-stretch": true,
	"font-feature-settings": true, "font-kerning": true, "font-variant-numeric": true,
	"line-height": true, "letter-spacing": true, "word-spacing": true, "word-break": true,
	"white-space": true, "hyphens": true, "tab-size": true, "text-size-adjust": true,
	"text-align": true, "text-align-last": true, "text-decoration": true,
	"text-decoration-line": true, "text-decoration-color": true, "text-decoration-style": true,
	"text-decoration-thickness": true, "text-transform": true, "text-indent": true,
	"text-shadow": true, "text-overflow": true, "text-underline-offset": true,
	"text-underline-position": true, "text-rendering": true,
	"direction": true, "unicode-bidi": true, "writing-mode": true, "vertical-align": true,
	"list-style": true, "list-style-type": true, "list-style-position": true, "list-style-image": true,
	"content": true, "quotes": true, "counter-reset": true, "counter-increment": true, "counter-set": true,
	// 动画与变换
	"animation": true, "animation-name": true, "animation-duration": true,
	"animation-timing-function": true, "animation-delay": true, "animation-iteration-count": true,
	"animation-direction": true, "animation-fill-mode": true, "animation-play-state": true,
	"transition": true, "transition-property": true, "transition-duration": true,
	"transition-timing-function": true, "transition-delay": true,
	"transform": true, "transform-origin": true, "transform-style": true,
	"perspective": true, "perspective-origin": true, "backface-visibility": true,
	"will-change": true, "scroll-behavior": true,
	"overscroll-behavior": true, "overscroll-behavior-x": true, "overscroll-behavior-y": true,
	// 滚动与触摸
	"scroll-snap-type": true, "scroll-snap-align": true, "scroll-snap-stop": true,
	"scroll-margin": true, "scroll-margin-top": true, "scroll-margin-right": true,
	"scroll-margin-bottom": true, "scroll-margin-left": true,
	"scroll-padding": true, "scroll-padding-top": true, "scroll-padding-right": true,
	"scroll-padding-bottom": true, "scroll-padding-left": true,
	"scrollbar-width": true, "scrollbar-color": true, "scrollbar-gutter": true,
	"touch-action": true,
	// SVG 填充描边（引用外部资源的 fill="url(#…)" 由值级黑名单拦截）
	"fill": true, "fill-opacity": true, "fill-rule": true,
	"stroke": true, "stroke-width": true, "stroke-opacity": true, "stroke-linecap": true,
	"stroke-linejoin": true, "stroke-dasharray": true, "stroke-dashoffset": true, "stroke-miterlimit": true,
	"stop-color": true, "stop-opacity": true, "flood-color": true, "flood-opacity": true, "lighting-color": true,
	"shape-rendering": true, "vector-effect": true,
	// 其他无害属性
	"all": true, "image-rendering": true, "print-color-adjust": true,
	// 常见非标准属性（需供应商前缀剥壳后命中）
	"osx-font-smoothing": true, "font-smoothing": true, "overlay-scrolling": true,
	"tap-highlight-color": true, "text-fill-color": true,
	"text-stroke": true, "text-stroke-width": true, "text-stroke-color": true,
	"box-orient": true, "box-align": true, "box-pack": true, "box-flex": true,
	"line-clamp": true,
}

// allowedProperty 属性名校验：自定义属性（--x，Design Token 扩展）放行，
// 供应商前缀剥壳后查白名单；behaviour/-moz-binding 等自然落入拒绝。
func allowedProperty(prop string) bool {
	if prop == "" {
		return false
	}
	// 自定义属性：名字仅允许字母数字连字符，值仍走值级黑名单
	if strings.HasPrefix(prop, "--") {
		if len(prop) <= 2 {
			return false
		}
		for i := 0; i < len(prop); i++ {
			if !isNameByte(prop[i]) {
				return false
			}
		}
		return true
	}
	for i := 0; i < len(prop); i++ {
		if !isNameByte(prop[i]) {
			return false
		}
	}
	for _, p := range []string{"-webkit-", "-moz-", "-ms-", "-o-"} {
		if strings.HasPrefix(prop, p) {
			prop = prop[len(p):]
			break
		}
	}
	return allowedCSSProperties[prop]
}
