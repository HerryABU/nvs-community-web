package security

import (
	"regexp"
	"strings"
	"sync"
)

// ═══ 违禁词库引擎（管理员可在后台维护，动态生效，无需改代码）═══
// 规则支持两种匹配方式：
//   - literal：字面关键词（自动 regexp.QuoteMeta 转义，填词即用）
//   - regex ：Go RE2 正则（支持 (?i) 忽略大小写、(?m) 多行、\p{Han} Unicode 属性等全方位正则）
// 三种动作：
//   - mark   ：命中不删不改，返回命中列表，由调用方写入 BadWordFlag 待审记录
//   - delete ：删除匹配内容
//   - replace：替换为 Replacement
// 适用范围 scope：all 或逗号分隔组合（chapter / comment / forum / blog）

// DynamicBadWord 一条违禁词规则（由 models/handler 注入）
type DynamicBadWord struct {
	ID          uint
	Name        string
	MatchType   string // literal / regex
	Pattern     string
	Action      string // mark / delete / replace
	Replacement string
	Scope       string // all / chapter,comment,forum,blog
}

type compiledBadWord struct {
	id      uint
	name    string
	re      *regexp.Regexp
	action  string
	replace string
	all     bool // scope 是否覆盖全部内容类型
	scopes  map[string]bool
}

var (
	badWordsMu sync.RWMutex
	badWords   []compiledBadWord
)

// SetDynamicBadWords 设置违禁词规则集（编译并缓存，无效规则自动跳过）
func SetDynamicBadWords(words []DynamicBadWord) {
	compiled := make([]compiledBadWord, 0, len(words))
	for _, w := range words {
		pattern := w.Pattern
		if pattern == "" {
			continue
		}
		if w.MatchType == "" || w.MatchType == "literal" {
			pattern = regexp.QuoteMeta(pattern) // 字面关键词：转义所有正则元字符
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue // 跳过无效正则，不影响其他规则
		}
		action := w.Action
		if action == "" {
			action = "mark"
		}
		c := compiledBadWord{
			id:      w.ID,
			name:    w.Name,
			re:      re,
			action:  action,
			replace: w.Replacement,
			all:     w.Scope == "" || w.Scope == "all",
			scopes:  make(map[string]bool),
		}
		for _, s := range strings.Split(w.Scope, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				c.scopes[s] = true
			}
		}
		compiled = append(compiled, c)
	}
	badWordsMu.Lock()
	badWords = compiled
	badWordsMu.Unlock()
}

// BadWordHit 一次违禁词命中
type BadWordHit struct {
	RuleID   uint   `json:"rule_id"`
	RuleName string `json:"rule_name"`
	Action   string `json:"action"`  // mark / delete / replace
	Matched  string `json:"matched"` // 命中的文本片段
	Index    int    `json:"index"`   // 命中起始位置
}

// ruleInScope 判断规则是否作用于指定内容类型
func (c *compiledBadWord) inScope(scope string) bool {
	if c.all {
		return true
	}
	return scope != "" && c.scopes[scope]
}

// ScanBadWords 扫描指定内容类型中的违禁词，返回全部命中（不修改文本）
func ScanBadWords(content, scope string) []BadWordHit {
	badWordsMu.RLock()
	rules := badWords
	badWordsMu.RUnlock()
	if len(rules) == 0 || content == "" {
		return nil
	}
	var hits []BadWordHit
	for _, r := range rules {
		if !r.inScope(scope) {
			continue
		}
		for _, idx := range r.re.FindAllStringIndex(content, -1) {
			hits = append(hits, BadWordHit{
				RuleID:   r.id,
				RuleName: r.name,
				Action:   r.action,
				Matched:  content[idx[0]:idx[1]],
				Index:    idx[0],
			})
		}
	}
	return hits
}

// ApplyBadWords 应用 delete/replace 动作净化文本；mark 规则不修改文本。
// 返回：净化后的文本 + 全部命中（含 mark，命中基于净化前原文）。
func ApplyBadWords(content, scope string) (string, []BadWordHit) {
	badWordsMu.RLock()
	rules := badWords
	badWordsMu.RUnlock()
	if len(rules) == 0 || content == "" {
		return content, nil
	}
	var hits []BadWordHit
	result := content
	for _, r := range rules {
		if !r.inScope(scope) {
			continue
		}
		switch r.action {
		case "replace":
			for _, idx := range r.re.FindAllStringIndex(result, -1) {
				hits = append(hits, BadWordHit{RuleID: r.id, RuleName: r.name, Action: r.action, Matched: result[idx[0]:idx[1]], Index: idx[0]})
			}
			result = r.re.ReplaceAllString(result, r.replace)
		case "delete":
			for _, idx := range r.re.FindAllStringIndex(result, -1) {
				hits = append(hits, BadWordHit{RuleID: r.id, RuleName: r.name, Action: r.action, Matched: result[idx[0]:idx[1]], Index: idx[0]})
			}
			result = r.re.ReplaceAllString(result, "")
		default: // mark：只记录命中，不改文本
			for _, idx := range r.re.FindAllStringIndex(content, -1) {
				hits = append(hits, BadWordHit{RuleID: r.id, RuleName: r.name, Action: r.action, Matched: content[idx[0]:idx[1]], Index: idx[0]})
			}
		}
	}
	return result, hits
}
