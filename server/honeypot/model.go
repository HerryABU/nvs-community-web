package honeypot

// ==================== 蜜罐 · 数据模型 ====================
// 设计见 docs/honeypot-design.md。v1 落地「主站内嵌诱饵」形态（可后续拆独立端口/二进制）：
//   - 诱饵路径命中 → 记事件（append-only）
//   - 等级由站长/管理员控制：off / low / medium / high
//   - 日志字段可勾选：是否记录 UA / Referer / X-Forwarded-For / Query / 请求头 / Body 摘要
// 铁律：蜜罐凭据全部一次性生成、与真实系统零复用；不碰真实库；不提供任何「放行」分支。

import (
	"time"

	"gorm.io/gorm"
)

// 蜜罐等级（站长可配）
const (
	LevelOff    = "off"    // 关闭：不挂任何诱饵
	LevelLow    = "low"    // 低：只放最像「运维疏忽」的少量诱饵（.env）
	LevelMedium = "medium" // 中：加 robots/.git/常见后台入口
	LevelHigh   = "high"   // 高：全量诱饵 + 慢速响应 + 命中即封禁
)

// 事件类型
const (
	EventDecoyHit   = "decoy_hit"   // 命中诱饵路径
	EventDecoyProbe = "decoy_probe" // 探测但不存在的诱饵（404 探路）
	EventExport     = "export"      // 管理员导出取证（自身审计）
)

// Setting 蜜罐与日志记录策略（单行，id=1）
type Setting struct {
	ID      uint   `gorm:"primaryKey" json:"id"`
	Enabled bool   `gorm:"not null;default:false" json:"enabled"`
	Level   string `gorm:"size:16;not null;default:off" json:"level"`

	// 日志记录字段（管理员勾选）：关掉的字段不落库，节省体积与合规风险
	RecordUA          bool   `gorm:"not null;default:true" json:"record_ua"`
	RecordReferer     bool   `gorm:"not null;default:true" json:"record_referer"`
	RecordXFF         bool   `gorm:"not null;default:false" json:"record_xff"`
	RecordQuery       bool   `gorm:"not null;default:true" json:"record_query"`
	RecordHeaders     bool   `gorm:"not null;default:false" json:"record_headers"`
	RecordBodySnippet bool   `gorm:"not null;default:false" json:"record_body_snippet"`
	BodySnippetLimit  int    `gorm:"not null;default:512" json:"body_snippet_limit"`
	HeaderAllowlist   string `gorm:"size:512;default:''" json:"header_allowlist"` // 逗号分隔，仅这些头会被记录

	// 处置与留存
	AutoBan       bool `gorm:"not null;default:false" json:"auto_ban"`
	BanMinutes    int  `gorm:"not null;default:60" json:"ban_minutes"`
	DelayMs       int  `gorm:"not null;default:0" json:"delay_ms"`        // 命中后延迟响应（拖住自动化）
	RetentionDays int  `gorm:"not null;default:90" json:"retention_days"` // 事件/指纹留存天数

	UpdatedAt time.Time `json:"updated_at"`
}

func (Setting) TableName() string { return "honeypot_settings" }

// Session 攻击会话（按 IP + UA 归一聚合）
type Session struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	HID        string    `gorm:"size:32;index" json:"hid"` // 会话标识（随机，非真实凭据）
	SrcIP      string    `gorm:"size:64;index" json:"src_ip"`
	UANorm     string    `gorm:"size:256" json:"ua_norm"`
	Referer    string    `gorm:"size:512" json:"referer"`
	XFF        string    `gorm:"size:256" json:"xff"`
	Country    string    `gorm:"size:8;default:''" json:"country"`
	EventCount int64     `gorm:"not null;default:0" json:"event_count"`
	MaxRisk    int       `gorm:"not null;default:0;index" json:"max_risk"`
	Banned     bool      `gorm:"not null;default:false" json:"banned"`
	FirstSeen  time.Time `json:"first_seen"`
	LastSeen   time.Time `gorm:"index" json:"last_seen"`
}

func (Session) TableName() string { return "honeypot_sessions" }

// Event 单次命中事件（append-only）
type Event struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	SessionID   uint      `gorm:"not null;index" json:"session_id"`
	Kind        string    `gorm:"size:24;not null;index" json:"kind"`
	Decoy       string    `gorm:"size:64;default:''" json:"decoy"` // 诱饵标识（env/git/admin…）
	Method      string    `gorm:"size:8" json:"method"`
	Path        string    `gorm:"size:512;index" json:"path"`
	Query       string    `gorm:"size:512" json:"query"`    // 按策略决定是否落库
	UA          string    `gorm:"size:512" json:"ua"`       // 按策略决定是否落库
	Referer     string    `gorm:"size:512" json:"referer"`  // 按策略决定是否落库
	XFF         string    `gorm:"size:256" json:"xff"`      // 按策略决定是否落库
	Headers     string    `gorm:"type:text" json:"headers"` // 仅白名单头，按策略
	BodySnippet string    `gorm:"type:text" json:"body_snippet"`
	Risk        int       `gorm:"not null;default:0" json:"risk"`
	Action      string    `gorm:"size:24;default:''" json:"action"` // record / delay / ban / banned_skip
	CreatedAt   time.Time `gorm:"index" json:"created_at"`
}

func (Event) TableName() string { return "honeypot_events" }

// Migrate 建表 + 初始化默认策略
func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&Setting{}, &Session{}, &Event{}); err != nil {
		return err
	}
	var n int64
	db.Model(&Setting{}).Count(&n)
	if n == 0 {
		db.Create(&Setting{
			ID: 1, Enabled: false, Level: LevelOff,
			RecordUA: true, RecordReferer: true, RecordQuery: true,
			BodySnippetLimit: 512, BanMinutes: 60, RetentionDays: 90,
			UpdatedAt: time.Now(),
		})
	}
	return nil
}

// NormalizeLevel 等级归一化
func NormalizeLevel(l string) string {
	switch l {
	case LevelLow, LevelMedium, LevelHigh:
		return l
	default:
		return LevelOff
	}
}
