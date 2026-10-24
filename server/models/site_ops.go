package models

// ==================== 运营位：画廊轮播 / 每日签到 / 访客设置（gap-audit #23）====================
// 这三样都是「运营侧非主线」功能，但缺了站长就没法做活动。设计上刻意做薄：
//   - 轮播：只存图 + 跳转 + 时段 + 排序，不做「智能投放」；
//   - 签到：只做「每天一次 + 连签加成」，奖励进代币账本（复用 AppendTokenLedger，不新开账）；
//   - 访客：只做「未登录能看什么」的开关，不做复杂权限矩阵。
//
// 铁律：奖励发放必须**幂等**（同一天重复请求只发一次），否则就是一个无限刷币漏洞。

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ─────────────── 轮播 ───────────────

// Carousel 首页轮播项
type Carousel struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	Title     string     `gorm:"size:64;not null" json:"title"`
	ImageURL  string     `gorm:"size:512;not null" json:"image_url"`
	LinkURL   string     `gorm:"size:512;default:''" json:"link_url"`
	Target    string     `gorm:"size:8;not null;default:blank" json:"target"`
	SortOrder int        `gorm:"not null;default:0" json:"sort_order"`
	Enabled   bool       `gorm:"not null;default:true" json:"enabled"`
	StartAt   *time.Time `json:"start_at"`
	EndAt     *time.Time `json:"end_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func (Carousel) TableName() string { return "carousels" }

// ActiveCarousels 前台取当前有效的轮播（后端过滤时段，避免前端信任问题）
func ActiveCarousels() ([]Carousel, error) {
	now := time.Now()
	var list []Carousel
	err := DB.Where("enabled = ?", true).
		Where("(start_at IS NULL OR start_at <= ?) AND (end_at IS NULL OR end_at >= ?)", now, now).
		Order("sort_order ASC, id DESC").Limit(10).Find(&list).Error
	return list, err
}

// SaveCarousel 新增/更新
func SaveCarousel(in Carousel) (*Carousel, error) {
	if strings.TrimSpace(in.Title) == "" {
		return nil, errors.New("标题不能为空")
	}
	if _, _, err := NormalizeNavURL(in.ImageURL, "self"); err != nil {
		return nil, errors.New("图片地址非法：" + err.Error())
	}
	if strings.TrimSpace(in.LinkURL) != "" {
		u, t, err := NormalizeNavURL(in.LinkURL, in.Target)
		if err != nil {
			return nil, errors.New("跳转地址非法：" + err.Error())
		}
		in.LinkURL, in.Target = u, t
	} else {
		in.Target = "self"
	}
	in.UpdatedAt = time.Now()
	if in.ID == 0 {
		wantEnabled := in.Enabled // 同上：Create 会用列默认值回填，先存意图值
		in.CreatedAt = time.Now()
		if err := DB.Create(&in).Error; err != nil {
			return nil, err
		}
		_ = DB.Model(&Carousel{}).Where("id = ?", in.ID).
			Update("enabled", wantEnabled).Error
		in.Enabled = wantEnabled
		return &in, nil
	}
	if err := DB.Model(&Carousel{}).Where("id = ?", in.ID).Updates(map[string]interface{}{
		"title": in.Title, "image_url": in.ImageURL, "link_url": in.LinkURL,
		"target": in.Target, "sort_order": in.SortOrder, "enabled": in.Enabled,
		"start_at": in.StartAt, "end_at": in.EndAt, "updated_at": in.UpdatedAt,
	}).Error; err != nil {
		return nil, err
	}
	return &in, nil
}

// DeleteCarousel 删除
func DeleteCarousel(id uint) error {
	return DB.Where("id = ?", id).Delete(&Carousel{}).Error
}

// AdminCarousels 管理端全量
func AdminCarousels() ([]Carousel, error) {
	var list []Carousel
	err := DB.Order("sort_order ASC, id DESC").Find(&list).Error
	return list, err
}

// ─────────────── 签到 ───────────────

// CheckInConfig 签到配置（platform_configs.checkin）
type CheckInConfig struct {
	Enabled     bool      `json:"enabled"`
	BaseReward  int       `json:"base_reward"`  // 每日基础奖励（代币）
	StreakBonus int       `json:"streak_bonus"` // 每连签一天额外加的（线性，封顶 MaxStreak）
	MaxStreak   int       `json:"max_streak"`   // 连签加成封顶天数
	RequirePost bool      `json:"require_post"` // 当天有发帖才可签（抑制纯薅）
	UpdatedAt   time.Time `json:"updated_at"`
}

const checkInCfgKey = "checkin"

// DefaultCheckInConfig 默认关闭（避免升级后凭空多出代币支出）
func DefaultCheckInConfig() CheckInConfig {
	return CheckInConfig{Enabled: false, BaseReward: 1, StreakBonus: 1, MaxStreak: 7}
}

// LoadCheckInConfig 读取
func LoadCheckInConfig() CheckInConfig {
	c := DefaultCheckInConfig()
	raw := GetPlatformConfig(checkInCfgKey)
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			return DefaultCheckInConfig()
		}
	}
	if c.BaseReward < 0 {
		c.BaseReward = 0
	}
	if c.StreakBonus < 0 {
		c.StreakBonus = 0
	}
	if c.MaxStreak <= 0 {
		c.MaxStreak = 7
	}
	return c
}

// SaveCheckInConfig 保存
func SaveCheckInConfig(c CheckInConfig) error {
	c.UpdatedAt = time.Now()
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return SetPlatformConfig(checkInCfgKey, string(b))
}

// CheckInRecord 签到记录（UserID + Date 唯一，天然幂等）
type CheckInRecord struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;uniqueIndex:uk_checkin_user_date" json:"user_id"`
	Date      string    `gorm:"size:10;not null;uniqueIndex:uk_checkin_user_date" json:"date"` // YYYY-MM-DD
	Streak    int       `gorm:"not null;default:1" json:"streak"`
	Reward    int       `gorm:"not null;default:0" json:"reward"`
	CreatedAt time.Time `json:"created_at"`
}

func (CheckInRecord) TableName() string { return "checkin_records" }

// CheckInResult 签到结果
type CheckInResult struct {
	Already bool   `json:"already"`
	Reward  int    `json:"reward"`
	Streak  int    `json:"streak"`
	Balance int64  `json:"balance"`
	Date    string `json:"date"`
}

// DoCheckIn 执行签到（幂等：同一天重复调用返回 Already=true，不再发奖）
func DoCheckIn(userID uint) (*CheckInResult, error) {
	cfg := LoadCheckInConfig()
	if !cfg.Enabled {
		return nil, errors.New("签到未开启")
	}
	if userID == 0 {
		return nil, errors.New("请先登录")
	}
	today := time.Now().Format("2006-01-02")
	var exist CheckInRecord
	if err := DB.Where("user_id = ? AND date = ?", userID, today).First(&exist).Error; err == nil {
		return &CheckInResult{Already: true, Reward: exist.Reward, Streak: exist.Streak,
			Balance: GetTokenBalance(userID), Date: today}, nil
	}
	// 连签：昨天有记录则 +1，否则从 1 开始
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	streak := 1
	var prev CheckInRecord
	if err := DB.Where("user_id = ? AND date = ?", userID, yesterday).First(&prev).Error; err == nil {
		streak = prev.Streak + 1
	}
	reward := cfg.BaseReward
	bonusDays := streak
	if bonusDays > cfg.MaxStreak {
		bonusDays = cfg.MaxStreak
	}
	if bonusDays > 1 {
		reward += cfg.StreakBonus * (bonusDays - 1)
	}
	// 先落记录（唯一索引兜底并发重复），再发币；发币失败则回滚删除
	rec := &CheckInRecord{UserID: userID, Date: today, Streak: streak, Reward: reward, CreatedAt: time.Now()}
	if err := DB.Create(rec).Error; err != nil {
		// 并发下被另一请求抢先：当作已签到
		return &CheckInResult{Already: true, Balance: GetTokenBalance(userID), Date: today}, nil
	}
	var balance int64
	if reward > 0 && IsTokenEnabled() {
		b, err := AppendTokenLedger(userID, "checkin", int64(reward), "checkin", rec.ID,
			"每日签到（连签 "+strconv.Itoa(streak)+" 天）", "")
		if err != nil {
			_ = DB.Where("id = ?", rec.ID).Delete(&CheckInRecord{}).Error
			return nil, err
		}
		balance = b
	} else {
		balance = GetTokenBalance(userID)
	}
	return &CheckInResult{Reward: reward, Streak: streak, Balance: balance, Date: today}, nil
}

// CheckInStatus 今日签到状态
func CheckInStatus(userID uint) map[string]interface{} {
	cfg := LoadCheckInConfig()
	today := time.Now().Format("2006-01-02")
	out := map[string]interface{}{"enabled": cfg.Enabled, "date": today, "checked": false, "streak": 0, "config": cfg}
	if userID == 0 {
		return out
	}
	var rec CheckInRecord
	if err := DB.Where("user_id = ? AND date = ?", userID, today).First(&rec).Error; err == nil {
		out["checked"] = true
		out["streak"] = rec.Streak
		out["reward"] = rec.Reward
	}
	var total int64
	_ = DB.Model(&CheckInRecord{}).Where("user_id = ?", userID).Count(&total).Error
	out["total_days"] = total
	return out
}

// CheckInStats 管理端：近 7 天签到人数
func CheckInStats() []map[string]interface{} {
	out := make([]map[string]interface{}, 0, 7)
	for i := 6; i >= 0; i-- {
		d := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		var n int64
		_ = DB.Model(&CheckInRecord{}).Where("date = ?", d).Count(&n).Error
		var sum int64
		_ = DB.Model(&CheckInRecord{}).Where("date = ?", d).Select("COALESCE(SUM(reward),0)").Scan(&sum).Error
		out = append(out, map[string]interface{}{"date": d, "users": n, "reward": sum})
	}
	return out
}

// ─────────────── 访客设置 ───────────────

// VisitSetting 未登录访客能做什么（platform_configs.visit_setting）
type VisitSetting struct {
	AllowGuestBrowse  bool      `json:"allow_guest_browse"`   // 未登录可浏览作品/博客/论坛
	AllowGuestSearch  bool      `json:"allow_guest_search"`   // 未登录可搜索
	AllowGuestReadAll bool      `json:"allow_guest_read_all"` // 未登录可读付费/会员内容（默认否）
	GuestMaxChapters  int       `json:"guest_max_chapters"`   // 未登录最多看几章（0=不限）
	ShowVisitCount    bool      `json:"show_visit_count"`     // 前台展示访问量
	BlockGuestPost    bool      `json:"block_guest_post"`     // 未登录禁止一切写操作（默认是）
	NoticeText        string    `json:"notice_text"`          // 访客提示文案
	UpdatedAt         time.Time `json:"updated_at"`
}

const visitCfgKey = "visit_setting"

// DefaultVisitSetting 默认：允许浏览、10 章限制、允许搜索
func DefaultVisitSetting() VisitSetting {
	return VisitSetting{
		AllowGuestBrowse: true, AllowGuestSearch: true, GuestMaxChapters: 10,
		ShowVisitCount: true, BlockGuestPost: true,
		NoticeText: "登录后可收藏、评论与阅读全部章节",
	}
}

// LoadVisitSetting 读取
func LoadVisitSetting() VisitSetting {
	c := DefaultVisitSetting()
	raw := GetPlatformConfig(visitCfgKey)
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			return DefaultVisitSetting()
		}
	}
	if c.GuestMaxChapters < 0 {
		c.GuestMaxChapters = 0
	}
	return c
}

// SaveVisitSetting 保存
func SaveVisitSetting(c VisitSetting) error {
	c.UpdatedAt = time.Now()
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return SetPlatformConfig(visitCfgKey, string(b))
}

// VisitStats 近 N 天访问量（复用 web_access_logs：UV 按 IP 去重）
func VisitStats(days int) []map[string]interface{} {
	if days <= 0 || days > 30 {
		days = 7
	}
	out := make([]map[string]interface{}, 0, days)
	for i := days - 1; i >= 0; i-- {
		d := time.Now().AddDate(0, 0, -i)
		start := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location())
		end := start.AddDate(0, 0, 1)
		var pv, uv int64
		_ = DB.Raw("SELECT COUNT(1) FROM web_access_logs WHERE created_at >= ? AND created_at < ?", start, end).
			Scan(&pv).Error
		_ = DB.Raw("SELECT COUNT(DISTINCT ip) FROM web_access_logs WHERE created_at >= ? AND created_at < ?", start, end).
			Scan(&uv).Error
		out = append(out, map[string]interface{}{"date": start.Format("2006-01-02"), "pv": pv, "uv": uv})
	}
	return out
}

// MigrateSiteOps 建表
func MigrateSiteOps(db *gorm.DB) error {
	return db.AutoMigrate(&Carousel{}, &CheckInRecord{})
}
