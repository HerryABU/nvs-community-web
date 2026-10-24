// Package risk 充值风控与退款联动（gap-audit #9）。
//
// 背景：以前只有「视频生成失败自动退款」这种**业务性退款**，
// 没有「充值行为本身可疑」的判定，也就谈不上和蜜罐/设备指纹联动。
//
// 本包的定位：**在入账前打分**，给出三种处置，而不是「一律拦」或「一律放」：
//   allow —— 正常入账
//   hold  —— 入账但打标，进人工复核队列（金额已到用户账上，不能被静默吞掉）
//   block —— 不入账，进人工复核队列（黑卡/盗刷/刷单的典型形态；等价于「暂缓到账」）
//
// 关键取舍：
//   - **绝不自动扣回已到账的余额**：钱的事必须有人的判断，程序只负责「不让可疑的钱继续进来」；
//   - 打分只用于排序与分诊，不宣称「判定作弊」——理由（reasons）全部落库，可解释、可申诉；
//   - 回调侧对 block 仍返回成功应答（避免渠道无限重试把队列打爆），
//     同时写事件让管理员看到「有一笔钱收了但没入账」，避免静默丢单。

package risk

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"nvs-server/models"

	"gorm.io/gorm"
)

// 处置
const (
	DecisionAllow = "allow"
	DecisionHold  = "hold"
	DecisionBlock = "block"
)

// 复核状态
const (
	StatusOpen      = "open"      // 待复核
	StatusCleared   = "cleared"   // 人工放行
	StatusConfirmed = "confirmed" // 人工确认异常
)

// Config 风控配置（platform_configs.payment_risk）
type Config struct {
	Enabled bool `json:"enabled"`

	HoldScore  int `json:"hold_score"`  // >= 该分数 → hold（默认 60）
	BlockScore int `json:"block_score"` // >= 该分数 → block（默认 85）

	RapidWindowMin int `json:"rapid_window_min"` // 短时高频窗口（分钟）
	RapidCount     int `json:"rapid_count"`      // 窗口内订单数阈值

	RefundWindowDays int     `json:"refund_window_days"`
	MaxRefundRate    float64 `json:"max_refund_rate"` // 0~1

	NewAccountHours int   `json:"new_account_hours"` // 注册多久内算新号
	BigAmountFen    int64 `json:"big_amount_fen"`    // 大额阈值（分）

	LinkHoneypot    bool      `json:"link_honeypot"`     // 与蜜罐联动（IP 高风险加分）
	LinkDeviceAbuse bool      `json:"link_device_abuse"` // 与设备指纹联动（同设备多账号加分）
	UpdatedAt       time.Time `json:"updated_at"`
}

const cfgKey = "payment_risk"

// DefaultConfig 默认开启但阈值保守（只挡明显异常的）
func DefaultConfig() Config {
	return Config{
		Enabled: true, HoldScore: 60, BlockScore: 85,
		RapidWindowMin: 10, RapidCount: 5,
		RefundWindowDays: 30, MaxRefundRate: 0.3,
		NewAccountHours: 2, BigAmountFen: 50000, // 500 元
		LinkHoneypot: true, LinkDeviceAbuse: true,
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
	if c.HoldScore <= 0 {
		c.HoldScore = 60
	}
	if c.BlockScore <= c.HoldScore {
		c.BlockScore = c.HoldScore + 25
	}
	if c.RapidWindowMin <= 0 {
		c.RapidWindowMin = 10
	}
	if c.RapidCount <= 0 {
		c.RapidCount = 5
	}
	if c.RefundWindowDays <= 0 {
		c.RefundWindowDays = 30
	}
	if c.NewAccountHours < 0 {
		c.NewAccountHours = 0
	}
	return c
}

// Save 保存配置
func Save(c Config) error {
	c.UpdatedAt = time.Now()
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return models.SetPlatformConfig(cfgKey, string(b))
}

// Event 风控事件（append-only，人工只能改 status/handler 字段）
type Event struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	OrderNo   string     `gorm:"size:32;index" json:"order_no"`
	UserID    uint       `gorm:"not null;index" json:"user_id"`
	Score     int        `gorm:"not null;default:0;index" json:"score"`
	Decision  string     `gorm:"size:8;not null;index" json:"decision"` // allow/hold/block
	Reasons   string     `gorm:"type:text" json:"reasons"`              // JSON 数组，可解释
	Payload   string     `gorm:"type:text" json:"payload"`              // 判定时的快照（金额/渠道/IP/设备）
	Status    string     `gorm:"size:16;not null;default:open;index" json:"status"`
	HandledBy uint       `gorm:"not null;default:0" json:"handled_by"`
	HandledAt *time.Time `json:"handled_at"`
	Note      string     `gorm:"size:512;default:''" json:"note"`
	CreatedAt time.Time  `gorm:"index" json:"created_at"`

	Username string `gorm:"-" json:"username,omitempty"`
}

func (Event) TableName() string { return "payment_risk_events" }

// EvalInput 判定入参
type EvalInput struct {
	UserID        uint
	OrderNo       string
	AmountFen     int64
	Channel       string
	IP            string
	DeviceID      string
	UserCreatedAt time.Time
}

// Result 判定结果
type Result struct {
	Score    int
	Decision string
	Reasons  []string
}

// Evaluate 打分（纯函数式：只读库，不写库，便于单测）
func Evaluate(in EvalInput) Result {
	cfg := Load()
	res := Result{Decision: DecisionAllow}
	if !cfg.Enabled {
		return res
	}
	add := func(score int, reason string) {
		res.Score += score
		res.Reasons = append(res.Reasons, reason)
	}

	// 1) 蜜罐联动：该 IP 在蜜罐里留下高风险记录 → 已确认的攻击者来充值
	if cfg.LinkHoneypot && honeypotRiskIP(in.IP) {
		add(40, "该 IP 在蜜罐中留有高风险记录（疑似攻击者/扫描器）")
	}

	// 2) 设备指纹联动：同设备多账号
	if cfg.LinkDeviceAbuse {
		if n := deviceAccounts(in.DeviceID); n >= 3 {
			add(25, fmt.Sprintf("同一设备已关联 %d 个账号", n))
		}
	}

	// 3) 短时高频充值（刷单/测卡）
	if cfg.RapidCount > 0 {
		if n := recentOrders(in.UserID, cfg.RapidWindowMin); n >= int64(cfg.RapidCount) {
			add(30, fmt.Sprintf("%d 分钟内下单 %d 笔（疑似测卡/刷单）", cfg.RapidWindowMin, n))
		}
	}

	// 4) 退款率异常（薅退款/骗赔）
	if rate, total := refundRate(in.UserID, cfg.RefundWindowDays); total >= 3 && rate > cfg.MaxRefundRate {
		add(35, fmt.Sprintf("%d 天内退款率 %.0f%%（%d 单）", cfg.RefundWindowDays, rate*100, total))
	}

	// 5) 新号 + 大额（盗号/黑卡典型）
	if cfg.NewAccountHours > 0 && !in.UserCreatedAt.IsZero() &&
		time.Since(in.UserCreatedAt) < time.Duration(cfg.NewAccountHours)*time.Hour &&
		in.AmountFen >= cfg.BigAmountFen {
		add(20, fmt.Sprintf("注册不足 %d 小时即大额充值 %s", cfg.NewAccountHours, yuan(in.AmountFen)))
	}

	// 6) 单笔超大额（仅提示，不单独构成拦截）
	if in.AmountFen >= 100000 {
		add(10, fmt.Sprintf("单笔金额较大：%s", yuan(in.AmountFen)))
	}

	switch {
	case res.Score >= cfg.BlockScore:
		res.Decision = DecisionBlock
	case res.Score >= cfg.HoldScore:
		res.Decision = DecisionHold
	}
	return res
}

// Record 落库（allow 也记，便于做漏斗与阈值校准；表增长用 TTL 清理）
func Record(in EvalInput, res Result) (*Event, error) {
	rs, _ := json.Marshal(res.Reasons)
	pl, _ := json.Marshal(map[string]interface{}{
		"amount_fen": in.AmountFen, "channel": in.Channel,
		"ip": in.IP, "device_id": in.DeviceID,
		"user_created_at": in.UserCreatedAt,
	})
	st := StatusOpen
	if res.Decision == DecisionAllow {
		st = StatusCleared // 放行的不需要复核，直接归档为已放行
	}
	ev := &Event{
		OrderNo: in.OrderNo, UserID: in.UserID, Score: res.Score,
		Decision: res.Decision, Reasons: string(rs), Payload: string(pl),
		Status: st, CreatedAt: time.Now(),
	}
	if res.Decision == DecisionAllow {
		now := time.Now()
		ev.HandledAt = &now
	}
	if err := models.DB.Create(ev).Error; err != nil {
		return nil, err
	}
	return ev, nil
}

// ─────────────── 只读辅助 ───────────────

// honeypotRiskIP 蜜罐高风险 IP（表不存在则 false）
func honeypotRiskIP(ip string) bool {
	if strings.TrimSpace(ip) == "" || models.DB == nil {
		return false
	}
	var n int64
	err := models.DB.Raw("SELECT COUNT(1) FROM honeypot_sessions WHERE src_ip = ? AND max_risk >= 60", ip).
		Scan(&n).Error
	return err == nil && n > 0
}

// deviceAccounts 同设备账号数（表不存在则 0）
func deviceAccounts(deviceID string) int64 {
	if strings.TrimSpace(deviceID) == "" || models.DB == nil {
		return 0
	}
	var n int64
	err := models.DB.Raw("SELECT COUNT(1) FROM gate_device_bindings WHERE device_id = ?", deviceID).
		Scan(&n).Error
	if err != nil {
		return 0
	}
	return n
}

// recentOrders 窗口内该用户的订单数（含失败/关闭，测卡会留下一串失败单）
func recentOrders(userID uint, minutes int) int64 {
	if userID == 0 {
		return 0
	}
	since := time.Now().Add(-time.Duration(minutes) * time.Minute)
	var n int64
	_ = models.DB.Model(&models.RechargeOrder{}).
		Where("user_id = ? AND created_at >= ?", userID, since).Count(&n).Error
	return n
}

// refundRate 退款率（退款单来自 recharge_orders.status = refunded，若不存在则按 0 处理）
func refundRate(userID uint, days int) (float64, int64) {
	if userID == 0 {
		return 0, 0
	}
	since := time.Now().AddDate(0, 0, -days)
	var total, refunded int64
	_ = models.DB.Model(&models.RechargeOrder{}).
		Where("user_id = ? AND created_at >= ?", userID, since).Count(&total).Error
	if total == 0 {
		return 0, 0
	}
	// refunded 可能存在于不同实现；用 status 模糊匹配避免硬编码枚举失败
	_ = models.DB.Model(&models.RechargeOrder{}).
		Where("user_id = ? AND created_at >= ? AND status LIKE ?", userID, since, "%refund%").
		Count(&refunded).Error
	return float64(refunded) / float64(total), total
}

func yuan(fen int64) string {
	return fmt.Sprintf("¥%.2f", float64(fen)/100)
}

// ─────────────── 复核操作 ───────────────

// List 事件列表（decision/status 可筛）
func List(page, size int, decision, status string) ([]Event, int64, error) {
	if page < 1 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 20
	}
	q := models.DB.Model(&Event{})
	if decision != "" {
		q = q.Where("decision = ?", decision)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	q.Count(&total)
	var list []Event
	if err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// Handle 人工复核：cleared（放行）/ confirmed（确认异常）
func Handle(id, operatorID uint, status, note string) error {
	if status != StatusCleared && status != StatusConfirmed {
		return fmt.Errorf("非法状态")
	}
	now := time.Now()
	return models.DB.Model(&Event{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status": status, "handled_by": operatorID, "handled_at": now, "note": note,
	}).Error
}

// Stats 概览（给后台卡片：近 7 天分诊分布）
func Stats() map[string]interface{} {
	since := time.Now().AddDate(0, 0, -7)
	type row struct {
		Decision string
		C        int64
	}
	var rows []row
	_ = models.DB.Model(&Event{}).Select("decision, COUNT(1) as c").
		Where("created_at >= ?", since).Group("decision").Scan(&rows).Error
	out := map[string]int64{DecisionAllow: 0, DecisionHold: 0, DecisionBlock: 0}
	for _, r := range rows {
		out[r.Decision] = r.C
	}
	var pending int64
	_ = models.DB.Model(&Event{}).Where("status = ?", StatusOpen).Count(&pending).Error
	return map[string]interface{}{
		"last7d": out, "pending": pending, "config": Load(),
	}
}

// Migrate 建表
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&Event{})
}
