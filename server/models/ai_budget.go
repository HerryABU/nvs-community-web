package models

// ==================== 全站总预算 + 高峰排队（gap-audit #12）====================
// 已有的是「单次上限 / 每日个人限额 / OC 群级预算」，缺的是**平台级总闸门**：
// 一个热帖被刷、一次 AI 互聊跑飞，成本照样能把站长整月预算烧穿。
//
// 两道闸门：
//   1. 预算：日 / 月 代币总消耗上限。到线后按 HardStop 决定「拒绝」还是「只告警」
//      （站长可能希望深夜继续跑、只想知道超了）。
//   2. 并发：同时在跑的调用数上限。超出就排队（有超时），避免把上游打到 429 连坐。
//
// 关键取舍：预算按 **AiCallLog.Cost 的实际结算值**累计，不按估算——
// 估算只用于「这一次会不会超」的预判；实际记账永远以落库值为准，避免估算偏差累积成假账。

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// AiBudgetConfig 平台级预算与排队配置（platform_configs.ai_budget）
type AiBudgetConfig struct {
	Enabled bool `json:"enabled"`

	DailyCap   int64 `json:"daily_cap"`   // 每日代币上限（0=不限）
	MonthlyCap int64 `json:"monthly_cap"` // 每月代币上限（0=不限）
	HardStop   bool  `json:"hard_stop"`   // true=到线直接拒绝；false=只记录告警继续跑

	QueueEnabled    bool  `json:"queue_enabled"`
	MaxConcurrent   int   `json:"max_concurrent"`    // 全站并发上限（0=不限）
	QueueTimeoutSec int   `json:"queue_timeout_sec"` // 排队超时（秒），超时即失败并提示
	PeakHours       []int `json:"peak_hours"`        // 高峰时段（0-23）
	PeakConcurrent  int   `json:"peak_concurrent"`   // 高峰时段并发上限（0=沿用 MaxConcurrent）
	PeakQueueOnly   bool  `json:"peak_queue_only"`   // 高峰只排队不拒绝

	AlertAtRatio float64   `json:"alert_at_ratio"` // 用量达到该比例时告警（0.8）
	UpdatedAt    time.Time `json:"updated_at"`
}

const aiBudgetKey = "ai_budget"

// DefaultAiBudgetConfig 默认：不限预算、开并发 24、允许排队 20 秒
func DefaultAiBudgetConfig() AiBudgetConfig {
	return AiBudgetConfig{
		Enabled: true, HardStop: false,
		QueueEnabled: true, MaxConcurrent: 24, QueueTimeoutSec: 20,
		AlertAtRatio: 0.8,
	}
}

func normalizeBudget(c AiBudgetConfig) AiBudgetConfig {
	if c.DailyCap < 0 {
		c.DailyCap = 0
	}
	if c.MonthlyCap < 0 {
		c.MonthlyCap = 0
	}
	if c.MaxConcurrent < 0 {
		c.MaxConcurrent = 0
	}
	if c.QueueTimeoutSec <= 0 {
		c.QueueTimeoutSec = 20
	}
	if c.QueueTimeoutSec > 300 {
		c.QueueTimeoutSec = 300
	}
	if c.AlertAtRatio <= 0 || c.AlertAtRatio > 1 {
		c.AlertAtRatio = 0.8
	}
	// 峰时上限不能超过全站上限（否则「高峰」反而更宽松，是配置错误而不是特性）
	if c.PeakConcurrent > c.MaxConcurrent && c.MaxConcurrent > 0 {
		c.PeakConcurrent = c.MaxConcurrent
	}
	hours := make([]int, 0, len(c.PeakHours))
	seen := map[int]bool{}
	for _, h := range c.PeakHours {
		if h >= 0 && h <= 23 && !seen[h] {
			seen[h] = true
			hours = append(hours, h)
		}
	}
	c.PeakHours = hours
	return c
}

// LoadAiBudgetConfig 读取配置
func LoadAiBudgetConfig() AiBudgetConfig {
	raw := GetPlatformConfig(aiBudgetKey)
	c := DefaultAiBudgetConfig()
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			return DefaultAiBudgetConfig()
		}
	}
	return normalizeBudget(c)
}

// SaveAiBudgetConfig 保存配置
func SaveAiBudgetConfig(c AiBudgetConfig) error {
	c = normalizeBudget(c)
	c.UpdatedAt = time.Now()
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return SetPlatformConfig(aiBudgetKey, string(b))
}

// AiUsage 用量快照
type AiUsage struct {
	Today        int64   `json:"today"`
	Month        int64   `json:"month"`
	DailyCap     int64   `json:"daily_cap"`
	MonthlyCap   int64   `json:"monthly_cap"`
	DailyRatio   float64 `json:"daily_ratio"`
	MonthlyRatio float64 `json:"monthly_ratio"`
	Exceeded     bool    `json:"exceeded"` // 任一上限已到
	Warn         bool    `json:"warn"`     // 达到告警比例
}

// GetAiUsage 计算当日/当月消耗（只统计实际扣费的成功调用）
func GetAiUsage() AiUsage {
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	cfg := LoadAiBudgetConfig()
	u := AiUsage{DailyCap: cfg.DailyCap, MonthlyCap: cfg.MonthlyCap}
	if DB == nil {
		return u
	}
	_ = DB.Model(&AiCallLog{}).Where("created_at >= ?", dayStart).
		Select("COALESCE(SUM(cost),0)").Scan(&u.Today).Error
	_ = DB.Model(&AiCallLog{}).Where("created_at >= ?", monthStart).
		Select("COALESCE(SUM(cost),0)").Scan(&u.Month).Error
	if cfg.DailyCap > 0 {
		u.DailyRatio = float64(u.Today) / float64(cfg.DailyCap)
		if u.Today >= cfg.DailyCap {
			u.Exceeded = true
		}
	}
	if cfg.MonthlyCap > 0 {
		u.MonthlyRatio = float64(u.Month) / float64(cfg.MonthlyCap)
		if u.Month >= cfg.MonthlyCap {
			u.Exceeded = true
		}
	}
	u.Warn = (cfg.DailyCap > 0 && u.DailyRatio >= cfg.AlertAtRatio) ||
		(cfg.MonthlyCap > 0 && u.MonthlyRatio >= cfg.AlertAtRatio)
	return u
}

// CheckGlobalBudget 预判这一次调用会不会撞上限（est 为本次估算消耗）
func CheckGlobalBudget(est int64) error {
	cfg := LoadAiBudgetConfig()
	if !cfg.Enabled || !cfg.HardStop {
		return nil // 不硬拦时只做观测（后台可见 warn），不阻断业务
	}
	u := GetAiUsage()
	if cfg.DailyCap > 0 && u.Today+est > cfg.DailyCap {
		return fmt.Errorf("今日 AI 预算已用尽（%d/%d），请明日再试或联系站长", u.Today, cfg.DailyCap)
	}
	if cfg.MonthlyCap > 0 && u.Month+est > cfg.MonthlyCap {
		return fmt.Errorf("本月 AI 预算已用尽（%d/%d），请联系站长", u.Month, cfg.MonthlyCap)
	}
	return nil
}

// IsPeakHour 当前是否高峰时段
func IsPeakHour(cfg AiBudgetConfig) bool {
	h := time.Now().Hour()
	for _, x := range cfg.PeakHours {
		if x == h {
			return true
		}
	}
	return false
}

// EffectiveConcurrency 当前生效并发上限
func EffectiveConcurrency(cfg AiBudgetConfig) int {
	if !cfg.QueueEnabled {
		return 0
	}
	limit := cfg.MaxConcurrent
	if IsPeakHour(cfg) && cfg.PeakConcurrent > 0 {
		limit = cfg.PeakConcurrent
	}
	return limit
}

// ─────────────── 并发闸门（进程内） ───────────────

var (
	slotMu    sync.Mutex
	slotInUse int
)

// AcquireSlot 获取一个并发名额；返回释放函数与排队耗时。
// limit<=0 表示不限（直接返回空释放函数）。
func AcquireSlot(limit int, timeout time.Duration) (func(), time.Duration, error) {
	if limit <= 0 {
		return func() {}, 0, nil
	}
	start := time.Now()
	for {
		slotMu.Lock()
		if slotInUse < limit {
			slotInUse++
			slotMu.Unlock()
			var once sync.Once
			release := func() {
				once.Do(func() {
					slotMu.Lock()
					if slotInUse > 0 {
						slotInUse--
					}
					slotMu.Unlock()
				})
			}
			return release, time.Since(start), nil
		}
		slotMu.Unlock()
		if time.Since(start) >= timeout {
			return nil, time.Since(start),
				fmt.Errorf("当前 AI 调用排队已满（上限 %d 并发，已等 %s），请稍后重试",
					limit, timeout.Truncate(time.Second))
		}
		time.Sleep(40 * time.Millisecond)
	}
}

// AiConcurrencyInUse 当前在跑数量（后台展示用）
func AiConcurrencyInUse() int {
	slotMu.Lock()
	defer slotMu.Unlock()
	return slotInUse
}

// ResetAiSlots 测试用：复位并发计数
func ResetAiSlots() {
	slotMu.Lock()
	slotInUse = 0
	slotMu.Unlock()
}
