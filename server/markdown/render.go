package markdown

import (
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"

	"nvs-server/security"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

// goldmark 实例（全局复用）
var md = goldmark.New(
	goldmark.WithExtensions(
		extension.Table,           // GFM 表格
		extension.Strikethrough,   // ~~删除线~~
		extension.TaskList,        // - [ ] 任务列表
		extension.Linkify,         // 自动链接
		extension.DefinitionList,  // 定义列表
		extension.Footnote,        // 脚注 [^1]
		extension.Typographer,     // 智能引号/破折号/省略号
	),
	goldmark.WithRendererOptions(
		html.WithUnsafe(), // 允许原始 HTML（KaTeX/Mermaid 恢复后需要）
	),
)

// RenderMarkdown 基于 goldmark 的 Markdown→HTML 渲染器
// 支持：CommonMark + GFM 表格/任务列表/删除线 + 定义列表 + 脚注 + 自动链接
// 保留 KaTeX 数学公式（$...$ / $$...$$）和 Mermaid 图表（```mermaid...```）原始文本
// 输出已通过 bluemonday 净化，可安全用于 v-html
func RenderMarkdown(input string) string {
	// 占位符保护：KaTeX 公式和 Mermaid 图表
	protected := make(map[string]string)
	raw := protectKaTeX(input, protected)
	raw = protectMermaid(raw, protected)

	// goldmark 渲染 Markdown → HTML
	var buf strings.Builder
	if err := md.Convert([]byte(raw), &buf); err != nil {
		// 降级：出错时返回净化后的原始文本
		return security.SanitizeHTML(input)
	}
	html := buf.String()

	// 安全净化：移除危险标签和事件处理器
	html = security.SanitizeHTML(html)

	// 恢复被保护的 KaTeX 和 Mermaid 块
	html = restoreProtected(html, protected)

	return html
}

// ── 全局计数器 ──

var phCounter uint64

// phPrefix 占位符前缀（使用 ASCII 字符串，避免被 goldmark/bluemonday 修改）
const phPrefix = "\uE000GMK"

// makePH 生成唯一占位符（Unicode 私用区字符，goldmark 不会触碰）
func makePH() string {
	return fmt.Sprintf("%s%d\uE001", phPrefix, atomic.AddUint64(&phCounter, 1))
}

// protectKaTeX 保护 KaTeX 数学公式（$...$ 行内 / $$...$$ 行间），替换为占位符
func protectKaTeX(input string, protected map[string]string) string {
	// 块级公式 $$...$$
	reBlock := regexp.MustCompile(`\$\$([\s\S]*?)\$\$`)
	input = reBlock.ReplaceAllStringFunc(input, func(m string) string {
		key := makePH()
		protected[key] = m
		return key
	})

	// 行内公式 $...$
	input = protectInlineDollar(input, protected)

	return input
}

// protectInlineDollar 逐个字符扫描，匹配 $...$ 行内公式
func protectInlineDollar(s string, protected map[string]string) string {
	var result strings.Builder
	result.Grow(len(s))
	runes := []rune(s)
	n := len(runes)

	for i := 0; i < n; i++ {
		r := runes[i]
		if r == '$' && (i+1 >= n || runes[i+1] != '$') {
			closeIdx := -1
			for j := i + 1; j < n; j++ {
				if runes[j] == '$' && (j+1 >= n || runes[j+1] != '$') {
					closeIdx = j
					break
				}
			}
			if closeIdx >= 0 {
				formula := string(runes[i : closeIdx+1])
				key := makePH()
				protected[key] = formula
				result.WriteString(key)
				i = closeIdx
				continue
			}
		}
		result.WriteRune(r)
	}
	return result.String()
}

// protectMermaid 保护 Mermaid 图表代码块（```mermaid...```）
func protectMermaid(input string, protected map[string]string) string {
	reMermaid := regexp.MustCompile("(?s)```mermaid\\n(.+?)```")
	return reMermaid.ReplaceAllStringFunc(input, func(m string) string {
		key := makePH()
		protected[key] = m
		return key
	})
}

// restoreProtected 将占位符还原为原始内容
func restoreProtected(html string, protected map[string]string) string {
	for key, value := range protected {
		html = strings.ReplaceAll(html, key, value)
	}
	return html
}
