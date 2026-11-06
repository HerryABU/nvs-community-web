package theme

import (
	"encoding/json"
	"errors"
	"nvs-server/utils/sanitize"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
	"nvs-server/models"
)

// SiteTheme 站点默认主题（F12 用户自定义网站主题，alist 式）。
// 单行配置表（ID 固定 1），管理员配置后对全部访客生效；
// 登录用户的个人主题（UserTheme）覆盖站点主题。
type SiteTheme struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	CSS      string `gorm:"type:text" json:"css"`       // 已消毒的主题 CSS（高级：原始 CSS）
	CustomJS string `gorm:"type:text" json:"custom_js"` // 已做注入安全处理的自定义 JS（管理员可写，全站访客执行）
	// JS 加载模式（alist「延迟加载」）：immediate 立即 / idle 首屏后空闲时 / delayed 延迟 N 秒
	CustomJSMode  string `gorm:"size:16;not null;default:immediate" json:"custom_js_mode"`
	CustomJSDelay int    `gorm:"not null;default:0" json:"custom_js_delay"`
	// 结构化主题设置（可视化控件生成，与原始 CSS 互补）：背景 / 毛玻璃 / 按钮 / 看板娘。
	Settings  string    `gorm:"type:text" json:"theme_settings"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (SiteTheme) TableName() string { return "site_themes" }

// ===== 结构化主题设置（可视化控件生成，对齐 alist 的可视化美化能力）=====
// 这些设置由前端 theme store 解析后生成结构化 CSS（nvs-theme-structured 样式标签），
// 覆盖站点级 CSS 变量（主色 / 圆角 / 玻璃 / 背景），与原始 CSS 文本互补而非互斥。

type ThemeBackground struct {
	Type  string `json:"type"`  // color 纯色 | gradient 渐变 | image 图片 URL
	Value string `json:"value"` // 颜色值 / 渐变串 / 图片 URL
}

type ThemeGlass struct {
	Enabled bool    `json:"enabled"`
	Blur    float64 `json:"blur"`    // 模糊半径 px
	Opacity float64 `json:"opacity"` // 玻璃底色不透明度 0~1
	Tint    string  `json:"tint"`    // 玻璃底色 HEX（如 #ffffff）
}

type ThemeButton struct {
	Radius  float64 `json:"radius"`  // 圆角 px
	Primary string  `json:"primary"` // 主色 HEX
}

type ThemeMascot struct {
	Enabled   bool   `json:"enabled"`    // 是否启用看板娘
	ScriptURL string `json:"script_url"` // 外部看板娘脚本 URL（兼容 alist/live2d 等任意方案）
}

type ThemeSettings struct {
	Background ThemeBackground `json:"background"`
	Glass      ThemeGlass      `json:"glass"`
	Button     ThemeButton     `json:"button"`
	Mascot     ThemeMascot     `json:"mascot"`
}

// DefaultThemeSettings 结构化简洁默认（不影响既有外观：毛玻璃关、无背景覆盖、按钮沿用系统主色）。
func DefaultThemeSettings() ThemeSettings {
	return ThemeSettings{
		Background: ThemeBackground{Type: "color", Value: ""},
		Glass:      ThemeGlass{Enabled: true, Blur: 20, Opacity: 0.6, Tint: "#ffffff"},
		Button:     ThemeButton{Radius: 12, Primary: ""},
		Mascot:     ThemeMascot{Enabled: false, ScriptURL: ""},
	}
}

var hexColorRe = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// ParseThemeSettings 解析并校验结构化设置，非法字段回退默认值；返回（规范化后的设置, 被回退/修正的提示列表）。
func ParseThemeSettings(s string) (ThemeSettings, []string) {
	out := DefaultThemeSettings()
	var removed []string
	if strings.TrimSpace(s) == "" {
		return out, removed
	}
	var raw ThemeSettings
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		removed = append(removed, "settings: 非法 JSON，已回退默认")
		return out, removed
	}
	// 背景
	if raw.Background.Type == "color" || raw.Background.Type == "gradient" || raw.Background.Type == "image" {
		out.Background.Type = raw.Background.Type
		out.Background.Value = strings.TrimSpace(raw.Background.Value)
	}
	// 毛玻璃
	if raw.Glass.Blur >= 0 && raw.Glass.Blur <= 60 {
		out.Glass.Blur = raw.Glass.Blur
	} else {
		removed = append(removed, "glass.blur: 超出 0~60，已回退")
	}
	if raw.Glass.Opacity >= 0 && raw.Glass.Opacity <= 1 {
		out.Glass.Opacity = raw.Glass.Opacity
	} else {
		removed = append(removed, "glass.opacity: 超出 0~1，已回退")
	}
	out.Glass.Enabled = raw.Glass.Enabled
	if hexColorRe.MatchString(raw.Glass.Tint) {
		out.Glass.Tint = raw.Glass.Tint
	} else if raw.Glass.Tint != "" {
		removed = append(removed, "glass.tint: 非法颜色，已回退")
	}
	// 按钮
	if raw.Button.Radius >= 0 && raw.Button.Radius <= 40 {
		out.Button.Radius = raw.Button.Radius
	} else {
		removed = append(removed, "button.radius: 超出 0~40，已回退")
	}
	if hexColorRe.MatchString(raw.Button.Primary) {
		out.Button.Primary = raw.Button.Primary
	} else if raw.Button.Primary != "" {
		removed = append(removed, "button.primary: 非法颜色，已忽略")
	}
	// 看板娘
	out.Mascot.Enabled = raw.Mascot.Enabled
	if raw.Mascot.ScriptURL != "" {
		if strings.HasPrefix(raw.Mascot.ScriptURL, "http://") || strings.HasPrefix(raw.Mascot.ScriptURL, "https://") {
			out.Mascot.ScriptURL = raw.Mascot.ScriptURL
		} else {
			removed = append(removed, "mascot.script_url: 仅允许 http/https，已忽略")
		}
	}
	return out, removed
}

// GetSiteTheme 读取站点默认主题 CSS；无记录返回空串（未配置）。
func GetSiteTheme() string {
	var t SiteTheme
	if err := models.DB.First(&t, 1).Error; err != nil {
		return ""
	}
	return t.CSS
}

// GetSiteThemeJS 读取站点默认主题 JS；无记录返回空串。
func GetSiteThemeJS() string {
	var t SiteTheme
	if err := models.DB.First(&t, 1).Error; err != nil {
		return ""
	}
	return t.CustomJS
}

// GetSiteThemeJSBundle 一次性取站点主题的 JS 及其加载模式。
func GetSiteThemeJSBundle() (js string, mode string, delay int) {
	var t SiteTheme
	if err := models.DB.First(&t, 1).Error; err != nil {
		return "", ThemeJSModeImmediate, 0
	}
	return t.CustomJS, NormalizeThemeJSMode(t.CustomJSMode), t.CustomJSDelay
}

// GetSiteThemeSettings 读取并解析站点结构化主题设置；无记录或非法返回默认值。
func GetSiteThemeSettings() (raw string, parsed ThemeSettings) {
	var t SiteTheme
	if err := models.DB.First(&t, 1).Error; err != nil {
		return "", DefaultThemeSettings()
	}
	parsed, _ = ParseThemeSettings(t.Settings)
	return t.Settings, parsed
}

// SetSiteTheme 保存站点默认主题：经服务端宽松消毒（管理员路径，不考量 XSS）后 Upsert 单行。
// settings 为结构化设置 JSON（可视化控件生成），解析失败回退默认，不阻断保存。
// 返回被移除的规则列表（供前端提示管理员）。
// 注意：站点主题仅管理员可写（UpdateSiteTheme 已 admincommon.EnsureAdmin 把关），
// 故走 SanitizeCSSRelaxed——放行 @import/@font-face、url() 外链与全部属性名。
func SetSiteTheme(css string, customJS string, jsMode string, jsDelay int, settings string) (removed []string, err error) {
	sanitized, removed := sanitize.SanitizeCSSRelaxed(css)
	sanitizedJS, jsRemoved := sanitize.SanitizeJS(customJS)
	removed = append(removed, jsRemoved...)
	// 结构化设置：解析 + 校验 + 规范化（非法字段回退默认）
	_, normalized := ParseThemeSettings(settings)
	settingsJSON, _ := json.Marshal(normalized)
	var t SiteTheme
	if ferr := models.DB.First(&t, 1).Error; ferr != nil {
		if !errors.Is(ferr, gorm.ErrRecordNotFound) {
			return removed, ferr
		}
		t = SiteTheme{ID: 1}
		if cerr := models.DB.Create(&t).Error; cerr != nil {
			return removed, cerr
		}
	}
	t.CSS = sanitized
	t.CustomJS = sanitizedJS
	t.CustomJSMode = NormalizeThemeJSMode(jsMode)
	if t.CustomJSMode == ThemeJSModeDelayed {
		t.CustomJSDelay = NormalizeThemeJSDelay(jsDelay)
	} else {
		t.CustomJSDelay = 0
	}
	t.Settings = string(settingsJSON)
	t.UpdatedAt = time.Now()
	return removed, models.DB.Save(&t).Error
}
