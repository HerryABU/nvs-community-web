package authorai

import "strings"

// ==================== 故事线结构化解析（后端真源，前后端约定一致） ====================
// 语法（与 prompttpl authorai-storyline 模板一致）：
//   [LINE:id|类型|标题] … [/LINE]        一条故事线（主线/支线）
//   事件行：序号 事件简述，可带标识符
//   [CROSS:线id]        多线交汇（可多个，逗号分隔）
//   [INFLUENCE:线id|影响简述]  一条线的事件改变另一条线走向（有方向：本线 → 目标线）
//   [IMPACT:关键词]     线内影响（该事件对本线后续的内在影响）
//   [A]                时间锚点声明；ret / returnA / returnA@线id 回溯

// StorylineEvent 故事线事件（含交织/影响/锚点/回溯标识）
type StorylineEvent struct {
	Text      string   `json:"text"`
	Cross     []string `json:"cross,omitempty"`
	Influence []string `json:"influence,omitempty"` // 目标线 id（方向：本线 → 目标线）
	Impact    []string `json:"impact,omitempty"`
	Anchors   []string `json:"anchors,omitempty"`
	ReturnTo  string   `json:"return_to,omitempty"`
	IsReturn  bool     `json:"is_return,omitempty"`
}

// Storyline 一条故事线（主线或支线）
type Storyline struct {
	ID        string           `json:"id"`
	Type      string           `json:"type"` // 主线 / 支线
	Title     string           `json:"title"`
	Events    []StorylineEvent `json:"events"`
	Cross     []string         `json:"cross,omitempty"`     // 本线涉及的所有交织目标
	Influence []string         `json:"influence,omitempty"` // 本线发出的所有影响目标
	Impact    []string         `json:"impact,omitempty"`    // 线内影响关键词
}

// ParseStorylines 解析故事线文本为结构化数据（供后端校验/统计/将来可视化共用）。
// 解析失败的行按普通事件处理，不丢内容。
func ParseStorylines(text string) []Storyline {
	var out []Storyline
	var cur *Storyline
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if m := matchLineOpen(line); m != nil {
			out = append(out, *m)
			cur = &out[len(out)-1]
			continue
		}
		if strings.HasPrefix(strings.ToUpper(line), "[/LINE") {
			cur = nil
			continue
		}
		if cur == nil {
			continue
		}
		ev := StorylineEvent{}
		ev.Cross = extractTagList(line, "CROSS")
		ev.Influence = extractTagList(line, "INFLUENCE")
		ev.Impact = extractTagList(line, "IMPACT")
		ev.Anchors = extractAnchor(line)
		ev.ReturnTo, ev.IsReturn = extractReturn(line)
		// 去掉所有标识符后剩余为事件正文
		body := stripTags(line)
		body = strings.TrimLeft(body, "0123456789.、 \t")
		if strings.TrimSpace(body) == "" {
			continue
		}
		ev.Text = strings.TrimSpace(body)
		cur.Events = append(cur.Events, ev)
		for _, c := range ev.Cross {
			if !contains(cur.Cross, c) {
				cur.Cross = append(cur.Cross, c)
			}
		}
		for _, inf := range ev.Influence {
			target := inf
			if i := strings.Index(inf, "|"); i >= 0 {
				target = strings.TrimSpace(inf[:i])
			}
			if target != "" && !contains(cur.Influence, target) {
				cur.Influence = append(cur.Influence, target)
			}
		}
		for _, im := range ev.Impact {
			if !contains(cur.Impact, im) {
				cur.Impact = append(cur.Impact, im)
			}
		}
	}
	return out
}

// matchLineOpen 解析 [LINE:id|类型|标题]
func matchLineOpen(line string) *Storyline {
	if !strings.HasPrefix(strings.ToUpper(line), "[LINE:") {
		return nil
	}
	end := strings.Index(line, "]")
	if end < 0 {
		return nil
	}
	parts := strings.Split(line[len("[LINE:"):end], "|")
	id := ""
	typ := "支线"
	title := ""
	if len(parts) > 0 {
		id = strings.TrimSpace(parts[0])
	}
	if len(parts) > 1 {
		typ = strings.TrimSpace(parts[1])
	}
	if len(parts) > 2 {
		title = strings.TrimSpace(parts[2])
	}
	if id == "" {
		id = title
	}
	if typ == "" {
		typ = "支线"
	}
	return &Storyline{ID: id, Type: typ, Title: title}
}

// extractTagList 提取 [TAG:...] 的值列表（支持逗号分隔与多标签）
func extractTagList(line, tag string) []string {
	var out []string
	upper := strings.ToUpper(line)
	marker := "[" + tag + ":"
	for {
		i := strings.Index(upper, marker)
		if i < 0 {
			break
		}
		rest := line[i+len(marker):]
		end := strings.Index(rest, "]")
		if end < 0 {
			break
		}
		val := rest[:end]
		for _, p := range strings.Split(val, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
		line = line[:i] + rest[end+1:]
		upper = strings.ToUpper(line)
	}
	return out
}

// extractAnchor 提取 [A] 锚点（可带描述）
func extractAnchor(line string) []string {
	var out []string
	for _, tok := range strings.Fields(line) {
		if strings.EqualFold(tok, "[A]") {
			out = append(out, "A")
		}
	}
	return out
}

// extractReturn 解析 ret / returnA / returnA@线id
func extractReturn(line string) (string, bool) {
	fields := strings.Fields(line)
	for _, f := range fields {
		lf := strings.ToLower(strings.Trim(f, "，。"))
		switch {
		case lf == "ret":
			return "recent", true
		case strings.HasPrefix(lf, "return"):
			target := strings.TrimPrefix(lf, "return")
			if target == "" {
				return "recent", true
			}
			if strings.Contains(target, "@") {
				return target, true
			}
			return target, true
		}
	}
	return "", false
}

// stripTags 去掉所有 [XXX:...] / [A] / ret 类标识符
func stripTags(line string) string {
	var sb strings.Builder
	depth := 0
	for i := 0; i < len(line); i++ {
		ch := line[i]
		if ch == '[' {
			depth++
			continue
		}
		if ch == ']' && depth > 0 {
			depth--
			continue
		}
		if depth == 0 {
			sb.WriteByte(ch)
		}
	}
	out := sb.String()
	// 去掉残留的 ret / returnX 标记
	for _, w := range []string{"ret", "returnA", "returnB"} {
		out = strings.ReplaceAll(out, w, " ")
	}
	return out
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
