package models

// ==================== AI 模型预设（绘画 / 视频） ====================
// 站长在「AI 模型池」点「预设」一键填充 base_url / model_id / api_kind / 计费默认值，
// 只需再补自己的密钥即可上线。base_url 留空的表示「填你的中转地址」——
// 这些厂商没有统一的公共 OpenAI 兼容端点，实际以站长所用中转/云厂商地址为准。

// AiModelPreset 模型预设
type AiModelPreset struct {
	Key          string `json:"key"`
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	ApiKind      string `json:"api_kind"`
	ModelID      string `json:"model_id"`
	BaseURL      string `json:"base_url"`
	PriceCall    int64  `json:"price_call"`
	ProviderHint string `json:"provider_hint"`
	Vendor       string `json:"vendor"`
	Note         string `json:"note"`
}

// presetVendor 预设键 → 厂商（前端「全部厂商」筛选用）
var presetVendor = map[string]string{
	"mj": "Midjourney", "niji": "Midjourney",
	"doubao-image": "字节跳动", "seedance": "字节跳动",
	"gpt-image-2": "OpenAI", "sora": "OpenAI",
	"nanobanana": "Google", "veo": "Google",
	"sd35": "Stability", "kling": "快手", "runway": "Runway",
	"hailuo": "MiniMax", "claude-sonnet": "Anthropic",
	"deepseek": "DeepSeek", "qwen": "阿里云",
}

// ListAiModelPresets 全部预设（绘画 + 视频 + 对话参考）
func ListAiModelPresets() []AiModelPreset {
	list := []AiModelPreset{
		// ---------- 绘画 ----------
		{
			Key: "mj", Name: "Midjourney", Kind: AiKindImage, ApiKind: AiApiMJ,
			ModelID: "MID_JOURNEY", BaseURL: "", PriceCall: 10, ProviderHint: "overseas",
			Note: "需 midjourney-proxy 兼容中转：base_url 填中转根地址（含 /mj 亦可），密钥填 mj-api-secret；模型标识留 MID_JOURNEY",
		},
		{
			Key: "niji", Name: "Niji·Journey", Kind: AiKindImage, ApiKind: AiApiMJ,
			ModelID: "NIJI_JOURNEY", BaseURL: "", PriceCall: 10, ProviderHint: "overseas",
			Note: "与 MJ 同一中转；模型标识填 NIJI_JOURNEY 即走动漫风格机器人",
		},
		{
			Key: "doubao-image", Name: "豆包 Seedream", Kind: AiKindImage, ApiKind: AiApiOpenAI,
			ModelID: "doubao-seedream-3-0-t2i-250415", BaseURL: "https://ark.cn-beijing.volces.com/api/v3",
			PriceCall: 5, ProviderHint: "domestic",
			Note: "火山方舟 OpenAI 兼容：密钥填 ARK API Key",
		},
		{
			Key: "gpt-image-2", Name: "GPT Image 2", Kind: AiKindImage, ApiKind: AiApiOpenAI,
			ModelID: "gpt-image-2", BaseURL: "https://api.openai.com/v1",
			PriceCall: 15, ProviderHint: "overseas",
			Note: "OpenAI 官方或兼容中转；如需改用中转只需替换 base_url",
		},
		{
			Key: "nanobanana", Name: "Nano Banana", Kind: AiKindImage, ApiKind: AiApiGemini,
			ModelID: "gemini-2.5-flash-image", BaseURL: "https://generativelanguage.googleapis.com",
			PriceCall: 8, ProviderHint: "overseas",
			Note: "Google generateContent 协议（responseModalities=IMAGE）；密钥填 Google AI Studio API Key。图像以 base64 内联返回",
		},
		{
			Key: "sd35", Name: "Stable Diffusion 3.5", Kind: AiKindImage, ApiKind: AiApiOpenAI,
			ModelID: "sd3.5-large", BaseURL: "", PriceCall: 4, ProviderHint: "",
			Note: "多数中转/SiliconFlow 提供 OpenAI 兼容 images/generations；base_url 填对应服务地址",
		},

		// ---------- 视频 ----------
		{
			Key: "kling", Name: "可灵 Kling", Kind: AiKindVideo, ApiKind: AiApiOpenAI,
			ModelID: "kling-v2", BaseURL: "", PriceCall: 30, ProviderHint: "domestic",
			Note: "需支持 OpenAI 风格 /v1/video/generations 提交 + /v1/video/generations/{id} 查询的中转",
		},
		{
			Key: "runway", Name: "Runway Gen-4", Kind: AiKindVideo, ApiKind: AiApiOpenAI,
			ModelID: "gen-4", BaseURL: "", PriceCall: 40, ProviderHint: "overseas",
			Note: "经中转接入；参数 duration/size 会原样透传",
		},
		{
			Key: "veo", Name: "Google Veo", Kind: AiKindVideo, ApiKind: AiApiOpenAI,
			ModelID: "veo-3", BaseURL: "", PriceCall: 45, ProviderHint: "overseas",
			Note: "官方为异步任务接口，建议经中转统一成 OpenAI 风格任务协议",
		},
		{
			Key: "sora", Name: "Sora", Kind: AiKindVideo, ApiKind: AiApiOpenAI,
			ModelID: "sora-2", BaseURL: "", PriceCall: 50, ProviderHint: "overseas",
			Note: "需中转提供任务提交 + 轮询接口",
		},
		{
			Key: "hailuo", Name: "海螺 Hailuo", Kind: AiKindVideo, ApiKind: AiApiOpenAI,
			ModelID: "MiniMax-Hailuo-02", BaseURL: "", PriceCall: 25, ProviderHint: "domestic",
			Note: "MiniMax 视频；可直连官方或经中转",
		},
		{
			Key: "seedance", Name: "豆包 Seedance", Kind: AiKindVideo, ApiKind: AiApiOpenAI,
			ModelID: "doubao-seedance-1-0-pro", BaseURL: "", PriceCall: 25, ProviderHint: "domestic",
			Note: "火山方舟视频生成；base_url 填方舟地址",
		},

		// ---------- 对话（参考） ----------
		{
			Key: "claude-sonnet", Name: "Claude Sonnet（官方 Messages API）", Kind: AiKindChat, ApiKind: AiApiClaude,
			ModelID: "claude-sonnet-4-5", BaseURL: "https://api.anthropic.com",
			PriceCall: 0, ProviderHint: "overseas",
			Note: "Anthropic 官方协议（/v1/messages + x-api-key + anthropic-version）；按 token 计费，请填输入/输出单价",
		},
		{
			Key: "deepseek", Name: "DeepSeek", Kind: AiKindChat, ApiKind: AiApiOpenAI,
			ModelID: "deepseek-chat", BaseURL: "https://api.deepseek.com/v1",
			PriceCall: 0, ProviderHint: "domestic",
			Note: "对话模型按 token 计费，请在「输入/输出单价」填写",
		},
		{
			Key: "qwen", Name: "通义千问", Kind: AiKindChat, ApiKind: AiApiOpenAI,
			ModelID: "qwen-plus", BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1",
			PriceCall: 0, ProviderHint: "domestic",
			Note: "阿里云 DashScope OpenAI 兼容模式",
		},
	}
	for i := range list {
		if v, ok := presetVendor[list[i].Key]; ok {
			list[i].Vendor = v
		}
	}
	return list
}
