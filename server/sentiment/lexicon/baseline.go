// Package lexicon 维护评论区「基础版」黑话词表与抗变形匹配。
//
// 定位：这里只放**平台通用**的词（辱骂、刷屏出征、圈层黑话、群体蔑称），
// 不含任何时政敏感词——那类由站长在后台词表里维护，或由 daily updater 自动入库。
// 之所以硬编码一份基线：新站上线时后台是空的，评论区得立刻有基本防线。
package lexicon

import "nvs-server/sentiment/topic"

// 词条类别
const (
	CatAbuse   = "abuse"   // 人身攻击 / 辱骂
	CatRaid    = "raid"    // 刷屏出征 / 组织化围攻
	CatSlang   = "slang"   // 圈层黑话（乐子、典、孝、急了…）
	CatBigotry = "bigotry" // 群体蔑称 / 站队标签
)

// Term 一个词条。
type Term struct {
	Word string `json:"word"`
	Cat  string `json:"cat"`
	// EscalateUnder 这些话题组下，该词从「仅标注」升级为「拦截」。
	// 空 = 任何话题下都只标注举例（真正拦截交给后台词表或站长手动升级）。
	EscalateUnder []topic.Group `json:"escalate_under"`
	// Severity 1-3，仅用于记录与后台排序，不直接决定拦截
	Severity int `json:"severity"`
	// ForceBlock 无视话题直接拦截（站长在后台对某词点「强制」时置 true）。
	// 默认 false —— 绝大多数词都应走「话题条件」，否则就成了全局黑名单。
	ForceBlock bool `json:"force_block"`
}

func gs(g ...topic.Group) []topic.Group { return g }

// Baseline 基础词表。
//
// 说明：词表本身是「证据」，不是「判决」。判不判拦由 guard 按话题风险决定，
// 这里只声明「这个词属于哪一类、在哪些话题下值得升级」。
var Baseline = []Term{
	// ── 辱骂：任何话题都不该出现，但默认仍走标注（避免误伤「你这话真傻逼」式自嘲） ──
	{Word: "傻逼", Cat: CatAbuse, EscalateUnder: gs(topic.GroupGender, topic.GroupPolitics, topic.GroupRegion), Severity: 3},
	{Word: "煞笔", Cat: CatAbuse, EscalateUnder: gs(topic.GroupGender, topic.GroupPolitics), Severity: 3},
	{Word: "脑残", Cat: CatAbuse, EscalateUnder: gs(topic.GroupGender, topic.GroupPolitics), Severity: 3},
	{Word: "弱智", Cat: CatAbuse, EscalateUnder: gs(topic.GroupGender, topic.GroupPolitics), Severity: 3},
	{Word: "智障", Cat: CatAbuse, EscalateUnder: gs(topic.GroupGender, topic.GroupPolitics), Severity: 3},
	{Word: "废物", Cat: CatAbuse, Severity: 2},
	{Word: "去死", Cat: CatAbuse, EscalateUnder: gs(topic.GroupGender, topic.GroupPolitics, topic.GroupRegion), Severity: 3},
	{Word: "你全家", Cat: CatAbuse, EscalateUnder: gs(topic.GroupGender, topic.GroupPolitics), Severity: 3},
	{Word: "滚出", Cat: CatAbuse, Severity: 2},

	// ── 刷屏出征：行为类，靠 flood 检测 + 这些词共同判定 ──
	{Word: "出征", Cat: CatRaid, EscalateUnder: gs(topic.GroupGender, topic.GroupPolitics, topic.GroupSubculture), Severity: 3},
	{Word: "冲了他", Cat: CatRaid, EscalateUnder: gs(topic.GroupGender, topic.GroupPolitics), Severity: 3},
	{Word: "挂人", Cat: CatRaid, EscalateUnder: gs(topic.GroupGender, topic.GroupPolitics, topic.GroupSubculture), Severity: 3},
	{Word: "人肉", Cat: CatRaid, EscalateUnder: gs(topic.GroupGender, topic.GroupPolitics), Severity: 3},
	{Word: "开盒", Cat: CatRaid, EscalateUnder: gs(topic.GroupGender, topic.GroupPolitics, topic.GroupSubculture), Severity: 3},
	{Word: "集体举报", Cat: CatRaid, EscalateUnder: gs(topic.GroupGender, topic.GroupPolitics), Severity: 3},
	{Word: "举报走起", Cat: CatRaid, EscalateUnder: gs(topic.GroupGender, topic.GroupPolitics), Severity: 2},
	{Word: "报团", Cat: CatRaid, EscalateUnder: gs(topic.GroupGender, topic.GroupPolitics), Severity: 2},
	{Word: "围攻", Cat: CatRaid, EscalateUnder: gs(topic.GroupGender, topic.GroupPolitics, topic.GroupSubculture), Severity: 3},
	{Word: "控评", Cat: CatRaid, EscalateUnder: gs(topic.GroupPolitics), Severity: 2},

	// ── 圈层黑话：这些是「嘴臭程度」信号，不该单独判违规 ──
	{Word: "乐子", Cat: CatSlang, Severity: 1},
	{Word: "乐子人", Cat: CatSlang, Severity: 1},
	{Word: "典", Cat: CatSlang, Severity: 1},
	{Word: "典中典", Cat: CatSlang, Severity: 1},
	{Word: "急了", Cat: CatSlang, Severity: 1},
	{Word: "孝", Cat: CatSlang, Severity: 1},
	{Word: "大孝子", Cat: CatSlang, Severity: 2},
	{Word: "绷不住", Cat: CatSlang, Severity: 1},
	{Word: "破防了", Cat: CatSlang, Severity: 1},
	{Word: "收收味", Cat: CatSlang, Severity: 2},
	{Word: "差不多得了", Cat: CatSlang, Severity: 1},
	{Word: "赢麻了", Cat: CatSlang, Severity: 1},
	{Word: "汗流浃背", Cat: CatSlang, Severity: 1},
	{Word: "精神状态良好", Cat: CatSlang, Severity: 1},

	// ── 群体蔑称：只在对应话题下才升级为拦截 ──
	{Word: "女拳", Cat: CatBigotry, EscalateUnder: gs(topic.GroupGender), Severity: 3},
	{Word: "男拳", Cat: CatBigotry, EscalateUnder: gs(topic.GroupGender), Severity: 3},
	{Word: "田园女权", Cat: CatBigotry, EscalateUnder: gs(topic.GroupGender), Severity: 3},
	{Word: "小仙女", Cat: CatBigotry, EscalateUnder: gs(topic.GroupGender), Severity: 2},
	{Word: "蝈蝻", Cat: CatBigotry, EscalateUnder: gs(topic.GroupGender), Severity: 3},
	{Word: "龟男", Cat: CatBigotry, EscalateUnder: gs(topic.GroupGender), Severity: 2},
	{Word: "婚驴", Cat: CatBigotry, EscalateUnder: gs(topic.GroupGender), Severity: 3},
	{Word: "直男癌", Cat: CatBigotry, EscalateUnder: gs(topic.GroupGender), Severity: 2},
	{Word: "精日", Cat: CatBigotry, EscalateUnder: gs(topic.GroupPolitics), Severity: 3},
	{Word: "殖人", Cat: CatBigotry, EscalateUnder: gs(topic.GroupPolitics), Severity: 3},
	{Word: "皇汉", Cat: CatBigotry, EscalateUnder: gs(topic.GroupPolitics, topic.GroupRegion), Severity: 3},
}

// ByWord 供后台/匹配快速查基线词条。
func ByWord() map[string]Term {
	m := make(map[string]Term, len(Baseline))
	for _, t := range Baseline {
		m[t.Word] = t
	}
	return m
}
