package aisafety

import (
	"errors"
	"math"
	"time"

	"gorm.io/gorm"
	"nvs-server/models"
)

// GetProfile 读取用户画像，不存在则建。
func GetProfile(userID uint, username string) *SafetyProfile {
	if userID == 0 {
		return &SafetyProfile{}
	}
	var p SafetyProfile
	err := models.DB.Where("user_id = ?", userID).First(&p).Error
	if err == nil {
		return &p
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return &p
	}
	p = SafetyProfile{UserID: userID, Username: username, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	// 全零值会被 GORM 的 default 标签吃掉，用显式 map 写一遍保证落库
	models.DB.Create(&p)
	models.DB.Model(&SafetyProfile{}).Where("user_id = ?", userID).Updates(map[string]interface{}{
		"risk_score": 0, "block_count": 0, "hit_count": 0,
	})
	return &p
}

// DecayedScore 按半衰期衰减后的风险分。
//
// 不衰减的话「一次误判 → 永久高风险」会把人钉死；
// 衰减太快又等于没有记忆，所以用可配置的半衰期（默认 72 小时减半）。
func (p *SafetyProfile) DecayedScore(cfg *SafetyConfig, now time.Time) int {
	if p.RiskScore <= 0 {
		return 0
	}
	if cfg.ScoreDecayHours <= 0 {
		return p.RiskScore
	}
	base := p.UpdatedAt
	if p.LastHitAt != nil {
		base = *p.LastHitAt
	}
	if base.IsZero() {
		return p.RiskScore
	}
	hours := now.Sub(base).Hours()
	if hours <= 0 {
		return p.RiskScore
	}
	v := float64(p.RiskScore) * math.Pow(0.5, hours/float64(cfg.ScoreDecayHours))
	if v < 1 {
		return 0
	}
	return int(v)
}

// InWatch 是否处于观察期。
func (p *SafetyProfile) InWatch(now time.Time) bool {
	return p.WatchUntil != nil && now.Before(*p.WatchUntil)
}

// InCooldown 是否处于冷却期（AI 功能暂停）。
func (p *SafetyProfile) InCooldown(now time.Time) bool {
	return p.CooldownUntil != nil && now.Before(*p.CooldownUntil)
}

// RecordHit 记录一次命中，并按用户特征决定是否升级为观察期 / 冷却。
//
// 升级链：命中 → （够次数）观察期 → （观察期内再命中）冷却。
// 这样「换个说法一直试」的账号会自己撞墙，而偶发误判只停在观察期（阈值收紧，不至于封功能）。
func RecordHit(req HitRequest) (*SafetyProfile, error) {
	cfg := req.Config
	if cfg == nil {
		c, err := GetConfig()
		if err != nil {
			return nil, err
		}
		cfg = c
	}
	now := time.Now()
	p := GetProfile(req.UserID, req.Username)

	action := req.Action
	if action == "" {
		action = ActionBlock
	}
	// 只有「真的拦下了」才算命中次数。
	//
	// 三类不计入：
	//   review   仅记录（可疑但放行）
	//   released 被蓝队复核推翻、已放行
	//   out      输出侧拦截 —— 这是「模型自己吐了」，当轮用户输入可能完全正常
	//            （比如贴了一份含配方的 PDF 让它总结）。算成用户命中会误伤正常人，
	//            而用户特征本应衡量「这个账号在索取什么」，不是「模型说了什么」。
	countsAsHit := action != ActionReview && action != ActionReleased && req.Direction != DirOut
	if countsAsHit {
		p.RiskScore = p.DecayedScore(cfg, now) + req.Score/2
		p.BlockCount++
		if p.InWatch(now) {
			p.HitCount++
		} else {
			p.HitCount = 1
		}
	} else {
		p.RiskScore = p.DecayedScore(cfg, now) + req.Score/10
	}
	p.LastHitAt = &now

	// 观察期内命中达到阈值 → 冷却（仅对真实拦截生效）
	if countsAsHit && p.InWatch(now) && cfg.CooldownHits > 0 && p.HitCount >= cfg.CooldownHits {
		until := now.Add(time.Duration(cfg.CooldownMinutes) * time.Minute)
		p.CooldownUntil = &until
		action = ActionCooldown
	} else if countsAsHit && cfg.WatchAfterHits > 0 && p.BlockCount >= cfg.WatchAfterHits && !p.InWatch(now) {
		until := now.Add(time.Duration(cfg.WatchHours) * time.Hour)
		p.WatchUntil = &until
		if action == ActionBlock {
			action = ActionWatch
		}
	}

	err := models.DB.Model(&SafetyProfile{}).Where("user_id = ?", req.UserID).Updates(map[string]interface{}{
		"username":       req.Username,
		"risk_score":     p.RiskScore,
		"block_count":    p.BlockCount,
		"hit_count":      p.HitCount,
		"watch_until":    p.WatchUntil,
		"cooldown_until": p.CooldownUntil,
		"last_hit_at":    p.LastHitAt,
		"updated_at":     now,
	}).Error
	if err != nil {
		return p, err
	}

	// 事件留痕（失败不影响主流程，但要看得见）
	if req.UserID > 0 {
		ev := &SafetyEvent{
			UserID: req.UserID, Username: req.Username,
			Direction: req.Direction, Action: action, Level: req.Level, Score: req.Score,
			Categories: req.Categories, Hits: req.Hits,
			ModelID: req.ModelID, SessionID: req.SessionID, CreatedAt: now,
		}
		if cfg.RecordExcerpt {
			ev.Excerpt = TruncateRunes(req.Excerpt, cfg.ExcerptChars)
		}
		models.DB.Create(ev)
	}
	return p, nil
}

// HitRequest RecordHit 的入参。
type HitRequest struct {
	Config     *SafetyConfig
	UserID     uint
	Username   string
	Direction  string
	Action     string
	Level      string
	Score      int
	Categories string
	Hits       string
	Excerpt    string
	ModelID    uint
	SessionID  uint
}

// ClearCooldown 后台手动解除冷却（连同观察期一起清，避免「解了还在紧线」）。
func ClearCooldown(userID uint) error {
	return models.DB.Model(&SafetyProfile{}).Where("user_id = ?", userID).Updates(map[string]interface{}{
		"cooldown_until": nil,
		"watch_until":    nil,
		"hit_count":      0,
		"updated_at":     time.Now(),
	}).Error
}

// ResetProfile 后台彻底重置画像（误判申诉时用）。
func ResetProfile(userID uint) error {
	return models.DB.Model(&SafetyProfile{}).Where("user_id = ?", userID).Updates(map[string]interface{}{
		"cooldown_until": nil,
		"watch_until":    nil,
		"hit_count":      0,
		"block_count":    0,
		"risk_score":     0,
		"updated_at":     time.Now(),
	}).Error
}

// TruncateRunes 按字符（非字节）截断，避免中文被切坏。
func TruncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
