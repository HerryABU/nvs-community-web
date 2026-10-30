package doctext

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

// zipFixture 造一个内存 zip。
func zipFixture(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func TestSupported(t *testing.T) {
	for _, ext := range []string{".txt", ".md", ".csv", ".json", ".docx", ".xlsx", ".pptx", ".pdf", ".doc"} {
		if !Supported(ext) {
			t.Errorf("期望支持 %s", ext)
		}
	}
	if Supported(".exe") || Supported(".zip") {
		t.Error("不应支持 exe/zip")
	}
}

func TestExtractPlain(t *testing.T) {
	// BOM + CRLF + 行尾空白
	data := []byte("\xEF\xBB\xBFhello\r\nworld  \r\n\r\n中文行\r\n")
	res, err := Extract(data, ".txt")
	if err != nil {
		t.Fatalf("extract txt: %v", err)
	}
	if !strings.Contains(res.Text, "hello") || !strings.Contains(res.Text, "中文行") {
		t.Errorf("文本抽取不完整: %q", res.Text)
	}
	if strings.Contains(res.Text, "\r") {
		t.Errorf("换行未统一: %q", res.Text)
	}
	if res.Method != "plain" {
		t.Errorf("method = %q, 期望 plain", res.Method)
	}
}

func TestExtractDocx(t *testing.T) {
	data := zipFixture(t, map[string]string{
		"[Content_Types].xml": `<Types/>`,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:body>
<w:p><w:r><w:t>Hello</w:t></w:r></w:p>
<w:p><w:r><w:t xml:space="preserve">中文正文</w:t></w:r></w:p>
<w:p><w:r><w:instrText> PAGE </w:instrText></w:r></w:p>
</w:body></w:document>`,
	})
	res, err := Extract(data, ".docx")
	if err != nil {
		t.Fatalf("extract docx: %v", err)
	}
	if !strings.Contains(res.Text, "Hello") {
		t.Errorf("缺少 Hello: %q", res.Text)
	}
	if !strings.Contains(res.Text, "中文正文") {
		t.Errorf("缺少中文正文: %q", res.Text)
	}
	if strings.Contains(res.Text, "PAGE") {
		t.Errorf("域代码不应被抽出来: %q", res.Text)
	}
}

func TestExtractDocxInvalid(t *testing.T) {
	data := zipFixture(t, map[string]string{"readme.txt": "not a docx"})
	if _, err := Extract(data, ".docx"); err == nil {
		t.Error("非 docx 应报错")
	}
}

func TestExtractXlsx(t *testing.T) {
	data := zipFixture(t, map[string]string{
		"xl/sharedStrings.xml": `<?xml version="1.0"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<si><t>姓名</t></si><si><t>分数</t></si>
</sst>`,
		"xl/worksheets/sheet1.xml": `<?xml version="1.0"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>
<row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c></row>
<row r="2"><c r="A2"><v>95</v></c></row>
</sheetData></worksheet>`,
	})
	res, err := Extract(data, ".xlsx")
	if err != nil {
		t.Fatalf("extract xlsx: %v", err)
	}
	if !strings.Contains(res.Text, "姓名") || !strings.Contains(res.Text, "分数") {
		t.Errorf("表头缺失: %q", res.Text)
	}
	if !strings.Contains(res.Text, "95") {
		t.Errorf("数值缺失: %q", res.Text)
	}
	if !strings.Contains(res.Text, "\t") {
		t.Errorf("列应按制表符分隔: %q", res.Text)
	}
}

func TestExtractPptx(t *testing.T) {
	data := zipFixture(t, map[string]string{
		"ppt/slides/slide1.xml": `<?xml version="1.0"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
       xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp>
<p:txBody><a:p><a:r><a:t>幻灯片标题</a:t></a:r></a:p></p:txBody>
</p:sp></p:spTree></p:cSld></p:sld>`,
	})
	res, err := Extract(data, ".pptx")
	if err != nil {
		t.Fatalf("extract pptx: %v", err)
	}
	if !strings.Contains(res.Text, "幻灯片标题") {
		t.Errorf("未抽到文本: %q", res.Text)
	}
}

func TestExtractPDF(t *testing.T) {
	// 未压缩的内容流
	pdf := "%PDF-1.4\n" +
		"1 0 obj\n<< /Length 44 >>\nstream\nBT /F1 12 Tf 72 720 Td (Hello PDF) Tj ET\nendstream\nendobj\n%%EOF\n"
	res, err := Extract([]byte(pdf), ".pdf")
	if err != nil {
		t.Fatalf("extract pdf: %v", err)
	}
	if !strings.Contains(res.Text, "Hello PDF") {
		t.Errorf("未抽到 PDF 文本: %q", res.Text)
	}
}

func TestExtractPDFScanned(t *testing.T) {
	// 没有文本算子的 PDF（相当于扫描件）→ 应报错，让调用方回退
	pdf := "%PDF-1.4\n1 0 obj\n<< /Length 4 >>\nstream\nq Q\nendstream\nendobj\n%%EOF\n"
	if _, err := Extract([]byte(pdf), ".pdf"); err == nil {
		t.Error("无文本的 PDF 应报错")
	}
}

func TestExtractPDFNotPDF(t *testing.T) {
	if _, err := Extract([]byte("hello"), ".pdf"); err == nil {
		t.Error("非 PDF 应报错")
	}
}

func TestTruncate(t *testing.T) {
	long := strings.Repeat("字", MaxCharsPerDoc+500)
	res, err := Extract([]byte(long), ".txt")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if !res.Truncated {
		t.Error("超长文档应标记 Truncated")
	}
	if len([]rune(res.Text)) > MaxCharsPerDoc+50 {
		t.Errorf("截断后仍过长: %d", len([]rune(res.Text)))
	}
}

func TestUnsupportedExt(t *testing.T) {
	if _, err := Extract([]byte("x"), ".exe"); err != ErrUnsupported {
		t.Errorf("err = %v, 期望 ErrUnsupported", err)
	}
}
