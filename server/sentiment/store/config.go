package store

import (
	"time"

	"gorm.io/gorm"
)

// configSchemaVersion 与 aisafety 同思路：新增列后老行的零值等于「关掉」，
// 界面看着已保存、实际功能从未启用。版本号变化时做一次性回填。
const configSchemaVersion = 1

// Config 舆情守卫配置（单行，id=1）。全部可调，代码里不留策略字面量。
type Config struct {
	ID uint `gorm:"primaryKey" json:"id"`

	// Enabled 总开关
	Enabled bool `gorm:"not null" json:"enabled"`
	// BlockRisk 达到该话题风险才允许「升级为拦截」（默认 midhigh）
	BlockRisk string `gorm:"size:16" json:"block_risk"`
	// FlagRisk 达到该风险才记录标注（默认 mid）
	FlagRisk string `gorm:"size:16" json:"flag_risk"`
	// FloodBlock 是否自动拦截刷屏 / 出征型行为
	FloodBlock bool `gorm:"not null" json:"flood_block"`

	// ── 每日自动更新 ──
	// LearnFromAI 对内：让 AI 从近期命中语料里产出候选黑话（入库待审或直接生效）
	LearnFromAI bool `gorm:"not null" json:"learn_from_ai"`
	// SearchEnabled 对外：调用上游 /search 拉取最新黑话
	SearchEnabled bool `gorm:"not null" json:"search_enabled"`
	// SearchModelID 用哪个模型的 base_url / api key 去请求 /search（0=自动挑一个启用的对话模型）
	SearchModelID uint `gorm:"not null;default:0" json:"search_model_id"`
	// SearchPath /search 的相对路径（不同网关命名可能不同，故可配）
	SearchPath string `gorm:"size:64" json:"search_path"`
	// SearchQuery 搜索词模板，{date} 会被替换为当天日期
	SearchQuery string `gorm:"size:255" json:"search_query"`
	// AutoUpdateHour 每天几点跑（0-23，服务器本地时区）
	AutoUpdateHour int `gorm:"not null;default:4" json:"auto_update_hour"`
	// DailyQuota 每天最多新增多少条词（防止词表爆炸）
	DailyQuota int `gorm:"not null;default:50" json:"daily_quota"`
	// RequireReview 自动学来的词是否先入 pending 等站长审
	RequireReview bool `gorm:"not null" json:"require_review"`

	// MinConfidence 标签置信度门槛：低于它的话题标注**只能标注、不能拦截**。
	// 标签是拦截策略的开关，AI 打标不可能 100% 准，低置信度必须转人工。
	MinConfidence int `gorm:"not null;default:60" json:"min_confidence"`

	// FoldChars 评论区长文本自动折叠阈值（字符数，0=不折叠）
	FoldChars int `gorm:"not null;default:400" json:"fold_chars"`

	SchemaVersion int       `gorm:"not null;default:0" json:"schema_version"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (Config) TableName() string { return "sentiment_configs" }

func defaultConfig() Config {
	return Config{
		ID: 1, Enabled: true,
		BlockRisk: "midhigh", FlagRisk: "mid",
		FloodBlock: true,
		// 自动更新默认关闭：它会产生对外请求，必须由站长显式开启
		LearnFromAI: false, SearchEnabled: false,
		SearchPath: "/search", SearchQuery: "网络 最新 黑话 缩写 {date}",
		AutoUpdateHour: 4, DailyQuota: 50, RequireReview: true,
		MinConfidence: 60,
		FoldChars:     400,
	}
}

// GetConfig 读配置；无行则按默认种子化。
func GetConfig() (*Config, error) {
	var c Config
	err := DB.Where("id = ?", 1).First(&c).Error
	if err == nil {
		return &c, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	seed := defaultConfig()
	seed.UpdatedAt = time.Now()
	seed.SchemaVersion = configSchemaVersion
	if e := DB.Create(&seed).Error; e != nil {
		return &seed, e
	}
	// Create 会跳过零值字段（GORM 行为），用显式 Updates 写回布尔/数值默认值
	DB.Model(&Config{}).Where("id = ?", 1).Updates(map[string]interface{}{
		"enabled":          seed.Enabled,
		"flood_block":      seed.FloodBlock,
		"learn_from_ai":    seed.LearnFromAI,
		"search_enabled":   seed.SearchEnabled,
		"require_review":   seed.RequireReview,
		"auto_update_hour": seed.AutoUpdateHour,
		"daily_quota":      seed.DailyQuota,
		"min_confidence":   seed.MinConfidence,
		"fold_chars":       seed.FoldChars,
		"schema_version":   seed.SchemaVersion,
	})
	return &seed, nil
}

// SaveConfig 保存配置。整条透传，避免「保存一次把没展示的字段抹掉」。
func (c *Config) Save() error {
	c.UpdatedAt = time.Now()
	c.SchemaVersion = configSchemaVersion
	fields := map[string]interface{}{
		"enabled":          c.Enabled,
		"block_risk":       c.BlockRisk,
		"flag_risk":        c.FlagRisk,
		"flood_block":      c.FloodBlock,
		"learn_from_ai":    c.LearnFromAI,
		"search_enabled":   c.SearchEnabled,
		"search_model_id":  c.SearchModelID,
		"search_path":      c.SearchPath,
		"search_query":     c.SearchQuery,
		"auto_update_hour": c.AutoUpdateHour,
		"daily_quota":      c.DailyQuota,
		"require_review":   c.RequireReview,
		"min_confidence":   c.MinConfidence,
		"fold_chars":       c.FoldChars,
		"schema_version":   c.SchemaVersion,
		"updated_at":       c.UpdatedAt,
	}
	return DB.Model(&Config{}).Where("id = ?", 1).Updates(fields).Error
}
