package models

import (
	"encoding/json"
	"time"
)

// VipApplication VIP 申请
type VipApplication struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;uniqueIndex" json:"user_id"`
	Status    string    `gorm:"size:16;default:pending" json:"status"` // pending/approved/rejected
	Reason    string    `gorm:"type:text" json:"reason"`
	ReviewedAt *time.Time `json:"reviewed_at"`
	CreatedAt time.Time `json:"created_at"`
	User      *User     `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (VipApplication) TableName() string { return "vip_applications" }

// Report 举报
type Report struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ReporterID uint      `gorm:"not null;index" json:"reporter_id"`
	TargetType string    `gorm:"size:16;not null" json:"target_type"` // novel/comment/thread
	TargetID   uint      `gorm:"not null" json:"target_id"`
	Reason     string    `gorm:"size:64;not null" json:"reason"`
	Detail     string    `gorm:"type:text" json:"detail"`
	Status     string    `gorm:"size:16;default:pending" json:"status"` // pending/accepted/rejected
	HandlerID  *uint     `json:"handler_id"`
	Verdict    string    `gorm:"type:text" json:"verdict"`
	CreatedAt  time.Time `json:"created_at"`
	Reporter   *User     `gorm:"foreignKey:ReporterID" json:"reporter,omitempty"`
}

func (Report) TableName() string { return "reports" }

// PlatformConfig 平台配置（站长面板）
type PlatformConfig struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Key       string    `gorm:"size:64;uniqueIndex;not null" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (PlatformConfig) TableName() string { return "platform_configs" }

// WithdrawalRequest 提现申请
type WithdrawalRequest struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	Amount    float64   `gorm:"not null" json:"amount"`
	Method    string    `gorm:"size:32" json:"method"`
	Account   string    `gorm:"size:128" json:"account"`
	Status    string    `gorm:"size:16;default:pending" json:"status"` // pending/approved/paid/rejected
	CreatedAt time.Time `json:"created_at"`
	User      *User     `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (WithdrawalRequest) TableName() string { return "withdrawal_requests" }

// EarningsRecord 收益记录
type EarningsRecord struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	NovelID   *uint     `json:"novel_id"`
	Amount    float64   `gorm:"not null" json:"amount"`
	Type      string    `gorm:"size:32;not null" json:"type"` // tip/subscription
	CreatedAt time.Time `json:"created_at"`
}

func (EarningsRecord) TableName() string { return "earnings_records" }

// FederatedSite 远程互通站点（模仿 alist 的站点互联）
type FederatedSite struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:128;not null" json:"name"`
	URL         string    `gorm:"size:512;not null" json:"url"`
	APIURL      string    `gorm:"size:512;not null" json:"api_url"`
	Description string    `gorm:"type:text" json:"description"`
	Status      string    `gorm:"size:16;default:active;index" json:"status"` // active/inactive
	LastSyncAt  *time.Time `json:"last_sync_at"`
	NovelCount  int       `gorm:"default:0" json:"novel_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (FederatedSite) TableName() string { return "federated_sites" }

// FederatedNovel 从远程站点同步来的作品缓存
type FederatedNovel struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	SiteID    uint      `gorm:"not null;index" json:"site_id"`
	RemoteID  uint      `gorm:"not null;index" json:"remote_id"`
	Title     string    `gorm:"size:256;not null" json:"title"`
	Category  string    `gorm:"size:64" json:"category"`
	Author    string    `gorm:"size:128" json:"author"`
	Summary   string    `gorm:"type:text" json:"summary"`
	CoverURL  string    `gorm:"size:512" json:"cover_url"`
	SourceURL string    `gorm:"size:512" json:"source_url"`
	CachedAt  time.Time `json:"cached_at"`
	Site      *FederatedSite `gorm:"foreignKey:SiteID" json:"site,omitempty"`
}

func (FederatedNovel) TableName() string { return "federated_novels" }

// BlacklistIP 黑名单 IP
type BlacklistIP struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	IP        string    `gorm:"size:45;uniqueIndex;not null" json:"ip"`
	Reason    string    `gorm:"size:256" json:"reason"`
	ExpiresAt *time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

func (BlacklistIP) TableName() string { return "blacklist_ips" }

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

// WallConfig 隔离墙配置
type WallConfig struct {
	Zones              []string `json:"zones"`
	Enabled            bool     `json:"enabled"`
	CrossDomainWarning bool     `json:"cross_domain_warning"`
}

// DefaultWallConfig 返回默认隔离墙配置
func DefaultWallConfig() WallConfig {
	return WallConfig{
		Zones:              []string{"同人区", "政治文学区"},
		Enabled:            true,
		CrossDomainWarning: true,
	}
}

// GetWallConfig 获取隔离墙配置
func GetWallConfig() WallConfig {
	val := GetPlatformConfig("wall_config")
	if val == "" {
		return DefaultWallConfig()
	}
	var cfg WallConfig
	if err := json.Unmarshal([]byte(val), &cfg); err != nil {
		return DefaultWallConfig()
	}
	return cfg
}

// SetWallConfig 保存隔离墙配置
func SetWallConfig(cfg WallConfig) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return SetPlatformConfig("wall_config", string(data))
}

// InitPlatformConfigs 初始化默认平台配置（仅在不存在时写入）
func InitPlatformConfigs() {
	defaults := map[string]string{
		"site_name":   "星海文学",
		"vip_enabled": "true",
		"categories":  `["硬科幻","奇幻","推演文学","架空历史","现实主义","悬疑推理","实验文学","同人区","政治区","讽刺文学","泛二次元区","其他"]`,
	}
	for k, v := range defaults {
		var count int64
		DB.Model(&PlatformConfig{}).Where("key = ?", k).Count(&count)
		if count == 0 {
			DB.Create(&PlatformConfig{Key: k, Value: v, UpdatedAt: time.Now()})
		}
	}
}
