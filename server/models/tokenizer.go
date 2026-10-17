package models

import (
	"regexp"
	"strings"
)

// ==================== 轻量分词器（中英混合，零外部依赖） ====================
//
// 设计目标：为「内容热词 / 搜索热词 / 标签热词」提供统一的切词能力。
// 不引第三方分词库（jieba 等），避免 license 审查与构建负担；策略：
//
//  1. 英文/数字词：整词（长度≥2），小写化，停用词过滤；
//  2. CJK 连续段：
//     - 长度 ≤ cjkShortRun（4 rune）：整段成词（短语/短查询整体保留，
//       如「修真」「小说推荐」「三体」）；
//     - 更长：二元切分（bigram），含虚字的二元组丢弃（长文降噪）。
//
// 该策略对两类输入都工作良好：搜索短语短而整取，评论/帖文长而降噪。
// 虚字短段（如「我的」「的了」）整段丢弃，防止短路径引入噪声。

const (
	// MaxTokenizeRunes 单次切分输入上限（rune 数），超长截断，防病态输入
	MaxTokenizeRunes = 2000
	// cjkShortRun CJK 连续段 ≤ 该长度时整段成词
	cjkShortRun = 4
)

var (
	// 英文词（长度≥2）
	enWordRe = regexp.MustCompile(`[a-zA-Z][a-zA-Z']{1,}`)
	// 连续中文段（长度≥2）
	cjkRunRe = regexp.MustCompile(`[\p{Han}]{2,}`)
)

// 中文高频虚字：短段整取或二元切分后，含这些字的词一律丢弃，降噪。
const cjkStopChars = "的了是在我有和就都不与及着或一个也没你说他她它们这那什么怎么为因所但还就可已很于上下里外被把让从往对着呢吧啊吗之其或并且如果那么这个人们年月日个次条种些"

// 英文停用词（小写）
var enStopwords = map[string]bool{
	"the": true, "and": true, "for": true, "are": true, "but": true, "not": true,
	"you": true, "all": true, "any": true, "can": true, "had": true, "her": true,
	"was": true, "one": true, "our": true, "out": true, "day": true, "get": true,
	"has": true, "him": true, "his": true, "how": true, "man": true, "new": true,
	"now": true, "old": true, "see": true, "two": true, "way": true, "who": true,
	"boy": true, "did": true, "its": true, "let": true, "put": true, "say": true,
	"she": true, "too": true, "use": true, "that": true, "with": true, "this": true,
	"from": true, "they": true, "have": true, "will": true, "your": true, "said": true,
}

// Tokenize 将一段中英混合文本切分为词列表（可能含重复，计数由调用方累加）。
// 空/纯空白输入返回 nil；超长输入按 MaxTokenizeRunes 截断。
func Tokenize(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	if rs := []rune(text); len(rs) > MaxTokenizeRunes {
		text = string(rs[:MaxTokenizeRunes])
	}

	var out []string

	// 1) 英文整词：小写化 + 停用词过滤
	for _, w := range enWordRe.FindAllString(strings.ToLower(text), -1) {
		if len(w) < 2 || enStopwords[w] {
			continue
		}
		out = append(out, w)
	}

	// 2) CJK 连续段：短段整取（全虚字则丢弃），长段二元切分
	for _, run := range cjkRunRe.FindAllString(text, -1) {
		rs := []rune(run)
		if len(rs) <= cjkShortRun {
			if allStopRune(rs) {
				continue
			}
			out = append(out, run)
			continue
		}
		for i := 0; i+2 <= len(rs); i++ {
			a, b := rs[i], rs[i+1]
			if strings.ContainsRune(cjkStopChars, a) || strings.ContainsRune(cjkStopChars, b) {
				continue
			}
			out = append(out, string([]rune{a, b}))
		}
	}
	return out
}

// allStopRune 判断 rune 序列是否全为虚字（短段整取路径的丢弃判据）。
// 注意不能对短段用「含虚字即丢」：「说」虽在虚字表（服务长文二元切分降噪），
// 但「小说」是本站核心词，全虚字判据才能保留它。
func allStopRune(rs []rune) bool {
	if len(rs) == 0 {
		return true
	}
	for _, r := range rs {
		if !strings.ContainsRune(cjkStopChars, r) {
			return false
		}
	}
	return true
}
