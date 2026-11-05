package quota

import (
	"time"

	"gorm.io/gorm"
	"nvs-server/models"
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
	ID             uint   `gorm:"primaryKey" json:"id"`
	UserID         uint   `json:"user_id"`
	IdentityType   string `gorm:"size:16;index" json:"identity_type"` // key / user / anonymous / bot
	UAFamily       string `gorm:"size:16;index" json:"ua_family"`     // browser / mobile / bot / tool / unknown
	OSName         string `gorm:"size:24;index" json:"os_name"`       // Windows / iOS / Android / macOS / Linux / unknown
	OSVersion      string `gorm:"size:24" json:"os_version"`          // 如 10/11、16.0、13.5
	BrowserName    string `gorm:"size:24;index" json:"browser_name"`  // Chrome / Edge / Firefox / Safari / Opera / unknown
	BrowserVersion string `gorm:"size:24" json:"browser_version"`     // 如 120.0.0.0
	DeviceType     string `gorm:"size:16;index" json:"device_type"`   // desktop / phone / tablet / unknown
	DeviceModel    string `gorm:"size:64" json:"device_model"`        // iPhone / iPad / Android 机型（未知为空）
	Method         string `gorm:"size:8" json:"method"`
	Path           string `gorm:"size:512;index" json:"path"`
	Query          string `gorm:"size:512" json:"query"`
	// Host 用户实际访问用的域名（含端口），例如 cpolar 域名、nginx 反代域名、
	// 局域网 IP:8000、localhost。取值优先级 X-Forwarded-Host > X-Real-Host > Host，
	// 因为经 cpolar / nginx 转发时 Host 头未必被原样带过来。
	Host string `gorm:"size:128;index" json:"host"`
	// Scheme http / https —— 同一域名可能两种协议都进得来，分开记才知道是不是走了 TLS
	Scheme        string    `gorm:"size:8" json:"scheme"`
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
	Host       string // 访问域名（模糊匹配）
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
	db := models.DB.Model(&WebAccessLog{})
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
	if q.Host != "" {
		db = db.Where("host LIKE ?", "%"+q.Host+"%")
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
	Total       int64            `json:"total"`
	Today       int64            `json:"today"`
	BotTotal    int64            `json:"bot_total"`
	AttackTotal int64            `json:"attack_total"` // 近 24h：is_bot 或 status>=400
	TopIPs      []map[string]any `json:"top_ips"`
	// TopHosts 访问域名排行：host / cnt / ips（独立 IP 数）
	// 用来回答「用户是从哪个地址进来的」——cpolar 域名、反代域名、内网 IP 会各占一行
	TopHosts        []map[string]any `json:"top_hosts"`
	TopPaths        []map[string]any `json:"top_paths"`
	StatusDist      []map[string]any `json:"status_dist"`
	UAFamilyDist    []map[string]any `json:"ua_family_dist"`
	OSNameDist      []map[string]any `json:"os_name_dist"`
	BrowserNameDist []map[string]any `json:"browser_name_dist"`
	DeviceTypeDist  []map[string]any `json:"device_type_dist"`
	BotReasonDist   []map[string]any `json:"bot_reason_dist"`
}

// GetWebAccessStats 计算诊断面板所需的各类聚合
func GetWebAccessStats() (*WebAccessStats, error) {
	s := &WebAccessStats{}
	today := time.Now().Format("2006-01-02")

	if err := models.DB.Model(&WebAccessLog{}).Count(&s.Total).Error; err != nil {
		return nil, err
	}
	if err := models.DB.Model(&WebAccessLog{}).Where("created_at >= ?", today+" 00:00:00").Count(&s.Today).Error; err != nil {
		return nil, err
	}
	if err := models.DB.Model(&WebAccessLog{}).Where("is_bot = ?", true).Count(&s.BotTotal).Error; err != nil {
		return nil, err
	}
	yesterday := time.Now().Add(-24 * time.Hour).Format("2006-01-02 15:04:05")
	if err := models.DB.Model(&WebAccessLog{}).
		Where("created_at >= ? AND ((is_bot = ?) OR (status_code >= ?))", yesterday, true, 400).
		Count(&s.AttackTotal).Error; err != nil {
		return nil, err
	}

	models.DB.Model(&WebAccessLog{}).Select("ip, COUNT(*) as cnt").Group("ip").Order("cnt desc").Limit(10).
		Scan(&s.TopIPs)
	// 访问域名：带独立 IP 数，单看请求量分不清「一个人刷」还是「一批人从移动网络来」
	models.DB.Model(&WebAccessLog{}).Where("host <> ''").
		Select("host, COUNT(*) as cnt, COUNT(DISTINCT ip) as ips").
		Group("host").Order("cnt desc").Limit(15).
		Scan(&s.TopHosts)
	models.DB.Model(&WebAccessLog{}).Select("path, COUNT(*) as cnt").Group("path").Order("cnt desc").Limit(10).
		Scan(&s.TopPaths)
	models.DB.Model(&WebAccessLog{}).Select("status_code, COUNT(*) as cnt").Group("status_code").Order("cnt desc").
		Scan(&s.StatusDist)
	models.DB.Model(&WebAccessLog{}).Select("ua_family as family, COUNT(*) as cnt").Group("ua_family").Order("cnt desc").
		Scan(&s.UAFamilyDist)
	models.DB.Model(&WebAccessLog{}).Where("os_name <> '' AND os_name <> 'unknown'").
		Select("os_name as name, COUNT(*) as cnt").Group("os_name").Order("cnt desc").Limit(10).
		Scan(&s.OSNameDist)
	models.DB.Model(&WebAccessLog{}).Where("browser_name <> '' AND browser_name <> 'unknown'").
		Select("browser_name as name, COUNT(*) as cnt").Group("browser_name").Order("cnt desc").Limit(10).
		Scan(&s.BrowserNameDist)
	models.DB.Model(&WebAccessLog{}).Where("device_type <> '' AND device_type <> 'unknown'").
		Select("device_type as name, COUNT(*) as cnt").Group("device_type").Order("cnt desc").
		Scan(&s.DeviceTypeDist)
	models.DB.Model(&WebAccessLog{}).Where("is_bot = ? AND bot_reason <> ''", true).
		Select("bot_reason as reason, COUNT(*) as cnt").Group("bot_reason").Order("cnt desc").Limit(10).
		Scan(&s.BotReasonDist)
	return s, nil
}

// ==================== 异步批量写入（不阻塞请求主流程） ====================

var webAccessChan = make(chan *WebAccessLog, 4096)

func init() {
	go webAccessWorker()
}

// webAccessWorker 后台批量刷盘 worker（models.DB 未就绪时静默丢弃，不 panic）
func webAccessWorker() {
	const batchSize = 200
	batch := make([]*WebAccessLog, 0, batchSize)
	ticker := time.NewTicker(500 * time.Millisecond)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		if models.DB == nil {
			batch = batch[:0]
			return
		}
		if err := models.DB.CreateInBatches(batch, batchSize).Error; err == nil {
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
	return models.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Where("1 = 1").Delete(&WebAccessLog{}).Error
}
