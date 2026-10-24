package scriptmode

// ==================== 渲染 ====================
// 纯话本：RenderHTML（script mode）
// 混排：RenderMixedHTML（对话/提示走气泡，其余段落走 Markdown——已由 markdown 包净化）

import (
	"html"
	"strings"

	"nvs-server/markdown"
)

// inline 行内渲染：转义 → 表情 → 换行
func inline(s string) string {
	return emoteSpan(strings.ReplaceAll(html.EscapeString(s), "\n", "<br>"))
}

// renderNodes 把节点序列渲染为 HTML
func renderNodes(nodes []Node) string {
	if len(nodes) == 0 {
		return `<div class="script-chapter"></div>`
	}
	var b strings.Builder
	b.WriteString(`<div class="script-chapter">`)
	for _, n := range nodes {
		switch n.Type {
		case NodeStage:
			b.WriteString(`<div class="script-stage">` + inline(n.Text) + `</div>`)
		case NodeNarration:
			b.WriteString(`<div class="script-narration">` + inline(n.Text) + `</div>`)
		case NodeProse:
			// 散文块：Markdown 渲染（内部已 SanitizeHTML）
			b.WriteString(`<div class="script-prose markdown-body">` + markdown.RenderMarkdown(n.Text) + `</div>`)
		case NodeImage:
			if !SafeURL(n.URL) {
				// 非法地址降级为纯文本提示，绝不拼进 src
				b.WriteString(`<div class="script-image-block sb-invalid">（图片地址不被允许）</div>`)
				continue
			}
			b.WriteString(`<div class="script-image-block">`)
			b.WriteString(`<img class="sb-image" src="` + html.EscapeString(n.URL) +
				`" alt="` + html.EscapeString(n.Alt) + `" loading="lazy" />`)
			if strings.TrimSpace(n.Alt) != "" {
				b.WriteString(`<div class="sb-image-caption">` + inline(n.Alt) + `</div>`)
			}
			b.WriteString(`</div>`)
		default:
			b.WriteString(`<div class="script-bubble sb-` + n.Side + `">`)
			b.WriteString(`<div class="sb-avatar">` + html.EscapeString(n.Initial) + `</div>`)
			b.WriteString(`<div class="sb-body">`)
			b.WriteString(`<div class="sb-name">` + html.EscapeString(n.Speaker) + `</div>`)
			b.WriteString(`<div class="sb-text">` + inline(n.Text) + `</div>`)
			b.WriteString(`</div></div>`)
		}
	}
	b.WriteString(`</div>`)
	return b.String()
}

// RenderHTML 纯话本渲染：所有文本转义后输出，表情与图片白名单处理
func RenderHTML(src string) string {
	return renderNodes(Parse(src))
}

// RenderMixedHTML 混排渲染：对话/舞台/旁白走气泡，其余连续段落走 Markdown
func RenderMixedHTML(src string) string {
	return renderNodes(ParseMixed(src))
}
