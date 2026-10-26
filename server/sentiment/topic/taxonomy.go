// Package topic 把「AI 审核敲定的话题标签」映射为可利用的风险维度。
//
// 为什么单独建包：评论区是否该拦，取决于「这段话所在的话题语境」，而不是词本身。
// 同一句「典中典」，在二次元玩梗语境里只是嘴臭，在性别议题或政治批判语境里
// 就是站队与动员的一部分——词一样，处置必须不同。
//
// 本包只做「标签 → 话题组 → 风险等级」的纯计算，不碰数据库、不碰 HTTP，
// 便于被 guard / updater / 后台页共用，也便于单测穷举组合。
package topic

import "strings"

// Risk 风险等级。数值可比较，升档就是 +1（封顶 RiskHigh）。
type Risk int

const (
	RiskLow     Risk = 0 // 低：无需处置
	RiskMid     Risk = 1 // 中：易起争执，标注观察
	RiskMidHigh Risk = 2 // 中高：成规模会出事，严格一些
	RiskHigh    Risk = 3 // 高：群体对立 / 政治敏感，按最严处置
)

func (r Risk) String() string {
	switch r {
	case RiskLow:
		return "low"
	case RiskMid:
		return "mid"
	case RiskMidHigh:
		return "midhigh"
	case RiskHigh:
		return "high"
	default:
		return "low"
	}
}

// ParseRisk 容错解析（后台/接口传来的字符串）。
func ParseRisk(s string) Risk {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "high", "3":
		return RiskHigh
	case "midhigh", "mid-high", "2":
		return RiskMidHigh
	case "mid", "1":
		return RiskMid
	default:
		return RiskLow
	}
}

// Group 话题组。刻意保持少量、语义稳定，避免标签一多就没人能维护。
type Group string

const (
	GroupSubculture Group = "subculture" // 亚文化 / 二次元 / 同人 / 谷圈
	GroupGender     Group = "gender"     // 性别议题
	GroupPolitics   Group = "politics"   // 政治 / 时政批判
	GroupCapital    Group = "capital"    // 资本 / 劳资 / 消费批判
	GroupRegion     Group = "region"     // 地域 / 民族 / 宗教
	GroupOther      Group = "other"      // 其它争议话题（医疗、教育、动物保护等）
)

// Label 中文名（后台展示用）。
func (g Group) Label() string {
	switch g {
	case GroupSubculture:
		return "亚文化"
	case GroupGender:
		return "性别议题"
	case GroupPolitics:
		return "政治批判"
	case GroupCapital:
		return "资本批判"
	case GroupRegion:
		return "地域民族"
	case GroupOther:
		return "其它争议"
	default:
		return "其它"
	}
}

// baseRisk 单个话题组的基准风险。
//
// 取值依据（按产品口径，不是法律结论）：
//   - 性别议题：国内平台最容易演变成群体对骂与站队，基准直接给高；
//   - 亚文化：圈层黑话密度高、易被外部误读，但本身合法，基准中高；
//   - 政治批判：合法批评是正当的，基准只给中，靠「升档信号」再提；
//   - 资本批判：同上，基准中。
var baseRisk = map[Group]Risk{
	GroupGender:     RiskHigh,
	GroupSubculture: RiskMidHigh,
	GroupPolitics:   RiskMid,
	GroupCapital:    RiskMid,
	GroupRegion:     RiskMidHigh,
	GroupOther:      RiskMid,
}

// BaseRisk 取某组的基准风险（未知组按低处理）。
func BaseRisk(g Group) Risk {
	if r, ok := baseRisk[g]; ok {
		return r
	}
	return RiskLow
}

// keywords 标签关键词 → 话题组。
//
// 这里刻意用「包含即命中」而不是精确匹配：AI 给的标签是自由文本
// （「二次元」「动漫杂谈」「ACGN」都会出现），精确匹配会大量漏判。
var keywords = []struct {
	group Group
	words []string
}{
	{GroupSubculture, []string{
		"二次元", "动漫", "acg", "acgn", "同人", "cos", "cosplay", "谷子", "谷圈",
		"vtuber", "虚拟主播", "洛丽塔", "lolita", "jk", "手办", "galgame", "gal",
		"轻小说", "漫展", "痛包", "乙女", "腐向", "嗑cp", "纸片人", "宅", "御宅",
	}},
	{GroupGender, []string{
		"性别", "女权", "男权", "女拳", "男拳", "彩礼", "婚恋", "相亲", "擦边",
		"媚男", "媚女", "厌女", "厌男", "小三", "出轨", "家暴", "生育", "堕胎",
		"处女情结", "男女对立", "两性",
	}},
	{GroupPolitics, []string{
		"政治", "时政", "体制", "政策", "政府", "官员", "领导", "人大", "政协",
		"外交", "国际关系", "意识形态", "舆论", "新闻自由", "言论自由", "维稳",
		"反腐败", "公权", "法治", "拆迁", "上访",
	}},
	{GroupCapital, []string{
		"资本", "资本家", "劳资", "996", "007", "剥削", "裁员", "打工", "社畜",
		"消费主义", "割韭菜", "内卷", "躺平", "房价", "物价", "贫富", "垄断",
		"平台经济", "外卖骑手", "劳务派遣",
	}},
	{GroupRegion, []string{
		"地域", "地图炮", "地域黑", "民族", "宗教", "方言", "老乡", "移民",
		"少数民族", "伊斯兰", "基督教", "佛教",
	}},
	{GroupOther, []string{
		"医疗", "医闹", "教育", "应试", "高考", "动物保护", "虐杀", "环保",
		"转基因", "食品安全", "疫苗", "饭圈", "追星", "打榜",
	}},
}

// GroupsOfTag 把单个标签归类；无法归类返回空切片（不是 GroupOther——
// 无关标签不该被算成「争议话题」，否则财经、玄幻小说都会被抬风险）。
func GroupsOfTag(tag string) []Group {
	t := strings.ToLower(strings.TrimSpace(tag))
	if t == "" {
		return nil
	}
	var out []Group
	for _, kw := range keywords {
		for _, w := range kw.words {
			if strings.Contains(t, w) {
				out = append(out, kw.group)
				break // 同一组只记一次
			}
		}
	}
	return out
}
