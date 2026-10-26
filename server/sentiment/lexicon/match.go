package lexicon

import (
	"strings"
	"unicode"

	"nvs-server/sentiment/topic"
)

// Hit 一次词条命中。
type Hit struct {
	Word          string
	Cat           string
	Severity      int
	EscalateUnder []topic.Group
	// Variant 是否靠「变形归一」才匹配上（原样没匹配，加壳被拆穿）
	Variant bool
}

// Scan 用内置基线词表扫描。
func Scan(text string) []Hit { return ScanWith(Baseline, text) }

// ScanWith 用给定词表扫描，供后台自定义词条复用同一套抗变形逻辑。
//
// 匹配策略：原样文本命中优先；否则看 compact 文本，
// 这样「乐·子」「傻 逼」「典典典典中典」都能命中，而正常句子不会因为
// 去掉标点就误命中（compact 只剥间隔符，不剥句读）。
//
// ⚠️ 单字词（典 / 孝 / 急）单独处理：直接 contains 会把「经典」「字典」
// 「孝顺」全判成黑话。这类词只有**独立成词**时才算命中
// （整条消息就是这个字、或前后是标点/空格、或连续重复 3 次以上）。
func ScanWith(terms []Term, text string) []Hit {
	if strings.TrimSpace(text) == "" || len(terms) == 0 {
		return nil
	}
	plain, compact := Normalize(text)
	plainLower := strings.ToLower(plain)
	compactLower := strings.ToLower(compact)
	rawLower := strings.ToLower(text)

	// 单字词的独立出现位置（一次算好，避免每个词重复扫）
	standaloneRunes := standaloneChars(text)

	var hits []Hit
	for _, t := range terms {
		if t.Word == "" {
			continue
		}
		w := strings.ToLower(t.Word)
		runes := []rune(w)

		var matched bool
		switch {
		case len(runes) == 1:
			// 独立成词 / 连续重复 / 整条消息只有它
			matched = standaloneRunes[runes[0]] || strings.Count(compactLower, w) >= 3
		default:
			matched = strings.Contains(plainLower, w) || strings.Contains(compactLower, w)
		}
		if !matched {
			continue
		}

		// Variant 的语义是「原样文本里没有，靠归一化才浮出来」。
		// 用原始文本判定，而不是用归一化后的文本 —— 否则零宽/全角这类
		// 已经被 Normalize 抹平的加壳会被记成「原样命中」，后台就看不到伪装。
		variant := !strings.Contains(rawLower, w)

		hits = append(hits, Hit{
			Word: t.Word, Cat: t.Cat, Severity: t.Severity,
			EscalateUnder: t.EscalateUnder, Variant: variant,
		})
	}
	return hits
}

// isBoundaryRune 标点 / 空格 / 符号 / 表情都算词边界。
func isBoundaryRune(r rune) bool {
	if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
		return true
	}
	// CJK 标点不在 IsPunct 的覆盖里（它们属于 Po 但 unicode 包已包含），
	// 兜底再加几个高频的。
	switch r {
	case '，', '。', '！', '？', '、', '；', '：', '“', '”', '‘', '’', '（', '）', '～', '…':
		return true
	}
	return false
}

// standaloneChars 找出「独立成词」的字符：前后都是边界，或整条消息只有它。
func standaloneChars(text string) map[rune]bool {
	out := map[rune]bool{}
	rs := []rune(text)
	for i, r := range rs {
		beforeOK := i == 0 || isBoundaryRune(rs[i-1])
		afterOK := i == len(rs)-1 || isBoundaryRune(rs[i+1])
		if beforeOK && afterOK {
			out[r] = true
		}
	}
	return out
}

// Flood 刷屏/出征的行为特征。词表只能抓「说了什么」，这里抓「怎么说的」。
type Flood struct {
	// Repeated 同一短语连续重复的最大次数
	Repeated int
	// Mentions @ 提及人数
	Mentions int
	// PunctuationRun 连续感叹/问号的最大长度（!!!!!）
	PunctuationRun int
	// Shouting 大段全大写或大量叠字（用于「喊话」判定）
	Shouting bool
}

// DetectFlood 检测刷屏特征。仅返回事实，是否处置由 guard 决定。
func DetectFlood(text string) Flood {
	var f Flood
	if text == "" {
		return f
	}
	// @提及数
	f.Mentions = strings.Count(text, "@")

	// 连续标点
	run := 0
	for _, r := range text {
		if r == '!' || r == '！' || r == '?' || r == '？' {
			run++
			if run > f.PunctuationRun {
				f.PunctuationRun = run
			}
		} else {
			run = 0
		}
	}

	// 短语重复：按 2-6 字的窗口统计出现次数（上限取最长重复）
	runes := []rune(text)
	if len(runes) >= 6 {
		for size := 2; size <= 6; size++ {
			if len(runes) < size*3 {
				continue
			}
			counts := map[string]int{}
			for i := 0; i+size <= len(runes); i++ {
				seg := string(runes[i : i+size])
				if strings.TrimSpace(seg) == "" {
					continue
				}
				counts[seg]++
				if counts[seg] > f.Repeated {
					f.Repeated = counts[seg]
				}
			}
		}
	}

	// 叠字喊话：同一字符连续 6 次以上
	dup, maxDup := rune(0), 0
	cur := 0
	for _, r := range runes {
		if r == dup {
			cur++
		} else {
			dup, cur = r, 1
		}
		if cur > maxDup {
			maxDup = cur
		}
	}
	f.Shouting = maxDup >= 6

	return f
}
