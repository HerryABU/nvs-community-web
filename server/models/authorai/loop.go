package authorai

import (
	"fmt"
	"regexp"
	"strings"
)

// ----------------------------------------------------------------------------
// 时间线 [LOOP] 嵌套循环 + [锚点]/回环语句：真源(文本) → 投影(结构化) + 纯 Go 校验
//
// 设计（对齐 webnovel-writer 的「真源划分」与 show-me-the-story 的实体化思路）：
//   - 真源：NovelProject.Timeline 文本，作者填写或 AI 生成，含自由文本 + 标注；
//   - 投影：LoopContext / TimelineEvent / TimelineAnchor 由纯 Go 解析器从文本派生，不另存表；
//   - 校验：嵌套是否平衡、ID 是否唯一、开闭是否匹配、回环目标是否已定义，全部由 Go 判定。
//
// 语法约定（宽松、对 AI 友好）：
//   [LOOP:id]     开启一个名为 id 的循环（可嵌套）
//   [LOOPEND:id]  关闭名为 id 的循环
//   [/LOOP]       关闭栈顶循环（LIFO）
//   [A]           声明一个名为 A 的时间锚点（单行；A 为字母数字下划线，且非 LOOP 类关键字）
//   ret           回环/回溯语句：回到「最近一个锚点」（无锚点则回时间线起点）
//   returnA/retA  回环/回溯语句：回到名为 A 的锚点（A 可为 [A] 标记，亦可为某 [LOOP:id] 的开启点）
//   其余每行视为一个时间线事件（携带「当前所在循环栈」上下文）
//
// 两类循环的区别：
//   - [LOOP]…[/LOOP] 是「整段循环块」，可视化画一段弧罩住区间；
//   - ret/returnA 是「点状回溯」，可视化从回溯点画一条回环箭线连回目标锚点。
//   二者互补，适用于时间循环/穿越类叙事（特殊回环）。
// ----------------------------------------------------------------------------

var (
	// 开循环：支持 [LOOP:id]（带标识）与 [LOOP]（无标识，自动编号）
	reLoopOpen = regexp.MustCompile(`(?i)\[LOOP(?::([^\]]+))?\]`)
	// 闭循环：支持 [/LOOP]（LIFO 关闭栈顶）与 [LOOPEND:id]（按标识关闭）
	reLoopClose = regexp.MustCompile(`(?i)\[\/LOOP\]|\[LOOPEND(?::([^\]]+))?\]`)
	// 锚点声明：[A]（单行，单个字母数字下划线标识，非 LOOP 类关键字）
	reAnchor = regexp.MustCompile(`(?i)^\[([A-Za-z][A-Za-z0-9_]*)\]`)
	// 回环/回溯语句：ret / returnA / retA（忽略大小写；仅做可视化识别与硬校验，语义由系统提示词定义）
	// 宽松匹配行首关键字：允许其后附带描述文本（如 "returnA 回到世界线起点"），避免与 AI 实际生成物冲突
	reReturn = regexp.MustCompile(`(?i)^ret(?:urn)?([A-Za-z0-9_]*)`)
)

// LoopContext 一个循环的投影结构（由文本解析派生）。
type LoopContext struct {
	LoopID         string `json:"loop_id"`
	ParentLoopID   string `json:"parent_loop_id"`   // 空 = 顶层循环
	Depth          int    `json:"depth"`            // 嵌套深度（顶层=1）
	OpenLine       int    `json:"open_line"`        // 开启所在行（1-based），0=未知
	CloseLine      int    `json:"close_line"`       // 关闭所在行（1-based），0=未关闭
	OpenEventIndex int    `json:"open_event_index"` // 开启时的事件索引（锚点语义：循环开启即隐式锚点）
}

// TimelineAnchor 时间线上的一个有名锚点（由 [A] 声明，或由 [LOOP:id] 开启隐式形成）。
type TimelineAnchor struct {
	Name       string `json:"name"`
	Line       int    `json:"line"`        // 声明所在行（1-based）
	EventIndex int    `json:"event_index"` // 锚点所在事件索引（指向其后第一个事件的索引；无后续事件则指末尾）
	FromLoop   bool   `json:"from_loop"`   // true=该锚点由 [LOOP:id] 开启隐式形成
}

// TimelineEvent 时间线中的一个事件（携带其发生时所处的循环栈）。
type TimelineEvent struct {
	Line     int      `json:"line"`      // 行号（1-based）
	Text     string   `json:"text"`      // 事件文本
	LoopID   string   `json:"loop_id"`   // 直属循环 ID（空=不在任何循环内）
	Stack    []string `json:"stack"`     // 当时的完整循环栈（从外到内）
	IsReturn bool     `json:"is_return"` // 是否为回环/回溯语句行（ret / returnA）
	ReturnTo string   `json:"return_to"` // 回环目标锚点名；空=回到时间线起点（最近锚点）
}

// LoopParseResult 解析结果 + 一致性问题（纯 Go 校验，不依赖模型）。
type LoopParseResult struct {
	Loops     []*LoopContext    `json:"loops"`
	Events    []TimelineEvent   `json:"events"`
	Anchors   []*TimelineAnchor `json:"anchors"` // 显式 [A] 锚点（不含循环隐式锚点）
	Errors    []string          `json:"errors"`
	HasLoop   bool              `json:"has_loop"`
	HasReturn bool              `json:"has_return"`
}

// ParseLoops 从时间线文本解析嵌套循环、锚点与回环语句，并做纯 Go 一致性校验。
func ParseLoops(text string) LoopParseResult {
	res := LoopParseResult{}
	if strings.TrimSpace(text) == "" {
		return res
	}
	// 栈：当前打开的循环 ID（从外到内）
	var stack []string
	// 已开启但未关闭的循环 → 其上下文
	opened := map[string]*LoopContext{}
	// 已出现过（开启）的循环 ID，保证唯一
	seen := map[string]bool{}
	// 显式锚点（[A]）→ 上下文
	anchorMap := map[string]*TimelineAnchor{}

	lines := strings.Split(text, "\n")
	for i, raw := range lines {
		lineNo := i + 1
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

		// 先处理循环开/闭标记（同一行可同时有开/关，顺序以出现先后为准）
		closeMatches := reLoopClose.FindAllStringSubmatch(line, -1)
		openMatches := reLoopOpen.FindAllStringSubmatch(line, -1)

		// 开启循环
		for _, m := range openMatches {
			id := strings.TrimSpace(m[1])
			if id == "" {
				// 无标识形式 [LOOP]：自动编号，避免与显式 ID 冲突
				id = fmt.Sprintf("loop_%d", len(opened)+1)
			}
			if seen[id] {
				res.Errors = append(res.Errors, fmt.Sprintf("第 %d 行：循环 ID「%s」重复开启", lineNo, id))
				continue
			}
			seen[id] = true
			parent := ""
			depth := 1
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
				depth = len(stack) + 1
			}
			ctx := &LoopContext{
				LoopID:         id,
				ParentLoopID:   parent,
				Depth:          depth,
				OpenLine:       lineNo,
				OpenEventIndex: len(res.Events),
			}
			opened[id] = ctx
			res.Loops = append(res.Loops, ctx)
			stack = append(stack, id)
		}

		// 关闭循环：[/LOOP]（无标识，关闭栈顶）或 [LOOPEND:id]（按标识关闭）
		for _, m := range closeMatches {
			id := strings.TrimSpace(m[1])
			if len(stack) == 0 {
				res.Errors = append(res.Errors, fmt.Sprintf("第 %d 行：循环结束但没有对应的开启", lineNo))
				continue
			}
			top := stack[len(stack)-1]
			if id != "" && id != top {
				res.Errors = append(res.Errors,
					fmt.Sprintf("第 %d 行：循环结束顺序不匹配（期望关闭「%s」，实际遇到「%s」）", lineNo, top, id))
				continue
			}
			closeID := id
			if closeID == "" {
				closeID = top
			}
			if c, ok := opened[closeID]; ok {
				c.CloseLine = lineNo
			}
			stack = stack[:len(stack)-1]
		}

		// 循环开/闭已处理则跳过后续（一行只承担一种语义，避免把 [LOOP:x] 当成事件）
		if len(openMatches) > 0 || len(closeMatches) > 0 {
			continue
		}

		// 锚点声明：[A]（可附带描述，如 [A] 1990 世界线开启；描述同时作为事件落轴）
		if am := reAnchor.FindStringSubmatch(line); am != nil {
			name := strings.TrimSpace(am[1])
			up := strings.ToUpper(name)
			if up == "LOOP" || strings.HasPrefix(up, "LOOPEND") {
				// 理论不可达（已被开/闭处理），保险：当普通事件
			} else {
				rest := strings.TrimSpace(strings.TrimPrefix(line, am[0]))
				if _, dup := anchorMap[name]; dup {
					res.Errors = append(res.Errors, fmt.Sprintf("第 %d 行：锚点「%s」重复定义", lineNo, name))
				} else {
					anchorIdx := len(res.Events)
					if rest != "" {
						// 锚点行附带事件文本 → 同时作为事件，避免时间线缺数据
						ev := TimelineEvent{
							Line:   lineNo,
							Text:   rest,
							LoopID: "",
							Stack:  append([]string{}, stack...),
						}
						if len(stack) > 0 {
							ev.LoopID = stack[len(stack)-1]
						}
						res.Events = append(res.Events, ev)
						anchorIdx = len(res.Events) - 1
					}
					anchorMap[name] = &TimelineAnchor{
						Name:       name,
						Line:       lineNo,
						EventIndex: anchorIdx,
					}
					res.Anchors = append(res.Anchors, anchorMap[name])
				}
				continue
			}
		}

		// 回环/回溯语句：ret / returnA / retA
		if rm := reReturn.FindStringSubmatch(line); rm != nil {
			target := strings.TrimSpace(rm[1])
			ev := TimelineEvent{
				Line:     lineNo,
				Text:     line,
				IsReturn: true,
				ReturnTo: target,
				Stack:    append([]string{}, stack...),
			}
			if len(stack) > 0 {
				ev.LoopID = stack[len(stack)-1]
			}
			res.Events = append(res.Events, ev)
			res.HasReturn = true
			// 注：回环目标是否“已定义”不做硬性报错——用户原始输入非格式化，需经 AI 洗稿归一化；
			// 含义与用法由系统提示词（authorai-timeline）定义，前端做尽力而为的渲染即可。
			continue
		}

		// 普通事件（去掉标记后的纯文本仍保留，便于 AI 阅读；这里记录原始行）
		ev := TimelineEvent{
			Line:   lineNo,
			Text:   line,
			LoopID: "",
			Stack:  append([]string{}, stack...),
		}
		if len(stack) > 0 {
			ev.LoopID = stack[len(stack)-1]
		}
		res.Events = append(res.Events, ev)
	}

	// 检查未关闭的循环
	for _, c := range res.Loops {
		if c.CloseLine == 0 {
			res.Errors = append(res.Errors, fmt.Sprintf("循环「%s」开启于第 %d 行但未关闭", c.LoopID, c.OpenLine))
		}
	}

	res.HasLoop = len(res.Loops) > 0
	return res
}

// SummarizeLoops 生成一段给 AI / 用户看的循环结构摘要（用于注入章节生成上下文或 UI 提示）。
func SummarizeLoops(text string) string {
	res := ParseLoops(text)
	if !res.HasLoop && !res.HasReturn {
		return ""
	}
	var b strings.Builder
	b.WriteString("【时间线嵌套循环结构】")
	for _, c := range res.Loops {
		indent := strings.Repeat("  ", c.Depth-1)
		parent := ""
		if c.ParentLoopID != "" {
			parent = "（父循环：" + c.ParentLoopID + "）"
		}
		state := "未关闭"
		if c.CloseLine > 0 {
			state = fmt.Sprintf("闭合于第 %d 行", c.CloseLine)
		}
		b.WriteString(fmt.Sprintf("\n%s- %s（深度%d）%s %s", indent, c.LoopID, c.Depth, parent, state))
	}
	// 显式锚点
	if len(res.Anchors) > 0 {
		b.WriteString("\n【时间锚点】")
		for _, a := range res.Anchors {
			b.WriteString(fmt.Sprintf("\n  · %s（第 %d 行）", a.Name, a.Line))
		}
	}
	// 回环语句
	if res.HasReturn {
		b.WriteString("\n【回环/回溯语句】")
		for _, e := range res.Events {
			if !e.IsReturn {
				continue
			}
			if e.ReturnTo == "" {
				b.WriteString(fmt.Sprintf("\n  · 第 %d 行 ret：回到最近锚点（无锚点则回时间线起点）", e.Line))
			} else {
				b.WriteString(fmt.Sprintf("\n  · 第 %d 行 return%s：回到锚点 %s", e.Line, e.ReturnTo, e.ReturnTo))
			}
		}
	}
	if len(res.Errors) > 0 {
		b.WriteString("\n⚠ 时间线存在一致性问题：")
		for _, e := range res.Errors {
			b.WriteString("\n  - " + e)
		}
		b.WriteString("\n请在续写时修正上述循环嵌套/回环目标问题，保持章节时间线自洽。")
	} else {
		b.WriteString("\n请在续写章节时维持上述循环嵌套与回环的时间线一致性。")
	}
	return b.String()
}

// ResolveAnchorEventIndex 解析回环目标名到事件索引（供可视化定位回环箭线端点）。
// 目标可为显式锚点 [A] 或循环开启隐式锚点；空串表示时间线起点（返回 -1）。
func (r *LoopParseResult) ResolveAnchorEventIndex(name string) int {
	if name == "" {
		return -1 // 时间线起点
	}
	for _, a := range r.Anchors {
		if a.Name == name {
			return a.EventIndex
		}
	}
	for _, l := range r.Loops {
		if l.LoopID == name {
			return l.OpenEventIndex
		}
	}
	return -2 // 未定义
}
