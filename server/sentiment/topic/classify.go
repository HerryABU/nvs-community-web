package topic

import (
	"encoding/json"
	"sort"
	"strings"
)

// Assessment 一次话题评估结果。由 AI 给的话题标签 + 文本本身共同推出。
type Assessment struct {
	// Tags 原始标签（AI 敲定的）
	Tags []string `json:"tags"`
	// Groups 归类后的话题组（去重、有序）
	Groups []Group `json:"groups"`
	// BaseRisk 仅由话题标签得出的基线风险
	BaseRisk Risk `json:"base_risk"`
	// Risk 叠加组合规则与文本升档信号后的最终风险
	Risk Risk `json:"risk"`
	// Severity 文本中的升档信号（动员 / 群体对立 / 极端化）
	Severity []string `json:"severity"`
	// PublicOpinionScore 高风险舆论可能性 0-100
	PublicOpinionScore int `json:"public_opinion_score"`
	// PublicOpinionHigh 是否达到「高风险舆论」门槛（>=70）
	PublicOpinionHigh bool `json:"public_opinion_high"`
	// Reasons 判定理由（后台展示，便于人复核）
	Reasons []string `json:"reasons"`
}

// severitySignals 会抬高风险的文本信号。
// 这三类不需要「敏感词」那么严：它们描述的是「动作」，不是「身份」，
// 命中即说明这段话在组织、站队或煽动。
var severitySignals = []struct {
	label string
	words []string
}{
	{"动员组织", []string{
		"出征", "冲了他", "冲他", "挂人", "人肉", "开盒", "举报到", "集体举报",
		"抵制", "曝光", "组队", "集合", "@全体", "报团", "围攻", "刷屏", "控评",
		"举报走起", "一起去", "都去", "给我们上",
	}},
	{"群体对立", []string{
		"女拳", "男拳", "田园女权", "蝈蝻", "小仙女", "龟男", "直男癌", "婚驴",
		"汉奸", "公知", "走狗", "五毛", "美分", "殖人", "精日", "皇汉",
	}},
	{"极端化", []string{
		"都该死", "杀光", "灭掉", "清除", "物理消灭", "打死", "弄死",
	}},
}

// comboRules 话题组合的风险改写。顺序无关，取命中项里最高的。
//
// 产品口径来自实际运营经验：单看一个话题都能吵，但「亚文化 + 政治批判」
// 这种组合才会真正组织化（圈层动员 + 立场对抗），所以直接顶到高。
var comboRules = []struct {
	groups []Group
	risk   Risk
	label  string
}{
	{[]Group{GroupSubculture, GroupPolitics}, RiskHigh, "亚文化 × 政治批判"},
	{[]Group{GroupSubculture, GroupGender}, RiskHigh, "亚文化 × 性别议题"},
	{[]Group{GroupGender, GroupPolitics}, RiskHigh, "性别议题 × 政治批判"},
	{[]Group{GroupPolitics, GroupCapital}, RiskMidHigh, "政治批判 × 资本批判"},
	{[]Group{GroupSubculture, GroupCapital}, RiskMidHigh, "亚文化 × 资本批判"},
	{[]Group{GroupGender, GroupCapital}, RiskHigh, "性别议题 × 资本批判"},
	{[]Group{GroupRegion, GroupPolitics}, RiskHigh, "地域民族 × 政治批判"},
}

// Classify 标签 → 话题组（去重、按固定顺序）。
func Classify(tags []string) []Group {
	seen := map[Group]bool{}
	out := []Group{}
	for _, t := range tags {
		for _, g := range GroupsOfTag(t) {
			if !seen[g] {
				seen[g] = true
				out = append(out, g)
			}
		}
	}
	// 固定顺序，保证同样的输入永远得到同样的切片顺序（便于缓存与断言）
	order := map[Group]int{
		GroupSubculture: 0, GroupGender: 1, GroupPolitics: 2,
		GroupCapital: 3, GroupRegion: 4, GroupOther: 5,
	}
	sort.SliceStable(out, func(i, j int) bool { return order[out[i]] < order[out[j]] })
	return out
}

// riskOfGroups 计算话题组合后的风险（不含文本升档）。
func riskOfGroups(groups []Group) (Risk, []string) {
	if len(groups) == 0 {
		return RiskLow, nil
	}
	var reasons []string
	risk := RiskLow
	for _, g := range groups {
		if r := BaseRisk(g); r > risk {
			risk = r
		}
	}
	// 两个以上争议话题并存，至少中高：多话题交叉是争吵升级的强预兆
	if len(groups) >= 2 && risk < RiskMidHigh {
		risk = RiskMidHigh
		reasons = append(reasons, "多话题交叉（≥2 组）")
	}
	has := func(g Group) bool {
		for _, x := range groups {
			if x == g {
				return true
			}
		}
		return false
	}
	for _, rule := range comboRules {
		ok := true
		for _, g := range rule.groups {
			if !has(g) {
				ok = false
				break
			}
		}
		if ok && rule.risk > risk {
			risk = rule.risk
			reasons = append(reasons, rule.label)
		}
	}
	return risk, reasons
}

// bump 升一档，封顶 RiskHigh。
func bump(r Risk) Risk {
	if r >= RiskHigh {
		return RiskHigh
	}
	return r + 1
}

// Assess 综合评估：话题标签给基线，文本信号负责升档，并给出舆论可能性。
func Assess(tags []string, text string) Assessment {
	groups := Classify(tags)
	base, reasons := riskOfGroups(groups)
	risk := base

	lower := strings.ToLower(text)
	var sev []string
	for _, sig := range severitySignals {
		for _, w := range sig.words {
			if strings.Contains(lower, w) {
				sev = append(sev, sig.label+":"+w)
				risk = bump(risk)
				reasons = append(reasons, "升档信号 "+sig.label+"（"+w+"）")
				// 同一类信号只升一档，否则一段骂战文本会被连升三级
				break
			}
		}
	}

	score := int(risk) * 27 // low 0 / mid 27 / midhigh 54 / high 81
	for _, s := range sev {
		if strings.HasPrefix(s, "动员组织") {
			score += 12
		}
		if strings.HasPrefix(s, "群体对立") {
			score += 8
		}
		if strings.HasPrefix(s, "极端化") {
			score += 10
		}
	}
	if score > 100 {
		score = 100
	}

	return Assessment{
		Tags:               tags,
		Groups:             groups,
		BaseRisk:           base,
		Risk:               risk,
		Severity:           sev,
		PublicOpinionScore: score,
		PublicOpinionHigh:  score >= 70,
		Reasons:            reasons,
	}
}

// ParseTagsJSON 解析库里存的 JSON 数组（AI 敲定的标签）。
// 解析失败返回 nil —— 标签坏了不该让整条评论审核失败，按无标签走。
func ParseTagsJSON(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}
