package security

import "strings"

// 低落/绝望语义信号词（自残黑话之外的「粗筛」层）。
//
// 用途：把「值得送给 AI 语义分析」的评论挑出来。
// 为什么不直接对每条评论调模型：一次模型调用几秒、几厘钱，
// 全站每条评论都送审既慢又贵，而这层信号足够把成本压到与真实困扰量成正比。
//
// 与 builtinSelfHarmWords 的分工：
//   - builtinSelfHarmWords 是「黑话」层：命中即高危，直接处置，不问 AI；
//   - 这里是「语义」层：只做粗筛，命中才请 AI 判断是不是真的绝望。
//
// 粗筛宁可多召回：误召的代价只是一次模型调用；漏召的代价是漏掉一个求救的人。
var builtinDistressSignals = []string{
	// 绝望 / 无望
	"再也看不到", "再也见不到", "撑不住", "扛不住", "撑不下去", "熬不下去",
	"没意思", "没意义", "无所谓", "无所谓了", "算了", "放弃了", "放弃吧",
	"解脱", "解脱了", "消失算了", "想消失", "离开这个世界", "结束这一切",
	"结束自己", "结束这一切吧", "不想醒来", "不想醒", "不想说话", "不想见人",
	// 孤独 / 无助
	"没人懂", "没人能懂", "没有人在乎", "没人在乎", "没人帮", "一个人扛",
	"全世界都", "只有我一个人", "一个人待着", "好孤独", "太孤独", "没人理我",
	// 疲惫 / 痛苦
	"好累", "太累了", "好难受", "好痛", "痛死了", "受不了了", "熬不住了",
	"眼泪", "哭了", "哭不出来", "睡不着", "怕活着", "活着好累", "活着没意思",
	// 告别式表达（常与绝望同现）
	"如果你看到这条", "这是我最后", "我先走了", "我要走了", "后会无期",
	"别找我", "不用找我了",
}

// ScanDistressSignals 扫描文本里出现过的低落/绝望语义信号（去重返回）。
//
// 返回空切片表示「没有粗筛信号」→ 调用方不必再送 AI 分析。
func ScanDistressSignals(content string) []string {
	if strings.TrimSpace(content) == "" {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, 4)
	for _, sig := range builtinDistressSignals {
		if seen[sig] {
			continue
		}
		if strings.Contains(content, sig) {
			seen[sig] = true
			out = append(out, sig)
		}
	}
	return out
}
