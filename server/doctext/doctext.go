// Package doctext 从常见文档格式中抽取纯文本，用于把用户上传的文档作为
// 「参考文本」注入给上游模型。
//
// 为什么需要它：直接把文档以 OpenAI 的 `file` content part 原样发给上游，
// 只有 Anthropic（Claude document block）这类上游读得懂；DeepSeek / Qwen /
// GLM / Moonshot 等 OpenAI 兼容上游会因为不认识的 content type 直接返回 400。
// 所以在平台侧先把文档解析成文本再注入，解析不出来（扫描件 PDF、旧版 .doc）
// 时由调用方回退到原样 file part，保留 Claude 那条路。
//
// 只用标准库实现（archive/zip + encoding/xml + compress/zlib），不引入第三方依赖。
package doctext

import (
	"bytes"
	"errors"
	"strings"
	"unicode"
)

// MaxCharsPerDoc 单个文档最多注入的字符数，超出截断，避免撑爆模型上下文。
const MaxCharsPerDoc = 12000

// ErrUnsupported 不支持本地抽取的扩展名。
var ErrUnsupported = errors.New("不支持的文档格式")

// Result 抽取结果。
type Result struct {
	Text      string // 抽取到的正文（可能已截断）
	Truncated bool   // 是否因超长被截断
	Method    string // 抽取方式：plain / ooxml / pdf / legacy
}

// Supported 该扩展名是否支持本地抽取。
func Supported(ext string) bool {
	switch strings.ToLower(ext) {
	case ".txt", ".md", ".csv", ".json", ".docx", ".xlsx", ".pptx", ".pdf", ".doc":
		return true
	}
	return false
}

// Extract 按扩展名抽取正文。抽取不出有效文本时返回 error，
// 调用方应据此回退到「原样发文件」的策略。
func Extract(data []byte, ext string) (Result, error) {
	switch strings.ToLower(ext) {
	case ".txt", ".md", ".csv", ".json":
		text := cleanPlain(data)
		if strings.TrimSpace(text) == "" {
			return Result{}, errors.New("文件内容为空")
		}
		return finish(text, "plain"), nil
	case ".docx":
		return extractDocx(data)
	case ".xlsx":
		return extractXlsx(data)
	case ".pptx":
		return extractPptx(data)
	case ".pdf":
		return extractPDF(data)
	case ".doc":
		return extractLegacyDoc(data)
	}
	return Result{}, ErrUnsupported
}

// ───────────────────────────── 内部工具 ─────────────────────────────

// cleanPlain 规整纯文本：去 BOM、去 NUL、统一换行、去行尾空白。
func cleanPlain(data []byte) string {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	data = bytes.ReplaceAll(data, []byte{0x00}, nil)
	s := strings.ReplaceAll(string(data), "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	return strings.Join(lines, "\n")
}

// finish 截断正文并组装结果。
func finish(text, method string) Result {
	r := []rune(text)
	if len(r) <= MaxCharsPerDoc {
		return Result{Text: text, Method: method}
	}
	return Result{
		Text:      string(r[:MaxCharsPerDoc]) + "\n……（文档过长，正文已截断）",
		Truncated: true,
		Method:    method,
	}
}

// usableRatio 有效字符占比：汉字/字母/数字/空白/标点算有效，
// 控制字符与私有区算无效。比例太低说明抽出来是乱码，应当放弃抽取结果。
func usableRatio(s string) float64 {
	if s == "" {
		return 0
	}
	good, total := 0, 0
	for _, r := range s {
		total++
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), unicode.IsSpace(r):
			good++
		case unicode.IsPunct(r), unicode.IsSymbol(r):
			good++
		}
	}
	return float64(good) / float64(total)
}

// looksLikeText 抽取结果的可用性判断（乱码则视为抽取失败）。
func looksLikeText(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return false
	}
	return usableRatio(s) >= 0.55
}
