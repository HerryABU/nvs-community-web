package theme

import (
	"strings"
	"time"

	"nvs-server/models"
	"nvs-server/utils/sanitize"
)

// AuthorTheme 作者公开主题（F12 作者主题 / 访客可见）。
// 每作者一行（author_id 主键）：作者配置后，其个人主页/作品页等作者域页面
// 对访客注入该 CSS；与个人主题（UserTheme）互相独立。
// 写入一律经服务端白名单消毒（sanitize.SanitizeCSS），与站点/个人主题同源安全模型。
type AuthorTheme struct {
	AuthorID  uint      `gorm:"primaryKey" json:"author_id"`
	CSS       string    `gorm:"type:text" json:"css"`                               // 已消毒的作者主题 CSS
	BlogStyle string    `gorm:"size:16;not null;default:classic" json:"blog_style"` // 博客主题：classic 简洁 / advanced 高级
	UpdatedAt time.Time `json:"updated_at"`
}

// 博客主题风格（作者可自由选择）
const (
	BlogStyleClassic  = "classic"
	BlogStyleAdvanced = "advanced"
)

// NormalizeBlogStyle 白名单校验，非法值回退 classic
func NormalizeBlogStyle(s string) string {
	if strings.EqualFold(strings.TrimSpace(s), BlogStyleAdvanced) {
		return BlogStyleAdvanced
	}
	return BlogStyleClassic
}

func (AuthorTheme) TableName() string { return "author_themes" }

// GetAuthorTheme 读取作者公开主题 CSS；无记录返回空串（未配置）。
func GetAuthorTheme(authorID uint) string {
	if authorID == 0 {
		return ""
	}
	var t AuthorTheme
	if err := models.DB.Where("author_id = ?", authorID).First(&t).Error; err != nil {
		return ""
	}
	return t.CSS
}

// GetAuthorBlogStyle 读取作者选择的博客主题风格（缺省 classic）
func GetAuthorBlogStyle(authorID uint) string {
	if authorID == 0 {
		return BlogStyleClassic
	}
	var t AuthorTheme
	if err := models.DB.Select("blog_style").Where("author_id = ?", authorID).First(&t).Error; err != nil {
		return BlogStyleClassic
	}
	return NormalizeBlogStyle(t.BlogStyle)
}

// SetAuthorTheme 保存作者公开主题：服务端强制白名单消毒后按作者 Upsert。
// 返回被移除的规则列表（供前端提示作者）。
func SetAuthorTheme(authorID uint, css string) (removed []string, err error) {
	return SetAuthorThemeWithStyle(authorID, css, "")
}

// SetAuthorThemeWithStyle 同时保存 CSS 与博客主题风格（空 style 表示不修改）。
func SetAuthorThemeWithStyle(authorID uint, css, style string) (removed []string, err error) {
	if authorID == 0 {
		return []string{}, nil
	}
	sanitized, removed := sanitize.SanitizeCSS(css)
	var t AuthorTheme
	if err := models.DB.Where("author_id = ?", authorID).First(&t).Error; err == nil {
		t.CSS = sanitized
		if strings.TrimSpace(style) != "" {
			t.BlogStyle = NormalizeBlogStyle(style)
		}
		t.UpdatedAt = time.Now()
		return removed, models.DB.Save(&t).Error
	}
	now := time.Now()
	return removed, models.DB.Create(&AuthorTheme{
		AuthorID: authorID, CSS: sanitized, BlogStyle: NormalizeBlogStyle(style), UpdatedAt: now,
	}).Error
}

// AdvancedThemeAuthors 返回已选择高级博客主题的作者 ID（供首页精选推流）
func AdvancedThemeAuthors() []uint {
	var ids []uint
	models.DB.Model(&AuthorTheme{}).Where("blog_style = ?", BlogStyleAdvanced).Pluck("author_id", &ids)
	return ids
}
