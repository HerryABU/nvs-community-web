package aigc

// ==================== AIGC 隐式标识（XMP）读写 ====================
//
// 为什么不用 GoMetadata：该库 v1.6.0 在 go.mod 里声明 `go >= 1.27`，
// 一旦引入就会把整个项目的 Go 版本与 toolchain 抬到 1.27（构建机要重下 SDK）。
// 而我们要写的东西很小 —— 一个 XMP packet 塞进 JPEG 的 APP1 / PNG 的 iTXt / WebP 的 XMP chunk，
// 用标准库百来行就能做完，且完全可控、可单元测试。
//
// 顺序铁律（需求 §11）：编码 → 写 XMP → **此后不得再重编码**，否则元数据丢失。

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"hash/crc32"
	"strings"
)

// AIGCFields 国标附录 E 的七个字段（一个都不能多、不能少）
type AIGCFields struct {
	Label             string `json:"Label"`
	ContentProducer   string `json:"ContentProducer"`
	ProduceID         string `json:"ProduceID"`
	ReservedCode1     string `json:"ReservedCode1"`
	ContentPropagator string `json:"ContentPropagator"`
	PropagateID       string `json:"PropagateID"`
	ReservedCode2     string `json:"ReservedCode2"`
}

// AIGCNamespace 国标字段常用的 XMP 命名空间（TC260 示例）
const AIGCNamespace = "http://www.tc260.org.cn/ns/AIGC/1.0/"

// PromptNamespace 提示词等「业务附加信息」走自己的命名空间，绝不混进法定字段
const PromptNamespace = "urn:myapp:prompt"

// BuildXMP 组装 XMP packet。prompt 非空时追加第二个 rdf:Description。
//
// 用属性（而非子元素）承载字段：exiftool -xmp:all 能直接列出 XMP-AIGC:Label，
// 核验方一眼能看懂；子元素形式在部分解析器里会退化成嵌套结构。
func BuildXMP(f AIGCFields, prompt string) []byte {
	var b strings.Builder
	b.WriteString(`<?xpacket begin="` + "\uFEFF" + `" id="W5M0MpCehiHzreSzNTczkc9d"?>` + "\n")
	b.WriteString(`<x:xmpmeta xmlns:x="adobe:ns:meta/">` + "\n")
	b.WriteString(` <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">` + "\n")
	b.WriteString(`  <rdf:Description rdf:about="" xmlns:AIGC="` + AIGCNamespace + `"` + "\n")
	// 注意用 "%s" 而不是 %q：%q 是 Go 语法转义（\" 而非 &quot;），会产出非法 XML
	fmt.Fprintf(&b, "    AIGC:Label=\"%s\"\n", escAttr(f.Label))
	fmt.Fprintf(&b, "    AIGC:ContentProducer=\"%s\"\n", escAttr(f.ContentProducer))
	fmt.Fprintf(&b, "    AIGC:ProduceID=\"%s\"\n", escAttr(f.ProduceID))
	fmt.Fprintf(&b, "    AIGC:ReservedCode1=\"%s\"\n", escAttr(f.ReservedCode1))
	fmt.Fprintf(&b, "    AIGC:ContentPropagator=\"%s\"\n", escAttr(f.ContentPropagator))
	fmt.Fprintf(&b, "    AIGC:PropagateID=\"%s\"\n", escAttr(f.PropagateID))
	fmt.Fprintf(&b, "    AIGC:ReservedCode2=\"%s\"/>\n", escAttr(f.ReservedCode2))
	if p := strings.TrimSpace(prompt); p != "" {
		b.WriteString(`  <rdf:Description rdf:about="" xmlns:prompt="` + PromptNamespace + `"` + "\n")
		fmt.Fprintf(&b, "    prompt:Text=\"%s\"/>\n", escAttr(p))
	}
	b.WriteString(` </rdf:RDF>` + "\n")
	b.WriteString(`</x:xmpmeta>` + "\n")
	b.WriteString(`<?xpacket end="w"?>`)
	return []byte(b.String())
}

func escAttr(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

// xmpApp1Header JPEG APP1 里 XMP 段的标识
const xmpApp1Header = "http://ns.adobe.com/xap/1.0/\x00"

// pngXMPKeyword PNG iTXt 里 XMP 的标准关键字
const pngXMPKeyword = "XML:com.adobe.xmp"

// InjectXMP 把 XMP packet 写进已编码的图片字节。
// mime 支持 image/jpeg、image/png；其它格式返回 ErrUnsupportedFormat。
func InjectXMP(data []byte, mime string, packet []byte) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(mime)) {
	case "image/jpeg", "image/jpg":
		return injectJPEG(data, packet)
	case "image/png":
		return injectPNG(data, packet)
	case "image/webp":
		return injectWebP(data, packet)
	default:
		return nil, ErrUnsupportedFormat
	}
}

// ErrUnsupportedFormat 该格式不支持写 XMP
var ErrUnsupportedFormat = errors.New("不支持的图片格式（仅支持 JPEG / PNG / WebP）")

// ---------- JPEG ----------

func injectJPEG(data, packet []byte) ([]byte, error) {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return nil, errors.New("不是合法的 JPEG")
	}
	payload := append([]byte(xmpApp1Header), packet...)
	if len(payload)+2 > 0xFFFF {
		return nil, errors.New("XMP 过大，超出 JPEG APP1 段上限")
	}

	// 找插入点：跳过 SOI 与紧随其后的 APP0(JFIF)，遇到下一个段前插入。
	// 已有 xap 段则原位置替换（保证只有一份隐式标识，国标要求）。
	pos := 2
	insertAt := 2
	for pos+4 <= len(data) {
		if data[pos] != 0xFF {
			break
		}
		marker := data[pos+1]
		if marker == 0xDA || marker == 0xD9 { // SOS / EOI
			break
		}
		segLen := int(binary.BigEndian.Uint16(data[pos+2 : pos+4]))
		if segLen < 2 || pos+2+segLen > len(data) {
			break
		}
		body := data[pos+4 : pos+2+segLen]
		isXMP := marker == 0xE1 && bytes.HasPrefix(body, []byte(xmpApp1Header))
		if isXMP {
			out := make([]byte, 0, len(data)-len(body)+len(payload))
			out = append(out, data[:pos]...)
			out = append(out, 0xFF, 0xE1, byte((len(payload)+2)>>8), byte((len(payload)+2)&0xFF))
			out = append(out, payload...)
			out = append(out, data[pos+2+segLen:]...)
			return out, nil
		}
		// APP0 之后就地插入即可；其余段（APPn/DQT 等）前插入也合法
		if marker == 0xE0 {
			insertAt = pos + 2 + segLen
		}
		pos += 2 + segLen
	}

	out := make([]byte, 0, len(data)+len(payload)+4)
	out = append(out, data[:insertAt]...)
	out = append(out, 0xFF, 0xE1, byte((len(payload)+2)>>8), byte((len(payload)+2)&0xFF))
	out = append(out, payload...)
	out = append(out, data[insertAt:]...)
	return out, nil
}

// ---------- PNG ----------

func injectPNG(data, packet []byte) ([]byte, error) {
	sig := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	if len(data) < 8 || !bytes.Equal(data[:8], sig) {
		return nil, errors.New("不是合法的 PNG")
	}
	chunk := buildPNGiTXt(pngXMPKeyword, packet)

	pos := 8
	insertAt := 8
	for pos+8 <= len(data) {
		length := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		typ := string(data[pos+4 : pos+8])
		if typ == "IEND" {
			break
		}
		if pos+12+length > len(data) {
			return nil, errors.New("PNG 数据损坏")
		}
		body := data[pos+8 : pos+8+length]
		if typ == "iTXt" && bytes.HasPrefix(body, []byte(pngXMPKeyword+"\x00")) {
			out := make([]byte, 0, len(data)-len(body)+len(packet)+32)
			out = append(out, data[:pos]...)
			out = append(out, chunk...)
			out = append(out, data[pos+12+length:]...)
			return out, nil
		}
		if typ == "IHDR" {
			insertAt = pos + 12 + length
		}
		pos += 12 + length
	}

	out := make([]byte, 0, len(data)+len(chunk))
	out = append(out, data[:insertAt]...)
	out = append(out, chunk...)
	out = append(out, data[insertAt:]...)
	return out, nil
}

// buildPNGiTXt 组装一个未压缩 iTXt chunk：keyword\0 压缩标志 压缩方法 语言\0 译名\0 正文
func buildPNGiTXt(keyword string, text []byte) []byte {
	body := make([]byte, 0, len(keyword)+len(text)+16)
	body = append(body, []byte(keyword)...)
	body = append(body, 0x00) // keyword 结束
	body = append(body, 0x00) // compression flag = 未压缩
	body = append(body, 0x00) // compression method
	body = append(body, 0x00) // language tag 空
	body = append(body, 0x00) // translated keyword 空
	body = append(body, text...)

	out := make([]byte, 0, len(body)+12)
	var l [4]byte
	binary.BigEndian.PutUint32(l[:], uint32(len(body)))
	out = append(out, l[:]...)
	out = append(out, []byte("iTXt")...)
	out = append(out, body...)
	crc := crc32.NewIEEE()
	crc.Write([]byte("iTXt"))
	crc.Write(body)
	binary.BigEndian.PutUint32(l[:], crc.Sum32())
	out = append(out, l[:]...)
	return out
}

// ---------- WebP ----------

func injectWebP(data, packet []byte) ([]byte, error) {
	if len(data) < 12 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return nil, errors.New("不是合法的 WebP")
	}
	// XMP chunk：FourCC "XMP " + 长度 + 数据（偶数对齐）
	chunk := make([]byte, 0, len(packet)+9)
	chunk = append(chunk, []byte("XMP ")...)
	var l [4]byte
	binary.LittleEndian.PutUint32(l[:], uint32(len(packet)))
	chunk = append(chunk, l[:]...)
	chunk = append(chunk, packet...)
	if len(packet)%2 == 1 {
		chunk = append(chunk, 0x00)
	}

	pos := 12
	for pos+8 <= len(data) {
		sz := int(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
		typ := string(data[pos : pos+4])
		if pos+8+sz > len(data) {
			break
		}
		if typ == "XMP " {
			out := make([]byte, 0, len(data)-sz+len(chunk))
			out = append(out, data[:pos]...)
			out = append(out, chunk...)
			out = append(out, data[pos+8+sz:]...)
			return fixRIFFSize(out), nil
		}
		pos += 8 + sz + sz%2
	}

	out := append([]byte{}, data...)
	out = append(out, chunk...)
	return fixRIFFSize(out), nil
}

// fixRIFFSize 改写 RIFF 长度字段（含 4 字节 "WEBP"）
func fixRIFFSize(b []byte) []byte {
	if len(b) < 8 {
		return b
	}
	var l [4]byte
	binary.LittleEndian.PutUint32(l[:], uint32(len(b)-8))
	copy(b[4:8], l[:])
	return b
}

// ---------- 读取 ----------

// ExtractXMP 从图片字节里取出 XMP packet（没有则返回 nil）
func ExtractXMP(data []byte, mime string) []byte {
	switch strings.ToLower(strings.TrimSpace(mime)) {
	case "image/jpeg", "image/jpg":
		pos := 2
		for pos+4 <= len(data) && data[pos] == 0xFF {
			marker := data[pos+1]
			if marker == 0xDA || marker == 0xD9 {
				break
			}
			segLen := int(binary.BigEndian.Uint16(data[pos+2 : pos+4]))
			if segLen < 2 || pos+2+segLen > len(data) {
				break
			}
			body := data[pos+4 : pos+2+segLen]
			if marker == 0xE1 && bytes.HasPrefix(body, []byte(xmpApp1Header)) {
				return body[len(xmpApp1Header):]
			}
			pos += 2 + segLen
		}
	case "image/png":
		pos := 8
		for pos+8 <= len(data) {
			length := int(binary.BigEndian.Uint32(data[pos : pos+4]))
			typ := string(data[pos+4 : pos+8])
			if typ == "IEND" || pos+12+length > len(data) {
				break
			}
			body := data[pos+8 : pos+8+length]
			prefix := []byte(pngXMPKeyword + "\x00\x00\x00\x00\x00")
			if typ == "iTXt" && bytes.HasPrefix(body, prefix) {
				return body[len(prefix):]
			}
			pos += 12 + length
		}
	case "image/webp":
		pos := 12
		for pos+8 <= len(data) {
			sz := int(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
			if pos+8+sz > len(data) {
				break
			}
			if string(data[pos:pos+4]) == "XMP " {
				return data[pos+8 : pos+8+sz]
			}
			pos += 8 + sz + sz%2
		}
	}
	return nil
}

// HasAIGC 幂等判据：已有 Label="1" 且 ProduceID 非空 → 视为已标识（国标：只保留一份）
func HasAIGC(packet []byte) bool {
	if len(packet) == 0 {
		return false
	}
	s := string(packet)
	label := strings.Contains(s, `AIGC:Label="1"`) || strings.Contains(s, "AIGC:Label='1'")
	// ProduceID 非空：`AIGC:ProduceID="xxx"`
	i := strings.Index(s, `AIGC:ProduceID="`)
	if i < 0 {
		return false
	}
	rest := s[i+len(`AIGC:ProduceID="`):]
	j := strings.IndexByte(rest, '"')
	if j <= 0 {
		return false
	}
	return label && strings.TrimSpace(rest[:j]) != ""
}

// ReadAIGCFields 从已有 XMP 里把七字段读回来（后台核验用）
func ReadAIGCFields(packet []byte) AIGCFields {
	s := string(packet)
	get := func(key string) string {
		p := `AIGC:` + key + `="`
		i := strings.Index(s, p)
		if i < 0 {
			return ""
		}
		rest := s[i+len(p):]
		j := strings.IndexByte(rest, '"')
		if j < 0 {
			return ""
		}
		return unescAttr(rest[:j])
	}
	return AIGCFields{
		Label:             get("Label"),
		ContentProducer:   get("ContentProducer"),
		ProduceID:         get("ProduceID"),
		ReservedCode1:     get("ReservedCode1"),
		ContentPropagator: get("ContentPropagator"),
		PropagateID:       get("PropagateID"),
		ReservedCode2:     get("ReservedCode2"),
	}
}

func unescAttr(s string) string {
	r := strings.NewReplacer(
		"&quot;", `"`, "&#34;", `"`, "&apos;", "'", "&#39;", "'",
		"&lt;", "<", "&gt;", ">", "&amp;", "&",
	)
	return r.Replace(s)
}
