package models

import "time"

// FederatedSite 远程互通站点（模仿 alist 的站点互联）
type FederatedSite struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Name        string     `gorm:"size:128;not null" json:"name"`
	URL         string     `gorm:"size:512;not null" json:"url"`
	APIURL      string     `gorm:"size:512;not null" json:"api_url"`
	Description string     `gorm:"type:text" json:"description"`
	Status      string     `gorm:"size:16;default:active;index" json:"status"`
	LastSyncAt  *time.Time `json:"last_sync_at"`
	NovelCount  int        `gorm:"default:0" json:"novel_count"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (FederatedSite) TableName() string { return "federated_sites" }

// FederatedNovel 从远程站点同步来的作品缓存
type FederatedNovel struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	SiteID    uint           `gorm:"not null;index" json:"site_id"`
	RemoteID  uint           `gorm:"not null;index" json:"remote_id"`
	Title     string         `gorm:"size:256;not null" json:"title"`
	Category  string         `gorm:"size:64" json:"category"`
	Author    string         `gorm:"size:128" json:"author"`
	Summary   string         `gorm:"type:text" json:"summary"`
	CoverURL  string         `gorm:"size:512" json:"cover_url"`
	SourceURL string         `gorm:"size:512" json:"source_url"`
	CachedAt  time.Time      `json:"cached_at"`
	Site      *FederatedSite `gorm:"foreignKey:SiteID" json:"site,omitempty"`
}

func (FederatedNovel) TableName() string { return "federated_novels" }

// FederatedInbound 入站联邦连接记录 —— 记录「谁连接了我」。
// 当远程站点访问本站公开联邦接口（/api/federated/novels 等）时，登记对方身份，
// 让被连的一方也能看到是哪个站点在同步/连接自己。
// 若对方携带本站签发的联邦 API Key，则同时记录 Key 前缀与签发方 —— 双向可见：
// 被链接的站能看到对方用的是哪一个 Key、是谁的。
type FederatedInbound struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:128" json:"name"`                 // 对方自称的站点名（source_name）
	URL       string    `gorm:"size:512" json:"url"`                  // 对方自称的站点地址（source_url）
	IP        string    `gorm:"size:64;index" json:"ip"`              // 来源 IP
	UserAgent string    `gorm:"size:512" json:"user_agent"`           // User-Agent
	Endpoint  string    `gorm:"size:64" json:"endpoint"`              // 命中的公开接口
	KeyPrefix string    `gorm:"size:24;default:''" json:"key_prefix"` // 对方使用的联邦 Key 前缀（nvs_fed_xxxx）
	Issuer    string    `gorm:"size:128;default:''" json:"issuer"`    // 该 Key 的签发方站名（谁发的 Key）
	Count     int       `gorm:"default:1" json:"count"`               // 累计访问次数
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
}

func (FederatedInbound) TableName() string { return "federated_inbound" }

// RecordFederatedInbound 记录一次入站联邦访问（按 IP+名称+端点 聚合计数）
func RecordFederatedInbound(name, url, ip, userAgent, endpoint, keyPrefix, issuer string) {
	now := time.Now()
	var rec FederatedInbound
	err := DB.Where("ip = ? AND name = ? AND endpoint = ?", ip, name, endpoint).First(&rec).Error
	if err != nil {
		DB.Create(&FederatedInbound{
			Name: name, URL: url, IP: ip, UserAgent: userAgent, Endpoint: endpoint,
			KeyPrefix: keyPrefix, Issuer: issuer,
			Count: 1, FirstSeen: now, LastSeen: now,
		})
		return
	}
	rec.Count++
	rec.LastSeen = now
	if url != "" {
		rec.URL = url
	}
	if userAgent != "" {
		rec.UserAgent = userAgent
	}
	if keyPrefix != "" {
		rec.KeyPrefix = keyPrefix
	}
	if issuer != "" {
		rec.Issuer = issuer
	}
	DB.Save(&rec)
}

// ListFederatedInbound 列出入站连接记录（最近 N 条）
func ListFederatedInbound(limit int) ([]FederatedInbound, error) {
	if limit <= 0 {
		limit = 200
	}
	var list []FederatedInbound
	err := DB.Order("last_seen DESC").Limit(limit).Find(&list).Error
	return list, err
}
