package oc

// ==================== 对话流 + 调度器 ====================
// 调度器解决「谁先说、谁回谁、什么时候停」：轮转/随机/触发词/手动；
// 硬约束：max_turns、单轮超时、用户可 pause/resume/stop/step（防无限循环）。

import (
	"errors"
	"math/rand"
	"strings"
	"time"

	"nvs-server/models"
	"nvs-server/security"
)

// CreateFlow 创建对话流
func CreateFlow(groupID, starterID, bookID uint, title, schedule string) (*Flow, error) {
	set := GetGroupSetting(groupID)
	if !set.Enabled {
		return nil, errors.New("该群未启用 OC 模式")
	}
	f := &Flow{
		GroupID: groupID, StarterID: starterID, BookID: bookID,
		Title:    strings.TrimSpace(security.SanitizeUserContent(title)),
		Schedule: NormalizeSchedule(schedule),
		Status:   FlowRunning, MaxTurns: set.MaxTurns,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := models.DB.Create(f).Error; err != nil {
		return nil, err
	}
	return f, nil
}

// GetFlow 读取对话流
func GetFlow(id uint) (*Flow, error) {
	var f Flow
	if err := models.DB.Where("id = ?", id).First(&f).Error; err != nil {
		return nil, errors.New("对话流不存在")
	}
	return &f, nil
}

// ListFlows 群内对话流
func ListFlows(groupID uint, limit int) ([]Flow, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	var list []Flow
	if err := models.DB.Where("group_id = ?", groupID).
		Order("updated_at DESC").Limit(limit).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// SetFlowStatus 暂停 / 继续 / 停止（用户可喊停）
func SetFlowStatus(id uint, status, reason string) error {
	f, err := GetFlow(id)
	if err != nil {
		return err
	}
	switch status {
	case FlowPaused, FlowStopped, FlowRunning:
	default:
		return errors.New("状态不合法")
	}
	if f.Status == FlowStopped && status == FlowRunning {
		return errors.New("已结束的对话流不能重启")
	}
	return models.DB.Model(&Flow{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":      status,
		"stop_reason": reason,
		"updated_at":  time.Now(),
	}).Error
}

// ListTurns 轮次列表（按 seq 升序）
func ListTurns(flowID uint, afterSeq, limit int) ([]Turn, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	q := models.DB.Where("flow_id = ?", flowID)
	if afterSeq > 0 {
		q = q.Where("seq > ?", afterSeq)
	}
	var list []Turn
	if err := q.Order("seq ASC").Limit(limit).Find(&list).Error; err != nil {
		return nil, err
	}
	// 角色名 + AI 标记
	ids := map[uint]bool{}
	for _, t := range list {
		if t.CharacterID > 0 {
			ids[t.CharacterID] = true
		}
	}
	nameOf := map[uint]string{}
	if len(ids) > 0 {
		keys := make([]uint, 0, len(ids))
		for id := range ids {
			keys = append(keys, id)
		}
		var chars []Character
		models.DB.Select("id, name").Where("id IN ?", keys).Find(&chars)
		for _, c := range chars {
			nameOf[c.ID] = c.Name
		}
	}
	for i := range list {
		list[i].CharacterName = nameOf[list[i].CharacterID]
		list[i].IsAI = list[i].UserID == 0
	}
	return list, nil
}

// nextSpeaker 调度：选出下一个发言的 AI 角色
func nextSpeaker(f *Flow, agents []Agent) (*Agent, error) {
	if len(agents) == 0 {
		return nil, errors.New("群内没有可用的 AI 角色")
	}
	enabled := make([]Agent, 0, len(agents))
	for _, a := range agents {
		if a.Enabled {
			enabled = append(enabled, a)
		}
	}
	if len(enabled) == 0 {
		return nil, errors.New("没有启用的 AI 角色")
	}
	switch NormalizeSchedule(f.Schedule) {
	case ScheduleRandom:
		return &enabled[rand.Intn(len(enabled))], nil
	case ScheduleRoundRobin:
		fallthrough
	default:
		idx := int(f.TurnCount % len(enabled))
		return &enabled[idx], nil
	}
}

// AddHumanTurn 真人以角色身份发言（seq 自增）
func AddHumanTurn(flowID, userID, characterID uint, content string) (*Turn, error) {
	f, err := GetFlow(flowID)
	if err != nil {
		return nil, err
	}
	if f.Status != FlowRunning {
		return nil, errors.New("对话流未在运行")
	}
	content = strings.TrimSpace(security.SanitizeUserContent(content))
	if content == "" {
		return nil, errors.New("发言内容不能为空")
	}
	content, _ = security.ApplyBadWords(content, "oc")
	t := &Turn{
		FlowID: f.ID, GroupID: f.GroupID, Seq: f.TurnCount + 1,
		CharacterID: characterID, UserID: userID, Content: content,
		Review: ReviewApproved, CreatedAt: time.Now(),
	}
	if err := models.DB.Create(t).Error; err != nil {
		return nil, err
	}
	_ = models.DB.Model(&Flow{}).Where("id = ?", f.ID).
		Updates(map[string]interface{}{"turn_count": f.TurnCount + 1, "updated_at": time.Now()}).Error
	go maybeAuditWindow(f.ID)
	return t, nil
}
