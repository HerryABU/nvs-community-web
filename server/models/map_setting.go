package models

import (
	"encoding/json"
	"strings"
	"time"
)

// ==================== 地图服务配置（标准地图合规） ====================
//
// 为什么要有这个：站内「访问来源地域」图此前用的是打包进前端的 GeoJSON
// （第三方行政边界派生数据），既不是审图件，也只能画国内。
// 对外展示必须用**标准地图**，因此这里做成可配置：
//
//   provider:
//     offline  —— 离线内置示意（仅国内、标注"非标准地图、仅供站内参考"）
//     tianditu —— 天地图（自然资源部官方，国内 + 全球都支持，需站长自填 key）
//
// ⚠️ key 绝不内置：平台不预置、不代填、不写死任何 key。
//    站长在后台填自己的天地图 key（tk），并在天地图控制台配置 Referer 白名单。

const mapSettingKey = "map_setting"

// MapSetting 地图服务配置
type MapSetting struct {
	Provider    string    `json:"provider"`     // offline | tianditu
	Scope       string    `json:"scope"`        // china | world（仅 tianditu 有意义）
	TiandituKey string    `json:"tianditu_key"` // 天地图 tk，站长自填；空=未配置
	ReviewNo    string    `json:"review_no"`    // 审图号，如 GS(2024)0650号；对外展示必须标注
	UpdatedAt   time.Time `json:"updated_at"`
}

// DefaultMapSetting 默认离线示意（保持历史行为，且不要求站长立刻去申请 key）
func DefaultMapSetting() MapSetting {
	return MapSetting{Provider: "offline", Scope: "china"}
}

// LoadMapSetting 读取配置（坏值回退默认）
func LoadMapSetting() MapSetting {
	c := DefaultMapSetting()
	raw := GetPlatformConfig(mapSettingKey)
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			return DefaultMapSetting()
		}
	}
	if c.Provider != "offline" && c.Provider != "tianditu" {
		c.Provider = "offline"
	}
	if c.Scope != "china" && c.Scope != "world" {
		c.Scope = "china"
	}
	return c
}

// SaveMapSetting 保存配置
func SaveMapSetting(c MapSetting) error {
	if c.Provider != "offline" && c.Provider != "tianditu" {
		c.Provider = "offline"
	}
	if c.Scope != "china" && c.Scope != "world" {
		c.Scope = "china"
	}
	c.UpdatedAt = time.Now()
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return SetPlatformConfig(mapSettingKey, string(b))
}

// PublicMapSetting 公开给前端的部分（天地图 key 本来就必须在前端加载 SDK 使用，
// 这属于该服务的固有形态 —— 站长需在天地图控制台配 Referer 白名单来防滥用）
func PublicMapSetting() map[string]interface{} {
	c := LoadMapSetting()
	return map[string]interface{}{
		"provider":  c.Provider,
		"scope":     c.Scope,
		"key":       c.TiandituKey,
		"review_no": c.ReviewNo,
		// 未配 key 时前端要提示去申请，而不是静默白屏
		"key_ready": strings.TrimSpace(c.TiandituKey) != "",
	}
}
