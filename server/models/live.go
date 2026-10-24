package models

// ==================== 直播入口（gap-audit #23 的最后一块）====================
// 先把边界说清楚（否则会被当成"本站自带推流"）：
// **本单一二进制进程不做推流、不做转码、不做 HLS 切片**——那需要流媒体服务器
// （SRS / nginx-rtmp / 云直播）+ 带宽，和"单二进制嵌入前端"的部署形态是两件事。
//
// 这里做的是**入口与嵌入**，两种形态覆盖绝大多数用法：
//   1. `embed_url`：外部直播间的 https 地址（B 站/腾讯会议/云直播控制台都能给），用 iframe 嵌入；
//   2. `video_url`：mp4/webm 直链，用原生 `<video>` 播（适合录播/低延迟单文件）。
//
// 安全约束（iframe 最容易出事的地方）：
//   - `embed_url` **只允许 https**（http 页面在 https 站点会被浏览器拦，且不安全）；
//   - 前端 iframe 强制 `sandbox`，不给 `allow-top-navigation`（防嵌套页面劫持整站）；
//   - 其它字段（海报/直链）走既有 URL 白名单思路：站内 `/uploads/` 或 http(s)。

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// 直播状态
const (
	LiveOffline = "offline" // 未开播（展示海报与预告）
	LivePreview = "preview" // 预告（展示开播时间）
	LiveOn      = "live"    // 直播中（挂播放器）
)

// LiveSetting 直播配置（platform_configs.live_setting）
type LiveSetting struct {
	Enabled bool   `json:"enabled"`
	Title   string `json:"title"`
	Status  string `json:"status"` // offline / preview / live

	EmbedURL  string `json:"embed_url"`  // 外部直播间（仅 https）
	VideoURL  string `json:"video_url"`  // mp4/webm 直链（可选）
	PosterURL string `json:"poster_url"` // 海报/占位图

	StartsAt *time.Time `json:"starts_at"`
	Notice   string     `json:"notice"`
	// ChatGroupID 关联群组：直播聊天复用群组（不新造聊天系统）
	ChatGroupID uint `json:"chat_group_id"`
	RequireLogin bool `json:"require_login"`

	UpdatedAt time.Time `json:"updated_at"`
}

const liveCfgKey = "live_setting"

// DefaultLiveSetting 默认关闭，状态未开播
func DefaultLiveSetting() LiveSetting {
	return LiveSetting{Enabled: false, Status: LiveOffline, Title: "直播间"}
}

// LoadLiveSetting 读取
func LoadLiveSetting() LiveSetting {
	c := DefaultLiveSetting()
	if raw := GetPlatformConfig(liveCfgKey); strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			return DefaultLiveSetting()
		}
	}
	return normalizeLive(c)
}

func normalizeLive(c LiveSetting) LiveSetting {
	switch c.Status {
	case LiveOn, LivePreview, LiveOffline:
	default:
		c.Status = LiveOffline
	}
	c.Title = strings.TrimSpace(c.Title)
	if c.Title == "" {
		c.Title = "直播间"
	}
	if n := len([]rune(c.Title)); n > 60 {
		c.Title = string([]rune(c.Title)[:60])
	}
	if n := len([]rune(c.Notice)); n > 500 {
		c.Notice = string([]rune(c.Notice)[:500])
	}
	return c
}

// ValidateLive 校验地址（保存时调用；返回错误说明哪个字段不对）
func ValidateLive(c LiveSetting) error {
	if u := strings.TrimSpace(c.EmbedURL); u != "" {
		low := strings.ToLower(u)
		if !strings.HasPrefix(low, "https://") {
			return errors.New("嵌入地址必须是 https（http 页面在 https 站点会被浏览器拦截）")
		}
		if strings.Contains(low, "javascript:") || strings.Contains(low, "data:") {
			return errors.New("嵌入地址非法")
		}
	}
	if u := strings.TrimSpace(c.VideoURL); u != "" {
		low := strings.ToLower(u)
		if !strings.HasPrefix(low, "http://") && !strings.HasPrefix(low, "https://") && !strings.HasPrefix(u, "/uploads/") {
			return errors.New("视频直链仅支持 http(s) 或站内 /uploads/")
		}
	}
	if u := strings.TrimSpace(c.PosterURL); u != "" {
		low := strings.ToLower(u)
		if !strings.HasPrefix(low, "http://") && !strings.HasPrefix(low, "https://") && !strings.HasPrefix(u, "/uploads/") {
			return errors.New("海报地址仅支持 http(s) 或站内 /uploads/")
		}
	}
	return nil
}

// SaveLiveSetting 保存
func SaveLiveSetting(c LiveSetting) error {
	if err := ValidateLive(c); err != nil {
		return err
	}
	c = normalizeLive(c)
	c.UpdatedAt = time.Now()
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return SetPlatformConfig(liveCfgKey, string(b))
}
