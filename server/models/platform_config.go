package models

import (
	"encoding/json"
	"strings"
	"time"
)

// PlatformConfig 平台配置（站长面板）
type PlatformConfig struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Key       string    `gorm:"size:64;uniqueIndex;not null" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (PlatformConfig) TableName() string { return "platform_configs" }

// ============ 平台配置辅助函数 ============

// GetPlatformConfig 获取单个平台配置值
func GetPlatformConfig(key string) string {
	var cfg PlatformConfig
	if err := DB.Where("key = ?", key).First(&cfg).Error; err != nil {
		return ""
	}
	return cfg.Value
}

// SetPlatformConfig 设置平台配置值（自动 upsert）
func SetPlatformConfig(key, value string) error {
	var cfg PlatformConfig
	err := DB.Where("key = ?", key).First(&cfg).Error
	now := time.Now()
	if err != nil {
		cfg = PlatformConfig{Key: key, Value: value, UpdatedAt: now}
		return DB.Create(&cfg).Error
	}
	cfg.Value = value
	cfg.UpdatedAt = now
	return DB.Save(&cfg).Error
}

// GetAllPlatformConfigs 获取所有平台配置
func GetAllPlatformConfigs() map[string]string {
	var cfgs []PlatformConfig
	DB.Find(&cfgs)
	result := make(map[string]string)
	for _, c := range cfgs {
		result[c.Key] = c.Value
	}
	return result
}

// IsVipEnabled 检查 VIP 付费是否开启
func IsVipEnabled() bool {
	return GetPlatformConfig("vip_enabled") != "false"
}

// GetSiteName 获取站点名称
func GetSiteName() string {
	name := GetPlatformConfig("site_name")
	if name == "" {
		return "星海文学" // 默认站名
	}
	return name
}

// GetCategories 获取分类列表（从平台配置读取，返回字符串数组）
func GetCategories() []string {
	val := GetPlatformConfig("categories")
	if val == "" {
		return []string{"硬科幻", "奇幻", "推演文学", "架空历史", "现实主义", "悬疑推理", "实验文学", "同人区", "政治区", "讽刺文学", "泛二次元区", "其他"}
	}
	var cats []string
	if err := json.Unmarshal([]byte(val), &cats); err != nil {
		return []string{"硬科幻", "奇幻", "推演文学", "架空历史", "现实主义", "悬疑推理", "实验文学", "同人区", "政治区", "讽刺文学", "泛二次元区", "其他"}
	}
	if len(cats) == 0 {
		return []string{"硬科幻", "奇幻", "推演文学", "架空历史", "现实主义", "悬疑推理", "实验文学", "同人区", "政治区", "讽刺文学", "泛二次元区", "其他"}
	}
	return cats
}

// GetRobotsTxt 获取 robots.txt 内容，站长可自定义
func GetRobotsTxt() string {
	txt := GetPlatformConfig("robots_txt")
	if txt == "" {
		return "User-agent: *\nAllow: /\n"
	}
	return txt
}

// InitPlatformConfigs 初始化默认平台配置（仅在不存在时写入）
func InitPlatformConfigs() {
	defaults := map[string]string{
		"site_name":       "星海文学",
		"vip_enabled":     "true",
		"email_verify":    "false",
		"captcha_enabled": "false",
		"categories":      `["硬科幻","奇幻","推演文学","架空历史","现实主义","悬疑推理","实验文学","同人区","政治区","讽刺文学","泛二次元区","其他"]`,
		"robots_txt":      "User-agent: *\nAllow: /\n",
	}
	for k, v := range defaults {
		var count int64
		DB.Model(&PlatformConfig{}).Where("key = ?", k).Count(&count)
		if count == 0 {
			DB.Create(&PlatformConfig{Key: k, Value: v, UpdatedAt: time.Now()})
		}
	}
}

// IsEmailVerifyEnabled 检查邮箱验证是否开启
func IsEmailVerifyEnabled() bool {
	return GetPlatformConfig("email_verify") == "true"
}

// IsCaptchaEnabled 检查滑块验证码是否开启
func IsCaptchaEnabled() bool {
	return GetPlatformConfig("captcha_enabled") == "true"
}

// ============ 邮箱域名黑白名单 ============

// parseDomainList 解析域名列表配置（JSON 数组，或逗号/换行/分号/空格分隔的纯文本）。
func parseDomainList(val string) []string {
	if val == "" {
		return nil
	}
	var list []string
	if err := json.Unmarshal([]byte(val), &list); err == nil {
		out := make([]string, 0, len(list))
		for _, d := range list {
			d = strings.TrimSpace(strings.ToLower(d))
			if d != "" {
				out = append(out, d)
			}
		}
		return out
	}
	// 兼容纯文本分隔
	fields := strings.FieldsFunc(val, func(r rune) bool {
		return r == ',' || r == '\n' || r == ';' || r == ' ' || r == '\t' || r == '\r'
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(strings.ToLower(f))
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

// GetEmailDomainWhitelist 获取允许注册的邮箱域名白名单（空 = 不限制）。
func GetEmailDomainWhitelist() []string {
	return parseDomainList(GetPlatformConfig("email_domain_whitelist"))
}

// GetEmailDomainBlacklist 获取禁止注册的邮箱域名黑名单（空 = 不限制）。
func GetEmailDomainBlacklist() []string {
	return parseDomainList(GetPlatformConfig("email_domain_blacklist"))
}

// extractEmailDomain 提取邮箱域名（小写、去空格）。格式非法返回空串。
func extractEmailDomain(email string) string {
	email = strings.TrimSpace(strings.ToLower(email))
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return ""
	}
	return email[at+1:]
}

// CheckEmailDomainAllowed 检查邮箱域名是否允许注册。
// 规则：先判黑名单（命中即拒），再判白名单（白名单非空且未命中即拒）。
// 返回 (allowed, reason)，allowed=false 时 reason 为拒绝原因。
func CheckEmailDomainAllowed(email string) (bool, string) {
	domain := extractEmailDomain(email)
	if domain == "" {
		return false, "邮箱格式无效"
	}
	for _, d := range GetEmailDomainBlacklist() {
		if d == domain {
			return false, "该邮箱域名（@" + domain + "）已被平台禁止注册"
		}
	}
	whitelist := GetEmailDomainWhitelist()
	if len(whitelist) > 0 {
		for _, d := range whitelist {
			if d == domain {
				return true, ""
			}
		}
		return false, "该邮箱域名（@" + domain + "）不在允许注册的域名列表内"
	}
	return true, ""
}

// GetSMTPConfigFromDB 从平台配置获取 SMTP 配置
func GetSMTPConfigFromDB() (host, port, user, pass, from string) {
	host = GetPlatformConfig("smtp_host")
	port = GetPlatformConfig("smtp_port")
	user = GetPlatformConfig("smtp_user")
	pass = GetPlatformConfig("smtp_password")
	from = GetPlatformConfig("smtp_from")
	if host == "" {
		host = "smtp.qq.com"
	}
	if port == "" {
		port = "587"
	}
	if from == "" {
		from = user
	}
	return
}