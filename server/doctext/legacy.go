package doctext

import (
	"errors"
	"strings"
)

// 旧版 .doc（Word 97-2003）是复合二进制格式，没有 XML 可解析。
// 这里做启发式抽取：找文件里最长的可打印字符游程。
//
// 局限：中文 .doc 的正文多为 GBK/UTF-16 混合存储，启发式可能抽不全或抽不到。
// 抽不到时由调用方回退到「原样发文件」，由支持 file part 的上游（如 Claude）自行解析。

func extractLegacyDoc(data []byte) (Result, error) {
	text := longestPrintableRun(data)
	if !looksLikeText(text) {
		return Result{}, errors.New("旧版 .doc 中没有可抽取的文本")
	}
	return finish(strings.TrimSpace(text), "legacy"), nil
}

// longestPrintableRun 取最长的可打印游程：
//   - ASCII 连续段（英文 .doc 正文常见形态）
//   - UTF-16LE 形态的 ASCII 段（高字节为 0）
func longestPrintableRun(data []byte) string {
	best := ""

	// ASCII 连续段
	start := -1
	for i := 0; i <= len(data); i++ {
		ok := i < len(data) && data[i] >= 0x20 && data[i] < 0x7f
		if ok {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			if i-start > len(best) {
				best = string(data[start:i])
			}
			start = -1
		}
	}

	// UTF-16LE 的 ASCII 段（每两个字节一个字符，高字节为 0）
	var sb strings.Builder
	flush := func() {
		if sb.Len() > len(best) {
			best = sb.String()
		}
		sb.Reset()
	}
	for i := 0; i+1 < len(data); i += 2 {
		lo, hi := data[i], data[i+1]
		if hi == 0 && lo >= 0x20 && lo < 0x7f {
			sb.WriteByte(lo)
			continue
		}
		flush()
	}
	flush()

	return best
}
