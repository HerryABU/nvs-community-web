package models

import (
	"errors"
	"time"

	"nvs-server/utils"

	"gorm.io/gorm"
)

// SiteTheme 站点默认主题（F12 用户自定义网站主题，alist 式）。
// 单行配置表（ID 固定 1），管理员配置后对全部访客生效；
// 登录用户的个人主题（UserTheme）覆盖站点主题。
type SiteTheme struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CSS       string    `gorm:"type:text" json:"css"` // 已消毒的主题 CSS
	UpdatedAt time.Time `json:"updated_at"`
}

func (SiteTheme) TableName() string { return "site_themes" }

// GetSiteTheme 读取站点默认主题 CSS；无记录返回空串（未配置）。
func GetSiteTheme() string {
	var t SiteTheme
	if err := DB.First(&t, 1).Error; err != nil {
		return ""
	}
	return t.CSS
}

// SetSiteTheme 保存站点默认主题：先经服务端白名单消毒再 Upsert 单行。
// 返回被移除的规则列表（供前端提示管理员）。
func SetSiteTheme(css string) (removed []string, err error) {
	sanitized, removed := utils.SanitizeCSS(css)
	var t SiteTheme
	if ferr := DB.First(&t, 1).Error; ferr != nil {
		if !errors.Is(ferr, gorm.ErrRecordNotFound) {
			return removed, ferr
		}
		t = SiteTheme{ID: 1}
		if cerr := DB.Create(&t).Error; cerr != nil {
			return removed, cerr
		}
	}
	t.CSS = sanitized
	t.UpdatedAt = time.Now()
	return removed, DB.Save(&t).Error
}
