package theme

import (
	"errors"
	"nvs-server/utils/sanitize"
	"time"

	"gorm.io/gorm"
	"nvs-server/models"
)

// SiteTheme 站点默认主题（F12 用户自定义网站主题，alist 式）。
// 单行配置表（ID 固定 1），管理员配置后对全部访客生效；
// 登录用户的个人主题（UserTheme）覆盖站点主题。
type SiteTheme struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	CSS      string `gorm:"type:text" json:"css"`       // 已消毒的主题 CSS
	CustomJS string `gorm:"type:text" json:"custom_js"` // 已做注入安全处理的自定义 JS（管理员可写，全站访客执行）
	// JS 加载模式（alist「延迟加载」）：immediate 立即 / idle 首屏后空闲时 / delayed 延迟 N 秒
	CustomJSMode  string    `gorm:"size:16;not null;default:immediate" json:"custom_js_mode"`
	CustomJSDelay int       `gorm:"not null;default:0" json:"custom_js_delay"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (SiteTheme) TableName() string { return "site_themes" }

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

// SetSiteTheme 保存站点默认主题：经服务端宽松消毒（管理员路径，不考量 XSS）后 Upsert 单行。
// 返回被移除的规则列表（供前端提示管理员）。
// 注意：站点主题仅管理员可写（UpdateSiteTheme 已 admincommon.EnsureAdmin 把关），
// 故走 SanitizeCSSRelaxed——放行 @import/@font-face、url() 外链与全部属性名。
func SetSiteTheme(css string, customJS string, jsMode string, jsDelay int) (removed []string, err error) {
	sanitized, removed := sanitize.SanitizeCSSRelaxed(css)
	sanitizedJS, jsRemoved := sanitize.SanitizeJS(customJS)
	removed = append(removed, jsRemoved...)
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
	t.UpdatedAt = time.Now()
	return removed, models.DB.Save(&t).Error
}
