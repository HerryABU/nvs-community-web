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
	"strings"
	"unicode/utf8"
)

// NodeType 节点类型
type NodeType string

const (
	NodeDialogue  NodeType = "dialogue"
	NodeNarration NodeType = "narration"
	NodeStage     NodeType = "stage"
	// NodeProse 散文块（仅混排模式出现）：走 Markdown 渲染，与气泡混在同一章
	NodeProse NodeType = "prose"
	// NodeImage 图片气泡：整行 ![说明](url)
	NodeImage NodeType = "image"
)

// Node 解析后的一个渲染节点
type Node struct {
	Type    NodeType `json:"type"`
	Speaker string   `json:"speaker,omitempty"`
	Text    string   `json:"text"`
	Side    string   `json:"side,omitempty"` // 对话气泡左右：left / right
	Initial string   `json:"initial,omitempty"`
	// 图片节点
	Alt string `json:"alt,omitempty"`
	URL string `json:"url,omitempty"`
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

// ==================== 显式左右标记（# 左侧 / ## 右侧） ====================
// 约定（话本小说 / 橙光 / 汤圆创作同款写法）：
//
//	#李明：早啊。      → 左侧气泡
//	##王芳：你也是。  → 右侧气泡
//	李明：随便一句。  → 不带标记：按角色首现顺序自动左右交替
//
// 安全约束：标记只有在「去掉标记后仍能解析出『角色：台词』」时才生效。
// 否则 `## 标题` 这类 Markdown 标题会被误伤成气泡，保持按旁白/散文处理。
// 显式标记一旦出现，该角色后续所有气泡都固定在同一侧（charSide 记忆）。

// explicitSide 拆出行首的 # / ## 左右标记
func explicitSide(line string) (side string, rest string, ok bool) {
	if strings.HasPrefix(line, "##") {
		return "right", strings.TrimSpace(line[2:]), true
	}
	if strings.HasPrefix(line, "#") {
		return "left", strings.TrimSpace(line[1:]), true
	}
	return "", "", false
}

// parseDialogue 解析一行对话，返回角色、台词、显式左右（空=自动分配）、是否是对话。
func parseDialogue(line string) (name, text, side string, ok bool) {
	if s, rest, has := explicitSide(line); has {
		if n, t, ok2 := splitSpeaker(rest); ok2 && t != "" && !narrationNames[n] {
			return n, t, s, true
		}
	}
	n, t, ok2 := splitSpeaker(line)
	if !ok2 || t == "" || narrationNames[n] {
		return "", "", "", false
	}
	return n, t, "", true
}

// Parse 把话本源文本解析为节点序列。
// 连续的同角色对话合并为一个气泡（Text 内用 \n 分段），符合话本小说网「一人连说多句」的观感。
//
// 左右归属：优先用行首的 # / ## 显式标记；没有标记的角色按首现顺序自动左右交替。
func Parse(src string) []Node {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.ReplaceAll(src, "\r", "\n")
	lines := strings.Split(src, "\n")

	nodes := make([]Node, 0, len(lines))
	charSide := map[string]string{} // 角色 → 固定左右
	autoCount := 0
	autoSide := func(name string) string {
		if s, ok := charSide[name]; ok {
			return s
		}
		s := "left"
		if autoCount%2 == 1 {
			s = "right"
		}
		autoCount++
		charSide[name] = s
		return s
	}

	// 用下标而不是指针记录「上一个可合并节点」：
	// append 可能触发底层数组扩容，旧的 *Node 会指向已被丢弃的旧数组，导致合并文本丢失。
	lastIdx := -1
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			lastIdx = -1
			continue
		}
		if alt, url, ok := ParseImage(line); ok {
			lastIdx = -1
			nodes = append(nodes, Node{Type: NodeImage, Alt: alt, URL: url})
			continue
		}
		if stage, ok := isStage(line); ok {
			lastIdx = -1
			nodes = append(nodes, Node{Type: NodeStage, Text: stage})
			continue
		}
		if name, text, explicit, ok := parseDialogue(line); ok {
			s := explicit
			if s == "" {
				s = autoSide(name)
			} else {
				charSide[name] = s // 显式标记：固定该角色左右
			}
			if lastIdx >= 0 {
				last := &nodes[lastIdx]
				if last.Type == NodeDialogue && last.Speaker == name && last.Side == s {
					last.Text += "\n" + text
					continue
				}
			}
			nodes = append(nodes, Node{Type: NodeDialogue, Speaker: name, Text: text, Side: s, Initial: initial(name)})
			lastIdx = len(nodes) - 1
			continue
		}
		// 无角色的行：旁白（含「角色名：」但台词为空的情形）
		if lastIdx >= 0 && nodes[lastIdx].Type == NodeNarration {
			nodes[lastIdx].Text += "\n" + line
			continue
		}
		nodes = append(nodes, Node{Type: NodeNarration, Text: line})
		lastIdx = len(nodes) - 1
	}
	return nodes
}

// ParseMixed 混排解析：对话/舞台/图片与纯话本一致，
// 其余连续非空行聚成一个「散文块」（NodeProse），由 Markdown 渲染。
// 用途：同一章里既有大段描写（散文）又有对话（气泡），不必整章二选一。
//
// 左右归属与 Parse 一致：# / ## 显式标记优先，其余按首现顺序交替。
func ParseMixed(src string) []Node {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.ReplaceAll(src, "\r", "\n")
	lines := strings.Split(src, "\n")

	nodes := make([]Node, 0, len(lines))
	charSide := map[string]string{}
	autoCount := 0
	autoSide := func(name string) string {
		if s, ok := charSide[name]; ok {
			return s
		}
		s := "left"
		if autoCount%2 == 1 {
			s = "right"
		}
		autoCount++
		charSide[name] = s
		return s
	}

	// 下标而非指针：append 扩容会让旧 *Node 指向废弃数组（合并文本会丢）
	lastIdx := -1
	prose := make([]string, 0, 8)
	flushProse := func() {
		if len(prose) == 0 {
			return
		}
		nodes = append(nodes, Node{Type: NodeProse, Text: strings.Join(prose, "\n")})
		prose = prose[:0]
	}

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			// 散文内部的空行是段落分隔（Markdown 语义），不切块；只有遇到对话/提示才结算
			if len(prose) > 0 {
				prose = append(prose, "")
			}
			lastIdx = -1
			continue
		}
		if alt, url, ok := ParseImage(line); ok {
			flushProse()
			lastIdx = -1
			nodes = append(nodes, Node{Type: NodeImage, Alt: alt, URL: url})
			continue
		}
		if stage, ok := isStage(line); ok {
			flushProse()
			lastIdx = -1
			nodes = append(nodes, Node{Type: NodeStage, Text: stage})
			continue
		}
		if name, text, explicit, ok := parseDialogue(line); ok {
			flushProse()
			s := explicit
			if s == "" {
				s = autoSide(name)
			} else {
				charSide[name] = s
			}
			if lastIdx >= 0 {
				last := &nodes[lastIdx]
				if last.Type == NodeDialogue && last.Speaker == name && last.Side == s {
					last.Text += "\n" + text
					continue
				}
			}
			nodes = append(nodes, Node{Type: NodeDialogue, Speaker: name, Text: text, Side: s, Initial: initial(name)})
			lastIdx = len(nodes) - 1
			continue
		}
		// 非对话行 → 攒成散文块（空行或遇到对话时结算）
		lastIdx = -1
		prose = append(prose, raw)
	}
	flushProse()
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
