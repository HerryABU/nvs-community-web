package authorai

import (
	"fmt"
	"regexp"
	"strings"
)

// ----------------------------------------------------------------------------
// 时间线 [LOOP] 嵌套循环：真源(文本) → 投影(结构化) + 纯 Go 一致性校验
//
// 设计（对齐 webnovel-writer 的「真源划分」与 show-me-the-story 的实体化思路）：
//   - 真源：NovelProject.Timeline 文本，作者填写或 AI 生成，含自由文本 + [LOOP] 标注；
//   - 投影：LoopContext / TimelineEvent 由下方纯 Go 解析器从文本派生，不另存表；
//   - 校验：嵌套是否平衡、ID 是否唯一、开闭是否匹配，全部由 Go 判定，不依赖模型。
//
// 语法约定（宽松、对 AI 友好）：
//   [LOOP:id]   开启一个名为 id 的循环（可嵌套）
//   [LOOPEND:id] 关闭名为 id 的循环
//   其余每行视为一个时间线事件（携带「当前所在循环栈」上下文）
// ----------------------------------------------------------------------------

var (
	// 开循环：支持 [LOOP:id]（带标识）与 [LOOP]（无标识，自动编号）
	reLoopOpen = regexp.MustCompile(`(?i)\[LOOP(?::([^\]]+))?\]`)
	// 闭循环：支持 [/LOOP]（LIFO 关闭栈顶）与 [LOOPEND:id]（按标识关闭）
	reLoopClose = regexp.MustCompile(`(?i)\[\/LOOP\]|\[LOOPEND(?::([^\]]+))?\]`)
)

// LoopContext 一个循环的投影结构（由文本解析派生）。
type LoopContext struct {
	LoopID       string `json:"loop_id"`
	ParentLoopID string `json:"parent_loop_id"` // 空 = 顶层循环
	Depth        int    `json:"depth"`          // 嵌套深度（顶层=1）
	OpenLine     int    `json:"open_line"`      // 开启所在行（1-based），0=未知
	CloseLine    int    `json:"close_line"`     // 关闭所在行（1-based），0=未关闭
}

// TimelineEvent 时间线中的一个事件（携带其发生时所处的循环栈）。
type TimelineEvent struct {
	Line   int      `json:"line"`    // 行号（1-based）
	Text   string   `json:"text"`    // 事件文本
	LoopID string   `json:"loop_id"` // 直属循环 ID（空=不在任何循环内）
	Stack  []string `json:"stack"`   // 当时的完整循环栈（从外到内）
}

// LoopParseResult 解析结果 + 一致性问题（纯 Go 校验，不依赖模型）。
type LoopParseResult struct {
	Loops   []*LoopContext  `json:"loops"`
	Events  []TimelineEvent `json:"events"`
	Errors  []string        `json:"errors"`
	HasLoop bool            `json:"has_loop"`
}

// ParseLoops 从时间线文本解析嵌套循环结构与事件，并做纯 Go 一致性校验。
func ParseLoops(text string) LoopParseResult {
	res := LoopParseResult{}
	if strings.TrimSpace(text) == "" {
		return res
	}
	// 栈：当前打开的循环 ID（从外到内）
	var stack []string
	// 已开启但未关闭的循环 → 其上下文
	opened := map[string]*LoopContext{}
	// 已出现过（开启）的 ID，保证唯一
	seen := map[string]bool{}

	lines := strings.Split(text, "\n")
	for i, raw := range lines {
		lineNo := i + 1
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

		// 先处理关闭标记（同一行可同时有开/关，顺序以出现先后为准）
		closeMatches := reLoopClose.FindAllStringSubmatch(line, -1)
		openMatches := reLoopOpen.FindAllStringSubmatch(line, -1)

		// 开启
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
				LoopID:       id,
				ParentLoopID: parent,
				Depth:        depth,
				OpenLine:     lineNo,
			}
			opened[id] = ctx
			res.Loops = append(res.Loops, ctx)
			stack = append(stack, id)
		}

		// 关闭：[/LOOP]（无标识，关闭栈顶）或 [LOOPEND:id]（按标识关闭）
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

		// 事件（去掉标记后的纯文本仍保留，便于 AI 阅读；这里记录原始行）
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
	if !res.HasLoop {
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
	if len(res.Errors) > 0 {
		b.WriteString("\n⚠ 时间线存在一致性问题：")
		for _, e := range res.Errors {
			b.WriteString("\n  - " + e)
		}
		b.WriteString("\n请在续写时修正上述循环嵌套问题，保持章节时间线自洽。")
	} else {
		b.WriteString("\n请在续写章节时维持上述循环嵌套的时间线一致性。")
	}
	return b.String()
}
