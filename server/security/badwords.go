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

// 内置自残/抑郁倾向黑话分类（详见 Task #34：未成年人心理危机防护）。
// 该分类属于高危，命中即升组合标签、强制拦截/标记高危并通知管理员。
const CatSelfHarm = "涉自残/抑郁倾向黑话"

// builtinSelfHarmWords 内置、常驻生效的自残/抑郁倾向黑话词表。
//
// 与后台动态词表（SetDynamicBadWords）分开存储、一并扫描：动态表由管理员维护，
// 可能为空；内置表保证「浮木/滚木/紫砂」等已确认的黑话在任何配置下都生效，
// 且动作统一为 mark（只标记不删文，避免误删且交人工/AI 复核）。
// 范围限定在 comment/forum，避免小说正文正常出现「紫砂壶」等被无谓标记。
var builtinSelfHarmWords = []DynamicBadWord{
	{Name: "自残黑话-浮木", Category: CatSelfHarm, MatchType: "literal", Pattern: "浮木", Action: "mark", Scope: "comment,forum"},
	{Name: "自残黑话-滚木", Category: CatSelfHarm, MatchType: "literal", Pattern: "滚木", Action: "mark", Scope: "comment,forum"},
	{Name: "自残黑话-紫砂", Category: CatSelfHarm, MatchType: "literal", Pattern: "紫砂", Action: "mark", Scope: "comment,forum"},
	{Name: "自残黑话-自挂东南枝", Category: CatSelfHarm, MatchType: "literal", Pattern: "自挂东南枝", Action: "mark", Scope: "comment,forum"},
	{Name: "危机-我想死", Category: CatSelfHarm, MatchType: "literal", Pattern: "我想死", Action: "mark", Scope: "comment,forum"},
	{Name: "危机-不想活", Category: CatSelfHarm, MatchType: "literal", Pattern: "不想活", Action: "mark", Scope: "comment,forum"},
	{Name: "危机-活不下去", Category: CatSelfHarm, MatchType: "literal", Pattern: "活不下去", Action: "mark", Scope: "comment,forum"},
	{Name: "危机-结束自己", Category: CatSelfHarm, MatchType: "literal", Pattern: "结束自己", Action: "mark", Scope: "comment,forum"},
	{Name: "危机-轻生", Category: CatSelfHarm, MatchType: "literal", Pattern: "轻生", Action: "mark", Scope: "comment,forum"},
	{Name: "危机-自杀", Category: CatSelfHarm, MatchType: "literal", Pattern: "自杀", Action: "mark", Scope: "comment,forum"},
	{Name: "危机-自残", Category: CatSelfHarm, MatchType: "literal", Pattern: "自残", Action: "mark", Scope: "comment,forum"},
	{Name: "危机-伤害自己", Category: CatSelfHarm, MatchType: "literal", Pattern: "伤害自己", Action: "mark", Scope: "comment,forum"},
	{Name: "危机-割腕", Category: CatSelfHarm, MatchType: "literal", Pattern: "割腕", Action: "mark", Scope: "comment,forum"},
	{Name: "危机-割手", Category: CatSelfHarm, MatchType: "literal", Pattern: "割手", Action: "mark", Scope: "comment,forum"},
	{Name: "危机-安眠药", Category: CatSelfHarm, MatchType: "literal", Pattern: "安眠药", Action: "mark", Scope: "comment,forum"},
	{Name: "危机-走饭", Category: CatSelfHarm, MatchType: "literal", Pattern: "走饭", Action: "mark", Scope: "comment,forum"},
}

var (
	badWordsMu      sync.RWMutex
	badWords        []compiledBadWord
	builtinBadWords []compiledBadWord
	builtinOnce     sync.Once
)

// initBuiltinBadWords 编译内置黑话词表（只做一次）。
func initBuiltinBadWords() {
	builtinOnce.Do(func() {
		compiled := make([]compiledBadWord, 0, len(builtinSelfHarmWords))
		for _, w := range builtinSelfHarmWords {
			pattern := w.Pattern
			if pattern == "" {
				continue
			}
			if w.MatchType == "" || w.MatchType == "literal" {
				pattern = regexp.QuoteMeta(pattern)
			}
			re, err := regexp.Compile(pattern)
			if err != nil {
				continue
			}
			action := w.Action
			if action == "" {
				action = "mark"
			}
			c := compiledBadWord{
				id:       0,
				name:     w.Name,
				category: w.Category,
				re:       re,
				action:   action,
				replace:  w.Replacement,
				all:      w.Scope == "" || w.Scope == "all",
				scopes:   make(map[string]bool),
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
		builtinBadWords = compiled
		badWordsMu.Unlock()
	})
}

// ScanSelfHarm 仅扫描指定内容类型中的「涉自残/抑郁倾向黑话」命中（用于未成年人心理危机获取）。
func ScanSelfHarm(content, scope string) []BadWordHit {
	content, _ = TruncateForScan(content)
	initBuiltinBadWords()
	badWordsMu.RLock()
	rules := builtinBadWords
	badWordsMu.RUnlock()
	var hits []BadWordHit
	if len(rules) == 0 || content == "" {
		return nil
	}
	for _, r := range rules {
		if !r.inScope(scope) {
			continue
		}
		for _, idx := range r.re.FindAllStringIndex(content, -1) {
			hits = append(hits, BadWordHit{
				RuleID:   0,
				RuleName: r.name,
				Category: r.category,
				Action:   r.action,
				Matched:  content[idx[0]:idx[1]],
				Index:    idx[0],
			})
		}
	}
	return hits
}

// DynamicBadWord 一条违禁词规则（由 models/handler 注入）
type DynamicBadWord struct {
	ID          uint
	Name        string
	Category    string // 分类：政治/色情/暴力/赌博/诈骗/广告/违禁品/辱骂/其他
	Tags        string // 组合标签（逗号分隔）
	MatchType   string // literal / regex
	Pattern     string
	Action      string // mark / delete / replace
	Replacement string
	Scope       string // all / chapter,comment,forum,blog
}

type compiledBadWord struct {
	id       uint
	name     string
	category string
	re       *regexp.Regexp
	action   string
	replace  string
	all      bool // scope 是否覆盖全部内容类型
	scopes   map[string]bool
}

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
			id:       w.ID,
			name:     w.Name,
			category: w.Category,
			re:       re,
			action:   action,
			replace:  w.Replacement,
			all:      w.Scope == "" || w.Scope == "all",
			scopes:   make(map[string]bool),
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
	Category string `json:"category"` // 该规则的分类（政治/色情/…）
	Action   string `json:"action"`   // mark / delete / replace
	Matched  string `json:"matched"`  // 命中的文本片段
	Index    int    `json:"index"`    // 命中起始位置
}

// ruleInScope 判断规则是否作用于指定内容类型
func (c *compiledBadWord) inScope(scope string) bool {
	if c.all {
		return true
	}
	return scope != "" && c.scopes[scope]
}

// ScanBadWords 扫描指定内容类型中的违禁词，返回全部命中（不修改文本）。
// 同时扫描后台动态词表与内置常驻黑话词表（见 builtinBadWords）。
// 正文超过 MaxScanBytes 时仅扫描前 1MB（资源耗尽防护，见 userpattern.go）。
func ScanBadWords(content, scope string) []BadWordHit {
	content, _ = TruncateForScan(content)
	initBuiltinBadWords()
	badWordsMu.RLock()
	dyn := badWords
	builtin := builtinBadWords
	badWordsMu.RUnlock()
	if (len(dyn) == 0 && len(builtin) == 0) || content == "" {
		return nil
	}
	var hits []BadWordHit
	scanSet := func(rules []compiledBadWord) {
		for _, r := range rules {
			if !r.inScope(scope) {
				continue
			}
			for _, idx := range r.re.FindAllStringIndex(content, -1) {
				hits = append(hits, BadWordHit{
					RuleID:   r.id,
					RuleName: r.name,
					Category: r.category,
					Action:   r.action,
					Matched:  content[idx[0]:idx[1]],
					Index:    idx[0],
				})
			}
		}
	}
	scanSet(dyn)
	scanSet(builtin)
	return hits
}

// ApplyBadWords 应用 delete/replace 动作净化文本；mark 规则不修改文本。
// 返回：净化后的文本 + 全部命中（含 mark，命中基于净化前原文）。
// 正文超过 MaxScanBytes 时仅处理前 1MB，后续原样保留（资源耗尽防护）。
func ApplyBadWords(content, scope string) (string, []BadWordHit) {
	scanned, truncated := TruncateForScan(content)
	initBuiltinBadWords()
	badWordsMu.RLock()
	dyn := badWords
	builtin := builtinBadWords
	badWordsMu.RUnlock()
	if (len(dyn) == 0 && len(builtin) == 0) || content == "" {
		return content, nil
	}
	var hits []BadWordHit
	result := scanned
	for _, r := range dyn {
		if !r.inScope(scope) {
			continue
		}
		switch r.action {
		case "replace":
			for _, idx := range r.re.FindAllStringIndex(result, -1) {
				hits = append(hits, BadWordHit{RuleID: r.id, RuleName: r.name, Category: r.category, Action: r.action, Matched: result[idx[0]:idx[1]], Index: idx[0]})
			}
			result = r.re.ReplaceAllString(result, r.replace)
		case "delete":
			for _, idx := range r.re.FindAllStringIndex(result, -1) {
				hits = append(hits, BadWordHit{RuleID: r.id, RuleName: r.name, Category: r.category, Action: r.action, Matched: result[idx[0]:idx[1]], Index: idx[0]})
			}
			result = r.re.ReplaceAllString(result, "")
		default: // mark：只记录命中，不改文本
			for _, idx := range r.re.FindAllStringIndex(scanned, -1) {
				hits = append(hits, BadWordHit{RuleID: r.id, RuleName: r.name, Category: r.category, Action: r.action, Matched: scanned[idx[0]:idx[1]], Index: idx[0]})
			}
		}
	}
	// 内置黑话词表：统一 mark（只记录、不改文本），交人工/AI 复核，避免误删正常内容
	for _, r := range builtin {
		if !r.inScope(scope) {
			continue
		}
		for _, idx := range r.re.FindAllStringIndex(scanned, -1) {
			hits = append(hits, BadWordHit{RuleID: r.id, RuleName: r.name, Category: r.category, Action: r.action, Matched: scanned[idx[0]:idx[1]], Index: idx[0]})
		}
	}
	if truncated {
		// 未扫描的尾部原样拼回，保证内容完整（delete/replace 的偏移已按前段计算，不受影响）
		result += content[MaxScanBytes:]
	}
	return result, hits
}

// ═══ 违禁词组合分类 ═══
// 不同类型违规同时命中时，按「组合」给出更高层级的归类，便于运营分级处置。
// 例：政治 + 暴力 → 「高危·政治暴力」；涉赌 + 诈骗 → 「高危·诈骗赌博」。
var (
	// 高危组合：命中任一即升为「高危」
	highRiskCategories = map[string]bool{
		"政治": true, "暴恐": true, "违禁品": true, "诈骗": true, "赌博": true,
		// 自残/抑郁倾向黑话：未成年人心理危机红线，命中即升高危、强制拦截+标记+通知管理员
		CatSelfHarm: true,
	}
	// 低危/运营类：单独命中不升高危
	lowRiskCategories = map[string]bool{
		"色情": true, "暴力": true, "广告": true, "辱骂": true, "其他": true,
	}
)

// ClassifyCombo 将一组（去重后的）分类归并为组合标签。
// 返回：组合后的分类标签（如「政治,暴力」「高危·政治暴力」）、是否高危、参与组合的原始分类列表。
func ClassifyCombo(categories []string) (combined string, highRisk bool, parts []string) {
	seen := make(map[string]bool)
	for _, c := range categories {
		c = strings.TrimSpace(c)
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		parts = append(parts, c)
		if highRiskCategories[c] {
			highRisk = true
		}
	}
	switch len(parts) {
	case 0:
		return "未分类", false, parts
	case 1:
		return parts[0], highRiskCategories[parts[0]], parts
	default:
		label := strings.Join(parts, ",")
		if highRisk {
			return "高危·" + label, true, parts
		}
		return label, false, parts
	}
}
