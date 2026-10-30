package doctext

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"sort"
	"strconv"
	"strings"
)

// ooxml 读取上限：单个 entry 解压后最多读 32MB（防 ZIP 炸弹）。
const ooxmlEntryLimit = 32 << 20

// zipOpen 打开 zip（不落盘，直接在内存里读）。
func zipOpen(data []byte) (*zip.Reader, error) {
	if len(data) == 0 {
		return nil, errors.New("空文件")
	}
	return zip.NewReader(bytes.NewReader(data), int64(len(data)))
}

// readEntry 读一个 zip entry（带上限）。
func readEntry(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, ooxmlEntryLimit))
}

// ───────────────────────────── docx ─────────────────────────────

func extractDocx(data []byte) (Result, error) {
	zr, err := zipOpen(data)
	if err != nil {
		return Result{}, err
	}
	for _, f := range zr.File {
		if f.Name != "word/document.xml" {
			continue
		}
		raw, err := readEntry(f)
		if err != nil {
			return Result{}, err
		}
		text := docxText(bytes.NewReader(raw))
		if !looksLikeText(text) {
			return Result{}, errors.New("docx 中没有可抽取的正文")
		}
		return finish(text, "ooxml"), nil
	}
	return Result{}, errors.New("不是有效的 docx（缺少 word/document.xml）")
}

// docxText 解析 word/document.xml：取 <w:t> 文本，段落/单元格处断行。
func docxText(r io.Reader) string {
	dec := xml.NewDecoder(r)
	var sb strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t":
				var s string
				if err := dec.DecodeElement(&s, &t); err == nil {
					sb.WriteString(s)
				}
			case "instrText", "delText":
				// 域代码与修订删除的文本：丢弃
				var s string
				_ = dec.DecodeElement(&s, &t)
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "p":
				sb.WriteString("\n")
			case "tc":
				sb.WriteString("\t")
			case "tab":
				sb.WriteString("\t")
			case "br", "cr":
				sb.WriteString("\n")
			}
		}
	}
	return sb.String()
}

// ───────────────────────────── xlsx ─────────────────────────────

func extractXlsx(data []byte) (Result, error) {
	zr, err := zipOpen(data)
	if err != nil {
		return Result{}, err
	}
	shared := readSharedStrings(zr)
	var sb strings.Builder
	for _, f := range sortedSheetFiles(zr, "xl/worksheets/sheet") {
		raw, err := readEntry(f)
		if err != nil {
			continue
		}
		text := xlsxSheetText(bytes.NewReader(raw), shared)
		if strings.TrimSpace(text) == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString("【工作表：" + strings.TrimPrefix(f.Name, "xl/worksheets/") + "】\n")
		sb.WriteString(text)
	}
	if !looksLikeText(sb.String()) {
		return Result{}, errors.New("xlsx 中没有可抽取的内容")
	}
	return finish(sb.String(), "ooxml"), nil
}

// readSharedStrings 读 xl/sharedStrings.xml。
func readSharedStrings(zr *zip.Reader) []string {
	var out []string
	for _, f := range zr.File {
		if f.Name != "xl/sharedStrings.xml" {
			continue
		}
		raw, err := readEntry(f)
		if err != nil {
			return out
		}
		dec := xml.NewDecoder(bytes.NewReader(raw))
		var cur strings.Builder
		inSI := false
		for {
			tok, err := dec.Token()
			if err != nil {
				break
			}
			switch t := tok.(type) {
			case xml.StartElement:
				switch t.Name.Local {
				case "si":
					inSI = true
					cur.Reset()
				case "t":
					if inSI {
						var s string
						if err := dec.DecodeElement(&s, &t); err == nil {
							cur.WriteString(s)
						}
					}
				}
			case xml.EndElement:
				if t.Name.Local == "si" {
					out = append(out, cur.String())
					inSI = false
				}
			}
		}
	}
	return out
}

// xlsxSheetText 把一个工作表解析成 TSV 文本。
func xlsxSheetText(r io.Reader, shared []string) string {
	dec := xml.NewDecoder(r)
	var rows []string
	var cells []string
	var val strings.Builder
	colIdx := 0
	cellType := ""
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "row":
				cells = nil
				colIdx = 0
			case "c":
				cellType = xmlAttr(t, "t")
				val.Reset()
				if c, ok := colFromRef(xmlAttr(t, "r")); ok {
					colIdx = c
				}
			case "v":
				var s string
				if err := dec.DecodeElement(&s, &t); err == nil {
					val.WriteString(s)
				}
			case "is":
				// 内联字符串：容器里还有 <t>
				for {
					tk, err := dec.Token()
					if err != nil {
						break
					}
					if se, ok := tk.(xml.StartElement); ok && se.Name.Local == "t" {
						var s string
						if err := dec.DecodeElement(&s, &se); err == nil {
							val.WriteString(s)
						}
					}
					if ee, ok := tk.(xml.EndElement); ok && ee.Name.Local == "is" {
						break
					}
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "c":
				text := val.String()
				if cellType == "s" {
					if i, err := strconv.Atoi(text); err == nil && i >= 0 && i < len(shared) {
						text = shared[i]
					}
				}
				for len(cells) < colIdx {
					cells = append(cells, "")
				}
				cells = append(cells, text)
				colIdx++
			case "row":
				rows = append(rows, strings.Join(cells, "\t"))
			}
		}
	}
	return strings.Join(rows, "\n")
}

// xmlAttr 取属性值。
func xmlAttr(e xml.StartElement, name string) string {
	for _, a := range e.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

// colFromRef 把单元格引用（如 "B3"）转成列序号（0 起）。
func colFromRef(ref string) (int, bool) {
	n := 0
	for i := 0; i < len(ref); i++ {
		c := ref[i]
		switch {
		case c >= 'A' && c <= 'Z':
			n = n*26 + int(c-'A'+1)
		case c >= 'a' && c <= 'z':
			n = n*26 + int(c-'a'+1)
		default:
			if n == 0 {
				return 0, false
			}
			return n - 1, true
		}
	}
	if n == 0 {
		return 0, false
	}
	return n - 1, true
}

// ───────────────────────────── pptx ─────────────────────────────

func extractPptx(data []byte) (Result, error) {
	zr, err := zipOpen(data)
	if err != nil {
		return Result{}, err
	}
	var sb strings.Builder
	for _, f := range sortedSheetFiles(zr, "ppt/slides/slide") {
		raw, err := readEntry(f)
		if err != nil {
			continue
		}
		text := pptxSlideText(bytes.NewReader(raw))
		if strings.TrimSpace(text) == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString("【幻灯片：" + strings.TrimPrefix(f.Name, "ppt/slides/") + "】\n")
		sb.WriteString(text)
	}
	if !looksLikeText(sb.String()) {
		return Result{}, errors.New("pptx 中没有可抽取的内容")
	}
	return finish(sb.String(), "ooxml"), nil
}

// pptxSlideText 取 <a:t> 文本。
func pptxSlideText(r io.Reader) string {
	dec := xml.NewDecoder(r)
	var sb strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "t" {
			var s string
			if err := dec.DecodeElement(&s, &se); err == nil {
				sb.WriteString(s)
				sb.WriteString("\n")
			}
		}
	}
	return sb.String()
}

// sortedSheetFiles 收集匹配前缀的 entry，并按文件名里的数字自然排序
// （slide2 排在 slide10 前面）。
func sortedSheetFiles(zr *zip.Reader, prefix string) []*zip.File {
	var files []*zip.File
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, prefix) && strings.HasSuffix(f.Name, ".xml") {
			files = append(files, f)
		}
	}
	sort.SliceStable(files, func(i, j int) bool {
		return naturalLess(files[i].Name, files[j].Name)
	})
	return files
}

// naturalLess 按「数字段按数值、其余按字典序」比较两个文件名。
func naturalLess(a, b string) bool {
	ai, bi := 0, 0
	for ai < len(a) && bi < len(b) {
		ac, bc := a[ai], b[bi]
		if isDigit(ac) && isDigit(bc) {
			as, ae := ai, ai
			for ae < len(a) && isDigit(a[ae]) {
				ae++
			}
			bs, be := bi, bi
			for be < len(b) && isDigit(b[be]) {
				be++
			}
			an, _ := strconv.Atoi(strings.TrimLeft(a[as:ae], "0"))
			bn, _ := strconv.Atoi(strings.TrimLeft(b[bs:be], "0"))
			if an != bn {
				return an < bn
			}
			ai, bi = ae, be
			continue
		}
		if ac != bc {
			return ac < bc
		}
		ai++
		bi++
	}
	return len(a)-ai < len(b)-bi
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }
