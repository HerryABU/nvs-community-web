package security

import (
	"encoding/base64"
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ==================== 编码混淆检测与递归解码 ====================
// 场景：评论/帖子用 Base64、URL 编码、Unicode 转义、Hex、HTML 实体等层层包装，
// 绕开违禁词与语义审核。这里对内容做**有限深度**递归解码，并给出「是否可疑」判断。
//
// 设计原则：
//   - 只解码「看起来像编码」的片段，避免把正常文本破坏掉；
//   - 解码深度有限（默认 5 层），防止无限循环；
//   - 若解码后仍无意义（无中文、无可读词、残留大量编码），标记 suspicious；
//   - 提供凯撒位移与逆向（倒序）尝试，用于「位移/倒装」类黑话。

var (
	// Base64：长度 >=8 且全为 base64 字符（允许末尾 =）
	base64Like = regexp.MustCompile(`^[A-Za-z0-9+/_\-]{8,}={0,2}$`)
	// URL 编码：连续 %XX（至少 3 组）
	urlEncodedLike = regexp.MustCompile(`(?:%[0-9A-Fa-f]{2}){3,}`)
	// Unicode 转义：\uXXXX 或 \UXXXXXXXX（至少 2 组）
	unicodeEscLike = regexp.MustCompile(`(?:\\u[0-9A-Fa-f]{4}|\\U[0-9A-Fa-f]{8}){2,}`)
	// Hex 串：偶数长度且全 hex（至少 8 位）
	hexLike = regexp.MustCompile(`^[0-9A-Fa-f]{8,}$`)
	// HTML 实体：&#x..; &#..; &amp; 等（至少 2 组）
	htmlEntLike = regexp.MustCompile(`(?:&#x[0-9A-Fa-f]+;|&#\d+;|[a-zA-Z]+&[a-zA-Z]+;){2,}`)
	// 纯编码字符占比
	encodedChar = regexp.MustCompile(`(?:%[0-9A-Fa-f]{2}|\\u[0-9A-Fa-f]{4}|&#x?[0-9A-Fa-f]+;)`)
)

// DecodeResult 解码结果
type DecodeResult struct {
	Text   string   // 解码后文本
	Layers []string // 每层解码方式（base64/url/unicode/hex/html）
	Depth  int
}

// LooksEncoded 粗略判断文本是否含明显编码痕迹（用于决定是否尝试解码）
func LooksEncoded(text string) bool {
	return urlEncodedLike.MatchString(text) || unicodeEscLike.MatchString(text) ||
		htmlEntLike.MatchString(text) || encodedChar.MatchString(text)
}

// DecodeLayers 递归解码（最多 maxDepth 层）。
// 每次只对「像编码」的部分解码；解码后不再像编码则停止。
func DecodeLayers(text string, maxDepth int) DecodeResult {
	if maxDepth <= 0 {
		maxDepth = 5
	}
	res := DecodeResult{Text: text}
	cur := text
	for i := 0; i < maxDepth; i++ {
		next, kind, ok := decodeOnce(cur)
		if !ok {
			break
		}
		res.Layers = append(res.Layers, kind)
		cur = next
	}
	res.Depth = len(res.Layers)
	res.Text = cur
	return res
}

// decodeOnce 尝试一种解码方式
func decodeOnce(s string) (string, string, bool) {
	// 1) HTML 实体（最先，避免 &amp; 干扰其他判断）
	if out, ok := decodeHTMLEntities(s); ok {
		return out, "html", true
	}
	// 2) Unicode 转义 \uXXXX
	if unicodeEscLike.MatchString(s) {
		if out, ok := decodeUnicodeEscapes(s); ok {
			return out, "unicode", true
		}
	}
	// 3) URL 编码 %XX
	if urlEncodedLike.MatchString(s) {
		if out, ok := decodeURLEscapes(s); ok {
			return out, "url", true
		}
	}
	// 4) 整段 Base64（去掉空白后判断）
	trimmed := strings.TrimSpace(s)
	if len(trimmed) >= 8 && len(trimmed) <= 4096 && base64Like.MatchString(trimmed) {
		if out, ok := tryBase64(trimmed); ok {
			return out, "base64", true
		}
	}
	// 5) 整段 Hex
	if len(trimmed) >= 8 && len(trimmed)%2 == 0 && hexLike.MatchString(trimmed) {
		if out, ok := tryHex(trimmed); ok {
			return out, "hex", true
		}
	}
	// 6) 片段级 Base64 / Hex（夹在正常文本中的编码块）
	if out, ok := decodeEmbeddedBase64(s); ok {
		return out, "base64", true
	}
	if out, ok := decodeEmbeddedHex(s); ok {
		return out, "hex", true
	}
	return s, "", false
}

// base64Fragment 连续 base64 字符片段（>=8，含末尾 =）
var base64Fragment = regexp.MustCompile(`[A-Za-z0-9+/_\-]{8,}={0,2}`)

// decodeEmbeddedBase64 对文本中「像 base64」的片段逐个尝试解码并替换
func decodeEmbeddedBase64(s string) (string, bool) {
	changed := false
	out := base64Fragment.ReplaceAllStringFunc(s, func(m string) string {
		// 太短或明显是正常英文单词则跳过（解码后必须是可读文本）
		if len(m) < 8 {
			return m
		}
		dec, ok := tryBase64(strings.TrimSpace(m))
		if !ok || dec == m {
			return m
		}
		// 解码结果需含中文或常见可读文本，避免把普通单词改写
		if securityReadability(dec) < 10 && !containsHan(dec) {
			return m
		}
		changed = true
		return dec
	})
	if !changed {
		return s, false
	}
	return out, true
}

// hexFragment 连续 hex 片段（偶数长度 >=8）
var hexFragment = regexp.MustCompile(`\b[0-9A-Fa-f]{8,}\b`)

func decodeEmbeddedHex(s string) (string, bool) {
	changed := false
	out := hexFragment.ReplaceAllStringFunc(s, func(m string) string {
		if len(m)%2 != 0 {
			return m
		}
		dec, ok := tryHex(m)
		if !ok || dec == m {
			return m
		}
		if securityReadability(dec) < 10 && !containsHan(dec) {
			return m
		}
		changed = true
		return dec
	})
	if !changed {
		return s, false
	}
	return out, true
}

func containsHan(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func securityReadability(s string) int {
	return ReadabilityScore(s)
}

func decodeHTMLEntities(s string) (string, bool) {
	if !htmlEntLike.MatchString(s) {
		return s, false
	}
	out := s
	// 数字实体 &#123; / &#x1F;
	numEnt := regexp.MustCompile(`&#(x?)([0-9A-Fa-f]+);`)
	changed := false
	out = numEnt.ReplaceAllStringFunc(out, func(m string) string {
		sub := numEnt.FindStringSubmatch(m)
		if len(sub) != 3 {
			return m
		}
		base := 10
		if sub[1] == "x" || sub[1] == "X" {
			base = 16
		}
		v, err := strconv.ParseInt(sub[2], base, 32)
		if err != nil || v <= 0 || v > 0x10FFFF {
			return m
		}
		changed = true
		return string(rune(v))
	})
	if !changed {
		return s, false
	}
	return out, true
}

func decodeUnicodeEscapes(s string) (string, bool) {
	if !unicodeEscLike.MatchString(s) {
		return s, false
	}
	// 单组匹配：\uXXXX 或 \UXXXXXXXX
	single := regexp.MustCompile(`\\u[0-9A-Fa-f]{4}|\\U[0-9A-Fa-f]{8}`)
	changed := false
	out := single.ReplaceAllStringFunc(s, func(m string) string {
		hexPart := m[2:]
		v, err := strconv.ParseInt(hexPart, 16, 32)
		if err != nil || v <= 0 || v > 0x10FFFF {
			return m
		}
		changed = true
		return string(rune(v))
	})
	if !changed {
		return s, false
	}
	return out, true
}

func decodeURLEscapes(s string) (string, bool) {
	if !urlEncodedLike.MatchString(s) {
		return s, false
	}
	changed := false
	out := urlEncodedLike.ReplaceAllStringFunc(s, func(m string) string {
		decoded := strings.Builder{}
		for i := 0; i < len(m); i += 3 {
			if i+2 >= len(m)+1 {
				break
			}
			v, err := strconv.ParseUint(m[i+1:i+3], 16, 8)
			if err != nil {
				return m
			}
			decoded.WriteByte(byte(v))
		}
		changed = true
		return decoded.String()
	})
	if !changed {
		return s, false
	}
	return out, true
}

func tryBase64(s string) (string, bool) {
	// 尝试标准 base64（含换行去除）
	clean := strings.NewReplacer("\n", "", "\r", "", " ", "", "\t", "").Replace(s)
	data, err := base64.StdEncoding.DecodeString(clean)
	if err != nil {
		// 尝试 URL-safe
		data, err = base64.URLEncoding.DecodeString(clean)
		if err != nil {
			return s, false
		}
	}
	if len(data) == 0 {
		return s, false
	}
	out := string(data)
	if !mostlyPrintable(out) {
		return s, false
	}
	return out, true
}

func tryHex(s string) (string, bool) {
	data, err := hex.DecodeString(s)
	if err != nil || len(data) == 0 {
		return s, false
	}
	out := string(data)
	if !mostlyPrintable(out) {
		return s, false
	}
	return out, true
}

// mostlyPrintable 判断字符串是否「基本可读」（可打印字符/中文/常见空白占比高）
func mostlyPrintable(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	if s == "" {
		return false
	}
	printable := 0
	total := 0
	for _, r := range s {
		total++
		if r == '\n' || r == '\r' || r == '\t' || r == ' ' {
			printable++
		} else if unicode.IsPrint(r) {
			printable++
		}
	}
	if total == 0 {
		return false
	}
	return float64(printable)/float64(total) >= 0.85
}

// CaesarShift 对 ASCII 字母做 n 位位移
func CaesarShift(s string, n int) string {
	var sb strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			sb.WriteRune('a' + (r-'a'+rune(n)%26+26)%26)
		case r >= 'A' && r <= 'Z':
			sb.WriteRune('A' + (r-'A'+rune(n)%26+26)%26)
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// TryCaesar 尝试 1..25 位移，返回可读性最好的文本与得分（0 表示没明显改善）
func TryCaesar(s string) (string, int) {
	best := s
	bestScore := ReadabilityScore(s)
	for n := 1; n < 26; n++ {
		cand := CaesarShift(s, n)
		sc := ReadabilityScore(cand)
		if sc > bestScore {
			best, bestScore = cand, sc
		}
	}
	return best, bestScore
}

// ReverseText 倒序（逆向语义尝试）
func ReverseText(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// 常见词（用于可读性评分）
var commonWords = []string{"的", "是", "我", "你", "他", "她", "它", "们", "在", "有", "和", "了", "就", "都", "而", "及", "与", "或", "这", "那", "不", "人", "说", "要", "去", "来", "上", "下", "中", "大", "小", "好", "可", "也", "很", "会", "能", "对", "为", "以", "及", "被", "把", "从", "到"}

// ReadabilityScore 可读性评分：中文比例 + 常见词命中 + 标点/空格合理性
func ReadabilityScore(s string) int {
	if s == "" {
		return 0
	}
	runes := []rune(s)
	cjk := 0
	for _, r := range runes {
		if unicode.Is(unicode.Han, r) {
			cjk++
		}
	}
	cjkRatio := float64(cjk) / float64(len(runes))
	score := int(cjkRatio * 60)
	hasCommon := false
	// 常见词命中（中文）
	for _, w := range commonWords {
		if strings.Contains(s, w) {
			score += 3
			hasCommon = true
		}
	}
	// 常见英文词命中（便于识别凯撒位移后的可读文本）
	lower := strings.ToLower(s)
	for _, w := range []string{"the", "and", "hello", "world", "this", "that", "with", "for", "you", "not", "are", "have"} {
		if strings.Contains(lower, w) {
			score += 4
			hasCommon = true
		}
	}
	// 既无中文也无常见词、且字母数字占比极高 → 扣分（疑似乱码/编码残留）
	if !hasCommon && cjkRatio < 0.05 {
		letterDigit := 0
		for _, r := range runes {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				letterDigit++
			}
		}
		if float64(letterDigit)/float64(len(runes)) > 0.8 {
			score -= 20
		}
	}
	if score < 0 {
		score = 0
	}
	return score
}

// SuspicionScore 编码可疑度评分（0-100）
func SuspicionScore(original, decoded string, layers []string) int {
	score := 0
	// 解码层数越多越可疑
	score += len(layers) * 12
	// 原文编码字符占比高
	if original != "" {
		enc := len(encodedChar.FindAllString(original, -1))
		score += min(30, enc*3)
	}
	// 解码后仍不可读
	if decoded != "" {
		rs := ReadabilityScore(decoded)
		if rs < 20 {
			score += 25
		} else if rs < 40 {
			score += 10
		}
	}
	// 解码后仍残留编码
	if LooksEncoded(decoded) {
		score += 20
	}
	return min(100, score)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
