package scriptmode

// ==================== 话本气泡体（script mode） ====================
// 形态参考话本小说网：正文以对话为主，按行解析为「气泡」，旁白与舞台提示居中排布。
//
// 源文本约定（纯文本，编辑器里一行一条）：
//   角色名：台词            → 对话气泡（同一角色连续多行合并为一个气泡内的多段）
//   【场景/动作】（）[]     → 舞台提示（居中，斜体灰字）
//   其余行                  → 旁白（居中，正文样式）
//   空行                    → 分隔（不产生节点）
//
// 安全性：所有文本一律 html.EscapeString 后再输出，绝不拼接原始 HTML。

import (
	"html"
	"strings"
	"unicode/utf8"
)

// NodeType 节点类型
type NodeType string

const (
	NodeDialogue  NodeType = "dialogue"
	NodeNarration NodeType = "narration"
	NodeStage     NodeType = "stage"
)

// Node 解析后的一个渲染节点
type Node struct {
	Type    NodeType `json:"type"`
	Speaker string   `json:"speaker,omitempty"`
	Text    string   `json:"text"`
	Side    string   `json:"side,omitempty"` // 对话气泡左右：left / right
	Initial string   `json:"initial,omitempty"`
}

// maxSpeakerLen 角色名最大长度（超过按旁白处理，避免长句被误判为对话）
const maxSpeakerLen = 16

// speakerSep 中英文冒号都认（全角「：」与半角「:」）
func splitSpeaker(line string) (string, string, bool) {
	for i, r := range line {
		if r == '：' || r == ':' {
			name := strings.TrimSpace(line[:i])
			text := strings.TrimSpace(line[i+utf8.RuneLen(r):])
			if name == "" {
				return "", "", false
			}
			// 角色名里不允许再出现冒号以外的标点堆砌（如整句 URL「https://」）
			if strings.ContainsAny(name, " \t") && utf8.RuneCountInString(name) > 6 {
				return "", "", false
			}
			if utf8.RuneCountInString(name) > maxSpeakerLen {
				return "", "", false
			}
			return name, text, true
		}
	}
	return "", "", false
}

// isStage 舞台提示：整行被 【】（）[]() 包裹
func isStage(line string) (string, bool) {
	pairs := [][2]string{{"【", "】"}, {"（", "）"}, {"[", "]"}, {"(", ")"}}
	for _, p := range pairs {
		if strings.HasPrefix(line, p[0]) && strings.HasSuffix(line, p[1]) && len(line) > len(p[0])+len(p[1])-1 {
			inner := strings.TrimSpace(line[len(p[0]) : len(line)-len(p[1])])
			if inner != "" {
				return inner, true
			}
		}
	}
	return "", false
}

// narrationNames 「旁白：xxx」这类保留名不当角色，直接落为旁白
var narrationNames = map[string]bool{"旁白": true, "旁述": true, "旁注": true, "画外音": true}

// initial 角色头像首字（中文取首字，英文取首字母）
func initial(name string) string {
	for _, r := range name {
		return string(r)
	}
	return "?"
}

// Parse 把话本源文本解析为节点序列。
// 连续的同角色对话合并为一个气泡（Text 内用 \n 分段），符合话本小说网「一人连说多句」的观感。
func Parse(src string) []Node {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.ReplaceAll(src, "\r", "\n")
	lines := strings.Split(src, "\n")

	nodes := make([]Node, 0, len(lines))
	order := map[string]int{} // 角色首次出现顺序 → 决定左右
	side := func(name string) string {
		idx, ok := order[name]
		if !ok {
			idx = len(order)
			order[name] = idx
		}
		if idx%2 == 0 {
			return "left"
		}
		return "right"
	}

	var last *Node
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			last = nil
			continue
		}
		if stage, ok := isStage(line); ok {
			last = nil
			nodes = append(nodes, Node{Type: NodeStage, Text: stage})
			continue
		}
		if name, text, ok := splitSpeaker(line); ok && text != "" && !narrationNames[name] {
			if last != nil && last.Type == NodeDialogue && last.Speaker == name {
				last.Text += "\n" + text
				continue
			}
			n := Node{Type: NodeDialogue, Speaker: name, Text: text, Side: side(name), Initial: initial(name)}
			nodes = append(nodes, n)
			last = &nodes[len(nodes)-1]
			continue
		}
		// 无角色的行：旁白（含「角色名：」但台词为空的情形）
		if last != nil && last.Type == NodeNarration {
			last.Text += "\n" + line
			continue
		}
		nodes = append(nodes, Node{Type: NodeNarration, Text: line})
		last = &nodes[len(nodes)-1]
	}
	return nodes
}

// Speakers 按首次出现顺序返回角色名（供编辑器展示角色列表）
func Speakers(src string) []string {
	out := make([]string, 0, 8)
	seen := map[string]bool{}
	for _, n := range Parse(src) {
		if n.Type == NodeDialogue && !seen[n.Speaker] {
			seen[n.Speaker] = true
			out = append(out, n.Speaker)
		}
	}
	return out
}

// RenderHTML 把话本源文本渲染为气泡 HTML（纯转义，无原始 HTML 拼接）
func RenderHTML(src string) string {
	nodes := Parse(src)
	if len(nodes) == 0 {
		return `<div class="script-chapter"></div>`
	}
	var b strings.Builder
	b.WriteString(`<div class="script-chapter">`)
	for _, n := range nodes {
		switch n.Type {
		case NodeStage:
			b.WriteString(`<div class="script-stage">` + escapeWithBreaks(n.Text) + `</div>`)
		case NodeNarration:
			b.WriteString(`<div class="script-narration">` + escapeWithBreaks(n.Text) + `</div>`)
		default:
			b.WriteString(`<div class="script-bubble sb-` + n.Side + `">`)
			b.WriteString(`<div class="sb-avatar">` + html.EscapeString(n.Initial) + `</div>`)
			b.WriteString(`<div class="sb-body">`)
			b.WriteString(`<div class="sb-name">` + html.EscapeString(n.Speaker) + `</div>`)
			b.WriteString(`<div class="sb-text">` + escapeWithBreaks(n.Text) + `</div>`)
			b.WriteString(`</div></div>`)
		}
	}
	b.WriteString(`</div>`)
	return b.String()
}

// escapeWithBreaks 转义后把 \n 变成 <br>（保证不引入未转义内容）
func escapeWithBreaks(s string) string {
	s = html.EscapeString(s)
	return strings.ReplaceAll(s, "\n", "<br>")
}

// CountDialogue 统计对话条数与角色数（编辑器侧提示用）
func CountDialogue(src string) (lines int, speakers int) {
	nodes := Parse(src)
	seen := map[string]bool{}
	for _, n := range nodes {
		if n.Type != NodeDialogue {
			continue
		}
		lines++
		if !seen[n.Speaker] {
			seen[n.Speaker] = true
			speakers++
		}
	}
	return lines, speakers
}
