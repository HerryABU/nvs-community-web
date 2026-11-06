package vipcard

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"nvs-server/models"
	"nvs-server/readperm"

	"gorm.io/gorm"
)

const cardAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789" // 去除易混淆 0/O/1/I/L

func genCode() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	groups := make([]string, 0, 4)
	for g := 0; g < 4; g++ {
		seg := make([]byte, 4)
		for i := 0; i < 4; i++ {
			seg[i] = cardAlphabet[int(buf[g*4+i])%len(cardAlphabet)]
		}
		groups = append(groups, string(seg))
	}
	return strings.Join(groups, "-")
}

func normalize(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	return strings.NewReplacer("-", "", " ", "", "\t", "", "\n", "", "\r", "").Replace(code)
}

func hashCode(code string) string {
	h := sha256.Sum256([]byte(normalize(code)))
	return hex.EncodeToString(h[:])
}

// Generate 批量生成 VIP 卡密，返回**一次性明文**（之后库里只有哈希）。
// 等级上限由站长在 #65 阅读权限配置里决定（VipMaxLevel）。
func Generate(adminID uint, count, level, days int, expiresAt *time.Time, note string) ([]string, error) {
	if count < 1 || count > 1000 {
		return nil, fmt.Errorf("生成数量需在 1 ~ 1000 之间")
	}
	cfg := readperm.Load()
	if level < 1 || level > cfg.VipMaxLevel {
		return nil, fmt.Errorf("会员等级需在 1 ~ %d 之间（站长设定）", cfg.VipMaxLevel)
	}
	if days < 1 || days > 3650 {
		return nil, fmt.Errorf("会员天数需在 1 ~ 3650 之间")
	}
	batch := "V" + time.Now().Format("20060102150405")
	codes := make([]string, 0, count)
	for i := 0; i < count; i++ {
		code := genCode()
		vc := VipCard{
			CodeHash:   hashCode(code),
			CodePrefix: strings.SplitN(code, "-", 2)[0],
			Level:      level,
			Days:       days,
			Status:     StatusUnused,
			BatchID:    batch,
			CreatedBy:  adminID,
			ExpiresAt:  expiresAt,
			Note:       note,
		}
		if err := models.DB.Create(&vc).Error; err != nil {
			return nil, err
		}
		codes = append(codes, code)
	}
	return codes, nil
}

// RedeemResult 兑换后返回的新会员状态。
type RedeemResult struct {
	Level int       `json:"level"`
	Until time.Time `json:"until"`
	Days  int       `json:"days"`
}

// Redeem 兑换：卡密占用与会员升级在**同一事务**内，任一步失败整体回滚，
// 不会出现「卡密被吃掉但没到账」或「到账了卡密还能再用」。
//
// 等级取 max(现有, 卡等级)，时长从 max(现在, 原到期) 起累加 —— 连兑多张不浪费。
func Redeem(userID uint, code string) (*RedeemResult, error) {
	if userID == 0 {
		return nil, fmt.Errorf("请先登录")
	}
	if strings.TrimSpace(code) == "" {
		return nil, fmt.Errorf("请输入卡密")
	}
	hash := hashCode(code)
	out := &RedeemResult{}

	err := models.DB.Transaction(func(tx *gorm.DB) error {
		var card VipCard
		if err := tx.Where("code_hash = ?", hash).First(&card).Error; err != nil {
			return fmt.Errorf("卡密无效")
		}
		switch card.Status {
		case StatusUsed:
			return fmt.Errorf("卡密已被使用")
		case StatusVoid:
			return fmt.Errorf("卡密已作废")
		}
		if card.ExpiresAt != nil && card.ExpiresAt.Before(time.Now()) {
			return fmt.Errorf("卡密已过期")
		}

		now := time.Now()
		// 原子占用：仅当仍为 unused 时更新，防并发重复兑换
		res := tx.Model(&VipCard{}).
			Where("id = ? AND status = ?", card.ID, StatusUnused).
			Updates(map[string]interface{}{"status": StatusUsed, "used_by": userID, "used_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return fmt.Errorf("卡密已被使用或已作废")
		}

		var u models.User
		if err := tx.Select("id", "member_until", "member_level").
			Where("id = ?", userID).First(&u).Error; err != nil {
			return fmt.Errorf("用户不存在")
		}
		base := now
		if u.MemberUntil != nil && u.MemberUntil.After(base) {
			base = *u.MemberUntil
		}
		until := base.AddDate(0, 0, card.Days)
		level := u.MemberLevel
		if card.Level > level {
			level = card.Level
		}
		if err := tx.Model(&models.User{}).Where("id = ?", userID).
			Updates(map[string]interface{}{"member_until": until, "member_level": level}).Error; err != nil {
			return err
		}
		out.Level, out.Until, out.Days = level, until, card.Days
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// List 管理端列表（可按状态 / 批次筛选），附使用者用户名。
func List(status, batch string, limit int) ([]VipCard, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	q := models.DB.Order("created_at DESC")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if batch != "" {
		q = q.Where("batch_id = ?", batch)
	}
	var cards []VipCard
	if err := q.Limit(limit).Find(&cards).Error; err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(cards))
	for _, c := range cards {
		if c.UsedBy != nil {
			ids = append(ids, *c.UsedBy)
		}
	}
	if len(ids) > 0 {
		var users []models.User
		_ = models.DB.Where("id IN ?", ids).Find(&users).Error
		names := map[uint]string{}
		for _, u := range users {
			names[u.ID] = u.Username
		}
		for i := range cards {
			if cards[i].UsedBy != nil {
				cards[i].UsedByUsername = names[*cards[i].UsedBy]
			}
		}
	}
	return cards, nil
}

// Void 作废一张未使用的卡密。
func Void(id uint) error {
	res := models.DB.Model(&VipCard{}).
		Where("id = ? AND status = ?", id, StatusUnused).
		Update("status", StatusVoid)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("仅可作废未使用的卡密")
	}
	return nil
}

// Membership 当前用户的会员状态（前端「我的会员」展示用）。
func Membership(userID uint) (level int, until *time.Time, err error) {
	if userID == 0 || models.DB == nil {
		return 0, nil, nil
	}
	var u models.User
	if err := models.DB.Select("id", "member_until", "member_level").
		Where("id = ?", userID).First(&u).Error; err != nil {
		return 0, nil, err
	}
	return u.MemberLevel, u.MemberUntil, nil
}
