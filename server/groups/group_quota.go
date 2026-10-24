package groups

import (
	"nvs-server/models"

	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ==================== 群组额度（建群数 / 人数 / 每日发帖） ====================
// 与「自建论坛额度」「每日发文额度」同一套思路：
//   - 全局配置走 platform_configs.group_quota（JSON），按角色给不同建群上限；
//   - 支持按人覆盖（-1 = 跟随角色/全局；0 = 禁止建群；>0 = 专属上限）；
//   - 人数上限与每日发帖上限是「群维度」的，站长可全局收紧，防止资源被单个群吃满。

const GroupQuotaConfigKey = "group_quota"

// GroupQuotaConfig 全局群组额度
type GroupQuotaConfig struct {
	Enabled     bool           `json:"enabled"`      // 群组功能总开关
	MaxPerUser  int            `json:"max_per_user"` // 兜底：每人建群上限（0=不限）
	Roles       map[string]int `json:"roles"`        // 角色 → 建群上限（0=不限，-1=禁止）
	MemberLimit int            `json:"member_limit"` // 单群人数上限（全局硬顶）
	DailyPosts  int            `json:"daily_posts"`  // 每人每日群组发帖上限（0=不限）
}

// DefaultGroupQuotaConfig 默认：开启，读者 1 个、作者 3 个、VIP 10 个、管理员不限
func DefaultGroupQuotaConfig() GroupQuotaConfig {
	return GroupQuotaConfig{
		Enabled:    true,
		MaxPerUser: 3,
		Roles: map[string]int{
			models.RoleReader:     1,
			models.RoleAuthor:     3,
			models.RoleVipAuthor:  10,
			models.RoleArbitrator: 3,
			models.RolePolice:     20,
			models.RoleAdmin:      0, // 0 = 不限
		},
		MemberLimit: 200,
		DailyPosts:  20,
	}
}

// normalizeGroupQuota 补齐角色表 + 数值下限保护
func normalizeGroupQuota(cfg GroupQuotaConfig) GroupQuotaConfig {
	def := DefaultGroupQuotaConfig()
	if cfg.Roles == nil {
		cfg.Roles = map[string]int{}
	}
	for role, v := range def.Roles {
		if _, ok := cfg.Roles[role]; !ok {
			cfg.Roles[role] = v
		}
	}
	for role, v := range cfg.Roles {
		if v < -1 {
			cfg.Roles[role] = -1
		}
		if v > 1000 {
			cfg.Roles[role] = 1000
		}
	}
	if cfg.MaxPerUser < 0 {
		cfg.MaxPerUser = 0
	}
	if cfg.MaxPerUser > 1000 {
		cfg.MaxPerUser = 1000
	}
	if cfg.MemberLimit < 2 {
		cfg.MemberLimit = 2
	}
	if cfg.MemberLimit > 10000 {
		cfg.MemberLimit = 10000
	}
	if cfg.DailyPosts < 0 {
		cfg.DailyPosts = 0
	}
	if cfg.DailyPosts > 500 {
		cfg.DailyPosts = 500
	}
	return cfg
}

// GetGroupQuotaConfig 读取全局群组额度（未配置返回默认）
func GetGroupQuotaConfig() GroupQuotaConfig {
	raw := strings.TrimSpace(models.GetPlatformConfig(GroupQuotaConfigKey))
	def := DefaultGroupQuotaConfig()
	if raw == "" {
		return def
	}
	var cfg GroupQuotaConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return def
	}
	return normalizeGroupQuota(cfg)
}

// SetGroupQuotaConfig 保存（钳位后落库）
func SetGroupQuotaConfig(cfg GroupQuotaConfig) error {
	cfg = normalizeGroupQuota(cfg)
	b, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return models.SetPlatformConfig(GroupQuotaConfigKey, string(b))
}

// UserGroupQuota 按人覆盖（MaxGroups：-1=跟随角色/全局；0=禁止；>0=专属）
type UserGroupQuota struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;uniqueIndex" json:"user_id"`
	MaxGroups int       `gorm:"not null;default:-1" json:"max_groups"`
	Note      string    `gorm:"size:128;default:''" json:"note"`
	UpdatedAt time.Time `json:"updated_at"`
	// 运行时
	Username string `gorm:"-" json:"username,omitempty"`
}

func (UserGroupQuota) TableName() string { return "user_group_quotas" }

// GetUserGroupQuota 取按人覆盖（无则 nil）
func GetUserGroupQuota(userID uint) *UserGroupQuota {
	var q UserGroupQuota
	if err := models.DB.Where("user_id = ?", userID).First(&q).Error; err != nil {
		return nil
	}
	return &q
}

// SetUserGroupQuota 写按人覆盖（map upsert，避免 -1/0 零值默认坑）
func SetUserGroupQuota(userID uint, maxGroups int, note string) error {
	if maxGroups < -1 {
		maxGroups = -1
	}
	if maxGroups > 1000 {
		maxGroups = 1000
	}
	now := time.Now()
	var n int64
	models.DB.Model(&UserGroupQuota{}).Where("user_id = ?", userID).Count(&n)
	if n > 0 {
		return models.DB.Model(&UserGroupQuota{}).Where("user_id = ?", userID).
			Updates(map[string]interface{}{
				"max_groups": maxGroups, "note": strings.TrimSpace(note), "updated_at": now,
			}).Error
	}
	q := &UserGroupQuota{UserID: userID, MaxGroups: maxGroups, Note: strings.TrimSpace(note), UpdatedAt: now}
	if err := models.DB.Create(q).Error; err != nil {
		return err
	}
	return models.DB.Model(&UserGroupQuota{}).Where("id = ?", q.ID).
		Update("max_groups", maxGroups).Error
}

// DeleteUserGroupQuota 删除按人覆盖
func DeleteUserGroupQuota(userID uint) error {
	return models.DB.Where("user_id = ?", userID).Delete(&UserGroupQuota{}).Error
}

// ListUserGroupQuotas 全部按人覆盖（附用户名）
func ListUserGroupQuotas() ([]UserGroupQuota, error) {
	var list []UserGroupQuota
	if err := models.DB.Order("updated_at DESC").Limit(500).Find(&list).Error; err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return list, nil
	}
	ids := make([]uint, 0, len(list))
	for _, q := range list {
		ids = append(ids, q.UserID)
	}
	var users []models.User
	models.DB.Select("id, username, nickname").Where("id IN ?", ids).Find(&users)
	byID := map[uint]models.User{}
	for _, u := range users {
		byID[u.ID] = u
	}
	for i := range list {
		if u, ok := byID[list[i].UserID]; ok {
			list[i].Username = displayName(u)
		}
	}
	return list, nil
}

// EffectiveGroupQuota 生效额度：maxGroups（-1/0/>0）、memberLimit、dailyPosts
// 优先级：平台管理员不限 > 按人覆盖 > 角色配置 > 全局兜底。
func EffectiveGroupQuota(userID uint, role string) (maxGroups, memberLimit, dailyPosts int) {
	cfg := GetGroupQuotaConfig()
	memberLimit, dailyPosts = cfg.MemberLimit, cfg.DailyPosts
	if role == models.RoleAdmin || role == models.RolePolice {
		return -1, memberLimit, dailyPosts
	}
	if q := GetUserGroupQuota(userID); q != nil && q.MaxGroups != -1 {
		return q.MaxGroups, memberLimit, dailyPosts
	}
	if v, ok := cfg.Roles[role]; ok {
		// 角色表里 0 表示不限 → 对外统一用 -1 表达「不限」
		if v == 0 {
			return -1, memberLimit, dailyPosts
		}
		return v, memberLimit, dailyPosts
	}
	if cfg.MaxPerUser == 0 {
		return -1, memberLimit, dailyPosts
	}
	return cfg.MaxPerUser, memberLimit, dailyPosts
}

// CheckGroupEnabled 群组功能是否开启（站长总开关）
func CheckGroupEnabled() error {
	if !GetGroupQuotaConfig().Enabled {
		return fmt.Errorf("群组功能已被站长关闭")
	}
	return nil
}
