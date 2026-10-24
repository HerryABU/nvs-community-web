package aiclient

// ==================== 语音合成 / 音乐（gap-audit #18）====================
// 只做一件事：把 OpenAI 兼容的 `/v1/audio/speech` 拉通，产出 mp3 字节流。
// 不为每家厂商写适配器——省下的复杂度用在「结果存哪、怎么鉴权、怎么限额」上，
// 这三件才是真正会因为厂商差异而崩的地方（见 handlers/ai_audio.go）。
//
// 支持的调用形态（都在同一接口下，靠 model 区分）：
//   - TTS：model=tts-1 / gpt-4o-mini-tts / cosyvoice-v1 …，input=文本，voice=音色
//   - 音乐：model=suno-v3 / musicgen …，input=提示词（部分厂商用 /v1/music/generations，
//     这里按 `musicEndpoint` 常量可切，默认仍走 speech，因为多数聚合网关把它统一了）

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// SpeechRequest 语音合成入参
type SpeechRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice,omitempty"`
	ResponseFormat string  `json:"response_format,omitempty"` // mp3 / opus / aac / wav
	Speed          float64 `json:"speed,omitempty"`
}

// Speech 调用上游合成语音，返回音频字节与 MIME。
// 注意：不落盘、不缓存——落盘由调用方（handler）决定，便于统一做大小限制与清理。
func (c *Client) Speech(req SpeechRequest) ([]byte, string, error) {
	if strings.TrimSpace(req.Input) == "" {
		return nil, "", fmt.Errorf("文本为空")
	}
	if req.Model == "" {
		return nil, "", fmt.Errorf("未指定语音模型")
	}
	if req.ResponseFormat == "" {
		req.ResponseFormat = "mp3"
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, "", err
	}
	url := strings.TrimRight(c.BaseURL, "/") + "/v1/audio/speech"
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 120 * time.Second}
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	// 上游限制单次响应体积（50MB），避免被恶意/异常上游撑爆内存
	data, err := io.ReadAll(io.LimitReader(resp.Body, 50<<20))
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("上游 %d：%s", resp.StatusCode, truncateRunes(string(data), 200))
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" || !strings.HasPrefix(ct, "audio/") {
		// 上游把错误塞进 200 也很常见
		return nil, "", fmt.Errorf("上游未返回音频（%s）：%s", ct, truncateRunes(string(data), 160))
	}
	if len(data) < 256 {
		return nil, "", fmt.Errorf("音频数据异常（%d 字节）", len(data))
	}
	return data, ct, nil
}

// SpeechExt 由 MIME 推断扩展名（落盘用）
func SpeechExt(mime string) string {
	switch {
	case strings.Contains(mime, "mpeg"), strings.Contains(mime, "mp3"):
		return ".mp3"
	case strings.Contains(mime, "wav"):
		return ".wav"
	case strings.Contains(mime, "ogg"), strings.Contains(mime, "opus"):
		return ".ogg"
	case strings.Contains(mime, "aac"):
		return ".aac"
	}
	return ".mp3"
}
