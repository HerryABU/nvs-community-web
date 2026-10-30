package doctext

import (
	"bytes"
	"compress/zlib"
	"errors"
	"io"
	"strconv"
	"strings"
)

// PDF 抽取是「尽力而为」：能拿到文字就拿去当参考文本，拿不到（扫描件、
// 加密文档、CJK 子集字体乱码）就让调用方回退到原样发文件。
//
// 做法：遍历 PDF 里的 stream 段 → 尝试 FlateDecode 解压 → 从内容流里
// 抓取文本显示算子（Tj / TJ / ' / "）里的字符串。

// pdfStreamInflateLimit 单个流解压上限。
const pdfStreamInflateLimit = 8 << 20

func extractPDF(data []byte) (Result, error) {
	if !bytes.HasPrefix(bytes.TrimLeft(data, " \r\n\t"), []byte("%PDF-")) {
		return Result{}, errors.New("不是有效的 PDF")
	}
	var sb strings.Builder
	rest := data
	for {
		start, ok := nextPDFStream(rest)
		if !ok {
			break
		}
		end := bytes.Index(rest[start:], []byte("endstream"))
		if end < 0 {
			break
		}
		seg := rest[start : start+end]
		rest = rest[start+end+len("endstream"):]

		content := inflatePDF(seg)
		if content == nil {
			// 没压缩（少数 PDF 直接裸存内容流）
			content = seg
		}
		if txt := pdfContentText(content); txt != "" {
			sb.WriteString(txt)
			sb.WriteString("\n")
		}
	}
	text := sb.String()
	if !looksLikeText(text) {
		return Result{}, errors.New("PDF 中没有可抽取的文本（可能是扫描件或加密文档）")
	}
	return finish(text, "pdf"), nil
}

// nextPDFStream 找到下一个 stream 段的数据起点（跳过 "endstream" 的误匹配）。
func nextPDFStream(b []byte) (int, bool) {
	idx := 0
	for {
		i := bytes.Index(b[idx:], []byte("stream"))
		if i < 0 {
			return 0, false
		}
		i += idx
		if i >= 3 && bytes.Equal(b[i-3:i], []byte("end")) {
			idx = i + len("stream")
			continue
		}
		j := i + len("stream")
		if j < len(b) && b[j] == '\r' {
			j++
		}
		if j < len(b) && b[j] == '\n' {
			j++
		}
		return j, true
	}
}

// inflatePDF 尝试 zlib（FlateDecode）解压；失败返回 nil。
func inflatePDF(seg []byte) []byte {
	zr, err := zlib.NewReader(bytes.NewReader(seg))
	if err != nil {
		return nil
	}
	defer zr.Close()
	out, err := io.ReadAll(io.LimitReader(zr, pdfStreamInflateLimit))
	if err != nil || len(out) == 0 {
		return nil
	}
	return out
}

// pdfContentText 从内容流里抓文本。
func pdfContentText(c []byte) string {
	var sb strings.Builder
	var pending strings.Builder
	showNext := false

	flush := func() {
		if pending.Len() > 0 {
			sb.WriteString(pending.String())
			pending.Reset()
		}
	}

	i, n := 0, len(c)
	for i < n {
		switch c[i] {
		case '(':
			raw, ni := readPDFLiteral(c, i)
			if showNext {
				sb.WriteString(decodePDFBytes(raw))
				showNext = false
			} else {
				pending.WriteString(decodePDFBytes(raw))
			}
			i = ni
			continue
		case '<':
			if i+1 < n && c[i+1] == '<' {
				i += 2
				continue
			}
			raw, ni := readPDFHex(c, i)
			if showNext {
				sb.WriteString(decodePDFBytes(raw))
				showNext = false
			} else {
				pending.WriteString(decodePDFBytes(raw))
			}
			i = ni
			continue
		case '\'', '"':
			// 文本显示算子：先换行再显示后面的字符串
			flush()
			sb.WriteString("\n")
			showNext = true
			i++
			continue
		}
		if isPDFOpChar(c[i]) {
			op, ni := readPDFOp(c, i)
			switch op {
			case "Tj", "TJ":
				flush()
			case "Td", "TD", "T*", "ET", "BT":
				flush()
				sb.WriteString("\n")
			}
			i = ni
			continue
		}
		i++
	}
	flush()
	return sb.String()
}

// isPDFOpChar 算子字符。
func isPDFOpChar(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || b == '*'
}

// readPDFOp 读一个算子名。
func readPDFOp(c []byte, i int) (string, int) {
	j := i
	for j < len(c) && isPDFOpChar(c[j]) {
		j++
	}
	if j == i {
		return "", i + 1
	}
	return string(c[i:j]), j
}

// readPDFLiteral 读 (...) 字面串（处理转义与嵌套括号）。
func readPDFLiteral(c []byte, i int) ([]byte, int) {
	var b []byte
	depth := 1
	j := i + 1
	for j < len(c) {
		ch := c[j]
		if ch == '\\' {
			j++
			if j >= len(c) {
				break
			}
			switch c[j] {
			case 'n':
				b = append(b, '\n')
				j++
			case 'r':
				b = append(b, '\r')
				j++
			case 't':
				b = append(b, '\t')
				j++
			case 'b':
				b = append(b, '\b')
				j++
			case 'f':
				b = append(b, '\f')
				j++
			case '(', ')', '\\':
				b = append(b, c[j])
				j++
			case '\r':
				j++
				if j < len(c) && c[j] == '\n' {
					j++
				}
			case '\n':
				j++
			default:
				if c[j] >= '0' && c[j] <= '7' {
					v := 0
					for k := 0; k < 3 && j < len(c) && c[j] >= '0' && c[j] <= '7'; k++ {
						v = v*8 + int(c[j]-'0')
						j++
					}
					b = append(b, byte(v))
				} else {
					b = append(b, c[j])
					j++
				}
			}
			continue
		}
		if ch == '(' {
			depth++
			b = append(b, ch)
			j++
			continue
		}
		if ch == ')' {
			depth--
			if depth == 0 {
				return b, j + 1
			}
			b = append(b, ch)
			j++
			continue
		}
		b = append(b, ch)
		j++
	}
	return b, j
}

// readPDFHex 读 <...> 十六进制串。
func readPDFHex(c []byte, i int) ([]byte, int) {
	j := i + 1
	var hexb []byte
	for j < len(c) && c[j] != '>' {
		ch := c[j]
		if (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F') {
			hexb = append(hexb, ch)
		}
		j++
	}
	if j < len(c) {
		j++ // 跳过 '>'
	}
	s := string(hexb)
	if len(s)%2 == 1 {
		s += "0"
	}
	raw := make([]byte, 0, len(s)/2)
	for k := 0; k+1 < len(s); k += 2 {
		v, err := strconv.ParseUint(s[k:k+2], 16, 8)
		if err != nil {
			return nil, j
		}
		raw = append(raw, byte(v))
	}
	return raw, j
}

// decodePDFBytes PDF 里的字符串可能是单字节也可能是 UTF-16BE。
// 高字节大量为 0 时按 UTF-16BE 解，否则按单字节近似解码。
func decodePDFBytes(raw []byte) string {
	if len(raw) >= 2 && len(raw)%2 == 0 {
		zeros := 0
		for i := 0; i < len(raw); i += 2 {
			if raw[i] == 0 {
				zeros++
			}
		}
		if zeros*2 >= len(raw) {
			runes := make([]rune, 0, len(raw)/2)
			for i := 0; i+1 < len(raw); i += 2 {
				runes = append(runes, rune(int(raw[i])<<8|int(raw[i+1])))
			}
			return string(runes)
		}
	}
	var sb strings.Builder
	for _, b := range raw {
		sb.WriteRune(rune(b))
	}
	return sb.String()
}
