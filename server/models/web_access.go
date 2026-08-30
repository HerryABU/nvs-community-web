package models

import (
	"time"

	"gorm.io/gorm"
)

// WebAccessLog 全量 Web 访问日志（数据库可查询版）。
// 与 middleware.AccessLog 的「文件日志」互为补充：文件日志保留完整请求头/正文用于合规归档，
// 本表保留结构化字段用于前台「网络诊断 / 用户分布 / 攻击流量」可视化（功能二）。
//
// 写入策略：AccessLog 中间件在 c.Next() 后把结构化条目投入异步 channel，
// 由后台 worker 批量 INSERT（每 200 条或 500ms flush 一次），不阻塞请求主流程；
// channel 满则丢弃（背压保护，绝不因日志拖垮业务）。
//
// ⚠️ TTL 说明：HTTP 应用层拿不到 IP 层 TTL（那是 IP 协议字段），故本表不存 ttl，
// 改用 XForwardedFor 链路长度（ProxyHops）近似「代理跳数」以辅助判断真实客户端位置。
type WebAccessLog struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	UserID        uint      `json:"user_id"`
	IdentityType  string    `gorm:"size:16;index" json:"identity_type"` // key / user / anonymous / bot
	UAFamily      string    `gorm:"size:16;index" json:"ua_family"`     // browser / mobile / bot / tool / unknown
	Method        string    `gorm:"size:8" json:"method"`
	Path          string    `gorm:"size:512;index" json:"path"`
	Query         string    `gorm:"size:512" json:"query"`
	IP            string    `gorm:"size:64;index" json:"ip"`
	XForwardedFor string    `gorm:"size:256" json:"x_forwarded_for"`
	ProxyHops     int       `json:"proxy_hops"` // X-Forwarded-For 逗号分隔跳数（近似代理深度）
	UA            string    `gorm:"size:512" json:"user_agent"`
	StatusCode    int       `gorm:"index" json:"status_code"`
	Size          int       `json:"size"`
	LatencyMs     int       `json:"latency_ms"`
	IsBot         bool      `gorm:"index" json:"is_bot"`
	BotReason     string    `gorm:"size:64" json:"bot_reason"`
	CreatedAt     time.Time `gorm:"index" json:"created_at"`
}

func (WebAccessLog) TableName() string { return "web_access_logs" }

// WebAccessQuery 列表查询条件
type WebAccessQuery struct {
	IP         string
	UA         string
	UAFamily   string
	Status     int
	IsBot      *bool
	Method     string
	Path       string
	Identity   string
	From       string // 2006-01-02 或 2006-01-02 15:04:05
	To         string
	OnlyAttack bool // 仅攻击流量（is_bot=true 或 status>=400）
	Page       int
	PageSize   int
}

// ListWebAccess 分页 + 多维筛选查询 Web 访问日志
func ListWebAccess(q WebAccessQuery) (items []WebAccessLog, total int64, err error) {
	if q.PageSize <= 0 {
		q.PageSize = 50
	}
	if q.Page <= 0 {
		q.Page = 1
	}
	db := DB.Model(&WebAccessLog{})
	if q.IP != "" {
		db = db.Where("ip LIKE ?", "%"+q.IP+"%")
	}
	if q.UA != "" {
		db = db.Where("user_agent LIKE ?", "%"+q.UA+"%")
	}
	if q.UAFamily != "" {
		db = db.Where("ua_family = ?", q.UAFamily)
	}
	if q.Status > 0 {
		db = db.Where("status_code = ?", q.Status)
	}
	if q.IsBot != nil {
		db = db.Where("is_bot = ?", *q.IsBot)
	}
	if q.Method != "" {
		db = db.Where("method = ?", q.Method)
	}
	if q.Path != "" {
		db = db.Where("path LIKE ?", "%"+q.Path+"%")
	}
	if q.Identity != "" {
		db = db.Where("identity_type = ?", q.Identity)
	}
	if q.From != "" {
		db = db.Where("created_at >= ?", q.From)
	}
	if q.To != "" {
		db = db.Where("created_at <= ?", q.To)
	}
	if q.OnlyAttack {
		db = db.Where("(is_bot = ?) OR (status_code >= ?)", true, 400)
	}
	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = db.Order("id desc").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&items).Error
	return items, total, err
}

// WebAccessStats 聚合统计（用于诊断面板）
type WebAccessStats struct {
	Total        int64             `json:"total"`
	Today        int64             `json:"today"`
	BotTotal     int64             `json:"bot_total"`
	AttackTotal  int64             `json:"attack_total"` // 近 24h：is_bot 或 status>=400
	TopIPs       []map[string]any  `json:"top_ips"`
	TopPaths     []map[string]any  `json:"top_paths"`
	StatusDist   []map[string]any  `json:"status_dist"`
	UAFamilyDist []map[string]any  `json:"ua_family_dist"`
	BotReasonDist []map[string]any `json:"bot_reason_dist"`
}

// GetWebAccessStats 计算诊断面板所需的各类聚合
func GetWebAccessStats() (*WebAccessStats, error) {
	s := &WebAccessStats{}
	today := time.Now().Format("2006-01-02")

	if err := DB.Model(&WebAccessLog{}).Count(&s.Total).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&WebAccessLog{}).Where("created_at >= ?", today+" 00:00:00").Count(&s.Today).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&WebAccessLog{}).Where("is_bot = ?", true).Count(&s.BotTotal).Error; err != nil {
		return nil, err
	}
	yesterday := time.Now().Add(-24 * time.Hour).Format("2006-01-02 15:04:05")
	if err := DB.Model(&WebAccessLog{}).
		Where("created_at >= ? AND ((is_bot = ?) OR (status_code >= ?))", yesterday, true, 400).
		Count(&s.AttackTotal).Error; err != nil {
		return nil, err
	}

	DB.Model(&WebAccessLog{}).Select("ip, COUNT(*) as cnt").Group("ip").Order("cnt desc").Limit(10).
		Scan(&s.TopIPs)
	DB.Model(&WebAccessLog{}).Select("path, COUNT(*) as cnt").Group("path").Order("cnt desc").Limit(10).
		Scan(&s.TopPaths)
	DB.Model(&WebAccessLog{}).Select("status_code, COUNT(*) as cnt").Group("status_code").Order("cnt desc").
		Scan(&s.StatusDist)
	DB.Model(&WebAccessLog{}).Select("ua_family as family, COUNT(*) as cnt").Group("ua_family").Order("cnt desc").
		Scan(&s.UAFamilyDist)
	DB.Model(&WebAccessLog{}).Where("is_bot = ? AND bot_reason <> ''", true).
		Select("bot_reason as reason, COUNT(*) as cnt").Group("bot_reason").Order("cnt desc").Limit(10).
		Scan(&s.BotReasonDist)
	return s, nil
}

// ==================== 异步批量写入（不阻塞请求主流程） ====================

var webAccessChan = make(chan *WebAccessLog, 4096)

func init() {
	go webAccessWorker()
}

// webAccessWorker 后台批量刷盘 worker（DB 未就绪时静默丢弃，不 panic）
func webAccessWorker() {
	const batchSize = 200
	batch := make([]*WebAccessLog, 0, batchSize)
	ticker := time.NewTicker(500 * time.Millisecond)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		if DB == nil {
			batch = batch[:0]
			return
		}
		if err := DB.CreateInBatches(batch, batchSize).Error; err == nil {
			batch = batch[:0]
		}
	}
	for {
		select {
		case e := <-webAccessChan:
			batch = append(batch, e)
			if len(batch) >= batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// EnqueueWebAccess 投递一条 Web 访问记录（异步写入；channel 满则丢弃，保护主流程）
func EnqueueWebAccess(e *WebAccessLog) {
	if e == nil {
		return
	}
	select {
	case webAccessChan <- e:
	default:
	}
}

// WebAccessLogCleanup 清空 Web 访问日志（管理员手动维护，保留合规证据由文件日志承担）
func WebAccessLogCleanup() error {
	return DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Where("1 = 1").Delete(&WebAccessLog{}).Error
}
