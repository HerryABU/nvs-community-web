// 参考图（图生图 / 混图 / 换脸）支持：OpenAI images/edits（multipart）、Gemini inlineData、MJ base64Array。
package aiclient

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// ImgRef 参考图（前端传 data URL，后端拆成 mime + 原始字节）
type ImgRef struct {
	Mime string
	Data []byte
}

// MaxRefImages 单次最多参考图张数
const MaxRefImages = 3

// ParseDataURL 解析 `data:image/png;base64,xxxx`；不接受远程 URL（避免 SSRF）
func ParseDataURL(s string) (*ImgRef, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "data:") {
		return nil, fmt.Errorf("参考图格式不支持（需上传文件，不支持外链）")
	}
	comma := strings.Index(s, ",")
	if comma < 0 {
		return nil, fmt.Errorf("参考图数据损坏")
	}
	head := s[len("data:"):comma]
	if !strings.Contains(head, "base64") {
		return nil, fmt.Errorf("参考图需为 base64 编码")
	}
	mime := strings.TrimSuffix(head, ";base64")
	if mime == "" {
		mime = "image/png"
	}
	if !strings.HasPrefix(mime, "image/") {
		return nil, fmt.Errorf("参考图类型不支持：%s", mime)
	}
	raw, err := base64.StdEncoding.DecodeString(s[comma+1:])
	if err != nil {
		return nil, fmt.Errorf("参考图 base64 解码失败")
	}
	if len(raw) > 8<<20 {
		return nil, fmt.Errorf("参考图过大（单张上限 8MB）")
	}
	return &ImgRef{Mime: mime, Data: raw}, nil
}

// ImageRefsFromDataURLs 批量解析并限流
func ImageRefsFromDataURLs(list []string) ([]ImgRef, error) {
	out := make([]ImgRef, 0, len(list))
	for i, s := range list {
		if i >= MaxRefImages {
			return nil, fmt.Errorf("参考图最多 %d 张", MaxRefImages)
		}
		r, err := ParseDataURL(s)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, nil
}

// EditImages OpenAI 兼容 /images/edits（multipart，支持多图，用于图生图/混图/换脸）
func (c *Client) EditImages(model, prompt, size string, refs []ImgRef, n int) ([]string, error) {
	if c.BaseURL == "" {
		return nil, fmt.Errorf("上游地址未配置")
	}
	if len(refs) == 0 {
		return nil, fmt.Errorf("缺少参考图")
	}
	if n <= 0 {
		n = 1
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("model", model)
	_ = w.WriteField("prompt", prompt)
	if size != "" {
		_ = w.WriteField("size", size)
	}
	_ = w.WriteField("n", fmt.Sprintf("%d", n))
	for i, r := range refs {
		ext := "png"
		if strings.Contains(r.Mime, "jpeg") || strings.Contains(r.Mime, "jpg") {
			ext = "jpg"
		} else if strings.Contains(r.Mime, "webp") {
			ext = "webp"
		}
		fw, err := w.CreateFormFile("image", fmt.Sprintf("ref%d.%s", i, ext))
		if err != nil {
			return nil, err
		}
		if _, err := fw.Write(r.Data); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	url := strings.TrimRight(c.BaseURL, "/") + "/images/edits"
	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	if c.HTTP.Timeout < 120*time.Second {
		c.HTTP.Timeout = 180 * time.Second
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("上游连接失败：%v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	var ir ImagesResponse
	if err := json.Unmarshal(raw, &ir); err != nil {
		return nil, fmt.Errorf("上游响应解析失败（HTTP %d）", resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var er ErrorResponse
		if json.Unmarshal(raw, &er) == nil && er.Error.Message != "" {
			return nil, fmt.Errorf("上游返回 %d：%s", resp.StatusCode, er.Error.Message)
		}
		return nil, fmt.Errorf("上游返回 %d", resp.StatusCode)
	}
	out := []string{}
	for _, d := range ir.Data {
		if d.URL != "" {
			out = append(out, d.URL)
		} else if d.B64JSON != "" {
			out = append(out, "data:image/png;base64,"+d.B64JSON)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("上游未返回图片")
	}
	return out, nil
}

// GeminiImageWithRefs 带参考图的 generateContent（Nanobanana 图生图/混图）
func (c *Client) GeminiImageWithRefs(model, prompt, aspectRatio string, refs []ImgRef) ([]string, string, error) {
	if len(refs) == 0 {
		return c.GeminiImage(model, prompt, aspectRatio)
	}
	if c.BaseURL == "" {
		return nil, "", fmt.Errorf("上游地址未配置")
	}
	parts := []geminiPart{{Text: prompt}}
	for _, r := range refs {
		parts = append(parts, geminiPart{InlineData: &geminiInline{
			MimeType: r.Mime,
			Data:     base64.StdEncoding.EncodeToString(r.Data),
		}})
	}
	var reqBody GeminiImageRequest
	reqBody.Contents = []geminiContent{{Parts: parts}}
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
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	var gr geminiResp
	if err := json.Unmarshal(raw, &gr); err != nil {
		return nil, "", fmt.Errorf("Gemini 响应解析失败（HTTP %d）", resp.StatusCode)
	}
	if gr.Error.Message != "" {
		return nil, "", fmt.Errorf("Gemini 返回错误：%s", gr.Error.Message)
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
		return nil, text, fmt.Errorf("未生成图片（可能被安全策略拦截）")
	}
	return images, text, nil
}

// MJImagineMulti 带参考图的 MJ 提交（base64Array 支持局部重绘/混图）
func (c *Client) MJImagineMulti(p MJImagineParams, refs []ImgRef, timeout time.Duration) ([]string, error) {
	if len(refs) > 0 {
		arr := make([]string, 0, len(refs))
		for _, r := range refs {
			arr = append(arr, "data:"+r.Mime+";base64,"+base64.StdEncoding.EncodeToString(r.Data))
		}
		p.Base64Arr = arr
	}
	return c.MJImagineAndWait(p, timeout)
}
