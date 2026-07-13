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
