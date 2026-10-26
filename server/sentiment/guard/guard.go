// Package guard 评论区舆情裁决：把「话题风险」和「命中词」合成一个动作。
//
// 核心口径（来自产品要求）：
//   - 话题为 X 时，才对该话题下的 Y/Z 类词**拦截**；
//   - 话题不到线时，只**标注**，不拦 —— 不能因为一句话里有「乐子」就删评论。
//
// 本包保持纯函数：不读库、不写库，输入输出都可穷举，便于单测与灰盒回归。
package guard

import (
	"strings"

	"nvs-server/sentiment/lexicon"
	"nvs-server/sentiment/topic"
)

// Options 判定参数（来自库里的 sentiment_configs，代码不留策略字面量）。
type Options struct {
	// BlockRisk 允许「升级为拦截」的风险门槛
	BlockRisk topic.Risk
	// FlagRisk 触发标注的风险门槛
	FlagRisk topic.Risk
	// FloodBlock 是否自动拦截刷屏 / 出征型行为
	FloodBlock bool
	// BlockSeverity 允许升级为拦截的最低词级（默认 2：辱骂/出征/蔑称；1=黑话只标注）。
	// 没有这一层，「亚文化话题 + 乐子」就会被判拦截 —— 等于把整个二次元评论区删光。
	BlockSeverity int
}

// Decision 一次裁决结果。
type Decision struct {
	Action  string
	Reason  string
	Risk    topic.Risk
	Groups  []topic.Group
	Hits    []lexicon.Hit
	Blocked []string // 升级为拦截的词
	Flagged []string // 仅标注的词
	Variant []string // 靠变形命中的词（取证用）
	Flood   lexicon.Flood
}

// 动作常量与 store 保持一致（不互相 import，避免 store→guard→store 的环）。
const (
	ActionAllow = "allow"
	ActionFlag  = "flag"
	ActionBlock = "block"
)

// isFlood 判定行为特征是否达到「刷屏 / 出征」。
//
// 阈值刻意保守：@ 5 人以上、同一短语重复 4 次以上且伴随长串感叹号、
// 或叠字喊话且带重复——正常用户表达激动不会同时满足这些。
func isFlood(f lexicon.Flood) bool {
	if f.Mentions >= 5 {
		return true
	}
	if f.Repeated >= 4 && f.PunctuationRun >= 5 {
		return true
	}
	if f.Shouting && f.Repeated >= 3 {
		return true
	}
	return false
}

// intersects 词条绑定的话题组是否与内容话题相交。
func intersects(a []topic.Group, b []topic.Group) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	for _, x := range a {
		for _, y := range b {
			if x == y {
				return true
			}
		}
	}
	return false
}

// Evaluate 执行裁决。
//
// 判定顺序（先硬后软，避免被「话题低风险 + 强制词」绕过）：
//  1. 有 ForceBlock 词 → 直接拦；
//  2. 有话题耦合词且话题风险达 BlockRisk → 拦；
//  3. 刷屏 / 出征行为达阈值且开启 → 拦；
//  4. 有命中且话题风险达 FlagRisk → 标注放行；
//  5. 其余 → 放行。
func Evaluate(text string, a topic.Assessment, opts Options, terms []lexicon.Term) Decision {
	d := Decision{Action: ActionAllow, Risk: a.Risk, Groups: a.Groups}

	hits := lexicon.ScanWith(terms, text)
	d.Hits = hits
	d.Flood = lexicon.DetectFlood(text)

	minSev := opts.BlockSeverity
	if minSev <= 0 {
		minSev = 2 // 默认：黑话(1) 不升级，辱骂/出征/蔑称(2+) 才可能升级
	}

	var forced, coupled, others []string
	for _, h := range hits {
		if h.Variant {
			d.Variant = append(d.Variant, h.Word)
		}
		switch {
		case h.ForceBlock:
			forced = append(forced, h.Word)
		case intersects(h.EscalateUnder, a.Groups):
			coupled = append(coupled, h.Word)
		case h.Severity >= minSev:
			// 词本身有攻击性，但没绑话题：靠「话题风险达线」来决定升不升级
			coupled = append(coupled, h.Word)
		default:
			others = append(others, h.Word)
		}
	}

	flood := d.Flood
	floodHit := isFlood(flood)

	// 1. 强制词
	if len(forced) > 0 {
		d.Action = ActionBlock
		d.Blocked = forced
		d.Flagged = append(coupled, others...)
		d.Reason = "强制拦截词：" + strings.Join(forced, "、")
		return d
	}

	// 2. 话题条件：达线才拦（这是本功能的核心分支）
	if len(coupled) > 0 && a.Risk >= opts.BlockRisk {
		d.Action = ActionBlock
		d.Blocked = coupled
		d.Flagged = others
		d.Reason = "话题风险 " + a.Risk.String() + "（" + groupsText(a.Groups) + "）下命中：" + strings.Join(coupled, "、")
		return d
	}

	// 3. 行为型刷屏 / 出征
	if floodHit && opts.FloodBlock {
		d.Action = ActionBlock
		d.Blocked = append(coupled, others...)
		d.Reason = "刷屏/出征行为：" + floodText(flood)
		return d
	}

	// 4. 标注（不拦）
	if len(hits) > 0 && a.Risk >= opts.FlagRisk {
		d.Action = ActionFlag
		d.Flagged = append(coupled, others...)
		d.Reason = "话题风险 " + a.Risk.String() + " 下命中（仅标注）：" + strings.Join(d.Flagged, "、")
		return d
	}
	// 有命中但话题风险极低：连标注都不做，只把证据留在 hits 里
	if len(hits) > 0 {
		d.Flagged = append(coupled, others...)
		d.Reason = "话题风险低，命中仅留证不处置"
	}
	return d
}

// groupsText 话题组的中文名拼接。
func groupsText(gs []topic.Group) string {
	if len(gs) == 0 {
		return "无话题"
	}
	out := make([]string, 0, len(gs))
	for _, g := range gs {
		out = append(out, g.Label())
	}
	return strings.Join(out, "+")
}

// floodText 行为特征的可读描述。
func floodText(f lexicon.Flood) string {
	var parts []string
	if f.Mentions >= 5 {
		parts = append(parts, "@多人")
	}
	if f.Repeated >= 4 {
		parts = append(parts, "短语重复")
	}
	if f.PunctuationRun >= 5 {
		parts = append(parts, "连续标点")
	}
	if f.Shouting {
		parts = append(parts, "叠字喊话")
	}
	if len(parts) == 0 {
		return "轻微"
	}
	return strings.Join(parts, "/")
}

// FoldThreshold 长文本折叠：返回该不该折叠（供前端调用，口径统一在服务端）。
func FoldThreshold(text string, foldChars int) bool {
	if foldChars <= 0 {
		return false
	}
	return len([]rune(text)) > foldChars
}
