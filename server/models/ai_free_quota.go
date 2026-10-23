package models

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// ==================== 免费 AI 额度（§5.3） ====================
// 平台级默认 + 按人覆盖：免费额度用尽才扣代币。默认关闭（站长开启即生效）。
// 语义（与每日发文限额对齐）：
//   - CallsPerDay：每日免费次数，-1=不限，0=不提供
//   - MaxTokensPerDay：每日免费 tokens 上限，-1=不限，0=不限（仅限次数）
// 按人覆盖列默认 -1 / 0，写入一律 map 精确写（GORM 零值坑）。

const (
	// AiFreeQuotaEnabledKey 免费额度总开关（默认关）
	AiFreeQuotaEnabledKey = "ai_free_quota_enabled"
	// AiFreeQuotaConfigKey 平台默认配置 JSON
	AiFreeQuotaConfigKey = "ai_free_quota"
)

// AiFreeQuotaConfig 平台默认免费额度
type AiFreeQuotaConfig struct {
	CallsPerDay     int `json:"calls_per_day"`     // 每日免费次数（-1 不限 / 0 不提供）
	MaxTokensPerDay int `json:"max_tokens_per_day"` // 每日免费 tokens 上限（-1 不限 / 0 不限）
}

// DefaultAiFreeQuotaConfig 默认：每日 5 次免费、tokens 不限
func DefaultAiFreeQuotaConfig() AiFreeQuotaConfig {
	return AiFreeQuotaConfig{CallsPerDay: 5, MaxTokensPerDay: 0}
}

func normalizeAiFreeQuotaConfig(cfg AiFreeQuotaConfig) AiFreeQuotaConfig {
	if cfg.CallsPerDay < -1 {
		cfg.CallsPerDay = 0
	}
	if cfg.CallsPerDay > 100000 {
		cfg.CallsPerDay = 100000
	}
	if cfg.MaxTokensPerDay < -1 {
		cfg.MaxTokensPerDay = 0
	}
	if cfg.MaxTokensPerDay > 100000000 {
		cfg.MaxTokensPerDay = 100000000
	}
	return cfg
}

// IsAiFreeQuotaEnabled 免费额度是否开启（默认关闭）
func IsAiFreeQuotaEnabled() bool {
	return GetPlatformConfig(AiFreeQuotaEnabledKey) == "true"
}

// SetAiFreeQuotaEnabled 设置免费额度开关
func SetAiFreeQuotaEnabled(on bool) error {
	if on {
		return SetPlatformConfig(AiFreeQuotaEnabledKey, "true")
	}
	return SetPlatformConfig(AiFreeQuotaEnabledKey, "false")
}

// GetAiFreeQuotaConfig 平台默认免费额度
func GetAiFreeQuotaConfig() AiFreeQuotaConfig {
	raw := strings.TrimSpace(GetPlatformConfig(AiFreeQuotaConfigKey))
	if raw == "" {
		return DefaultAiFreeQuotaConfig()
	}
	var cfg AiFreeQuotaConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return DefaultAiFreeQuotaConfig()
	}
	return normalizeAiFreeQuotaConfig(cfg)
}

// SetAiFreeQuotaConfig 保存平台默认免费额度
func SetAiFreeQuotaConfig(cfg AiFreeQuotaConfig) error {
	cfg = normalizeAiFreeQuotaConfig(cfg)
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return SetPlatformConfig(AiFreeQuotaConfigKey, string(raw))
}

// UserAiFreeQuota 按人免费额度覆盖（无记录=跟随平台默认）
type UserAiFreeQuota struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	UserID          uint      `gorm:"uniqueIndex;not null" json:"user_id"`
	DailyCalls      int       `gorm:"not null;default:-1" json:"daily_calls"`      // -1 不限 / 0 禁止
	DailyTokens     int       `gorm:"not null;default:0" json:"daily_tokens"`      // 0 不限 / -1 禁止 tokens 维度
	Note            string    `gorm:"size:256;default:''" json:"note"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	Username        string    `gorm:"-" json:"username,omitempty"` // 列表填充
}

func (UserAiFreeQuota) TableName() string { return "user_ai_free_quotas" }

// GetUserAiFreeQuotaOverride 查询按人覆盖（无记录返回 nil）
func GetUserAiFreeQuotaOverride(userID uint) *UserAiFreeQuota {
	var q UserAiFreeQuota
	if err := DB.Where("user_id = ?", userID).First(&q).Error; err != nil {
		return nil
	}
	return &q
}

// SetUserAiFreeQuotaOverride UPSERT 按人覆盖（原生 SQL upsert，零值安全）
func SetUserAiFreeQuotaOverride(userID uint, dailyCalls, dailyTokens int, note string) error {
	if dailyCalls < -1 {
		dailyCalls = -1
	}
	if dailyTokens < -1 {
		dailyTokens = -1
	}
	note = strings.TrimSpace(note)
	now := time.Now()
	return DB.Exec(
		"INSERT INTO user_ai_free_quotas (user_id, daily_calls, daily_tokens, note, created_at, updated_at) "+
			"VALUES (?, ?, ?, ?, ?, ?) "+
			"ON CONFLICT(user_id) DO UPDATE SET daily_calls=excluded.daily_calls, "+
			"daily_tokens=excluded.daily_tokens, note=excluded.note, updated_at=excluded.updated_at",
		userID, dailyCalls, dailyTokens, note, now, now).Error
}

// DeleteUserAiFreeQuotaOverride 删除按人覆盖（恢复平台默认）
func DeleteUserAiFreeQuotaOverride(userID uint) error {
	return DB.Where("user_id = ?", userID).Delete(&UserAiFreeQuota{}).Error
}

// ListUserAiFreeQuotas 全部按人覆盖（管理员列表）
func ListUserAiFreeQuotas() ([]UserAiFreeQuota, error) {
	var list []UserAiFreeQuota
	err := DB.Order("updated_at DESC").Find(&list).Error
	return list, err
}

// EffectiveAiFreeLimit 用户生效的免费额度（按人覆盖优先）
func EffectiveAiFreeLimit(userID uint) (calls, tokens int) {
	cfg := GetAiFreeQuotaConfig()
	calls, tokens = cfg.CallsPerDay, cfg.MaxTokensPerDay
	if o := GetUserAiFreeQuotaOverride(userID); o != nil {
		calls, tokens = o.DailyCalls, o.DailyTokens
	}
	return calls, tokens
}

// UserAiFreeUsage 每日免费额度用量（user_id + day 唯一）
type UserAiFreeUsage struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"uniqueIndex:idx_ai_free_usage,priority:1;not null" json:"user_id"`
	Day        string    `gorm:"size:10;uniqueIndex:idx_ai_free_usage,priority:2;not null" json:"day"` // YYYY-MM-DD
	CallsUsed  int       `gorm:"not null;default:0" json:"calls_used"`
	TokensUsed int       `gorm:"not null;default:0" json:"tokens_used"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (UserAiFreeUsage) TableName() string { return "user_ai_free_usages" }

func aiFreeToday() string { return time.Now().Format("2006-01-02") }

// GetTodayAiFreeUsage 今日已用免费额度
func GetTodayAiFreeUsage(userID uint) (calls, tokens int64) {
	var u UserAiFreeUsage
	if err := DB.Where("user_id = ? AND day = ?", userID, aiFreeToday()).First(&u).Error; err != nil {
		return 0, 0
	}
	return int64(u.CallsUsed), int64(u.TokensUsed)
}

// RecordAiFreeUsage 记录一次免费额度消耗（upsert 自增）
func RecordAiFreeUsage(userID uint, calls, tokens int) error {
	day := aiFreeToday()
	var u UserAiFreeUsage
	err := DB.Where("user_id = ? AND day = ?", userID, day).First(&u).Error
	if err != nil {
		return DB.Create(&UserAiFreeUsage{
			UserID: userID, Day: day, CallsUsed: calls, TokensUsed: tokens,
			UpdatedAt: time.Now(),
		}).Error
	}
	return DB.Model(&UserAiFreeUsage{}).Where("id = ?", u.ID).Updates(map[string]interface{}{
		"calls_used":  u.CallsUsed + calls,
		"tokens_used": u.TokensUsed + tokens,
		"updated_at":  time.Now(),
	}).Error
}

// CheckAiFreeQuota 判定本次调用能否走免费额度
// 返回：eligible（可免费）、callsLeft/tokensLeft（剩余，-1=不限）
func CheckAiFreeQuota(userID uint, estTokens int) (eligible bool, callsLeft, tokensLeft int) {
	if !IsAiFreeQuotaEnabled() {
		return false, 0, 0
	}
	calls, tokens := EffectiveAiFreeLimit(userID)
	if calls == 0 {
		return false, 0, 0 // 禁止免费
	}
	usedCalls, usedTokens := GetTodayAiFreeUsage(userID)
	if calls > 0 && usedCalls >= int64(calls) {
		return false, 0, 0 // 次数用尽
	}
	if tokens > 0 && usedTokens+int64(estTokens) > int64(tokens) {
		return false, 0, 0 // tokens 预算不足
	}
	callsLeft = -1
	if calls > 0 {
		callsLeft = calls - int(usedCalls)
	}
	tokensLeft = -1
	if tokens > 0 {
		tokensLeft = tokens - int(usedTokens)
	}
	return true, callsLeft, tokensLeft
}

// AiFreeQuotaSummary 用户免费额度摘要（handler 组装响应用）
type AiFreeQuotaSummary struct {
	Enabled     bool  `json:"enabled"`
	CallsLeft   int   `json:"calls_left"`   // -1=不限
	TokensLeft  int   `json:"tokens_left"`  // -1=不限
	UsedCalls   int64 `json:"used_calls"`
	UsedTokens  int64 `json:"used_tokens"`
	CallsPerDay int   `json:"calls_per_day"`     // 生效上限（-1 不限）
	MaxTokens   int   `json:"max_tokens_per_day"` // 生效上限（-1 不限）
}

// SummarizeAiFreeQuota 汇总当前用户免费额度状态
func SummarizeAiFreeQuota(userID uint) AiFreeQuotaSummary {
	calls, tokens := EffectiveAiFreeLimit(userID)
	usedCalls, usedTokens := GetTodayAiFreeUsage(userID)
	eligible, callsLeft, tokensLeft := CheckAiFreeQuota(userID, 0)
	s := AiFreeQuotaSummary{
		Enabled:     IsAiFreeQuotaEnabled(),
		CallsLeft:   0, TokensLeft: 0,
		UsedCalls:   usedCalls, UsedTokens: usedTokens,
		CallsPerDay: calls, MaxTokens: tokens,
	}
	if !IsAiFreeQuotaEnabled() || calls == 0 {
		return s
	}
	if eligible {
		s.CallsLeft, s.TokensLeft = callsLeft, tokensLeft
	}
	return s
}

// AiFreeLimitDesc 限额描述（管理端展示）
func AiFreeLimitDesc(calls, tokens int) string {
	cd := "不限"
	if calls == 0 {
		cd = "不提供"
	} else if calls > 0 {
		cd = strconv.Itoa(calls) + " 次/日"
	}
	td := "不限"
	if tokens == 0 {
		td = "不限"
	} else if tokens > 0 {
		td = strconv.Itoa(tokens) + " tokens/日"
	} else {
		td = "不可用"
	}
	return "次数 " + cd + " · tokens " + td
}
