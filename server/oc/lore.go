package oc

// ==================== 世界书（Lorebook）激活引擎 ====================
// 借鉴 SillyTavern（酒馆）的 World Info：
//   设定不整本塞进上下文（那是「固定税」），而是按键触发（「条件开销」）——
//   只有最近对话里出现主/次触发词时才注入，从而让世界书能装下远超上下文的设定量。
//
// 流程：扫描最近 N 条发言 → 命中条目（键匹配/常驻/概率/包含组/逻辑）→
//       按 priority 应用 token 预算 → 按 insertion_order + position 装配进提示词。

import (
	"math/rand"
	"sort"
	"strings"
)

// ActivatedEntry 被激活的条目（含装配位置）
type ActivatedEntry struct {
	Entry    WorldEntry `json:"entry"`
	Position int        `json:"position"`
	Order    int        `json:"order"`
	Tokens   int        `json:"tokens"`
}

// ActivationResult 激活结果
type ActivationResult struct {
	Before []ActivatedEntry `json:"before"` // 角色定义前
	After  []ActivatedEntry `json:"after"`  // 角色定义后
	InChat []ActivatedEntry `json:"in_chat"`
	Total  int              `json:"total"`
}

// splitKeys 逗号/顿号分隔的触发词
func splitKeys(s string) []string {
	out := make([]string, 0, 4)
	for _, part := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == '，' || r == '、' || r == ';' || r == '；'
	}) {
		k := strings.TrimSpace(part)
		if k != "" {
			out = append(out, k)
		}
	}
	return out
}

// estimateTokens 粗估 token（中文按 1.5 字/token 近似，与模型侧估算同量级即可）
func estimateTokens(s string) int {
	n := len([]rune(s))
	if n == 0 {
		return 0
	}
	return n/2 + 1
}

// containsKey 文本是否命中触发词
func containsKey(text, key string, caseSensitive, wholeWord bool) bool {
	if key == "" {
		return false
	}
	if !caseSensitive {
		text = strings.ToLower(text)
		key = strings.ToLower(key)
	}
	if !strings.Contains(text, key) {
		return false
	}
	if !wholeWord {
		return true
	}
	// 整词匹配：前后需为单词边界（中文按「非字母数字」处理）
	for i := 0; i+len(key) <= len(text); {
		idx := strings.Index(text[i:], key)
		if idx < 0 {
			break
		}
		start := i + idx
		end := start + len(key)
		okLeft := start == 0 || !isWordByte(text[start-1])
		okRight := end >= len(text) || !isWordByte(text[end])
		if okLeft && okRight {
			return true
		}
		i = start + 1
	}
	return false
}

func isWordByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
}

// matchEntry 判断条目是否被激活
func matchEntry(e WorldEntry, scanText string, randVal int) bool {
	if !e.Enabled {
		return false
	}
	if e.Constant {
		return true // 常驻：忽略触发词
	}
	if e.Probability > 0 && e.Probability < 100 && randVal > e.Probability {
		return false
	}
	primaries := splitKeys(e.Key)
	secondaries := splitKeys(e.SecondaryKeys)

	hitPrimary := false
	for _, k := range primaries {
		if containsKey(scanText, k, e.CaseSensitive, e.WholeWord) {
			hitPrimary = true
			break
		}
	}
	if len(primaries) == 0 {
		hitPrimary = true // 无主键：视为不按键触发（只有 constant 才注入）
	}
	hitSecondary := false
	for _, k := range secondaries {
		if containsKey(scanText, k, e.CaseSensitive, e.WholeWord) {
			hitSecondary = true
			break
		}
	}

	switch e.Logic {
	case 1: // NOT ALL：主键命中且次键不命中
		return hitPrimary && !hitSecondary
	case 2: // NOT ANY：主键或次键任一命中即可（宽松）
		return hitPrimary || hitSecondary
	case 3: // AND ALL：主、次键都要命中
		return hitPrimary && hitSecondary
	default: // AND ANY：主键命中；开启 selective 时要求次键也命中
		if e.Selective && len(secondaries) > 0 {
			return hitPrimary && hitSecondary
		}
		return hitPrimary
	}
}

// Activate 计算本轮应注入的条目（含预算与包含组）
func Activate(book *WorldBook, entries []WorldEntry, recent []Turn) ActivationResult {
	res := ActivationResult{}
	if book == nil || len(entries) == 0 {
		return res
	}
	// 扫描文本：最近 ScanDepth 条发言
	depth := book.ScanDepth
	if depth <= 0 {
		depth = 20
	}
	from := 0
	if len(recent) > depth {
		from = len(recent) - depth
	}
	var sb strings.Builder
	for _, t := range recent[from:] {
		sb.WriteString(t.Content)
		sb.WriteString("\n")
	}
	scanText := sb.String()

	// 1) 命中判定
	var hits []ActivatedEntry
	for _, e := range entries {
		rv := rand.Intn(100) + 1
		if !matchEntry(e, scanText, rv) {
			continue
		}
		hits = append(hits, ActivatedEntry{
			Entry: e, Position: e.Position, Order: e.InsertionOrder,
			Tokens: estimateTokens(e.Content),
		})
	}

	// 2) 包含组：同组只保留 insertion_order 最小的一条
	hits = applyGroups(hits)

	// 3) token 预算：按 priority（小者更重要）+ insertion_order 排序后截断
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Entry.Priority != hits[j].Entry.Priority {
			return hits[i].Entry.Priority < hits[j].Entry.Priority
		}
		return hits[i].Order < hits[j].Order
	})
	budget := book.TokenBudget
	if budget <= 0 {
		budget = 800
	}
	used := 0
	kept := make([]ActivatedEntry, 0, len(hits))
	for _, h := range hits {
		if used+h.Tokens > budget && len(kept) > 0 {
			continue
		}
		used += h.Tokens
		kept = append(kept, h)
	}

	// 4) 递归扫描（可选）：命中条目的内容里出现其他条目的键 → 也激活（只做一轮）
	if book.RecursiveScan {
		extra := recursivePass(entries, kept, budget-used)
		kept = append(kept, extra...)
	}

	// 5) 按位置分组并排序
	for _, h := range kept {
		switch h.Position {
		case PosBeforeChar:
			res.Before = append(res.Before, h)
		case PosInChat:
			res.InChat = append(res.InChat, h)
		default:
			res.After = append(res.After, h)
		}
	}
	sorter := func(list []ActivatedEntry) {
		sort.SliceStable(list, func(i, j int) bool {
			if list[i].Order != list[j].Order {
				return list[i].Order < list[j].Order
			}
			return list[i].Entry.ID < list[j].Entry.ID
		})
	}
	sorter(res.Before)
	sorter(res.After)
	sorter(res.InChat)
	res.Total = len(kept)
	return res
}

func applyGroups(list []ActivatedEntry) []ActivatedEntry {
	best := map[string]int{}
	for i, h := range list {
		if h.Entry.Group == "" {
			continue
		}
		if j, ok := best[h.Entry.Group]; !ok || h.Order < list[j].Order {
			best[h.Entry.Group] = i
		}
	}
	out := make([]ActivatedEntry, 0, len(list))
	for i, h := range list {
		if h.Entry.Group == "" {
			out = append(out, h)
			continue
		}
		if best[h.Entry.Group] == i {
			out = append(out, h)
		}
	}
	return out
}

func recursivePass(all []WorldEntry, kept []ActivatedEntry, leftBudget int) []ActivatedEntry {
	if leftBudget <= 0 || len(kept) == 0 {
		return nil
	}
	keptIDs := map[uint]bool{}
	for _, k := range kept {
		keptIDs[k.Entry.ID] = true
	}
	var ctx strings.Builder
	for _, k := range kept {
		ctx.WriteString(k.Entry.Content)
		ctx.WriteString("\n")
	}
	text := ctx.String()
	var out []ActivatedEntry
	used := 0
	for _, e := range all {
		if keptIDs[e.ID] || !e.Enabled || e.Constant {
			continue
		}
		for _, key := range splitKeys(e.Key) {
			if containsKey(text, key, e.CaseSensitive, e.WholeWord) {
				tk := estimateTokens(e.Content)
				if used+tk > leftBudget {
					return out
				}
				used += tk
				out = append(out, ActivatedEntry{Entry: e, Position: e.Position, Order: e.InsertionOrder, Tokens: tk})
				break
			}
		}
	}
	return out
}

// RenderLore 把激活结果渲染为可插入提示词的文本片段
func RenderLore(act ActivationResult) (before, after, inChat string) {
	var b, a, c strings.Builder
	for _, h := range act.Before {
		b.WriteString("【" + h.Entry.Key + "】" + h.Entry.Content + "\n")
	}
	for _, h := range act.After {
		a.WriteString("【" + h.Entry.Key + "】" + h.Entry.Content + "\n")
	}
	for _, h := range act.InChat {
		c.WriteString("【" + h.Entry.Key + "】" + h.Entry.Content + "\n")
	}
	return b.String(), a.String(), c.String()
}

// ExpandMacros 宏替换：{{char}} → 角色名，{{user}} → 对方（或「对方」）
func ExpandMacros(s, charName, userName string) string {
	s = strings.ReplaceAll(s, "{{char}}", charName)
	s = strings.ReplaceAll(s, "{{Char}}", charName)
	s = strings.ReplaceAll(s, "{{user}}", userName)
	s = strings.ReplaceAll(s, "{{User}}", userName)
	return s
}
