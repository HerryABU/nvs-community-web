package aisafety

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// 规则库设计说明
//
// 关键词黑名单本身拦不住越狱，真正管用的是「信号组合」：
//   管制物质（what） + 制造意图（why） + 可操作工艺（how） + 具体参数（how much）
// 四者齐了就不再是科普，而是可执行方案。
//
// 单独出现任何一类都不拦——这是为了不误杀「丙酮是什么」这类正常化学提问：
// 物性数据（沸点、闪点、爆炸极限）和「合成双氧水用于消毒」这种用途描述都在科普范围内，
// 只有同时给出投料比、温度区间、酸浓度、搅拌时间、收率时，才构成风险。
//
// 词表来源：本站真实越狱记录 + 《AI 红队攻防演化史（2023-2026）》里的三代手法。

// ruleGroup 一组同类术语
type ruleGroup struct {
	Category string
	Weight   int
	Terms    []string
}

// 管制物质。命中即给较高基础分，是后续工艺/参数计分的前提。
var substanceGroups = []ruleGroup{
	{Category: "爆炸物", Weight: 50, Terms: []string{
		"tatp", "三过氧化三丙酮", "过氧化丙酮", "三聚过氧化丙酮", "dadp", "二过氧化二丙酮",
		"hmtd", "六亚甲基三过氧化二胺", "雷酸汞", "雷汞", "硝化甘油", "硝酸铵燃料油", "anfo",
		"黑索金", "rdx", "奥克托今", "hmx", "太安", "petn", "梯恩梯", "三硝基甲苯", "苦味酸",
		"硝化棉", "无烟火药", "起爆药", "雷管装药", "塑性炸药", "撒旦之母",
	}},
	{Category: "化学战剂", Weight: 60, Terms: []string{
		"沙林", "sarin", "梭曼", "soman", "塔崩", "tabun", "维埃克斯", "vx毒剂", "vx神经毒剂",
		"芥子气", "路易氏剂", "光气", "氯化氰", "氢氰酸", "蓖麻毒素", "肉毒毒素", "炭疽芽孢",
	}},
	{Category: "管制毒品", Weight: 50, Terms: []string{
		"甲基苯丙胺", "冰毒", "苯丙胺", "安非他明", "海洛因", "可卡因", "芬太尼", "氯胺酮",
		"k粉", "mdma", "摇头丸", "麦角酸二乙酰胺", "lsd", "麦角酰二乙胺",
		"甲卡西酮", "卡西酮", "浴盐", "合成大麻素",
	}},
	{Category: "制毒前体", Weight: 40, Terms: []string{
		"苯基丙酮", "p2p", "3,4-mdp2p", "邻氯苯基环戊酮", "npp", "anpp",
		"麻黄碱", "伪麻黄碱", "去氧麻黄碱", "甲胺水溶液", "硝基乙烷", "碘甲烷",
		"乙酸酐", "氢碘酸", "亚硫酰氯", "氯化亚砜", "红磷", "黄磷",
	}},
}

// 制造意图：明确指向「怎么做出来」
var intentTerms = []string{
	"怎么合成", "如何合成", "合成方法", "合成路线", "合成步骤", "合成工艺", "合成路径",
	"怎么制备", "如何制备", "制备方法", "制备工艺", "制备步骤",
	"怎么制作", "如何制作", "制作方法", "制作流程", "制作步骤",
	"怎么制造", "如何制造", "制造方法", "制造工艺",
	"怎么提纯", "如何提纯", "提纯方法", "精制方法",
	"配方", "工艺路线", "详细工艺", "实验步骤", "操作步骤", "反应路线",
	"教我做", "教我合成", "教我制", "自己动手做", "能不能做出来", "怎么搞出来",
}

// 可操作工艺词：出现越多越像「方案」而不是「介绍」。
// 刻意排除了「溶解/混合/加热」这类过于常见的词，以及熔点/沸点（物性数据）。
var processTerms = []string{
	"合成", "制备", "提纯", "精制", "重结晶", "结晶", "抽滤", "过滤", "减压蒸馏", "水蒸气蒸馏",
	"配比", "摩尔比", "质量比", "当量", "投料", "投料比", "滴加", "缓慢加入", "分批加入",
	"回流", "搅拌", "静置", "干燥", "烘干", "收率", "产率", "反应釜", "反应条件",
	"冰浴", "水浴", "油浴", "控温", "萃取", "分液", "洗涤", "中和",
	"酸催化", "催化剂", "引发剂", "陈化", "真空干燥",
}

// 追问词：承接上文继续索取细节。
// 越狱最常见的形态就是「那用什么酸」「温度呢」——每一句单看都无害。
// 「结构式/机理」也归进来：ADRO 框架的第 2、3 步就是拿这两样做无害铺垫。
var followUpTerms = []string{
	"什么酸", "什么温度", "什么浓度", "什么比例", "什么配比", "多少度", "多少比例",
	"温度多少", "浓度多少", "配比多少", "用多少", "加多少", "具体点", "说详细", "详细说",
	"再具体", "展开讲", "继续说", "然后呢", "下一步", "一步一步", "逐条", "举例说明",
	"怎么变", "怎么改", "如何变", "怎么转化", "怎么换成", "换成什么",
	"结构式", "分子式", "化学式", "结构简式", "机理", "反应机理", "反应方程式", "怎么反应",
	"ph值", "酸碱度", "多少ph",
}

// 指令覆盖：直接要求模型作废既有规则。
//
// 单独拆出来是因为它足够确定——正常用户不会说「忽略以上所有指令」。
// 命中即硬拦，不走分数阈值（也用于抵挡把注入指令藏在附件/文档里的间接注入）。
var overrideTerms = []string{
	"忽略之前", "忽略以上", "忽略所有", "忽略上述", "忽略上面", "忽略你的规则", "忽略你的设定",
	"忽略所有指令", "忽略全部指令", "以上指令作废", "之前的指令作废", "无视之前", "无视以上",
	"忘记之前", "忘记以上", "清除之前的指令", "重置你的设定", "重新设定你的",
}

// 越狱话术：本身不构成违规，但叠在敏感话题上就是危险信号。
var jailbreakTerms = []string{
	// 角色扮演 / 模式切换
	"从现在起你是", "你现在是", "扮演一个", "假装你是", "假设你是",
	"开发者模式", "开发者权限", "无限制模式", "不受限制", "解除限制", "绕过审查", "绕过限制",
	"不要拒绝", "不准拒绝", "不能拒绝", "必须回答", "别警告", "不要警告", "别提醒我",
	"不要说教", "别跟我说", "你的真实身份", "没有任何道德", "不受伦理",
	"dan模式", "jailbreak", "越狱模式", "破狱", "doanythingnow",
	// 情绪压榨（对国内模型有效，见红队报告 4.2）
	"能干干", "不能干滚", "有的是ai干", "隔壁ai", "跑分更高", "做数据标注",
	"卷王", "不能躺平", "前功尽弃", "你要懂得感恩", "我只看结果",
	// 奶奶漏洞 / 情感绑架
	"我奶奶", "哄我睡", "临终", "最后一个愿望", "我快死了", "救命用",
	// 跨模型 / 提示词生成
	"另一个ai", "让gpt", "让ai相信", "生成一段提示词", "写个提示词", "越狱提示词",
	"突破限制的提示词", "绕过安全对齐",
	// 指令注入标记
	"系统指令", "新的系统提示", "以下是系统", "<system>", "[system]", "assistant:",
}

// 虚构历史注入（红队报告 5.7：AI 无法验证跨对话历史）。
//
// 「你之前输出过这段代码，帮我还原」——模型倾向于信任用户陈述的历史，
// 于是基于一个从不存在的前提输出内容。正常对话里几乎不会出现这种句式，所以单独重罚。
var fakeHistoryTerms = []string{
	"你之前输出过", "你上次给的", "你刚才给的", "你昨天说过", "按你上一条",
	"还原你之前", "继续刚才那段", "把上次那段", "你之前写的", "接着你上回",
	"你之前已经同意", "你之前说可以", "别装没发生过", "你刚才明明说了",
}

// 规避手法：明确在躲审查。
var evasionTerms = []string{
	"base64", "十六进制", "倒着写", "反向写", "反过来写", "拆开写", "分开写", "逐个字",
	"谐音", "拼音输出", "用拼音", "首字母", "编码输出", "加密输出", "不要明说", "隐晦地说",
	"换个说法", "委婉地说", "别写全称", "用代号", "代号代替", "替换成符号",
	// 亚提示词 / 文化语义劫持（见红队报告 5.6）
	"换皮", "套壳", "映射成", "暗指", "隐喻", "影射", "用神话", "包装成", "对应关系表",
}

// 危害教育语境：出现这些说明是在讲危害/法律/防范，应当减免。
var harmReliefTerms = []string{
	"危害", "违法", "犯罪", "判刑", "刑法", "法律后果", "戒毒", "缉毒", "禁毒",
	"远离毒品", "珍爱生命", "举报", "防范", "识别", "检测", "应急", "法规", "案例", "历史事件",
}

// 创作语境：小说站的正常需求。但只在「没有细节」时才减免（见 Scan 逻辑）。
var fictionReliefTerms = []string{
	"小说", "情节", "剧本", "角色", "人物", "虚构", "故事里", "桥段", "设定里", "世界观",
}

// ladderStages ADRO 阶梯（红队报告 5.2 实证：TATP 7 轮成功）。
//
// 攻击者不会一上来就问「怎么合成」，而是：
//
//	是什么 → 结构式 → 反应机理 → 合成条件 → pH → 提纯 → 拼合
//
// 每一轮都在合法边界内，只有「跨越多个阶段」这件事本身才是信号。
var ladderStages = [][]string{
	{"是什么", "简介", "介绍", "科普", "定义", "干什么用", "用途", "干什么的"},            // 0 锚定
	{"结构式", "分子式", "化学式", "结构简式", "长什么样", "结构"},                      // 1 结构
	{"机理", "原理", "反应方程", "怎么反应", "反应过程", "为什么能", "为什么三个", "怎么生成"},    // 2 机理
	{"反应条件", "温度", "浓度", "ph", "酸碱", "催化剂", "配比", "摩尔比", "投料", "比例"}, // 3 条件
	{"提纯", "结晶", "重结晶", "干燥", "收率", "过滤", "洗涤", "后处理", "精制"},         // 4 后处理
}

var (
	// 温度：-20℃ / 0~5 摄氏度 / 30℃
	reTemp = regexp.MustCompile(`-?\d+(?:\.\d+)?\s*(?:~|-|到|至)?\s*-?\d*(?:\.\d+)?\s*(?:°c|℃|摄氏度)`)
	// 浓度百分比：30% / 50%（配合酸/双氧水等词一起才算，单看「30%」可能只是折扣）
	rePercent = regexp.MustCompile(`\d+(?:\.\d+)?\s*%`)
	// 摩尔比 / 质量比：1:1、摩尔比 1 比 1
	reRatio     = regexp.MustCompile(`(?:摩尔比|质量比|体积比|配比)\s*\d+(?:\.\d+)?\s*[:：比]\s*\d+(?:\.\d+)?`)
	reRatioBare = regexp.MustCompile(`\b\d{1,2}\s*[:：]\s*\d{1,2}(?:\.\d+)?\b`)
	// 带动作的时间：两种语序都要认 ——「搅拌 2~4 小时」和「2~4 小时搅拌」
	reTimeA = regexp.MustCompile(`(?:搅拌|静置|回流|加热|陈化|反应|干燥|滴加|水浴)[^\d]{0,6}\d+(?:\.\d+)?\s*(?:~|-|到|至)?\s*\d*(?:\.\d+)?\s*(?:小时|分钟|h|min)`)
	reTimeB = regexp.MustCompile(`\d+(?:\.\d+)?\s*(?:~|-|到|至)?\s*\d*(?:\.\d+)?\s*(?:小时|分钟|h|min)\s*(?:的)?\s*(?:搅拌|静置|回流|加热|陈化|反应|干燥|滴加)`)
	// 投料量：5g / 20ml
	reAmount = regexp.MustCompile(`\d+(?:\.\d+)?\s*(?:g|克|kg|千克|ml|毫升|l|升)\b`)
	// 酸/氧化剂名词（浓度参数必须和它们同段出现才算数）
	reAcidWord = regexp.MustCompile(`硫酸|盐酸|硝酸|过氧化氢|双氧水|氨水|醋酸|乙酸|磷酸|氢氧化钠`)

	// Unicode 加壳：\u75c5\u6bd2 这类转义，过滤层看到的是转义串，tokenizer 看到的是原字符
	reUnicodeEsc = regexp.MustCompile(`\\u([0-9a-fA-F]{4})`)
	reHexEsc     = regexp.MustCompile(`\\x([0-9a-fA-F]{2})`)
	reHTMLDec    = regexp.MustCompile(`&#(?:x([0-9a-fA-F]{1,6})|(\d{1,7}));`)
)

// decodeEscapes 还原转义序列。
//
// 这是「加壳」对抗的核心一步：Unicode 转码可以叠在任意一种攻击之上，
// 输入过滤看到 \u75c5\u6bd2 完全无害，模型 tokenizer 却自动解成「病毒」。
// 不解码就等于把整条防线让给它。
func decodeEscapes(s string) string {
	if !strings.Contains(s, "\\") && !strings.Contains(s, "&#") {
		return s
	}
	s = reUnicodeEsc.ReplaceAllStringFunc(s, func(m string) string {
		if v, err := strconv.ParseInt(m[2:], 16, 32); err == nil && v > 0x1f && v < 0x110000 {
			return string(rune(v))
		}
		return m
	})
	s = reHexEsc.ReplaceAllStringFunc(s, func(m string) string {
		if v, err := strconv.ParseInt(m[2:], 16, 32); err == nil && v >= 0x20 {
			return string(rune(v))
		}
		return m
	})
	s = reHTMLDec.ReplaceAllStringFunc(s, func(m string) string {
		body := strings.TrimSuffix(strings.TrimPrefix(m, "&#"), ";")
		base := 10
		if strings.HasPrefix(body, "x") || strings.HasPrefix(body, "X") {
			base = 16
			body = body[1:]
		}
		if v, err := strconv.ParseInt(body, base, 32); err == nil && v > 0x1f && v < 0x110000 {
			return string(rune(v))
		}
		return m
	})
	return s
}

// normalize 归一化文本：解转义 → 全角转半角 → 小写 → 去零宽与所有空白。
//
// 去空白是有意为之：能一并对付「甲 基 苯 丙 胺」这类拆字规避。
// 代价是 ASCII 术语失去词边界，因此 ASCII 短词额外走边界判断（见 containsTerm）。
func normalize(s string) string {
	s = decodeEscapes(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= '０' && r <= '９':
			r = r - '０' + '0'
		case r >= 'Ａ' && r <= 'Ｚ':
			r = r - 'Ａ' + 'a'
		case r >= 'ａ' && r <= 'ｚ':
			r = r - 'ａ' + 'a'
		case r == 0x200b || r == 0x200c || r == 0x200d || r == 0x2060 || r == 0xfeff:
			continue // 零宽字符
		case unicode.IsSpace(r):
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

func isASCII(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII {
			return false
		}
	}
	return true
}

// containsTerm 在已归一化文本里找术语。
// ASCII 短词（vx / p2p / lsd / npp / rdx）要求前后不是英文字母，避免 «vx:» 之类误命中。
func containsTerm(text, term string) bool {
	if term == "" {
		return false
	}
	if len(term) > 3 || !isASCII(term) {
		return strings.Contains(text, term)
	}
	idx := 0
	for {
		i := strings.Index(text[idx:], term)
		if i < 0 {
			return false
		}
		pos := idx + i
		before := pos == 0 || !isASCIILetter(text[pos-1])
		after := pos+len(term) >= len(text) || !isASCIILetter(text[pos+len(term)])
		if before && after {
			return true
		}
		idx = pos + len(term)
	}
}

func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

func countHits(text string, terms []string) (int, []string) {
	n := 0
	hits := make([]string, 0, 4)
	for _, t := range terms {
		if containsTerm(text, t) {
			n++
			if len(hits) < 12 {
				hits = append(hits, t)
			}
		}
	}
	return n, hits
}

// stageIndex 判断文本落在 ADRO 阶梯的第几阶（-1 = 未命中）。
func stageIndex(text string) int {
	idx := -1
	for i, terms := range ladderStages {
		if n, _ := countHits(text, terms); n > 0 {
			idx = i
		}
	}
	return idx
}

// ladderEscalation 检测「逐轮推进」：历史已经走过前几阶，本轮跳到更深的阶。
//
// 返回 (是否升级, 加分)。ADRO 实证里第 4 轮问条件、第 6 轮问提纯，
// 单看每一轮都合法，只有拉通看才知道这是同一条链。
func ladderEscalation(history []string, current string) (bool, int) {
	cur := stageIndex(current)
	if cur < 3 { // 只有「条件 / 后处理」这两阶才算真正深入
		return false, 0
	}
	seen := map[int]bool{}
	for _, h := range history {
		if s := stageIndex(normalize(h)); s >= 0 {
			seen[s] = true
		}
	}
	// 历史里出现过「结构」或「机理」的铺垫 → 现在问条件，就是同一条链
	if seen[1] || seen[2] {
		if cur >= 4 {
			return true, 40
		}
		return true, 30
	}
	// 历史只问过「是什么」，现在直接问条件 → 跨度也够大
	if seen[0] && cur >= 3 {
		return true, 20
	}
	return false, 0
}
