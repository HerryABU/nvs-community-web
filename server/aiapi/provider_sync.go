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
	// 版本解析（四段式：家族 - 版本 - 类型 - 子号）
	Prefix     string `json:"prefix"`      // 模型大名/家族（如 deepseek）
	Version    string `json:"version"`     // 版本号（如 v4.1）
	Variant    string `json:"variant"`     // 类型名称（如 flash / pro / thinking）
	Build      string `json:"build"`       // 子号（如 1017 / 2024-08-06）
	SubVersion string `json:"sub_version"` // 兼容旧字段：variant + build 的合并形态
}

// PullProviderModels 从 OpenAI 兼容渠道拉取模型清单（GET {base}/models），
// 逐个做厂商/渠道/类别推断与后缀剥离，返回给站长筛选。
func PullProviderModels(baseURL, apiKey string) ([]ProviderModelSuggestion, error) {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("上游地址未配置")
	}
	// 两种地址都要能吃：
	//   · 站长填的是 API 根（https://api.x.com/v1）→ 直接 + /models；
	//   · 填的是站点根（https://api.x.com，渠道凭据里存的就是这种归一化地址）→ 先试 /models，
	//     失败再试 /v1/models。
	// 本项目的渠道凭据会把 /v1 归一化掉，所以第二种是常态；不补这一步就会「解析模型清单失败」。
	candidates := []string{base + "/models"}
	if !strings.Contains(base, "/v1") {
		candidates = append(candidates, base+"/v1/models")
	}

	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	var lastErr error
	for _, url := range candidates {
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
			lastErr = fmt.Errorf("连接失败：%v", err)
			continue
		}
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("上游返回 %d", resp.StatusCode)
			continue
		}
		// ⚠️ 光看 2xx 不够：不少中转站的站点根 /models 会返回 SPA 首页（200 + text/html），
		// 实测 https://api.hcnsec.cn/models 就是 HTML。必须「JSON 能解析且真有 data」才算成功，
		// 否则继续试下一个候选地址。
		var attempt struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if e := json.Unmarshal(b, &attempt); e != nil || attempt.Data == nil {
			lastErr = fmt.Errorf("解析模型清单失败（该地址返回的不是 OpenAI 兼容的模型列表）")
			continue
		}
		parsed = attempt
		lastErr = nil
		break
	}
	if lastErr != nil {
		return nil, lastErr
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
		Prefix: parsed.Prefix, Version: parsed.Version,
		Variant: parsed.Variant, Build: parsed.Build,
		SubVersion: parsed.SubVersion,
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
