package aiclient

import "testing"

func TestSplitDataURL(t *testing.T) {
	mt, data := SplitDataURL("data:application/pdf;base64,QUJD")
	if mt != "application/pdf" || data != "QUJD" {
		t.Fatalf("got %q %q", mt, data)
	}
	if mt, _ := SplitDataURL("https://x/a.png"); mt != "" {
		t.Fatalf("非 data URL 应返回空，得到 %q", mt)
	}
	if mt, _ := SplitDataURL("data:image/png,raw"); mt != "" {
		t.Fatalf("缺 ;base64 应返回空，得到 %q", mt)
	}
}

func TestMultimodalContentDegrades(t *testing.T) {
	// 无附件 → 纯字符串（兼容不接受数组的上游）
	if v := MultimodalContent("你好", nil, nil); v != "你好" {
		t.Fatalf("无附件应退化为字符串，得到 %#v", v)
	}
	// 只有图片 → parts 数组
	v := MultimodalContent("看图", []ImageURLPart{{URL: "data:image/png;base64,AA"}}, nil)
	parts, ok := v.([]ContentPart)
	if !ok || len(parts) != 2 || parts[1].Type != "image_url" {
		t.Fatalf("图片未生成 parts：%#v", v)
	}
	// 文件 → type=file
	v = MultimodalContent("读文档", nil, []FilePart{{Filename: "a.pdf", FileData: "data:application/pdf;base64,AA"}})
	parts, ok = v.([]ContentPart)
	if !ok || len(parts) != 2 || parts[1].Type != "file" || parts[1].File == nil {
		t.Fatalf("文件未生成 parts：%#v", v)
	}
}

func TestClaudeBlocksFromParts(t *testing.T) {
	blocks := ClaudeBlocksFromParts([]ContentPart{
		{Type: "text", Text: "看这个"},
		{Type: "image_url", ImageURL: &ImageURLPart{URL: "data:image/png;base64,AA"}},
		{Type: "image_url", ImageURL: &ImageURLPart{URL: "https://x/a.png"}},
		{Type: "file", File: &FilePart{Filename: "a.pdf", FileData: "data:application/pdf;base64,BB"}},
	})
	if len(blocks) != 4 {
		t.Fatalf("期望 4 个 block，得到 %d：%#v", len(blocks), blocks)
	}
	// 内联图片 → image/base64
	src := blocks[1]["source"].(map[string]any)
	if blocks[1]["type"] != "image" || src["type"] != "base64" || src["media_type"] != "image/png" {
		t.Fatalf("内联图片转换错误：%#v", blocks[1])
	}
	// 远程图片 → image/url
	src = blocks[2]["source"].(map[string]any)
	if src["type"] != "url" {
		t.Fatalf("远程图片应走 url：%#v", blocks[2])
	}
	// 文件 → document/base64
	src = blocks[3]["source"].(map[string]any)
	if blocks[3]["type"] != "document" || src["media_type"] != "application/pdf" || src["data"] != "BB" {
		t.Fatalf("文件转换错误：%#v", blocks[3])
	}
}
