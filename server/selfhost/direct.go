package selfhost

// 管理员「端口直派」：把对外路径直接指派到本机端口，与 GWh 的路由表一样即时生效，
// 不绑定自托管项目、不走内容审核（配置者仅限管理员，属于受信任操作）。

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"nvs-server/models"
	"nvs-server/selfhost/target"
)

type DirectRoute struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	Prefix        string    `gorm:"size:128;not null;uniqueIndex" json:"prefix"`
	Port          int       `gorm:"not null" json:"port"` // 本机端口 1..65535
	RootPath      string    `gorm:"size:256;default:''" json:"root_path"`
	TargetURL     string    `gorm:"size:512;default:''" json:"target_url"` // 可选完整目标；管理员可用它配置自指/嵌套路由
	CookieHat     string    `gorm:"size:64;default:''" json:"cookie_hat"`  // GWh 同款 cookie 帽子前缀（如 app__）
	DropForeign   bool      `gorm:"default:false" json:"drop_foreign_cookies"`
	TLSSkipVerify bool      `gorm:"default:false" json:"tls_insecure_skip_verify"`
	SDKOriginAuth bool      `gorm:"default:false" json:"sdk_origin_auth"`
	WXAppid       string    `gorm:"size:64;default:''" json:"wx_appid"`
	Enabled       bool      `gorm:"default:true;index" json:"enabled"`
	Description   string    `gorm:"size:512;default:''" json:"description"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (DirectRoute) TableName() string { return "selfhost_direct_routes" }

func MigrateDirect(db *gorm.DB) error { return db.AutoMigrate(&DirectRoute{}) }

func ListDirectRoutes() ([]DirectRoute, error) {
	var list []DirectRoute
	err := models.DB.Order("prefix ASC").Find(&list).Error
	return list, err
}

func ListEnabledDirectRoutes() ([]DirectRoute, error) {
	var list []DirectRoute
	err := models.DB.Where("enabled = ?", true).Find(&list).Error
	return list, err
}

func normalizeRootPath(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || s == "/" {
		return ""
	}
	if !strings.HasPrefix(s, "/") {
		s = "/" + s
	}
	if strings.Contains(s, "..") {
		return ""
	}
	return strings.TrimSuffix(s, "/")
}

func SaveDirectRoute(in DirectRoute) (*DirectRoute, error) {
	in.Prefix = strings.ToLower(strings.TrimSpace(in.Prefix))
	if err := ValidatePrefix(in.Prefix); err != nil {
		return nil, err
	}
	if in.Port < 1 || in.Port > 65535 {
		return nil, errors.New("端口必须在 1-65535 之间")
	}
	in.RootPath = normalizeRootPath(in.RootPath)
	in.CookieHat = strings.TrimSpace(in.CookieHat)
	in.TargetURL = strings.TrimSpace(in.TargetURL)
	if in.TargetURL != "" {
		p, err := target.Parse(in.TargetURL)
		if err != nil || (p.Scheme != "http" && p.Scheme != "https") {
			return nil, errors.New("完整目标地址不合法（只支持 http/https，如 http://127.0.0.1:8000/other）")
		}
	}
	in.Description = strings.TrimSpace(in.Description)

	var old DirectRoute
	notFound := models.DB.Where("prefix = ?", in.Prefix).First(&old).Error != nil
	if !notFound {
		in.ID = old.ID
		in.CreatedAt = old.CreatedAt
		if err := models.DB.Save(&in).Error; err != nil {
			return nil, err
		}
	} else if err := models.DB.Create(&in).Error; err != nil {
		return nil, err
	}
	InvalidateRoutes()
	return &in, nil
}

func ToggleDirectRoute(id uint, enabled bool) error {
	if err := models.DB.Model(&DirectRoute{}).Where("id = ?", id).Update("enabled", enabled).Error; err != nil {
		return err
	}
	InvalidateRoutes()
	return nil
}

func DeleteDirectRoute(id uint) error {
	if err := models.DB.Delete(&DirectRoute{}, id).Error; err != nil {
		return err
	}
	InvalidateRoutes()
	return nil
}
