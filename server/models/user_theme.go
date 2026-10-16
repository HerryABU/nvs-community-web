package models

import (
	"time"

	"nvs-server/utils"
)

// UserTheme 用户个人主题（F12 用户自定义网站主题，alist 式）。
// 每用户一行（user_id 唯一索引），保存后全局生效（跨设备，存服务端）；
// 优先级高于站点默认主题（SiteTheme）。
type UserTheme struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"uniqueIndex;not null" json:"user_id"`
	CSS       string    `gorm:"type:text" json:"css"` // 已消毒的主题 CSS
	UpdatedAt time.Time `json:"updated_at"`
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

// SetUserTheme 保存用户个人主题：服务端强制白名单消毒后按用户 Upsert。
// 返回被移除的规则列表（供前端提示用户）。
func SetUserTheme(userID uint, css string) (removed []string, err error) {
	if userID == 0 {
		return []string{}, nil
	}
	sanitized, removed := utils.SanitizeCSS(css)
	var t UserTheme
	if err := DB.Where("user_id = ?", userID).First(&t).Error; err == nil {
		t.CSS = sanitized
		t.UpdatedAt = time.Now()
		return removed, DB.Save(&t).Error
	}
	now := time.Now()
	return removed, DB.Create(&UserTheme{UserID: userID, CSS: sanitized, UpdatedAt: now}).Error
}
