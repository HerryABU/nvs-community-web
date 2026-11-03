package models

import (
	"time"

	"nvs-server/utils"
)

// UserTheme 用户个人主题（F12 用户自定义网站主题，alist 式）。
// 每用户一行（user_id 唯一索引），保存后全局生效（跨设备，存服务端）；
// 优先级高于站点默认主题（SiteTheme）。
type UserTheme struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	UserID   uint   `gorm:"uniqueIndex;not null" json:"user_id"`
	CSS      string `gorm:"type:text" json:"css"`       // 已消毒的主题 CSS
	CustomJS string `gorm:"type:text" json:"custom_js"` // 已做注入安全处理的自定义 JS（仅本人登录态执行）
	// JS 加载模式（alist「延迟加载」）：immediate 立即 / idle 首屏后空闲时 / delayed 延迟 N 秒
	CustomJSMode  string    `gorm:"size:16;not null;default:immediate" json:"custom_js_mode"`
	CustomJSDelay int       `gorm:"not null;default:0" json:"custom_js_delay"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (UserTheme) TableName() string { return "user_themes" }

// GetUserTheme 读取用户个人主题 CSS；无记录返回空串（未配置）。
func GetUserTheme(userID uint) string {
	if userID == 0 {
		return ""
	}
	var t UserTheme
	if err := DB.Where("user_id = ?", userID).First(&t).Error; err != nil {
		return ""
	}
	return t.CSS
}

// GetUserThemeJS 读取用户个人主题 JS；无记录返回空串。
func GetUserThemeJS(userID uint) string {
	if userID == 0 {
		return ""
	}
	var t UserTheme
	if err := DB.Where("user_id = ?", userID).First(&t).Error; err != nil {
		return ""
	}
	return t.CustomJS
}

// GetUserThemeJSBundle 一次性取个人主题的 JS 及其加载模式。
func GetUserThemeJSBundle(userID uint) (js string, mode string, delay int) {
	if userID == 0 {
		return "", ThemeJSModeImmediate, 0
	}
	var t UserTheme
	if err := DB.Where("user_id = ?", userID).First(&t).Error; err != nil {
		return "", ThemeJSModeImmediate, 0
	}
	return t.CustomJS, NormalizeThemeJSMode(t.CustomJSMode), t.CustomJSDelay
}

// SetUserTheme 保存用户个人主题：服务端强制白名单消毒后按用户 Upsert。
// 返回被移除的规则列表（供前端提示用户）。
func SetUserTheme(userID uint, css string, customJS string, jsMode string, jsDelay int) (removed []string, err error) {
	if userID == 0 {
		return []string{}, nil
	}
	sanitized, removed := utils.SanitizeCSS(css)
	sanitizedJS, jsRemoved := utils.SanitizeJS(customJS)
	removed = append(removed, jsRemoved...)
	var t UserTheme
	mode := NormalizeThemeJSMode(jsMode)
	delay := 0
	if mode == ThemeJSModeDelayed {
		delay = NormalizeThemeJSDelay(jsDelay)
	}
	if err := DB.Where("user_id = ?", userID).First(&t).Error; err == nil {
		t.CSS = sanitized
		t.CustomJS = sanitizedJS
		t.CustomJSMode = mode
		t.CustomJSDelay = delay
		t.UpdatedAt = time.Now()
		return removed, DB.Save(&t).Error
	}
	now := time.Now()
	return removed, DB.Create(&UserTheme{UserID: userID, CSS: sanitized, CustomJS: sanitizedJS,
		CustomJSMode: mode, CustomJSDelay: delay, UpdatedAt: now}).Error
}
