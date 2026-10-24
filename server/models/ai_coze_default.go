package models

// ==================== 扣子（Coze）全局默认配置 ====================
import (
	"encoding/json"
	"strings"
)

func GetAiCozeDefault() AiCozeDefault {
	out := AiCozeDefault{BaseURL: "https://api.coze.cn", TimeoutSeconds: 150}
	raw := GetPlatformConfig("ai_coze_default")
	if strings.TrimSpace(raw) == "" {
		return out
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return AiCozeDefault{BaseURL: "https://api.coze.cn", TimeoutSeconds: 150}
	}
	if strings.TrimSpace(out.BaseURL) == "" {
		out.BaseURL = "https://api.coze.cn"
	}
	if out.TimeoutSeconds < 30 || out.TimeoutSeconds > 600 {
		out.TimeoutSeconds = 150
	}
	return out
}

func SetAiCozeDefault(d AiCozeDefault) error {
	if strings.TrimSpace(d.BaseURL) == "" {
		d.BaseURL = "https://api.coze.cn"
	}
	if d.TimeoutSeconds < 30 || d.TimeoutSeconds > 600 {
		d.TimeoutSeconds = 150
	}
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return SetPlatformConfig("ai_coze_default", string(b))
}
