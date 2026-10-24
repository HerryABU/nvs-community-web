package models

import (
	"encoding/json"
	"fmt"
	"time"
)

// ==================== 每日发文限额 / 用户配额体系（需求 §13.4） ====================
//
// 站长后台按「用户等级(role) × 每日上限」配置每日发布数量 + 可选每日发布总字数；
// 支持按人覆盖；管理员可全局调、可按人覆盖。
// AI 辅助生成内容同样计入：在所有发布入口统一 CheckDailyPublish 预检 + RecordPublish 记账，
// 防止 AI 批量生成绕过限额。
//
// 设计纪律（对齐 web_quota）：
//   - 总配置走 platform_configs（JSON），默认关闭（enabled=false），不影响既有站点行为；
//   - 按人覆盖的「0 = 禁止发布 / 0 = 不限字数」是零值，必须用 map 精确 upsert，
//     避免 GORM struct Create 把零值当「未设置」而套用列默认值 -1（静默失效）；
//   - 计数基于 append-only 流水表 user_publish_logs（内容被删也留痕、口径统一）。
//
// 数值语义：
//   count    -1=不限数量；0=禁止发布；>0=每日数量上限
//   word_cap -1=不限字数；0=不限字数；>0=每日总字数上限

const DailyPublishConfigKey = "daily_publish"

// 发布内容类型（计入发文限额的维度）
const (
	PublishKindNovel     = "novel"
	PublishKindChapter   = "chapter"
	PublishKindBlog      = "blog"
	PublishKindThread    = "thread"
	PublishKindGroupPost = "group_post" // 群组发帖（同样计入每日发文额度）
)

// DailyPublishRoleLimit 单个等级的每日限额
type DailyPublishRoleLimit struct {
	Count   int `json:"count"`    // 每日发布数量上限；-1=不限；0=禁止发布
	WordCap int `json:"word_cap"` // 每日发布总字数上限；<=0=不限字数
}

// DailyPublishConfig 每日发文限额总配置
type DailyPublishConfig struct {
	Enabled bool                             `json:"enabled"` // 总开关（默认 false）
	Roles   map[string]DailyPublishRoleLimit `json:"roles"`   // role → 限额
}

// DefaultDailyPublishConfig 默认配置（关闭 + 保守的等级限额，一旦开启即有 sane 值）
func DefaultDailyPublishConfig() DailyPublishConfig {
	return DailyPublishConfig{
		Enabled: false,
		Roles: map[string]DailyPublishRoleLimit{
			RoleReader:     {Count: 5, WordCap: 0},
			RoleAuthor:     {Count: 20, WordCap: 0},
			RoleVipAuthor:  {Count: 50, WordCap: 0},
			RoleArbitrator: {Count: 20, WordCap: 0},
			RolePolice:     {Count: 100, WordCap: 0},
			RoleAdmin:      {Count: -1, WordCap: 0}, // 管理员不限
		},
	}
}

// normalizeDailyPublishConfig 补齐缺失等级 + 数值下限保护
func normalizeDailyPublishConfig(cfg DailyPublishConfig) DailyPublishConfig {
	def := DefaultDailyPublishConfig()
	if cfg.Roles == nil {
		cfg.Roles = map[string]DailyPublishRoleLimit{}
	}
	for role, d := range def.Roles {
		if _, ok := cfg.Roles[role]; !ok {
			cfg.Roles[role] = d
		}
	}
	for role, lim := range cfg.Roles {
		if lim.Count < -1 {
			lim.Count = -1
		}
		if lim.WordCap < 0 {
			lim.WordCap = 0
		}
		cfg.Roles[role] = lim
	}
	return cfg
}

// GetDailyPublishConfig 读取配置（缺省回退默认）
func GetDailyPublishConfig() DailyPublishConfig {
	cfg := DefaultDailyPublishConfig()
	v := GetPlatformConfig(DailyPublishConfigKey)
	if v == "" {
		return cfg
	}
	var parsed DailyPublishConfig
	if err := json.Unmarshal([]byte(v), &parsed); err != nil {
		return cfg
	}
	return normalizeDailyPublishConfig(parsed)
}

// SetDailyPublishConfig 保存配置
func SetDailyPublishConfig(cfg DailyPublishConfig) error {
	cfg = normalizeDailyPublishConfig(cfg)
	b, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return SetPlatformConfig(DailyPublishConfigKey, string(b))
}

// ---------- 按人覆盖 ----------

// UserPublishQuota 管理员对单个用户「每日发文限额」的覆盖（数量 + 字数）。
// count: -1=跟随角色默认；0=禁止发布；>0=专属每日数量上限。
// word_cap: -1=跟随角色默认；0=不限字数；>0=专属每日字数上限。
type UserPublishQuota struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;uniqueIndex" json:"user_id"`
	Count     int       `gorm:"default:-1" json:"count"`
	WordCap   int       `gorm:"default:-1" json:"word_cap"`
	Note      string    `gorm:"size:256;default:''" json:"note"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Username string `gorm:"-" json:"username,omitempty"`
}

func (UserPublishQuota) TableName() string { return "user_publish_quotas" }

// GetUserPublishQuotaOverride 读取某用户的发文限额覆盖（无记录返回 nil = 跟随角色）
func GetUserPublishQuotaOverride(userID uint) *UserPublishQuota {
	if userID == 0 {
		return nil
	}
	var q UserPublishQuota
	if err := DB.Where("user_id = ?", userID).First(&q).Error; err != nil {
		return nil
	}
	return &q
}

// SetUserPublishQuotaOverride 写入/更新某用户的发文限额覆盖（UPSERT，数量 + 字数双维度）。
// ⚠️ 必须用 map 显式 upsert：count 的「0 = 禁止发布」与 word_cap 的「0 = 不限」都是零值，
// 若走 struct Create，GORM 会忽略零值字段而套用列默认值 -1，导致限制静默失效。
func SetUserPublishQuotaOverride(userID uint, count, wordCap int, note string) error {
	if userID == 0 {
		return fmt.Errorf("用户 ID 无效")
	}
	if count < -1 {
		count = -1
	}
	if count > 100000 {
		count = 100000
	}
	if wordCap < -1 {
		wordCap = -1
	}
	if wordCap > 100000000 {
		wordCap = 100000000
	}
	now := time.Now()
	cols := map[string]interface{}{
		"user_id":    userID,
		"count":      count,
		"word_cap":   wordCap,
		"note":       note,
		"updated_at": now,
	}
	if GetUserPublishQuotaOverride(userID) != nil {
		return DB.Model(&UserPublishQuota{}).Where("user_id = ?", userID).Updates(cols).Error
	}
	cols["created_at"] = now
	return DB.Model(&UserPublishQuota{}).Create(cols).Error
}

// DeleteUserPublishQuotaOverride 撤销按人覆盖（恢复跟随角色默认）
func DeleteUserPublishQuotaOverride(userID uint) error {
	return DB.Where("user_id = ?", userID).Delete(&UserPublishQuota{}).Error
}

// ListUserPublishQuotas 列出所有被单独设置过发文限额的用户（附用户名）
func ListUserPublishQuotas() ([]UserPublishQuota, error) {
	var list []UserPublishQuota
	if err := DB.Order("updated_at DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return list, nil
	}
	ids := make([]uint, 0, len(list))
	for _, q := range list {
		ids = append(ids, q.UserID)
	}
	var users []User
	DB.Where("id IN ?", ids).Find(&users)
	names := map[uint]string{}
	for _, u := range users {
		names[u.ID] = u.Username
	}
	for i := range list {
		list[i].Username = names[list[i].UserID]
	}
	return list, nil
}

// EffectivePublishLimit 计算某用户生效的 (countLimit, wordLimit)：角色默认 + 按人覆盖优先
func EffectivePublishLimit(userID uint, role string) (countLimit, wordLimit int) {
	cfg := GetDailyPublishConfig()
	lim, ok := cfg.Roles[role]
	if !ok {
		lim = cfg.Roles[RoleReader] // 未知角色按最保守的 reader 处理
	}
	countLimit, wordLimit = lim.Count, lim.WordCap
	if o := GetUserPublishQuotaOverride(userID); o != nil {
		if o.Count >= 0 {
			countLimit = o.Count
		}
		if o.WordCap >= 0 {
			wordLimit = o.WordCap
		}
	}
	return
}

// ---------- 发布流水 + 当日统计 ----------

// UserPublishLog 发布流水（append-only）：每次成功发布记一条，用于当日计数/字数统计。
type UserPublishLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index:idx_publish_user_day,priority:1" json:"user_id"`
	Kind      string    `gorm:"size:16;not null;index" json:"kind"` // novel/chapter/blog/thread
	WordCount int       `gorm:"default:0" json:"word_count"`
	TargetID  uint      `gorm:"default:0;index" json:"target_id"`
	CreatedAt time.Time `gorm:"index:idx_publish_user_day,priority:2" json:"created_at"`
}

func (UserPublishLog) TableName() string { return "user_publish_logs" }

// startOfToday 当日 0 点（本地时区）
func startOfToday() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

// DailyResetTime 下次额度重置时间（次日 0 点），供前端展示
func DailyResetTime() time.Time {
	return startOfToday().Add(24 * time.Hour)
}

// CountTodayPublish 统计某用户今日已发布数量与总字数
func CountTodayPublish(userID uint) (count int64, words int64) {
	if userID == 0 {
		return 0, 0
	}
	since := startOfToday()
	DB.Model(&UserPublishLog{}).Where("user_id = ? AND created_at >= ?", userID, since).Count(&count)
	DB.Model(&UserPublishLog{}).Where("user_id = ? AND created_at >= ?", userID, since).
		Select("COALESCE(SUM(word_count), 0)").Scan(&words)
	return
}

// RecordPublish 记录一次发布流水（成功发布后调用）
func RecordPublish(userID uint, kind string, targetID uint, wordCount int) error {
	return DB.Create(&UserPublishLog{
		UserID:    userID,
		Kind:      kind,
		TargetID:  targetID,
		WordCount: wordCount,
		CreatedAt: time.Now(),
	}).Error
}

// CheckDailyPublish 预检当日是否允许发布（不记录，仅判断）。
// 返回 allowed + 当日已用/上限（数量与字数）。总开关关闭时直接放行。
func CheckDailyPublish(userID uint, role, kind string, addWords int) (allowed bool, usedCount, countLimit, usedWords, wordLimit int) {
	if !GetDailyPublishConfig().Enabled {
		return true, 0, -1, 0, -1
	}
	countLimit, wordLimit = EffectivePublishLimit(userID, role)
	uc, uw := CountTodayPublish(userID)
	usedCount, usedWords = int(uc), int(uw)
	// 数量：0=禁止；>0 且已达上限即拒
	if countLimit == 0 {
		return false, usedCount, countLimit, usedWords, wordLimit
	}
	if countLimit > 0 && usedCount >= countLimit {
		return false, usedCount, countLimit, usedWords, wordLimit
	}
	// 字数：上限 >0 且本次将超出即拒
	if wordLimit > 0 && usedWords+addWords > wordLimit {
		return false, usedCount, countLimit, usedWords, wordLimit
	}
	return true, usedCount, countLimit, usedWords, wordLimit
}
