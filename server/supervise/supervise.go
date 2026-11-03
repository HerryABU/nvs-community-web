// Package supervise 以疏代堵的「名单」子系统（v4.8.0 新增）。
//
// 背景：AI 安全闸原来只有「观察期 / 冷却」这类**功能性节流**，
// 没有一张**治理侧可见、可升级、可对接警方**的名单。
// 绥靖模式（TerminateAppease）下，用户完全无感，但其行为被静默记入「可疑名单」；
// 一旦恶意明显（监管模型 B 判定分越线或命中确证信号），升级到「恶意名单」，
// 并在用户面前展示引导内容（以疏代堵，不弹硬拦截）。
//
// 设计取舍（与 risk / police 包同口径）：
//   - 名单只存**引用与结论**（用户画像摘要、触发原因、证据引用），不存内容本体，
//     证据本体在各模块按原 TTL 过期，避免名单变成「永久证据仓」；
//   - 升级单向：suspect → malicious，不允许 Upsert 把 malicious 降回 suspect
//     （降级必须走管理员人工 Clear，留痕）；
//   - 所有人工操作（升级 / 清除 / 改档 / 加注）写 SuperviseLog，可审计、可复盘。
package supervise

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"nvs-server/models"

	"gorm.io/gorm"
)

// 名单档位
const (
	TierSuspect   = "suspect"   // 可疑名单（绥靖静默入档）
	TierMalicious = "malicious" // 恶意名单（确证/升级）
)

// 状态
const (
	StatusActive  = "active"  // 在档
	StatusCleared = "cleared" // 已清除（管理员人工）
)

// 来源（谁把人放进名单）
const (
	SrcChain  = "chain"  // 跨轮拼装链检测
	SrcJudge  = "judge"  // 监管模型 B 判定
	SrcWatch  = "watch"  // 观察期命中
	SrcRule   = "rule"   // 规则命中
	SrcManual = "manual" // 管理员手动
	SrcAppeal = "appeal" // 申诉复核后入档
)

// SupervisedUser 被监管用户（每人最多一行）。
type SupervisedUser struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	UserID       uint       `gorm:"uniqueIndex;not null" json:"user_id"`
	Username     string     `gorm:"size:64" json:"username"`
	Tier         string     `gorm:"size:16;not null;default:suspect;index" json:"tier"`
	Score        int        `json:"score"` // 累积风险分（只增不减，除非 Clear 清零）
	Source       string     `gorm:"size:24;not null;default:manual;index" json:"source"`
	Reasons      string     `gorm:"type:text" json:"reasons"`       // JSON 数组：触发原因
	EvidenceRefs string     `gorm:"type:text" json:"evidence_refs"` // JSON 数组：{type,ref,note}
	Status       string     `gorm:"size:16;not null;default:active;index" json:"status"`
	HandledBy    uint       `gorm:"not null;default:0" json:"handled_by"`
	HandledAt    *time.Time `json:"handled_at"`
	Note         string     `gorm:"size:512;default:''" json:"note"`
	FirstSeen    time.Time  `json:"first_seen"`
	LastEvent    time.Time  `json:"last_event"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (SupervisedUser) TableName() string { return "supervised_users" }

// SuperviseLog 名单操作日志（升级 / 清除 / 改档 / 加注）。
type SuperviseLog struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	SupervisedID uint      `gorm:"not null;index" json:"supervised_id"`
	UserID       uint      `gorm:"not null;index" json:"user_id"`
	ActorID      uint      `gorm:"not null" json:"actor_id"`
	Action       string    `gorm:"size:24;not null" json:"action"` // upsert/escalate/clear/set_tier/note
	FromTo       string    `gorm:"size:64;default:''" json:"from_to"`
	Note         string    `gorm:"size:1024;default:''" json:"note"`
	CreatedAt    time.Time `gorm:"index" json:"created_at"`

	ActorName string `gorm:"-" json:"actor_name,omitempty"`
}

func (SuperviseLog) TableName() string { return "supervise_logs" }

// EvidenceRef 证据引用（不存内容本体）。
type EvidenceRef struct {
	Type string `json:"type"` // ai_event / honeypot / post / session / ticket
	Ref  string `json:"ref"`  // 对应记录 ID 或定位串
	Note string `json:"note,omitempty"`
}

// Upsert 入档 / 追加。已存在则：风险分累加、追加原因、刷新 LastEvent；
// 档位**只升不降**（suspect → malicious），降级必须走 Clear / SetTier 人工留痕。
//
// scoreDelta 本次新增风险分（可为 0）；reason 本次触发原因（空则忽略）；
// ref 本次证据引用（空则忽略）。
func Upsert(userID uint, username, tier, source string, scoreDelta int, reason string, ref *EvidenceRef) (*SupervisedUser, error) {
	if userID == 0 {
		return nil, errors.New("用户 ID 非法")
	}
	tier = normalizeTier(tier)
	source = normalizeSource(source)

	var su SupervisedUser
	err := models.DB.Where("user_id = ?", userID).First(&su).Error
	now := time.Now()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		su = SupervisedUser{
			UserID:    userID,
			Username:  username,
			Tier:      tier,
			Score:     scoreDelta,
			Source:    source,
			Status:    StatusActive,
			FirstSeen: now,
			LastEvent: now,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if r := buildReasons(nil, reason); r != "" {
			su.Reasons = r
		}
		if rf := buildRefs(nil, ref); rf != "" {
			su.EvidenceRefs = rf
		}
		if e := models.DB.Create(&su).Error; e != nil {
			return nil, e
		}
		_ = addLog(&su, 0, "upsert", "", fmt.Sprintf("入档(%s) 来源=%s 分=%d", tier, source, scoreDelta))
		return &su, nil
	}
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{"updated_at": now, "last_event": now, "status": StatusActive}
	if su.Status == StatusCleared {
		// 已被清除的重新触发：恢复在档，但保留历史原因
		updates["status"] = StatusActive
	}
	if scoreDelta != 0 {
		newScore := su.Score + scoreDelta
		if newScore < 0 {
			newScore = 0
		}
		updates["score"] = newScore
	}
	// 档位只升不降
	if tier == TierMalicious && su.Tier != TierMalicious {
		updates["tier"] = TierMalicious
		_ = addLog(&su, 0, "escalate", su.Tier+"→"+TierMalicious, fmt.Sprintf("自动升级 来源=%s", source))
	}
	if strings.TrimSpace(reason) != "" {
		updates["reasons"] = buildReasons(&su, reason)
	}
	if ref != nil {
		updates["evidence_refs"] = buildRefs(&su, ref)
	}
	if err := models.DB.Model(&SupervisedUser{}).Where("id = ?", su.ID).Updates(updates).Error; err != nil {
		return nil, err
	}
	// 重新读取以返回最新
	_ = models.DB.Where("id = ?", su.ID).First(&su).Error
	return &su, nil
}

// Escalate 直接升级到恶意名单（判定分越线时调用）。
func Escalate(userID uint, reason string) (*SupervisedUser, error) {
	var su SupervisedUser
	if err := models.DB.Where("user_id = ?", userID).First(&su).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Upsert(userID, "", TierMalicious, SrcJudge, 0, reason, nil)
		}
		return nil, err
	}
	if su.Tier == TierMalicious {
		return &su, nil
	}
	now := time.Now()
	updates := map[string]interface{}{
		"tier": TierMalicious, "updated_at": now, "last_event": now, "status": StatusActive,
	}
	if strings.TrimSpace(reason) != "" {
		updates["reasons"] = buildReasons(&su, reason)
	}
	if err := models.DB.Model(&SupervisedUser{}).Where("id = ?", su.ID).Updates(updates).Error; err != nil {
		return nil, err
	}
	_ = addLog(&su, 0, "escalate", su.Tier+"→"+TierMalicious, reason)
	_ = models.DB.Where("id = ?", su.ID).First(&su).Error
	return &su, nil
}

// List 列表（按档位 / 状态筛）。
func List(page, size int, tier, status string) ([]SupervisedUser, int64, error) {
	if page < 1 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 20
	}
	q := models.DB.Model(&SupervisedUser{})
	if tier != "" {
		q = q.Where("tier = ?", tier)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	q.Count(&total)
	var list []SupervisedUser
	if err := q.Order("CASE tier WHEN 'malicious' THEN 0 ELSE 1 END, score DESC, id DESC").
		Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// Get 详情 + 日志。
func Get(id uint) (*SupervisedUser, []SuperviseLog, error) {
	var su SupervisedUser
	if err := models.DB.Where("id = ?", id).First(&su).Error; err != nil {
		return nil, nil, err
	}
	var logs []SuperviseLog
	_ = models.DB.Where("supervised_id = ?", id).Order("id ASC").Find(&logs).Error
	return &su, logs, nil
}

// GetByUser 按用户取在档记录。
func GetByUser(userID uint) (*SupervisedUser, error) {
	var su SupervisedUser
	if err := models.DB.Where("user_id = ?", userID).First(&su).Error; err != nil {
		return nil, err
	}
	return &su, nil
}

// SetTier 管理员手动改档（含降级，必须留痕）。
func SetTier(id, operatorID uint, tier, note string) error {
	var su SupervisedUser
	if err := models.DB.Where("id = ?", id).First(&su).Error; err != nil {
		return err
	}
	tier = normalizeTier(tier)
	if tier == su.Tier {
		return nil
	}
	fromTo := su.Tier + "→" + tier
	now := time.Now()
	if err := models.DB.Model(&SupervisedUser{}).Where("id = ?", id).Updates(map[string]interface{}{
		"tier": tier, "updated_at": now, "last_event": now, "status": StatusActive,
	}).Error; err != nil {
		return err
	}
	return addLog(&su, operatorID, "set_tier", fromTo, note)
}

// Clear 管理员清除（不在档，但保留历史原因与日志）。
func Clear(id, operatorID uint, note string) error {
	var su SupervisedUser
	if err := models.DB.Where("id = ?", id).First(&su).Error; err != nil {
		return err
	}
	if su.Status == StatusCleared {
		return nil
	}
	now := nowPtr()
	if err := models.DB.Model(&SupervisedUser{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status": StatusCleared, "handled_by": operatorID, "handled_at": now, "updated_at": now,
	}).Error; err != nil {
		return err
	}
	return addLog(&su, operatorID, "clear", su.Status+"→"+StatusCleared, note)
}

// AddNote 加注（不改档位 / 状态）。
func AddNote(id, operatorID uint, note string) error {
	var su SupervisedUser
	if err := models.DB.Where("id = ?", id).First(&su).Error; err != nil {
		return err
	}
	if strings.TrimSpace(note) == "" {
		return errors.New("内容不能为空")
	}
	now := time.Now()
	_ = models.DB.Model(&SupervisedUser{}).Where("id = ?", id).Updates(map[string]interface{}{
		"updated_at": now, "last_event": now,
	}).Error
	return addLog(&su, operatorID, "note", "", note)
}

// AddEvidence 追加证据引用。
func AddEvidence(id uint, ref EvidenceRef) error {
	var su SupervisedUser
	if err := models.DB.Where("id = ?", id).First(&su).Error; err != nil {
		return err
	}
	now := time.Now()
	updates := map[string]interface{}{"evidence_refs": buildRefs(&su, &ref), "updated_at": now, "last_event": now}
	return models.DB.Model(&SupervisedUser{}).Where("id = ?", id).Updates(updates).Error
}

// Stats 概览（给后台卡片）。
func Stats() map[string]interface{} {
	out := map[string]int64{TierSuspect: 0, TierMalicious: 0}
	type row struct {
		Tier string
		C    int64
	}
	var rows []row
	_ = models.DB.Model(&SupervisedUser{}).Select("tier, COUNT(1) as c").
		Where("status = ?", StatusActive).Group("tier").Scan(&rows).Error
	for _, r := range rows {
		out[r.Tier] = r.C
	}
	var cleared int64
	_ = models.DB.Model(&SupervisedUser{}).Where("status = ?", StatusCleared).Count(&cleared).Error
	return map[string]interface{}{
		"by_tier": out, "cleared": cleared,
	}
}

// Migrate 建表。
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&SupervisedUser{}, &SuperviseLog{})
}

// ─────────────── 内部辅助 ───────────────

func normalizeTier(t string) string {
	switch t {
	case TierMalicious:
		return TierMalicious
	}
	return TierSuspect
}

func normalizeSource(s string) string {
	switch s {
	case SrcChain, SrcJudge, SrcWatch, SrcRule, SrcManual, SrcAppeal:
		return s
	}
	return SrcManual
}

func buildReasons(existing *SupervisedUser, reason string) string {
	var arr []string
	if existing != nil && strings.TrimSpace(existing.Reasons) != "" {
		_ = json.Unmarshal([]byte(existing.Reasons), &arr)
	}
	if strings.TrimSpace(reason) != "" {
		arr = append(arr, reason)
	}
	if len(arr) > 50 {
		arr = arr[len(arr)-50:] // 防止无限增长
	}
	b, _ := json.Marshal(arr)
	return string(b)
}

func buildRefs(existing *SupervisedUser, ref *EvidenceRef) string {
	var arr []EvidenceRef
	if existing != nil && strings.TrimSpace(existing.EvidenceRefs) != "" {
		_ = json.Unmarshal([]byte(existing.EvidenceRefs), &arr)
	}
	if ref != nil && strings.TrimSpace(ref.Ref) != "" {
		arr = append(arr, *ref)
	}
	if len(arr) > 100 {
		arr = arr[len(arr)-100:]
	}
	b, _ := json.Marshal(arr)
	return string(b)
}

func addLog(su *SupervisedUser, actorID uint, action, fromTo, note string) error {
	return models.DB.Create(&SuperviseLog{
		SupervisedID: su.ID, UserID: su.UserID, ActorID: actorID,
		Action: action, FromTo: fromTo, Note: truncate(note, 1000), CreatedAt: time.Now(),
	}).Error
}

func nowPtr() *time.Time {
	t := time.Now()
	return &t
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
