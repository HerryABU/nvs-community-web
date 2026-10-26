package lexicon

import (
	"strings"
	"unicode"
)

// zeroWidth 零宽与方向控制字符：最常用的「加壳」手段，肉眼看不见但能断开关键词。
var zeroWidth = map[rune]bool{
	0x200B: true, 0x200C: true, 0x200D: true, 0x200E: true, 0x200F: true,
	0x2060: true, 0x2061: true, 0x2062: true, 0x2063: true, 0xFEFF: true,
	0x00AD: true, // soft hyphen
}

// separators 会被剥离的「间隔符」：在两个汉字之间塞这些是经典绕过手法。
// 注意不含 。，！ 这类真正的句读——跨句拼接会制造大量假命中。
var separator = map[rune]bool{
	' ': true, '\t': true, '\n': true, '\r': true,
	'·': true, '・': true, '•': true, '‧': true, '∙': true,
	'.': true, '*': true, '-': true, '_': true, '~': true,
	'|': true, '/': true, '\\': true, '+': true, '=': true,
	'\'': true, '"': true, '`': true, '^': true,
}

// variantRunes 常见形近/异体替换（保守集合，只放确定无歧义的）
var variantRunes = map[rune]rune{
	'屄': '逼',
	'殘': '残',
	'廢': '废',
	'樂': '乐',
	'媽': '妈',
	'傻': '傻', // 占位，保持 map 语义清晰
	'腦': '脑',
	'罵': '骂',
	'戱': '戏',
}

// asciiFold 全角 ASCII（！＂＃…～）折半角，同时处理全角空格。
func asciiFold(r rune) rune {
	if r == 0x3000 {
		return ' '
	}
	if r >= 0xFF01 && r <= 0xFF5E {
		return r - 0xFEE0
	}
	return r
}

// Normalize 归一化：全角折半角、去零宽、异体字归一、压缩连续重复字符。
//
// 返回两套文本：
//   - plain  保留分隔符（用于「本来就是分开写的词」的精确匹配）
//   - compact 剥离分隔符并压缩重复字符（用于「乐·子」「典典典」这类加壳）
func Normalize(s string) (plain, compact string) {
	var p, c strings.Builder
	p.Grow(len(s))
	c.Grow(len(s))

	var lastKept rune
	for _, r := range s {
		if zeroWidth[r] {
			continue
		}
		r = asciiFold(r)
		if v, ok := variantRunes[r]; ok && v != r {
			r = v
		}
		if unicode.IsControl(r) {
			continue
		}
		p.WriteRune(r)
		if separator[r] {
			continue // compact 里不要分隔符
		}
		// 压缩连续重复（典典典典 → 典），但保留两个：像「哈哈哈」要留出情绪痕迹
		if r == lastKept {
			continue
		}
		c.WriteRune(r)
		lastKept = r
	}
	return p.String(), c.String()
}
