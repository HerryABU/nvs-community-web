package models

import (
	"encoding/json"
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

// InitPlatformConfigs 初始化默认平台配置（仅在不存在时写入）
func InitPlatformConfigs() {
	defaults := map[string]string{
		"site_name":       "星海文学",
		"vip_enabled":     "true",
		"email_verify":    "false",
		"captcha_enabled": "false",
		"categories":      `["硬科幻","奇幻","推演文学","架空历史","现实主义","悬疑推理","实验文学","同人区","政治区","讽刺文学","泛二次元区","其他"]`,
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
