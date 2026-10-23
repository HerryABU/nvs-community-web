package models

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"nvs-server/utils"
)

// ==================== @ai 评论区机器人（§5.6） ====================
// 评论中 @ai（或 @智能体名）召唤 AI 回复：异步生成、以机器人身份发评论、
// 发起人付费（免费额度→代币预检）、每日限流防刷。默认关闭。

const (
	// AiBotEnabledKey 机器人总开关（默认关）
	AiBotEnabledKey = "ai_bot_enabled"
	// AiBotConfigKey 机器人配置 JSON
	AiBotConfigKey = "ai_bot_config"
	// AiBotUsername 机器人用户名（不可登录：随机密码哈希）
	AiBotUsername = "ai_assistant"
	// AiBotNickname 机器人昵称
	AiBotNickname = "AI 助手"
)

// AiBotConfig 机器人配置
type AiBotConfig struct {
	DailyLimit int    `json:"daily_limit"` // 每用户每日召唤次数（0=禁止，-1=不限）
	AgentID    uint   `json:"agent_id"`    // 默认智能体（0=通用写作助手）
	Signature  string `json:"signature"`   // 回复落款（如「—— AI 助手，由 @ 召唤」）
}

// DefaultAiBotConfig 默认：每日 3 次、通用助手
func DefaultAiBotConfig() AiBotConfig {
	return AiBotConfig{DailyLimit: 3, AgentID: 0, Signature: "—— AI 助手"}
}

func normalizeAiBotConfig(cfg AiBotConfig) AiBotConfig {
	if cfg.DailyLimit < -1 {
		cfg.DailyLimit = 0
	}
	if cfg.DailyLimit > 1000 {
		cfg.DailyLimit = 1000
	}
	if len([]rune(cfg.Signature)) > 64 {
		cfg.Signature = string([]rune(cfg.Signature)[:64])
	}
	return cfg
}

// IsAiBotEnabled 机器人是否开启（默认关闭）
func IsAiBotEnabled() bool {
	return GetPlatformConfig(AiBotEnabledKey) == "true"
}

// SetAiBotEnabled 设置机器人开关
func SetAiBotEnabled(on bool) error {
	if on {
		return SetPlatformConfig(AiBotEnabledKey, "true")
	}
	return SetPlatformConfig(AiBotEnabledKey, "false")
}

// GetAiBotConfig 机器人配置
func GetAiBotConfig() AiBotConfig {
	raw := strings.TrimSpace(GetPlatformConfig(AiBotConfigKey))
	if raw == "" {
		return DefaultAiBotConfig()
	}
	var cfg AiBotConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return DefaultAiBotConfig()
	}
	return normalizeAiBotConfig(cfg)
}

// SetAiBotConfig 保存机器人配置
func SetAiBotConfig(cfg AiBotConfig) error {
	cfg = normalizeAiBotConfig(cfg)
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return SetPlatformConfig(AiBotConfigKey, string(raw))
}

// ---------- 机器人用户 ----------

// EnsureAiBotUser 确保机器人用户存在（幂等；返回用户 ID）
func EnsureAiBotUser() (uint, error) {
	var u User
	if err := DB.Where("username = ?", AiBotUsername).First(&u).Error; err == nil {
		return u.ID, nil
	}
	// 随机密码哈希：无人可知，等价于禁止登录
	randBytes := make([]byte, 32)
	if _, err := rand.Read(randBytes); err != nil {
		return 0, err
	}
	hash, err := utils.HashPassword("bot!" + hex.EncodeToString(randBytes))
	if err != nil {
		return 0, err
	}
	bot := &User{
		Username: AiBotUsername, Email: AiBotUsername + "@system.local",
		PasswordHash: hash, Nickname: AiBotNickname, Role: "reader",
	}
	if err := DB.Create(bot).Error; err != nil {
		return 0, err
	}
	return bot.ID, nil
}

// ---------- 召唤限流 ----------

// AiBotUsage 每日召唤用量（user_id + day）
type AiBotUsage struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"uniqueIndex:idx_ai_bot_usage,priority:1;not null" json:"user_id"`
	Day       string    `gorm:"size:10;uniqueIndex:idx_ai_bot_usage,priority:2;not null" json:"day"`
	CallsUsed int       `gorm:"not null;default:0" json:"calls_used"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (AiBotUsage) TableName() string { return "ai_bot_usages" }

// CheckAiBotQuota 今日是否还能召唤（limit：0=禁止，-1=不限）
func CheckAiBotQuota(userID uint, limit int) bool {
	if limit == 0 {
		return false
	}
	if limit < 0 {
		return true
	}
	var u AiBotUsage
	if err := DB.Where("user_id = ? AND day = ?", userID, time.Now().Format("2006-01-02")).
		First(&u).Error; err != nil {
		return true
	}
	return u.CallsUsed < limit
}

// RecordAiBotUsage 记一次召唤
func RecordAiBotUsage(userID uint) {
	day := time.Now().Format("2006-01-02")
	var u AiBotUsage
	if err := DB.Where("user_id = ? AND day = ?", userID, day).First(&u).Error; err != nil {
		_ = DB.Create(&AiBotUsage{UserID: userID, Day: day, CallsUsed: 1, UpdatedAt: time.Now()}).Error
		return
	}
	_ = DB.Model(&AiBotUsage{}).Where("id = ?", u.ID).Updates(map[string]interface{}{
		"calls_used": u.CallsUsed + 1, "updated_at": time.Now(),
	}).Error
}

// CountTodayAiBotUsage 今日已召唤次数（status 展示）
func CountTodayAiBotUsage(userID uint) int64 {
	var u AiBotUsage
	if err := DB.Where("user_id = ? AND day = ?", userID, time.Now().Format("2006-01-02")).
		First(&u).Error; err != nil {
		return 0
	}
	return int64(u.CallsUsed)
}

// ---------- 召唤解析 ----------

// ParseAiBotInvocation 解析评论中的 @ai 召唤。
// 返回 (agentName, prompt, ok)：agentName 为空表示通用助手；
// prompt 为去掉召唤标记后的剩余内容。
func ParseAiBotInvocation(content string) (agentName, prompt string, ok bool) {
	lower := strings.ToLower(content)
	// @ai / @ai助手 / @AI 助手（词首或空白后）
	idx := -1
	for _, marker := range []string{"@ai助手", "@ai 助手", "@ai-assistant", "@ai"} {
		if i := strings.Index(lower, marker); i >= 0 && (idx < 0 || i < idx) {
			idx = i
			if marker == "@ai" {
				// 确认 @ai 不是更长用户名的一部分（如 @aiko）：后随非字母数字
				end := i + len(marker)
				if end < len(lower) {
					ch := lower[end]
					if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' {
						idx = -1
						continue
					}
				}
			}
			prompt = strings.TrimSpace(content[:i] + " " + content[i+len(marker):])
			return "", prompt, true
		}
	}
	// @智能体名（AiAgent.name 精确匹配，如「@写作教练」）
	agents, err := ListAiAgents(true)
	if err == nil {
		for _, a := range agents {
			at := "@" + a.Name
			if i := strings.Index(content, at); i >= 0 {
				prompt = strings.TrimSpace(content[:i] + " " + content[i+len(at):])
				return a.Name, prompt, true
			}
		}
	}
	return "", "", false
}
