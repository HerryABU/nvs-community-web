package models

import (
	"time"

	"nvs-server/utils"
)

// AuthorTheme 作者公开主题（F12 作者主题 / 访客可见）。
// 每作者一行（author_id 主键）：作者配置后，其个人主页/作品页等作者域页面
// 对访客注入该 CSS；与个人主题（UserTheme）互相独立。
// 写入一律经服务端白名单消毒（utils.SanitizeCSS），与站点/个人主题同源安全模型。
type AuthorTheme struct {
	AuthorID  uint      `gorm:"primaryKey" json:"author_id"`
	CSS       string    `gorm:"type:text" json:"css"` // 已消毒的作者主题 CSS
	UpdatedAt time.Time `json:"updated_at"`
}

func (AuthorTheme) TableName() string { return "author_themes" }

// GetAuthorTheme 读取作者公开主题 CSS；无记录返回空串（未配置）。
func GetAuthorTheme(authorID uint) string {
	if authorID == 0 {
		return ""
	}
	var t AuthorTheme
	if err := DB.Where("author_id = ?", authorID).First(&t).Error; err != nil {
		return ""
	}
	return t.CSS
}

// SetAuthorTheme 保存作者公开主题：服务端强制白名单消毒后按作者 Upsert。
// 返回被移除的规则列表（供前端提示作者）。
func SetAuthorTheme(authorID uint, css string) (removed []string, err error) {
	if authorID == 0 {
		return []string{}, nil
	}
	sanitized, removed := utils.SanitizeCSS(css)
	var t AuthorTheme
	if err := DB.Where("author_id = ?", authorID).First(&t).Error; err == nil {
		t.CSS = sanitized
		t.UpdatedAt = time.Now()
		return removed, DB.Save(&t).Error
	}
	now := time.Now()
	return removed, DB.Create(&AuthorTheme{AuthorID: authorID, CSS: sanitized, UpdatedAt: now}).Error
}
