// Google Gemini 图像生成适配器（nanobanana / gemini-2.5-flash-image 等图像模型）
// 走原生 generateContent：responseModalities=["TEXT","IMAGE"]，返回 inlineData(base64)。
package aiclient

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// GeminiImageRequest generateContent 请求
type GeminiImageRequest struct {
	Contents         []geminiContent `json:"contents"`
	GenerationConfig struct {
		ResponseModalities []string `json:"responseModalities,omitempty"`
	} `json:"generationConfig"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text       string          `json:"text,omitempty"`
	InlineData *geminiInline   `json:"inlineData,omitempty"`
}

type geminiInline struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

// geminiResp 响应（只取需要的字段）
type geminiResp struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	PromptFeedback struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback"`
	Error struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
}

// GeminiImage 生成图片，返回 data URL 列表（base64 内联，前端可直接展示）
// model 形如 "gemini-2.5-flash-image" / "gemini-3-pro-image-preview"
func (c *Client) GeminiImage(model, prompt, aspectRatio string) ([]string, string, error) {
	if c.BaseURL == "" {
		return nil, "", fmt.Errorf("上游地址未配置")
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nil, "", fmt.Errorf("提示词为空")
	}
	// 画幅：Gemini 无 size 参数，用提示词约束（官方推荐写法）
	switch strings.ToLower(strings.TrimSpace(aspectRatio)) {
	case "16:9", "3:2", "1792x1024":
		prompt += "\n\n(aspect ratio 16:9, landscape)"
	case "9:16", "2:3", "1024x1792":
		prompt += "\n\n(aspect ratio 9:16, portrait)"
	case "1:1", "1024x1024":
		prompt += "\n\n(aspect ratio 1:1, square)"
	}
	var reqBody GeminiImageRequest
	reqBody.Contents = []geminiContent{{Parts: []geminiPart{{Text: prompt}}}}
	reqBody.GenerationConfig.ResponseModalities = []string{"TEXT", "IMAGE"}
	body, _ := json.Marshal(reqBody)

	base := strings.TrimRight(c.BaseURL, "/")
	var url string
	if strings.Contains(base, "/v1beta") || strings.Contains(base, "/v1") {
		url = base + "/models/" + model + ":generateContent"
	} else {
		url = base + "/v1beta/models/" + model + ":generateContent"
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("x-goog-api-key", c.APIKey)
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("Gemini 上游连接失败：%v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20)) // 图片 base64 较大，放宽到 32MB
	var gr geminiResp
	if uerr := json.Unmarshal(raw, &gr); uerr != nil {
		return nil, "", fmt.Errorf("Gemini 响应解析失败（HTTP %d）", resp.StatusCode)
	}
	if gr.Error.Message != "" {
		return nil, "", fmt.Errorf("Gemini 返回错误：%s", gr.Error.Message)
	}
	if gr.PromptFeedback.BlockReason != "" {
		return nil, "", fmt.Errorf("内容被上游安全策略拦截：%s", gr.PromptFeedback.BlockReason)
	}
	if len(gr.Candidates) == 0 {
		return nil, "", fmt.Errorf("Gemini 未返回候选结果")
	}
	images := []string{}
	text := ""
	for _, p := range gr.Candidates[0].Content.Parts {
		if p.InlineData != nil && p.InlineData.Data != "" {
			mime := p.InlineData.MimeType
			if mime == "" {
				mime = "image/png"
			}
			images = append(images, "data:"+mime+";base64,"+p.InlineData.Data)
		}
		if p.Text != "" {
			text += p.Text
		}
	}
	if len(images) == 0 {
		if text != "" {
			return nil, text, fmt.Errorf("上游只返回了文本未返回图片：%s", truncateRunes(text, 120))
		}
		return nil, "", fmt.Errorf("未生成图片（可能被安全策略拦截，请换描述重试）")
	}
	// 兜底校验 base64 合法性，避免把半截数据传给前端
	for _, u := range images {
		if i := strings.Index(u, ","); i > 0 {
			if _, e := base64.StdEncoding.DecodeString(u[i+1:]); e != nil {
				return nil, "", fmt.Errorf("上游返回的图片数据损坏")
			}
		}
	}
	return images, text, nil
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
