package ai

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nvs-server/config"
)

// writeUpload 把内容写进 uploads/ai/2026/10/ 并返回对外的 URL。
func writeUpload(t *testing.T, name string, content []byte) string {
	t.Helper()
	dir := filepath.Join(config.UploadDir, "ai", "2026", "10")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return "/uploads/ai/2026/10/" + name
}

// docxFixture 造一个最小 docx。
func docxFixture(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatalf("zip create: %v", err)
	}
	if _, err := w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:body><w:p><w:r><w:t>参考正文内容</w:t></w:r></w:p></w:body></w:document>`)); err != nil {
		t.Fatalf("zip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

// 能抽出正文的文档 → 走「参考文本」，而不是原样 file part。
func TestBuildAttachmentParts_DocBecomesReferenceText(t *testing.T) {
	old := config.UploadDir
	config.UploadDir = t.TempDir()
	defer func() { config.UploadDir = old }()

	url := writeUpload(t, "方案.docx", docxFixture(t))
	images, files, docs, err := buildAttachmentParts([]AiChatAttachment{
		{URL: url, Name: "方案.docx", Kind: "file"},
	})
	if err != nil {
		t.Fatalf("buildAttachmentParts: %v", err)
	}
	if len(images) != 0 {
		t.Errorf("文档不该进图片组: %d", len(images))
	}
	if len(files) != 0 {
		t.Errorf("能抽正文的文档不该回退成 file part: %d", len(files))
	}
	if len(docs) != 1 {
		t.Fatalf("应产出 1 段参考文本，得到 %d", len(docs))
	}
	if !strings.Contains(docs[0], "参考正文内容") {
		t.Errorf("参考文本缺正文: %q", docs[0])
	}
	if !strings.Contains(docs[0], "方案.docx") {
		t.Errorf("参考文本应带上文件名: %q", docs[0])
	}
}

// 图片 → 多模态 image_url，不产生参考文本。
func TestBuildAttachmentParts_ImageStaysMultimodal(t *testing.T) {
	old := config.UploadDir
	config.UploadDir = t.TempDir()
	defer func() { config.UploadDir = old }()

	url := writeUpload(t, "a.png", []byte("\x89PNG\r\n\x1a\nfake"))
	images, files, docs, err := buildAttachmentParts([]AiChatAttachment{
		{URL: url, Name: "a.png", Kind: "image"},
	})
	if err != nil {
		t.Fatalf("buildAttachmentParts: %v", err)
	}
	if len(images) != 1 {
		t.Errorf("图片应进多模态组: %d", len(images))
	}
	if !strings.HasPrefix(images[0].URL, "data:image/png;base64,") {
		t.Errorf("图片应是 data URL: %.40s", images[0].URL)
	}
	if len(files) != 0 || len(docs) != 0 {
		t.Errorf("图片不该产生 file part / 参考文本: files=%d docs=%d", len(files), len(docs))
	}
}

// 抽不出正文的文档（扫描件 PDF）→ 回退成原样 file part（留给 Claude 读）。
func TestBuildAttachmentParts_ScannedPDCFallsBackToFilePart(t *testing.T) {
	old := config.UploadDir
	config.UploadDir = t.TempDir()
	defer func() { config.UploadDir = old }()

	pdf := "%PDF-1.4\n1 0 obj\n<< /Length 4 >>\nstream\nq Q\nendstream\nendobj\n%%EOF\n"
	url := writeUpload(t, "scan.pdf", []byte(pdf))
	images, files, docs, err := buildAttachmentParts([]AiChatAttachment{
		{URL: url, Name: "scan.pdf", Kind: "file"},
	})
	if err != nil {
		t.Fatalf("buildAttachmentParts: %v", err)
	}
	if len(docs) != 0 {
		t.Errorf("扫描件不应产出参考文本: %q", docs)
	}
	if len(files) != 1 {
		t.Fatalf("扫描件应回退成 file part: %d", len(files))
	}
	if files[0].Filename != "scan.pdf" {
		t.Errorf("文件名不对: %q", files[0].Filename)
	}
	if len(images) != 0 {
		t.Errorf("不该有图片: %d", len(images))
	}
}

// 路径穿越必须被挡住。
func TestBuildAttachmentParts_RejectsPathTraversal(t *testing.T) {
	old := config.UploadDir
	config.UploadDir = t.TempDir()
	defer func() { config.UploadDir = old }()

	_, _, _, err := buildAttachmentParts([]AiChatAttachment{
		{URL: "/uploads/ai/../../secret.txt", Name: "x.txt", Kind: "file"},
	})
	if err == nil {
		t.Error("路径穿越应被拒绝")
	}
}
