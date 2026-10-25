package aiapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	vparse "nvs-server/aiapi/vendor"
)

// ProviderModelSuggestion 从渠道拉取的模型条目 + 服务端推断的分类信息。
// 前端把推断结果展示给站长，由站长按需勾选导入并决定是否对用户开放。
type ProviderModelSuggestion struct {
	RawID    string `json:"raw_id"`   // 上游原始模型标识（落库为 ModelID）
	Alias    string `json:"alias"`    // 去除尾部后缀后的展示别名（管理员可覆盖）
	Vendor   string `json:"vendor"`   // 推断厂商（OpenAI/DeepSeek/阿里通义…）
	Channel  string `json:"channel"`  // 推断渠道分组（openai/anthropic/alibaba…）
	Category string `json:"category"` // 推断类别 chat/image/video/audio/embedding/reasoning
	// 版本解析（厂商管理：前缀-版本-亚版本）
	Prefix     string `json:"prefix"`      // 前缀（如 deepseek）
	Version    string `json:"version"`     // 版本（如 v3.1）
	SubVersion string `json:"sub_version"` // 亚版本（如 turbo）
}

// PullProviderModels 从 OpenAI 兼容渠道拉取模型清单（GET {base}/models），
// 逐个做厂商/渠道/类别推断与后缀剥离，返回给站长筛选。
func PullProviderModels(baseURL, apiKey string) ([]ProviderModelSuggestion, error) {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("上游地址未配置")
	}
	url := base + "/models"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("连接失败：%v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("上游返回 %d", resp.StatusCode)
	}
	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("解析模型清单失败")
	}
	out := make([]ProviderModelSuggestion, 0, len(parsed.Data))
	for _, d := range parsed.Data {
		if strings.TrimSpace(d.ID) == "" {
			continue
		}
		out = append(out, ClassifyModel(d.ID))
	}
	return out, nil
}

// ClassifyModel 根据原始模型标识推断厂商/渠道/类别，并去除尾部后缀生成展示别名。
func ClassifyModel(raw string) ProviderModelSuggestion {
	s := strings.TrimSpace(raw)
	lower := strings.ToLower(s)
	vendor, channel := inferVendor(lower)
	category := inferCategory(lower)
	alias := stripSuffix(s)
	parsed := vparse.ParseModelID(s)
	return ProviderModelSuggestion{
		RawID: s, Alias: alias, Vendor: vendor, Channel: channel, Category: category,
		Prefix: parsed.Prefix, Version: parsed.Version, SubVersion: parsed.SubVersion,
	}
}

func inferVendor(lower string) (string, string) {
	cases := []struct {
		vendor, channel string
		matches         []string
	}{
		{"OpenAI", "openai", []string{"gpt-", "o1", "o3", "o4", "chatgpt", "text-embedding", "dall-e", "tts-", "whisper", "babbage", "davinci", "text-moderation"}},
		{"Anthropic", "anthropic", []string{"claude"}},
		{"Google", "google", []string{"gemini", "palm", "imagen"}},
		{"DeepSeek", "deepseek", []string{"deepseek"}},
		{"阿里通义", "alibaba", []string{"qwen", "tongyi", "dashscope", "bailian"}},
		{"智谱", "zhipu", []string{"glm", "chatglm"}},
		{"Moonshot", "moonshot", []string{"moonshot", "kimi"}},
		{"零一万物", "01ai", []string{"yi-"}},
		{"百川", "baichuan", []string{"baichuan"}},
		{"百度", "baidu", []string{"ernie", "wenxin"}},
		{"MiniMax", "minimax", []string{"abab", "minimax"}},
		{"阶跃星辰", "stepfun", []string{"step-"}},
		{"腾讯", "tencent", []string{"hunyuan"}},
		{"xAI", "xai", []string{"grok"}},
		{"Meta", "meta", []string{"llama"}},
		{"Mistral", "mistral", []string{"mistral"}},
		{"Cohere", "cohere", []string{"command-"}},
	}
	for _, c := range cases {
		for _, m := range c.matches {
			if strings.Contains(lower, m) {
				return c.vendor, c.channel
			}
		}
	}
	return "未知", "custom"
}

// inferCategory 推断模型类别（video 优先于 image，避免 kling/veo 误判）。
func inferCategory(lower string) string {
	switch {
	case strings.Contains(lower, "video") || strings.Contains(lower, "sora") || strings.Contains(lower, "runway") || strings.Contains(lower, "kling"):
		return "video"
	case strings.Contains(lower, "image") || strings.Contains(lower, "imagen") || strings.Contains(lower, "dall-e") ||
		strings.Contains(lower, "flux") || strings.Contains(lower, "sdxl") || strings.Contains(lower, "stable-diffusion") ||
		strings.Contains(lower, "draw") || strings.Contains(lower, "seedream") || strings.Contains(lower, "wan") || strings.Contains(lower, "midjourney"):
		return "image"
	case strings.Contains(lower, "tts") || strings.Contains(lower, "audio") || strings.Contains(lower, "whisper") ||
		strings.Contains(lower, "voice") || strings.Contains(lower, "speech") || strings.Contains(lower, "bark") || strings.Contains(lower, "music"):
		return "audio"
	case strings.Contains(lower, "embed"):
		return "embedding"
	case strings.Contains(lower, "reason") || strings.Contains(lower, "thinking") || strings.Contains(lower, "r1") ||
		strings.Contains(lower, "o1") || strings.Contains(lower, "o3") || strings.Contains(lower, "o4") ||
		strings.Contains(lower, "qwq") || strings.Contains(lower, "qvq") || strings.Contains(lower, "deepseek-reasoner"):
		return "reasoning"
	default:
		return "chat"
	}
}

// suffixTokens 已知尾部后缀词（去除后生成干净别名）。
var suffixTokens = []string{
	"preview", "latest", "beta", "exp", "experimental", "snapshot",
	"turbo", "instruct", "chat", "vision", "pro", "mini", "nano", "base",
	"online", "0813", "0801", "0125", "0115", "1106", "0613", "0314", "0131",
}

// stripSuffix 去除尾部日期与已知后缀词，得到干净展示别名。
func stripSuffix(raw string) string {
	s := raw
	// 去尾部日期：20240801 / 2024-01-01
	re := regexp.MustCompile(`[-:](20\d{6}|20\d{2}[-:]\d{2}[-:]\d{2})$`)
	s = re.ReplaceAllString(s, "")
	for {
		changed := false
		low := strings.ToLower(s)
		for _, tok := range suffixTokens {
			if strings.HasSuffix(low, "-"+tok) || strings.HasSuffix(low, ":"+tok) {
				s = s[:len(s)-len(tok)-1]
				changed = true
				break
			}
		}
		if !changed {
			break
		}
	}
	if s == "" {
		s = raw
	}
	return s
}
