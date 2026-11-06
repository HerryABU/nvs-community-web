package readperm

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"nvs-server/models"
)

// ConfigKey 阅读权限配置在 platform_configs 中的键。
const ConfigKey = "read_permission_config"

// Config 站长可配置的阅读权限策略。
//
// EnabledLevels：作者可选择的等级集合；未启用的等级在保存时会被拒绝，
// 且历史内容若使用了被停用的等级，判定时保守降级为「登录可见」（避免内容意外全公开）。
// Labels：站长可覆盖的等级展示文案。
// VipMinLevel：vip 等级要求的最低会员等级（作品侧门槛）。
// VipMaxLevel：#66 卡密可发行的最高会员等级（发行侧上限）。两者都由站长决定，
// 形成「站长定区间 → 卡密发等级 → 作品设门槛」的完整链路。
// PaidMinPrice：paid 等级的最低售价（代币），防止作者把付费内容标成 0。
// 站长是否开放「付费解锁」= EnabledLevels 是否包含 paid。
type Config struct {
	EnabledLevels []string          `json:"enabled_levels"`
	Labels        map[string]string `json:"labels"`
	VipMinLevel   int               `json:"vip_min_level"`
	VipMaxLevel   int               `json:"vip_max_level"`
	PaidMinPrice  int64             `json:"paid_min_price"`
}

// DefaultConfig 默认策略：除「付费解锁」外全部开放（支付通道由 #67 落地后再启用）。
func DefaultConfig() Config {
	return Config{
		EnabledLevels: []string{
			LevelPublic, LevelLogin, LevelFollower, LevelVip, LevelPrivate,
		},
		Labels:       map[string]string{},
		VipMinLevel:  1,
		VipMaxLevel:  3,
		PaidMinPrice: 1,
	}
}

// normalize 清洗配置：去重、剔除非内置等级、保证 public 恒在列表首位。
func (c *Config) normalize() {
	seen := map[string]bool{LevelPublic: true}
	out := []string{LevelPublic}
	for _, l := range c.EnabledLevels {
		l = strings.TrimSpace(l)
		if !IsKnownLevel(l) || seen[l] {
			continue
		}
		seen[l] = true
		out = append(out, l)
	}
	c.EnabledLevels = out
	if c.Labels == nil {
		c.Labels = map[string]string{}
	}
	if c.VipMinLevel < 0 {
		c.VipMinLevel = 0
	}
	if c.VipMinLevel > 99 {
		c.VipMinLevel = 99
	}
	// 0 = #65 时期的旧配置没有该字段，回退默认上限（不能直接夹到 1）
	if c.VipMaxLevel <= 0 {
		c.VipMaxLevel = DefaultConfig().VipMaxLevel
	}
	if c.VipMaxLevel > 99 {
		c.VipMaxLevel = 99
	}
	// 发行上限不得低于作品门槛下限，否则卡密永远无法满足任何 vip 作品
	if c.VipMaxLevel < c.VipMinLevel {
		c.VipMaxLevel = c.VipMinLevel
	}
	if c.PaidMinPrice < 0 {
		c.PaidMinPrice = 0
	}
}

// IsEnabled 某等级当前是否对作者开放。
func (c Config) IsEnabled(level string) bool {
	if level == LevelPublic {
		return true
	}
	for _, l := range c.EnabledLevels {
		if l == level {
			return true
		}
	}
	return false
}

// Validate 保存前校验（管理端调用）。
func (c *Config) Validate() error {
	c.normalize()
	if c.VipMinLevel > 99 {
		return errors.New("VIP 等级上限为 99")
	}
	if c.PaidMinPrice < 0 {
		return errors.New("付费最低价不能为负")
	}
	return nil
}

// 配置挂在正文读取路径上，逐次查库会放大读压力；这里做 15 秒进程内缓存
// （与站内其它单行设置的缓存口径一致），保存后立刻失效。
var (
	cacheMu   sync.RWMutex
	cacheVal  Config
	cacheAt   time.Time
	cacheTTL  = 15 * time.Second
	cacheInit bool
)

// InvalidateCache 让缓存失效（保存配置 / 测试切库时调用）。
func InvalidateCache() {
	cacheMu.Lock()
	cacheInit = false
	cacheAt = time.Time{}
	cacheMu.Unlock()
}

// Load 读取配置；配置缺失 / 损坏时回退默认值（不影响线上读取）。
func Load() Config {
	cacheMu.RLock()
	if cacheInit && time.Since(cacheAt) < cacheTTL {
		c := cacheVal
		cacheMu.RUnlock()
		return c
	}
	cacheMu.RUnlock()

	var c Config
	raw := strings.TrimSpace(models.GetPlatformConfig(ConfigKey))
	if raw == "" {
		c = DefaultConfig()
	} else if err := json.Unmarshal([]byte(raw), &c); err != nil {
		c = DefaultConfig()
	}
	c.normalize()

	cacheMu.Lock()
	cacheVal, cacheAt, cacheInit = c, time.Now(), true
	cacheMu.Unlock()
	return c
}

// Save 保存配置（管理端调用）。
func Save(c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	if err := models.SetPlatformConfig(ConfigKey, string(b)); err != nil {
		return err
	}
	InvalidateCache()
	return nil
}

// AvailableLevels 返回当前开放的等级及其文案（供后台与作者端下拉使用）。
func AvailableLevels() []map[string]interface{} {
	cfg := Load()
	out := make([]map[string]interface{}, 0, len(cfg.EnabledLevels))
	for _, l := range cfg.EnabledLevels {
		out = append(out, map[string]interface{}{
			"key":   l,
			"label": Label(cfg, l),
		})
	}
	return out // 保持配置顺序，不做额外排序
}

// EffectivePermission 兼容老数据：未设置（空串）时按 wall_enabled 映射，
// 非法值一律回退 public（宁可沿用旧逻辑，不把内容锁死）。
func EffectivePermission(permission string, wallEnabled bool) string {
	if permission == "" {
		if wallEnabled {
			return LevelLogin
		}
		return LevelPublic
	}
	if !IsKnownLevel(permission) {
		return LevelPublic
	}
	return permission
}

// ValidateSelection 作者保存作品时校验等级选择与售价。
func ValidateSelection(level string, price int64) error {
	cfg := Load()
	eff := EffectivePermission(level, false)
	if !cfg.IsEnabled(eff) {
		return errors.New("该阅读权限等级已由站长停用")
	}
	if eff == LevelPaid && price < cfg.PaidMinPrice {
		return errors.New("付费内容的售价不得低于站长设定的最低价")
	}
	return nil
}
