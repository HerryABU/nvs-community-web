// Package gate 注册与发帖门槛 + 设备指纹防刷（gap-audit #14）。
//
// 为什么单独一层：防刷不是「一个开关」，而是几道闸门叠起来——
//
//	邀请码（挡住批量注册）→ 设备指纹（挡住同一台机器开小号）→ 账号年龄（挡住注册即刷）
//	→ 每日限额（挡住慢速刷）、蜜罐命中 IP（挡住已被识别的攻击者）。
//
// 这几道闸门的判定都收在这一个包里，处理器只调 `CanRegister` / `CanPost`，
// 避免每个入口各写一份「满 N 小时才能发」的口径（迟早会不一致）。
package gate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"nvs-server/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ─────────────── 配置 ───────────────

// Config 门槛配置（存 platform_configs.register_gate）
type Config struct {
	Enabled bool `json:"enabled"`

	// 注册
	RequireInvite        bool `json:"require_invite"`          // 必须邀请码
	MaxAccountsPerDevice int  `json:"max_accounts_per_device"` // 同设备最多几个账号（0=不限）
	BlockHoneypotIP      bool `json:"block_honeypot_ip"`       // 蜜罐命中 IP 直接拒绝
	BlockDisposableEmail bool `json:"block_disposable_email"`  // 临时邮箱域名拒绝（简单后缀表）

	// 发文/评论门槛（账号年龄，小时）
	PostMinAgeHours    int `json:"post_min_age_hours"`
	CommentMinAgeHours int `json:"comment_min_age_hours"`

	// 新号每日上限（0=不限；与既有每日发文限额叠加取更严的一方）
	NewUserDailyPosts int `json:"new_user_daily_posts"`
	NewUserDays       int `json:"new_user_days"` // 注册多少天内算新号

	UpdatedAt time.Time `json:"updated_at"`
}

const cfgKey = "register_gate"

// DefaultConfig 默认全关（升级不改变现有站点行为）
func DefaultConfig() Config {
	return Config{
		Enabled:              false,
		RequireInvite:        false,
		MaxAccountsPerDevice: 3,
		BlockHoneypotIP:      true,
		PostMinAgeHours:      0,
		CommentMinAgeHours:   0,
		NewUserDailyPosts:    5,
		NewUserDays:          3,
	}
}

// Load 读取配置
func Load() Config {
	raw := models.GetPlatformConfig(cfgKey)
	c := DefaultConfig()
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			return DefaultConfig()
		}
	}
	return normalize(c)
}

func normalize(c Config) Config {
	if c.MaxAccountsPerDevice < 0 {
		c.MaxAccountsPerDevice = 0
	}
	if c.PostMinAgeHours < 0 {
		c.PostMinAgeHours = 0
	}
	if c.CommentMinAgeHours < 0 {
		c.CommentMinAgeHours = 0
	}
	if c.NewUserDays <= 0 {
		c.NewUserDays = 3
	}
	if c.NewUserDailyPosts < 0 {
		c.NewUserDailyPosts = 0
	}
	return c
}

// Save 保存配置
func Save(c Config) error {
	c = normalize(c)
	c.UpdatedAt = time.Now()
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return models.SetPlatformConfig(cfgKey, string(b))
}

// ─────────────── 设备指纹 ───────────────

// DeviceBinding 设备与账号的绑定（一次注册一条）
type DeviceBinding struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	DeviceID  string    `gorm:"size:64;not null;index" json:"device_id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	IP        string    `gorm:"size:64;default:'';index" json:"ip"`
	UAHash    string    `gorm:"size:64;default:''" json:"ua_hash"`
	Source    string    `gorm:"size:16;default:''" json:"source"` // header / fallback
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

func (DeviceBinding) TableName() string { return "gate_device_bindings" }

// DeviceIDFrom 取设备标识：
//  1. 前端传 `X-Device-Id`（localStorage 持久化的随机串，最可靠）
//  2. 退化到 `X-Device-Fingerprint`（前端 canvas/UA 哈希）
//  3. 再退化到 服务端 hash(ip|ua)（最弱：同一 NAT 下多用户会撞，仅作兜底）
//
// 注意：前两条都可被攻击者伪造/清空，所以设备指纹只能「提高成本」，
// 不能当唯一身份。真正的强约束是邀请码 + 账号年龄 + 蜜罐 IP。
func DeviceIDFrom(c *gin.Context) (id, source string) {
	if v := strings.TrimSpace(c.GetHeader("X-Device-Id")); len(v) >= 8 && len(v) <= 64 {
		return v, "header"
	}
	if v := strings.TrimSpace(c.GetHeader("X-Device-Fingerprint")); len(v) >= 8 {
		return "fp_" + shortHash(v), "header"
	}
	ip := c.ClientIP()
	ua := c.GetHeader("User-Agent")
	return "srv_" + shortHash(ip+"|"+ua), "fallback"
}

func shortHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:24]
}

// Bind 记录一次设备绑定（同一 (device,user) 幂等）
func Bind(db *gorm.DB, deviceID string, userID uint, ip, ua, source string) error {
	if deviceID == "" || userID == 0 {
		return nil
	}
	if db == nil {
		db = models.DB
	}
	var n int64
	if err := db.Model(&DeviceBinding{}).
		Where("device_id = ? AND user_id = ?", deviceID, userID).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	return db.Create(&DeviceBinding{
		DeviceID: deviceID, UserID: userID, IP: truncate(ip, 64),
		UAHash: shortHash(ua), Source: source, CreatedAt: time.Now(),
	}).Error
}

// AccountsOnDevice 该设备已注册的账号数
func AccountsOnDevice(deviceID string) int64 {
	if deviceID == "" {
		return 0
	}
	var n int64
	_ = models.DB.Model(&DeviceBinding{}).Where("device_id = ?", deviceID).Count(&n).Error
	return n
}

// ─────────────── 判定 ───────────────

// Err 门槛拒绝（带可展示文案）
type Err struct {
	Code string // invite_required / device_limit / account_too_new / honeypot_ip / disposable_email
	Msg  string
}

func (e *Err) Error() string { return e.Msg }

// RegisterInput 注册判定入参
type RegisterInput struct {
	IP       string
	DeviceID string
	Email    string
	Invite   string // 用户填的邀请码（RequireInvite 时必填）
}

// CanRegister 注册闸门（返回 nil 表示放行；邀请码校验在 ValidateInvite 里单独做，因为要消费次数）
func CanRegister(in RegisterInput) error {
	cfg := Load()
	if !cfg.Enabled {
		return nil
	}
	if cfg.BlockHoneypotIP && HoneypotHitIP(in.IP) {
		return &Err{Code: "honeypot_ip", Msg: "当前网络环境存在异常，暂时无法注册"}
	}
	if cfg.RequireInvite && strings.TrimSpace(in.Invite) == "" {
		return &Err{Code: "invite_required", Msg: "本站注册需要邀请码"}
	}
	if cfg.MaxAccountsPerDevice > 0 {
		if n := AccountsOnDevice(in.DeviceID); int(n) >= cfg.MaxAccountsPerDevice {
			return &Err{Code: "device_limit",
				Msg: fmt.Sprintf("该设备已注册 %d 个账号，达到上限（如需申诉请联系站长）", n)}
		}
	}
	if cfg.BlockDisposableEmail && IsDisposableEmail(in.Email) {
		return &Err{Code: "disposable_email", Msg: "不支持临时邮箱注册，请使用常用邮箱"}
	}
	return nil
}

// PostGateInput 发文/评论判定入参
type PostGateInput struct {
	UserID     uint
	CreatedAt  time.Time
	Kind       string // post / comment
	TodayCount int    // 今日已发条数（由调用方传入，gate 不查业务表）
}

// CanPost 发文/评论闸门
func CanPost(in PostGateInput) error {
	cfg := Load()
	if !cfg.Enabled {
		return nil
	}
	age := time.Since(in.CreatedAt)
	minHours := cfg.PostMinAgeHours
	if in.Kind == "comment" {
		minHours = cfg.CommentMinAgeHours
	}
	if minHours > 0 && age < time.Duration(minHours)*time.Hour {
		left := time.Duration(minHours)*time.Hour - age
		return &Err{Code: "account_too_new",
			Msg: fmt.Sprintf("账号注册未满 %d 小时，还需等待 %s 才能%s",
				minHours, humanDuration(left), kindLabel(in.Kind))}
	}
	// 新号每日上限
	if cfg.NewUserDailyPosts > 0 && age < time.Duration(cfg.NewUserDays)*24*time.Hour {
		if in.TodayCount >= cfg.NewUserDailyPosts {
			return &Err{Code: "new_user_daily_limit",
				Msg: fmt.Sprintf("新账号每日最多 %d 条，明天再来或先熟悉社区", cfg.NewUserDailyPosts)}
		}
	}
	return nil
}

func kindLabel(kind string) string {
	if kind == "comment" {
		return "评论"
	}
	return "发布"
}

func humanDuration(d time.Duration) string {
	if d < time.Minute {
		return "不到 1 分钟"
	}
	if d < time.Hour {
		return fmt.Sprintf("%d 分钟", int(d.Minutes())+1)
	}
	return fmt.Sprintf("%d 小时", int(d.Hours())+1)
}

// HoneypotHitIP 该 IP 是否在蜜罐里留下过高风险记录（>=60）。
// 用原生 SQL 而不是 import honeypot，避免 gate ↔ honeypot 循环依赖；
// 表不存在（未升级/未建站）时静默返回 false。
func HoneypotHitIP(ip string) bool {
	if strings.TrimSpace(ip) == "" || models.DB == nil {
		return false
	}
	var n int64
	err := models.DB.Raw(
		"SELECT COUNT(1) FROM honeypot_sessions WHERE src_ip = ? AND max_risk >= 60", ip).
		Scan(&n).Error
	if err != nil {
		return false
	}
	return n > 0
}

// disposableSuffixes 常见临时邮箱后缀（够用即可，不做大词库）
var disposableSuffixes = []string{
	"mailinator.com", "guerrillamail.com", "10minutemail.com", "tempmail.com",
	"throwawaymail.com", "yopmail.com", "trashmail.com", "sharklasers.com",
	"getnada.com", "maildrop.cc", "temp-mail.org", "fakeinbox.com",
}

// IsDisposableEmail 临时邮箱判定
func IsDisposableEmail(email string) bool {
	e := strings.ToLower(strings.TrimSpace(email))
	i := strings.LastIndex(e, "@")
	if i < 0 {
		return false
	}
	domain := e[i+1:]
	for _, s := range disposableSuffixes {
		if domain == s || strings.HasSuffix(domain, "."+s) {
			return true
		}
	}
	return false
}

// ─────────────── 邀请码 ───────────────

// Invite 邀请码（站长/管理员生成，可限次、限时）
type Invite struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	Code      string     `gorm:"size:32;uniqueIndex;not null" json:"code"`
	CreatedBy uint       `gorm:"not null;index" json:"created_by"`
	MaxUses   int        `gorm:"not null;default:1" json:"max_uses"`
	Used      int        `gorm:"not null;default:0" json:"used"`
	Note      string     `gorm:"size:128;default:''" json:"note"`
	ExpiresAt *time.Time `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
}

func (Invite) TableName() string { return "gate_invites" }

// ErrInvite 邀请码不可用
var ErrInvite = errors.New("邀请码无效或已用尽")

// CreateInvite 生成邀请码
func CreateInvite(createdBy uint, maxUses int, note string, ttlHours int) (*Invite, error) {
	if maxUses <= 0 {
		maxUses = 1
	}
	code := "INV-" + strings.ToUpper(shortHash(fmt.Sprintf("%d-%d-%s", createdBy, time.Now().UnixNano(), note))[:10])
	inv := &Invite{Code: code, CreatedBy: createdBy, MaxUses: maxUses, Note: truncate(note, 128), CreatedAt: time.Now()}
	if ttlHours > 0 {
		t := time.Now().Add(time.Duration(ttlHours) * time.Hour)
		inv.ExpiresAt = &t
	}
	if err := models.DB.Create(inv).Error; err != nil {
		return nil, err
	}
	return inv, nil
}

// ListInvites 邀请码列表
func ListInvites(page, size int) ([]Invite, int64, error) {
	if page < 1 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 20
	}
	var list []Invite
	var total int64
	q := models.DB.Model(&Invite{})
	q.Count(&total)
	if err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// UseInvite 消费一次邀请码（注册流程内调用；事务里做，防并发超发）
func UseInvite(tx *gorm.DB, code string) error {
	if tx == nil {
		tx = models.DB
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return ErrInvite
	}
	var inv Invite
	if err := tx.Where("code = ?", code).First(&inv).Error; err != nil {
		return ErrInvite
	}
	if inv.ExpiresAt != nil && time.Now().After(*inv.ExpiresAt) {
		return ErrInvite
	}
	res := tx.Model(&Invite{}).
		Where("id = ? AND used < max_uses", inv.ID).
		UpdateColumn("used", gorm.Expr("used + 1"))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrInvite
	}
	return nil
}

// RevokeInvite 作废邀请码（把上限压到已用次数）
func RevokeInvite(id uint) error {
	return models.DB.Model(&Invite{}).Where("id = ?", id).
		Update("max_uses", gorm.Expr("used")).Error
}

// Migrate 建表
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&DeviceBinding{}, &Invite{})
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
