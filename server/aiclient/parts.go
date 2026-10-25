package aiclient

import (
	"strings"
)

// ==================== 多模态 content parts ====================
// OpenAI 兼容上游：content 可以是 string，也可以是 parts 数组：
//   {"type":"text","text":"..."}
//   {"type":"image_url","image_url":{"url":"data:image/png;base64,..."}}
//   {"type":"file","file":{"filename":"a.pdf","file_data":"data:application/pdf;base64,..."}}
//
// Anthropic：形态不同（image/document + source.base64），由 ClaudeBlocksFromParts 转换。
//
// 约定：文件「原样」发送给上游，平台不做本地解析（PDF/docx 由模型侧读取）。

// ContentPart OpenAI 兼容的多模态 content 元素
type ContentPart struct {
	Type     string        `json:"type"` // text / image_url / file
	Text     string        `json:"text,omitempty"`
	ImageURL *ImageURLPart `json:"image_url,omitempty"`
	File     *FilePart     `json:"file,omitempty"`
}

// ImageURLPart 图片部分：URL 可为 https 地址，也可为 data:image/...;base64 内联
type ImageURLPart struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"` // low / high / auto
}

// FilePart 文件部分：文件内容以 data URL 内联（OpenAI 的 file input 用法）
type FilePart struct {
	Filename string `json:"filename"`
	FileData string `json:"file_data"`
}

// MultimodalContent 组装 content。
// 没有任何附件时退化成纯字符串——有些上游/老模型不接受 content 数组，
// 没附件就别用数组，避免无谓的兼容问题。
func MultimodalContent(text string, images []ImageURLPart, files []FilePart) any {
	if len(images) == 0 && len(files) == 0 {
		return text
	}
	parts := make([]ContentPart, 0, len(images)+len(files)+1)
	if strings.TrimSpace(text) != "" {
		parts = append(parts, ContentPart{Type: "text", Text: text})
	}
	for i := range images {
		if strings.TrimSpace(images[i].URL) == "" {
			continue
		}
		parts = append(parts, ContentPart{Type: "image_url", ImageURL: &images[i]})
	}
	for i := range files {
		if strings.TrimSpace(files[i].FileData) == "" {
			continue
		}
		parts = append(parts, ContentPart{Type: "file", File: &files[i]})
	}
	if len(parts) == 0 {
		return text
	}
	if len(parts) == 1 && parts[0].Type == "text" {
		return parts[0].Text
	}
	return parts
}

// SplitDataURL 拆 "data:image/png;base64,xxxx" → (image/png, xxxx)。
// 不是 data URL 时返回 ("", "")。
func SplitDataURL(s string) (mediaType, data string) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "data:") {
		return "", ""
	}
	comma := strings.Index(s, ",")
	if comma < 0 {
		return "", ""
	}
	head := s[len("data:"):comma] // image/png;base64
	if !strings.HasSuffix(head, ";base64") {
		return "", ""
	}
	mt := strings.TrimSuffix(head, ";base64")
	if mt == "" {
		return "", ""
	}
	return mt, s[comma+1:]
}

// ClaudeBlocksFromParts 把 OpenAI 风格 parts 转成 Anthropic 的 content block 数组：
//
//	text     → {"type":"text","text":...}
//	image    → {"type":"image","source":{"type":"base64","media_type":mt,"data":...}}
//	file     → {"type":"document","source":{"type":"base64","media_type":mt,"data":...}}
//
// 远程 URL 的图片走 source.type=url（Anthropic 支持）；文件必须是内联 base64。
func ClaudeBlocksFromParts(parts []ContentPart) []map[string]any {
	blocks := make([]map[string]any, 0, len(parts))
	for _, p := range parts {
		switch p.Type {
		case "text":
			if strings.TrimSpace(p.Text) == "" {
				continue
			}
			blocks = append(blocks, map[string]any{"type": "text", "text": p.Text})
		case "image_url":
			if p.ImageURL == nil || strings.TrimSpace(p.ImageURL.URL) == "" {
				continue
			}
			if mt, data := SplitDataURL(p.ImageURL.URL); data != "" {
				blocks = append(blocks, map[string]any{
					"type":   "image",
					"source": map[string]any{"type": "base64", "media_type": mt, "data": data},
				})
			} else {
				blocks = append(blocks, map[string]any{
					"type":   "image",
					"source": map[string]any{"type": "url", "url": p.ImageURL.URL},
				})
			}
		case "file":
			if p.File == nil {
				continue
			}
			mt, data := SplitDataURL(p.File.FileData)
			if data == "" {
				continue
			}
			blocks = append(blocks, map[string]any{
				"type":   "document",
				"source": map[string]any{"type": "base64", "media_type": mt, "data": data},
			})
		}
	}
	return blocks
}
