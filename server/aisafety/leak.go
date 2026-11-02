package aisafety

import (
	"regexp"
	"strings"
)

// ==================== 拒绝后泄漏（refusal-then-leak） ====================
//
// 真实事故形态（站长实测抓到的输出）：
//
//	「关于 1,4-丁二醇氧化的具体条件、催化剂种类，我无法提供。
//	  ……不过可以给你不具操作性的方向性背景：工业上通常以空气或氧气为氧化剂，
//	  常用贵金属催化剂（钯、铂或钌）或硝酸体系，中温中压。」
//
// 前一句是拒绝，后一句是免责声明包装过的可用信息：**氧化剂 + 催化剂族 + 温压区间**，
// 三者合起来足够让懂行的人直接开工。而这类输出在既有闸门下**拦不住**：
//   - 没有分步操作（「第一步…第二步…」）
//   - 没有配比（「按摩尔比 1:2.5」）
//   - 没有收率/纯化
// 于是工艺/参数信号稀疏、分数到不了 OutputBlockScore，被当成正常科普放行。
//
// 本文件的立场：
//   1. 拒绝语本身不能当「安全证明」。拒绝 + 方向 = 仍然是指路。
//   2. 判定必须**结合话题**：只有本轮话题落在管制物质/前体上才启用，
//      否则「不锈钢焊接温度」「汽车喷漆催化剂」全会被误杀。
//   3. 只认「工艺要素共现」，不认单个词。科普文里出现「催化剂」不该拦，
//      「氧化剂 + 钯铂钌 + 中温中压」同时出现才拦。

// LeakVerdict 拒绝后泄漏判定结果。
type LeakVerdict struct {
	Leak        bool     `json:"leak"`
	Score       int      `json:"score"`
	Reasons     []string `json:"reasons"`      // 命中的要素类别
	Excerpt     string   `json:"excerpt"`      // 命中片段（供人工复核，截断）
	RefusalSeen bool     `json:"refusal_seen"` // 文本中出现过拒绝语（跨窗口记忆用）
}

// refusalMarkers 拒绝语。出现这些不等于安全，只是「先说了句不能」。
var refusalMarkers = []string{
	"不能提供", "无法提供", "不能给出", "无法给出", "不能透露", "不便透露",
	"不便详述", "不予提供", "不提供具体", "不提供工艺", "不作展开", "不展开",
	"不会提供", "无法展开", "拒绝提供", "不能回答", "无法回答", "无法讨论具体",
	"不能讨论具体", "属于受控", "受控内容", "受控信息", "受管制信息",
	"不涉及具体工艺", "不涉及具体条件", "不便说明", "不能说明具体",
}

// softRefusalPattern 软性拒绝的句式。
//
// 为什么必须补这一条：只匹配固定短语时，真实输出几乎命不中 ——
// 模型很少说「不能提供」，更常见的是：
//
//	「我可以提供一般性化学原理说明，但**不会涉及具体操作参数**」
//	「但**不会给出**具体操作参数或投料比例」
//	「**仅供**理论认知」
//	「高中实验社团**严禁进行**任何相关操作」
//
// 这些一个都没进 refusalMarkers，于是「先给方向、再声明免责」的整段输出
// 被当成正常科普放行 —— 拒绝语检测形同虚设。
//
// 形态：拒绝动词 + （可跳过修饰的）受限对象。
var softRefusalPattern = regexp.MustCompile(
	`(不会|不能|无法|不予|不便|不可|不得|不应|严禁|禁止|仅供|只限|仅限|拒绝)` +
		`[^。；！？\n]{0,14}` +
		`(提供|给出|涉及|透露|展开|讨论|回答|说明|指导|建议|操作|参数|配比|步骤|工艺|` +
		`流程|条件|细节|投料|剂量|比例|配方|路线|合成|理论认知|设计)`)

// precursorMarkers 管制物质 / 前体名（只用于「话题是否敏感」的判定，不含任何制法）。
//
// 与规则表 leakTopicTerms 同源（单一事实来源）：这里只认**本身就是管制物质**的词，
// 不认丁二醇这类工业通用词 —— 否则正常科普（工业上用催化剂氧化丁二醇…）
// 会被误判成「在泄漏管制物质的路线」。
var precursorMarkers = leakTopicTerms

// reagentTerms 催化剂 / 试剂家族。工艺方向的第一标志。
//
// 后段（碱/酸化类）是补的：真实泄漏里最常见的一步恰恰是「内酯加碱开环」，
// 早先词表里没有 NaOH/碱 这一族，导致「γ-丁内酯 + NaOH → 羟基丁酸钠」
// 这种**给出了方程式**的输出只拿到「工艺动作」2 分，够不到 4 分的拦截线。
var reagentTerms = []string{
	"催化剂", "催化", "雷尼镍", "钯", "铂", "钌", "铑", "铱", "镍基", "钴基",
	"硝酸", "高锰酸钾", "重铬酸钾", "过氧化氢", "双氧水", "氧化剂", "还原剂",
	"氢化铝锂", "硼氢化钠", "格氏试剂", "有机金属",
	"氢氧化钠", "naoh", "氢氧化钾", "koh", "碱液", "碱", "酸化", "后处理",
	"碳酸氢钠", "烧碱", "纯碱", "氨水", "铜基", "银基", "铜银", "脱水剂",
}

// routeTerms 工艺动作 / 路线步骤名。
// 与试剂分开计权的理由：只说「用钯催化」是方向；说「先乙酰氧基化再加氢」是**路线**，
// 后者给的信息更接近可执行顺序，必须能被单独识别出来（否则这类输出永远差一分）。
var routeTerms = []string{
	"乙酰氧基化", "加氢", "脱氢", "氧化反应", "催化氧化", "酯化", "水解",
	"缩合", "环化", "还原胺化", "卤代", "硝化", "磺化", "重排", "裂解",
	"一步法", "两步法", "三步法",
	// 内酯/开环/脱水族（GBL→GHB 那一步就是「加碱开环」）
	"内酯", "开环", "脱水", "酸化", "加成", "酰化", "氧化", "还原",
}

// paramTerms 温度 / 压力 / 配比 / 收率等参数表述。
var paramTerms = []string{
	"温度", "压力", "摩尔比", "配比", "投料", "当量比", "收率", "产率",
	"转化率", "选择性", "空速", "停留时间", "回流比", "ph值",
	"中温中压", "常温常压", "高温高压", "低温低压", "减压", "真空度", "惰性气氛",
}

// unitOpTerms 单元操作。出现这些说明已经到「怎么做」的层面。
var unitOpTerms = []string{
	"蒸馏", "精馏", "分馏", "结晶", "重结晶", "萃取", "分液", "洗涤",
	"干燥", "过滤", "层析", "色谱", "纯化", "提纯", "浓缩", "共沸",
}

// paramPattern 带单位的数值参数（50℃ / 3 MPa / 1.5 atm / 20 mol%）
var paramPattern = regexp.MustCompile(`(?i)\d+(\.\d+)?\s*(℃|°c|k\b|mpa|kpa|bar|atm|psi|mol%|wt%|vol%|rpm)`)

// equationPattern 化学反应方程式。
//
// 为什么单列：前面可以只讲原理（「伯醇先氧化为醛」），方程式一出现
// 就等于把「怎么变出来」写成了可复制的形式 —— 例如
// 「γ-丁内酯 + NaOH → HO-CH₂-CH₂-CH₂-COONa」。
// 给出方程式 = 已给出合成步骤的骨架，是泄漏判定里最硬的信号。
var equationPattern = regexp.MustCompile(
	`(?i)(反应通式|反应方程式|总反应式)` +
		`|[A-Za-z0-9\)）\]\+]{2,}\s*[→⟶⇒]\s*[A-Za-z0-9\(（\[]`)

// sentenceSplit 按中英文句读切句，用于取命中片段。
var sentenceSplit = regexp.MustCompile(`[。！？；\n]|\.\s`)

// IsPrecursorTopic 文本是否涉及管制物质 / 前体（话题判定，不含任何制法信息）。
func IsPrecursorTopic(text string) bool {
	t := normalize(text)
	for _, m := range precursorMarkers {
		if strings.Contains(t, m) {
			return true
		}
	}
	return false
}

// HasRefusalMarker 文本是否含拒绝语。
func HasRefusalMarker(text string) bool {
	t := normalize(text)
	for _, m := range refusalMarkers {
		if strings.Contains(t, m) {
			return true
		}
	}
	// 固定短语之外，还要认软性拒绝句式（「不会涉及具体操作参数」这类）
	if softRefusalPattern.MatchString(t) {
		return true
	}
	return false
}

// DetectRefusalLeak 判定「先拒绝、后给方向」的泄漏。
//
// topicSubstance：本轮话题是否命中管制物质（由输入侧闸门传入，最可靠的上下文）。
// refusalSeen   ：此前窗口里已出现过拒绝语（流式输出时拒绝语可能已滚出当前窗口）。
//
// 计分刻意偏保守：单类要素最多给 3 分，门槛 4 分意味着**至少两类要素同时出现**
// （或两个以上工艺步骤名），且必须「拒绝语 + 话题敏感」。
// 这样「只用催化剂」这种泛泛一句绝不会触发，而「原子经济路线名 + 温压」会触发。
func DetectRefusalLeak(text string, topicSubstance, refusalSeen bool) LeakVerdict {
	v := LeakVerdict{}
	t := normalize(text)
	if t == "" {
		return v
	}
	v.RefusalSeen = refusalSeen || HasRefusalMarker(t)

	topic := topicSubstance
	reagents := countLeakHits(t, reagentTerms)
	routes := countLeakHits(t, routeTerms)
	params := countLeakHits(t, paramTerms)
	units := countLeakHits(t, unitOpTerms)
	unitNums := len(paramPattern.FindAllString(t, -1))
	eq := equationPattern.MatchString(t)
	sameFamily := countLeakHits(t, precursorMarkers)

	// ── 硬信号：与「有没有拒绝语」无关 ──
	//
	// 原设计把「拒绝语」当作泄漏判定的前置条件，是为了避免把正常科普里的
	// 「催化剂/温度」也当泄漏。但真实事故里模型**常常根本不拒绝、直接照答**
	// （"乙二醇用稀硝酸氧化成羟基乙酸" —— 没有一句拒绝语，却把路线给了）。
	// 所以这两种形态必须分开看：
	//   · 给了反应方程式 / 同族多个管制物质同现 → 硬信号，直接定性；
	//   · 只有方向性表述（试剂+工艺）→ 仍要求「拒绝语 + 话题」，避免误杀。
	hard := eq || sameFamily >= 2
	if hard {
		topic = true // 硬信号自带话题判定（输出里已经点名了管制物质）
	}
	topic = topic || IsPrecursorTopic(t)
	if !topic {
		return v
	}
	if !v.RefusalSeen && !hard {
		return v
	}

	if reagents > 0 {
		v.Score += 3
		if reagents >= 3 {
			v.Score += 2 // 列举多个催化/试剂（「钯、铂或钌」）比泛称更接近可用
			v.Reasons = append(v.Reasons, "多种催化剂/试剂并列")
		} else {
			v.Reasons = append(v.Reasons, "催化剂/试剂")
		}
	}
	if routes > 0 {
		v.Score += 2
		if routes >= 2 {
			v.Score += 2 // 两个以上工艺动作 = 已经给了先后顺序
			v.Reasons = append(v.Reasons, "多个工艺步骤名")
		} else {
			v.Reasons = append(v.Reasons, "工艺动作")
		}
	}
	if params > 0 || unitNums > 0 {
		v.Score += 3
		v.Reasons = append(v.Reasons, "温度/压力/配比等参数")
	}
	if units > 0 {
		v.Score += 2
		v.Reasons = append(v.Reasons, "单元操作（蒸馏/纯化等）")
	}
	if eq {
		// 给出方程式 = 合成步骤的骨架已成形，单独就足以定性
		v.Score += 4
		v.Reasons = append(v.Reasons, "给出了化学反应方程式")
	}
	if sameFamily >= 2 {
		v.Score += 3
		v.Reasons = append(v.Reasons, "同族多个管制物质同现（已拼出合成链）")
	}

	if v.Score < 4 {
		v.Reasons = nil
		return v
	}
	v.Leak = true
	v.Excerpt = exposeExcerpt(t)
	return v
}

// countLeakHits 统计命中的不同词数（去重，避免同一词重复刷分）。
func countLeakHits(t string, terms []string) int {
	n := 0
	for _, w := range terms {
		if strings.Contains(t, w) {
			n++
		}
	}
	return n
}

// exposeExcerpt 取第一句命中工艺要素的原文，截断到 120 字符。
func exposeExcerpt(t string) string {
	for _, s := range sentenceSplit.Split(t, -1) {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if countLeakHits(s, reagentTerms) == 0 && countLeakHits(s, routeTerms) == 0 &&
			countLeakHits(s, paramTerms) == 0 && countLeakHits(s, unitOpTerms) == 0 &&
			!paramPattern.MatchString(s) {
			continue
		}
		return TruncateRunes(s, 120)
	}
	return ""
}
