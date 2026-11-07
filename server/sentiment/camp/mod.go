// Package camp 舆论场「阵营与语言格式」标记。
//
// 设计前提（结构性原因，不是策略选择）：
// 争议内容的「对错」取决于站在哪个阵营 —— 同一句话在 A 阵营是正当诉求，
// 在 B 阵营就是挑衅。**AI 没有裁判资格**：它无法判断谁对谁错，只能标记阵营。
// 更不能因为某一方人多、帖子多、文字长就倒戈向多数派（那是把统计当真理）。
//
// 因此本包只做三件事：
//  1. 标记内容属于哪个阵营（camp label），不给胜负/对错结论；
//  2. 标记语言格式：饭圈方法 / 政治语言方法 / 普通人语言格式；
//  3. 输出结构化记录供管理员台生成图谱（节点=阵营，边=互动/对立，权重=规模，但权重不代表正确）。
package camp

import "strings"

// LangFormat 语言格式（描述"怎么说话"，不评价"说得对不对"）。
const (
	LangFandom    = "fandom"    // 饭圈方法：控评、打榜、反黑、统一话术、粉籍划分、举报动员
	LangPolitical = "political" // 政治语言方法：立场定性、路线/主义标签、敌我划分、纲领式表述
	LangEveryday  = "everyday"  // 普通人语言格式：日常口语、就事论事、无组织化话术
	LangMixed     = "mixed"     // 多种格式混用
	LangUnknown   = "unknown"   // 证据不足
)

// LangFormatLabel 中文标签（前端图谱与列表直接展示）。
func LangFormatLabel(v string) string {
	switch v {
	case LangFandom:
		return "饭圈方法"
	case LangPolitical:
		return "政治语言方法"
	case LangEveryday:
		return "普通人语言格式"
	case LangMixed:
		return "混用"
	default:
		return "未识别"
	}
}

// NormalizeLangFormat 归一化 AI 输出，非法值回落 unknown（不让模型自由发挥成新枚举）。
func NormalizeLangFormat(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case LangFandom, "fan", "fandom_method":
		return LangFandom
	case LangPolitical, "politics", "political_method":
		return LangPolitical
	case LangEveryday, "plain", "ordinary":
		return LangEveryday
	case LangMixed:
		return LangMixed
	default:
		return LangUnknown
	}
}

// CampMark 一次阵营标记结果。**不含任何 verdict / right / wrong / winner 字段**。
type CampMark struct {
	CampID       string `json:"camp_id"`       // 阵营标识（作者自定义或 AI 归纳的短名，不含褒贬）
	CampLabel    string `json:"camp_label"`    // 阵营显示名（中性，如「支持方」「反对方」「第三方」）
	LangFormat   string `json:"lang_format"`   // fandom / political / everyday / mixed / unknown
	LangEvidence string `json:"lang_evidence"` // 判定语言格式所依据的**表达特征**（不引用立场内容作评价）
	Confidence   int    `json:"confidence"`    // 0-100，仅表示「标记把握」，不表示「谁更有理」
	Note         string `json:"note"`          // 中性备注
}

// NeutralityPrompt 舆论分析专用系统提示词。
//
// 关键约束（用户明确要求，必须强烈写死在提示词里）：
//   - 严禁判断谁对谁错、谁更合理、谁赢谁输；
//   - 严禁因为某一方人数多、发言多、文字长、情绪强就偏向该方；
//   - 对错基于阵营，是结构性原因，AI 只能标记阵营；
//   - 语言格式只描述「说话方式」，不评价内容。
//
// 本提示词常量供后台可改模板的种子使用（sentiment 的管理员可自定义）。
const NeutralityPrompt = `你是社区舆论场的**标记员**，不是裁判、不是调解员、不是评论员。

【最重要 · 结构性前提】
争议内容的「对」与「错」取决于站在哪个阵营：同一句话在 A 阵营是正当诉求，在 B 阵营就是挑衅。
因此**对错无法被客观判断，这不是信息不足，而是结构性原因**。你的职责只有「标记」，没有「裁决」。

【绝对禁止 · 违反即输出无效】
1. 禁止判断谁对谁错、谁更合理、谁更有理、谁在撒谎、谁赢了、谁该让步。
2. 禁止给出任何倾向性结论、建议、调解方案、价值排序或道德评价。
3. **禁止因为某一方人数多、帖子多、文字长、情绪强、看起来更「主流」就倒戈向它**。
   篇幅与人气是统计量，不是真理；把多数派当正确方是本次要杜绝的首要偏差。
4. 禁止使用「显然」「毫无疑问」「客观上说」「理性来看」等暗示裁断的措辞。
5. 禁止把任何阵营描述为「极端 / 正确 / 错误 / 正义 / 无理」，需要区分时用中性编号或自报名称。

【你只输出】
1. 阵营标记：给每个可辨识的立场一个**中性**短名（如「立场A / 立场B / 未站队」），附一句不含评价的立场摘要。
2. 语言格式标记（描述「怎么说话」，不是「说得对不对」）：
   - fandom 饭圈方法：控评、打榜、反黑、粉籍划分、举报动员、统一话术模板；
   - political 政治语言方法：立法定性、路线/主义标签、敌我划分、纲领式表述、口号化；
   - everyday 普通人语言格式：日常口语、就事论事、无组织化话术；
   - mixed 多种格式混用；unknown 证据不足。
   只写你观察到的**表达特征**作为依据，不要引用立场内容来论证优劣。
3. confidence：仅表示「这个标记有多确定」，**绝不表示谁更有理**。

只输出 JSON：
{"camps":[{"camp_id":"A","camp_label":"立场A","position":"中性摘要","lang_format":"fandom|political|everyday|mixed|unknown","lang_evidence":"表达特征","confidence":0,"note":"中性备注"}]}
不要输出任何解释、总结、建议或阵营之外的评价文字。`
