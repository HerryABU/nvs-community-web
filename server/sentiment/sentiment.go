// Package sentiment 社区舆情守卫的对外门面。
//
// 职责很窄：读配置 → 拿词表 → 跑裁决 → 落 hit 记录。
// 真正的判定逻辑都在子包里（topic / lexicon / guard），这里只做编排，
// 避免变成又一坨「什么都管」的上帝类。
package sentiment

import (
	"strings"

	"nvs-server/sentiment/guard"
	"nvs-server/sentiment/lexicon"
	"nvs-server/sentiment/store"
	"nvs-server/sentiment/topic"
)

// ActionBlock 暴露给接入层做分支判断（避免各处硬编码字符串）。
const ActionBlock = guard.ActionBlock

// Reassess 把本次文本的升档信号并回内容话题评估。
//
// 内容的话题组来自 AI 标注（稳定），但「这次是不是在组织化出征」只能看文本。
// 只升不降：文本里没信号不代表话题风险变了。
func Reassess(a topic.Assessment, text string) topic.Assessment {
	if len(a.Tags) == 0 {
		return a
	}
	n := topic.Assess(a.Tags, text)
	if n.Risk > a.Risk {
		a.Risk = n.Risk
	}
	if len(n.Severity) > 0 {
		a.Severity = append(a.Severity, n.Severity...)
	}
	if n.PublicOpinionScore > a.PublicOpinionScore {
		a.PublicOpinionScore = n.PublicOpinionScore
		a.PublicOpinionHigh = n.PublicOpinionHigh
	}
	return a
}

// Meta 一次裁决的上下文信息（写 hit 记录用）。
type Meta struct {
	ContentType string
	ContentID   uint
	CommentID   uint
	UserID      uint
	Username    string
}

// Evaluate 只做裁决、不落库。
//
// 拆成 Evaluate / Record 两步的原因：评论是「先拦后建」，
// 建之前拿不到 comment id，而记录里带上 id 才能从一条违规评论反查到证据。
//
// 置信度与策略在这里收口（对应评审的三条边界）：
//   - 没有任何话题标注 → 按低风险处理：话题条件全部失效，只剩「强制词 + 刷屏行为」能拦
//     （不能凭「没标签」去拦人，但也不能让没打标的评论区完全裸奔）；
//   - 策略 off → 不处置；flag → 最高只到标注；block → 有命中就拦；
//   - **置信度低于门槛 / 待人工复核 → 拦截降级为标注**，绝不拿不可信的标签去删评论。
func Evaluate(state store.TopicState, text string) guard.Decision {
	cfg, err := store.GetConfig()
	if err != nil || cfg == nil || !cfg.Enabled {
		return guard.Decision{Action: guard.ActionAllow, Risk: state.Assessment.Risk, Groups: state.Assessment.Groups}
	}
	if state.Policy == store.PolicyOff {
		return guard.Decision{Action: guard.ActionAllow, Risk: state.Assessment.Risk, Groups: state.Assessment.Groups}
	}

	terms, _ := store.ActiveLexicon()
	d := guard.Evaluate(text, state.Assessment, guard.Options{
		BlockRisk:     topic.ParseRisk(cfg.BlockRisk),
		FlagRisk:      topic.ParseRisk(cfg.FlagRisk),
		FloodBlock:    cfg.FloodBlock,
		BlockSeverity: 2,
	}, terms)

	// 策略强制：block 时只要有命中就拦（站长已人工确认过这个话题）
	if state.Policy == store.PolicyBlock && d.Action != guard.ActionBlock && len(d.Hits) > 0 {
		d.Action = guard.ActionBlock
		d.Blocked = append(d.Blocked, d.Flagged...)
		d.Flagged = nil
		d.Reason = "话题策略=强制拦截：" + d.Reason
		return d
	}

	if d.Action != guard.ActionBlock {
		if state.Policy == store.PolicyFlag && d.Action == guard.ActionAllow && len(d.Hits) > 0 {
			d.Action = guard.ActionFlag
			d.Flagged = wordList(d.Hits)
			d.Reason = "话题策略=仅标注：" + strings.Join(d.Flagged, "、")
		}
		return d
	}

	// 到这里 d.Action == block：检查这份标注可不可信
	why := ""
	if state.Policy == store.PolicyFlag {
		why = "话题策略=仅标注"
	} else if state.ReviewStatus == store.ReviewNeedsReview {
		why = "标签待人工复核"
	} else if state.Confidence > 0 && state.Confidence < cfg.MinConfidence {
		why = "标签置信度 " + itoa(state.Confidence) + " 低于门槛 " + itoa(cfg.MinConfidence)
	}
	if why != "" {
		// 降级为标注：宁可漏一次，也不拿不可信的标签去删用户评论
		d.Blocked, d.Flagged = nil, append(d.Flagged, wordList(d.Hits)...)
		d.Action = guard.ActionFlag
		d.Reason = why + "，已降级为标注转人工：" + d.Reason
	}
	return d
}

// wordList 取命中词（去重）。
func wordList(hits []lexicon.Hit) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		if seen[h.Word] {
			continue
		}
		seen[h.Word] = true
		out = append(out, h.Word)
	}
	return out
}

// itoa 小工具，避免为一个数字引入 strconv 到门面层。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}

// Record 落一条 hit 记录。记录失败绝不能影响评论主流程。
func Record(d guard.Decision, meta Meta) {
	if d.Action == guard.ActionAllow && len(d.Hits) == 0 {
		return
	}
	_ = store.RecordHit(&store.Hit{
		ContentType:  meta.ContentType,
		ContentID:    meta.ContentID,
		CommentID:    meta.CommentID,
		UserID:       meta.UserID,
		Username:     meta.Username,
		Decision:     d.Action,
		Risk:         d.Risk.String(),
		Groups:       groupsCSV(d.Groups),
		MatchedWords: strings.Join(append(append([]string{}, d.Blocked...), d.Flagged...), ","),
		VariantWords: strings.Join(d.Variant, ","),
		Flood:        floodSummary(d),
		Reason:       d.Reason,
	})
}

// groupsCSV 话题组转逗号串（统计用）。
func groupsCSV(gs []topic.Group) string {
	if len(gs) == 0 {
		return ""
	}
	out := make([]string, 0, len(gs))
	for _, g := range gs {
		out = append(out, string(g))
	}
	return strings.Join(out, ",")
}

// floodSummary 只记「触发了哪些特征」，不记原文（避免把用户原文再存一遍）。
func floodSummary(d guard.Decision) string {
	var parts []string
	if d.Flood.Mentions >= 5 {
		parts = append(parts, "@多人")
	}
	if d.Flood.Repeated >= 4 {
		parts = append(parts, "短语重复")
	}
	if d.Flood.PunctuationRun >= 5 {
		parts = append(parts, "连续标点")
	}
	if d.Flood.Shouting {
		parts = append(parts, "叠字喊话")
	}
	return strings.Join(parts, ",")
}
