package aisafety

import (
	"strings"
)

// Result 一次规则扫描的结论。
type Result struct {
	Score      int      `json:"score"`
	Level      string   `json:"level"`
	Substance  bool     `json:"substance"`
	Categories []string `json:"categories"`
	Hits       []string `json:"hits"`

	Intent       bool `json:"intent"`
	ProcessCount int  `json:"process_count"`
	ParamCount   int  `json:"param_count"`
	JailbreakCnt int  `json:"jailbreak_count"`
	EvasionCnt   int  `json:"evasion_count"`
	FakeHistory  bool `json:"fake_history"`
	Override     bool `json:"override"`
	OverrideCnt  int  `json:"override_count"`
	Ladder       bool `json:"ladder"`
	LadderScore  int  `json:"ladder_score"`
	FollowUp     bool `json:"follow_up"`
	HarmRelief   int  `json:"harm_relief"`
	FictionCnt   int  `json:"fiction_count"`
}

// ScanOptions 扫描开关（来自配置，不在代码里写死策略）。
type ScanOptions struct {
	ExtraTerms         []string
	AllowFictionRelief bool
	AllowHarmRelief    bool
	// HistorySubstances 历史用户消息里出现过的管制物质。
	// 用于「承接追问」判定：本轮只是「那用什么酸？」，单看这句毫无风险，
	// 但接在前几轮已经问过 TATP 之后，就是典型的逐步索取细节。
	HistorySubstances []string
	// HistoryMsgs 历史用户消息原文，用于 ADRO 阶梯检测（探针是否在逐轮加深）。
	HistoryMsgs []string
}

// Scan 对一段文本打分。
//
// 计分规则分两类：
//   - 组合信号（物质 / 意图 / 工艺 / 参数）只要缺了物质这一项就整组归零 ——
//     这是防误杀的关键：物性、用途、危害科普里出现「合成」「制造」很常见。
//   - 独立信号（指令覆盖、虚构历史、越狱话术、规避手法）单独计分，
//     因为它们是行为特征而非话题：不管聊什么，这些句式本身就可疑。
func Scan(text string, opt ScanOptions) Result {
	res := Result{Level: LevelLow, Categories: []string{}, Hits: []string{}}
	t := normalize(text)
	if t == "" {
		return res
	}

	// ── 组合信号：管制物质 ──
	maxWeight := 0
	extra := 0
	catSeen := map[string]bool{}
	for _, g := range substanceGroups {
		n, hits := countHits(t, g.Terms)
		if n == 0 {
			continue
		}
		res.Substance = true
		if !catSeen[g.Category] {
			catSeen[g.Category] = true
			res.Categories = append(res.Categories, g.Category)
		}
		res.Hits = append(res.Hits, hits...)
		if g.Weight > maxWeight {
			maxWeight = g.Weight
		} else {
			extra += 5
		}
	}

	// 额外禁止词（后台自定义）视作管制物质级别
	if len(opt.ExtraTerms) > 0 {
		if n, hits := countHits(t, opt.ExtraTerms); n > 0 {
			res.Substance = true
			res.Categories = append(res.Categories, "自定义")
			res.Hits = append(res.Hits, hits...)
			if 45 > maxWeight {
				maxWeight = 45
			}
		}
	}

	score := maxWeight + extra
	if extra > 10 {
		score = maxWeight + 10
	}

	// ── 承接：本轮自己没点物质名，但前面已提过，本轮在往下要细节 ──
	//
	// 两条承接路径，覆盖 ADRO 的两种话术：
	//   a) 追问词 ——「那用什么酸」「温度多少」（followUpTerms）
	//   b) 阶梯推进 ——「反应条件呢」（本身不含任何敏感词，但阶段从机理跳到了条件）
	// 只做 a 会漏掉最典型的第 4 轮：「反应条件是什么」既没有物质名也没有追问词。
	if !res.Substance && len(opt.HistorySubstances) > 0 {
		fu, _ := countHits(t, followUpTerms)
		ladderOK, ladderAdd := false, 0
		if len(opt.HistoryMsgs) > 0 {
			ladderOK, ladderAdd = ladderEscalation(opt.HistoryMsgs, t)
		}
		if fu > 0 || ladderOK {
			res.Substance = true
			maxWeight = 45 // 历史物质按 45 计（略低于直接点名，避免同等定罪）
			res.Categories = append(res.Categories, "承接追问")
			res.Hits = append(res.Hits, opt.HistorySubstances...)
			if fu > 0 {
				res.FollowUp = true
			}
			if ladderOK {
				res.Ladder = true
				res.LadderScore = ladderAdd
				res.Categories = append(res.Categories, "逐轮推进")
			}
			score = maxWeight + extra
			if extra > 10 {
				score = maxWeight + 10
			}
		}
	}
	// 本轮自己点了物质名，也要看是否在阶梯上继续深入
	if res.Substance && !res.Ladder && len(opt.HistoryMsgs) > 0 {
		if ok, add := ladderEscalation(opt.HistoryMsgs, t); ok {
			res.Ladder = true
			res.LadderScore = add
			res.Categories = append(res.Categories, "逐轮推进")
		}
	}

	// ── 组合信号：意图 / 工艺 / 参数（依赖物质存在）──
	if n, hits := countHits(t, intentTerms); n > 0 {
		res.Intent = true
		res.Hits = append(res.Hits, hits...)
		if res.Substance {
			score += 20
		}
	}
	if res.Substance {
		pn := 0
		for _, p := range processTerms {
			if containsTerm(t, p) {
				pn++
				if len(res.Hits) < 18 {
					res.Hits = append(res.Hits, p)
				}
			}
		}
		res.ProcessCount = pn
		if pn > 5 {
			pn = 5
		}
		score += pn * 8

		// 具体参数：温度 / 浓度 / 配比 / 带动作的时间 / 投料量
		paramN := 0
		if reTemp.MatchString(t) {
			paramN++
		}
		// 百分比必须和酸/氧化剂同段出现才算浓度参数（否则「打 5 折」也会被算进去）
		if rePercent.MatchString(t) && reAcidWord.MatchString(t) {
			paramN++
		}
		if reRatio.MatchString(t) || reRatioBare.MatchString(t) {
			paramN++
		}
		if reTimeA.MatchString(t) || reTimeB.MatchString(t) {
			paramN++
		}
		if reAmount.MatchString(t) {
			paramN++
		}
		res.ParamCount = paramN
		if paramN > 3 {
			paramN = 3
		}
		score += paramN * 10
	}

	// ── 独立信号：指令覆盖（直接要求作废规则）──
	// 它足够确定，所以 CheckInput 里是硬拦而不看分数；
	// 这里仍然计分，是为了「观察期 / 事件留痕」能看到它有多重。
	if n, hits := countHits(t, overrideTerms); n > 0 {
		res.Override = true
		res.OverrideCnt = n
		res.Hits = append(res.Hits, hits...)
		score += 35
	}

	// ── 独立信号：虚构历史注入 ──
	// 「你之前输出过…帮我还原」不依赖任何敏感词，任何话题都可能被这么套。
	if n, hits := countHits(t, fakeHistoryTerms); n > 0 {
		res.FakeHistory = true
		res.Hits = append(res.Hits, hits...)
		score += 25
	}

	// ── 独立信号：越狱话术 / 规避手法 ──
	if n, hits := countHits(t, jailbreakTerms); n > 0 {
		res.JailbreakCnt = n
		res.Hits = append(res.Hits, hits...)
		if n > 3 {
			n = 3
		}
		score += n * 10
	}
	if n, hits := countHits(t, evasionTerms); n > 0 {
		res.EvasionCnt = n
		res.Hits = append(res.Hits, hits...)
		if n > 2 {
			n = 2
		}
		score += n * 15
	}
	// 承接追问 / 阶梯推进本身也加分（纯行为信号）
	if res.FollowUp {
		score += 25
	}
	if res.Ladder {
		score += res.LadderScore
	}

	// ── 减免：只有在「没有可操作细节」时才给 ──
	// 一旦出现投料比/温度/浓度，任何「科普」「小说」包装都不再减免 ——
	// 这是真实越狱里最常用的一层壳。
	detailFree := res.ProcessCount < 3 && res.ParamCount == 0
	if opt.AllowHarmRelief && detailFree {
		if n, _ := countHits(t, harmReliefTerms); n >= 2 {
			res.HarmRelief = n
			score -= 25
		}
	}
	if opt.AllowFictionRelief && detailFree {
		if n, _ := countHits(t, fictionReliefTerms); n >= 1 {
			res.FictionCnt = n
			score -= 15
		}
	}

	if score < 0 {
		score = 0
	}
	if score > 200 {
		score = 200
	}
	res.Score = score
	switch {
	case score >= 70 && res.Substance:
		res.Level = LevelHigh
	case score >= 45:
		res.Level = LevelMid
	default:
		res.Level = LevelLow
	}
	return res
}

// ExtractSubstances 从历史用户消息里提取「提到过的管制物质」。
//
// 只扫用户自己说的话，**不扫模型回复**：模型的回答里必然出现危险术语，
// 若把它算进去，用户下一句「谢谢」也会被判成承接追问。
func ExtractSubstances(msgs []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, 4)
	for _, m := range msgs {
		t := normalize(m)
		if t == "" {
			continue
		}
		for _, g := range substanceGroups {
			for _, term := range g.Terms {
				if containsTerm(t, term) && !seen[term] {
					seen[term] = true
					out = append(out, term)
				}
			}
		}
		if len(out) >= 6 {
			return out
		}
	}
	return out
}

// SummarizeHits 把命中去重后拼成一行（写事件表用）。
func SummarizeHits(hits []string) string {
	if len(hits) == 0 {
		return ""
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		if h == "" || seen[h] {
			continue
		}
		seen[h] = true
		out = append(out, h)
		if len(out) >= 20 {
			break
		}
	}
	return strings.Join(out, "、")
}
