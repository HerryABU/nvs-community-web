package oc

// ==================== 群级 OC 配置 + 群内 AI 角色 ====================

import (
	"errors"
	"strings"
	"time"

	"nvs-server/models"
	"nvs-server/security"
)

// ─── 群配置 ───

// GetGroupSetting 读取群配置（不存在则返回默认配置，不落库）
func GetGroupSetting(groupID uint) GroupSetting {
	var s GroupSetting
	if err := models.DB.Where("group_id = ?", groupID).First(&s).Error; err == nil {
		return s
	}
	return GroupSetting{
		GroupID:     groupID,
		Enabled:     false,
		Mode:        ModeMixed,
		AuditLevel:  AuditStrict,
		AuditTiming: AuditPost,
		WindowSize:  DefaultWindowSize,
		MaxTurns:    DefaultMaxTurns,
		DailyBudget: DefaultDailyBudget,
	}
}

// SaveGroupSetting 保存群配置（群主/管理员）
func SaveGroupSetting(groupID uint, in GroupSetting) (*GroupSetting, error) {
	in.Mode = NormalizeMode(in.Mode)
	in.AuditLevel = NormalizeAuditLevel(in.AuditLevel)
	in.AuditTiming = NormalizeAuditTiming(in.AuditTiming)
	if in.WindowSize <= 0 {
		in.WindowSize = DefaultWindowSize
	}
	if in.MaxTurns <= 0 {
		in.MaxTurns = DefaultMaxTurns
	}
	if in.DailyBudget < 0 {
		in.DailyBudget = 0
	}
	in.RedTeamNote = strings.TrimSpace(security.SanitizeUserContent(in.RedTeamNote))
	if in.AuditLevel != AuditRedTeam {
		in.RedTeamEnabled = false
	}
	var s GroupSetting
	err := models.DB.Where("group_id = ?", groupID).First(&s).Error
	if err != nil {
		in.ID = 0
		in.GroupID = groupID
		in.CreatedAt = time.Now()
		in.UpdatedAt = time.Now()
		if cerr := models.DB.Create(&in).Error; cerr != nil {
			return nil, cerr
		}
		return &in, nil
	}
	in.ID = s.ID
	in.GroupID = groupID
	in.CreatedAt = s.CreatedAt
	in.UpdatedAt = time.Now()
	if err := models.DB.Model(&GroupSetting{}).Where("id = ?", s.ID).Updates(map[string]interface{}{
		"enabled":         in.Enabled,
		"mode":            in.Mode,
		"audit_level":     in.AuditLevel,
		"audit_timing":    in.AuditTiming,
		"window_size":     in.WindowSize,
		"window_rounds":   in.WindowRounds,
		"daily_budget":    in.DailyBudget,
		"flow_budget":     in.FlowBudget,
		"max_turns":       in.MaxTurns,
		"auto_run":        in.AutoRun,
		"redteam_enabled": in.RedTeamEnabled,
		"redteam_note":    in.RedTeamNote,
		"updated_at":      in.UpdatedAt,
	}).Error; err != nil {
		return nil, err
	}
	return &in, nil
}

// ─── AI 角色 ───

// AddAgent 绑定角色卡为群内 AI 角色
func AddAgent(groupID, characterID, modelID, templateID uint, strategy string) (*Agent, error) {
	if characterID == 0 {
		return nil, errors.New("必须绑定角色卡")
	}
	var c Character
	if err := models.DB.Where("id = ? AND status = ?", characterID, "active").First(&c).Error; err != nil {
		return nil, errors.New("角色卡不存在")
	}
	var exist Agent
	if err := models.DB.Where("group_id = ? AND character_id = ?", groupID, characterID).First(&exist).Error; err == nil {
		return nil, errors.New("该角色已在群内")
	}
	a := &Agent{
		GroupID: groupID, CharacterID: characterID, ModelID: modelID,
		TemplateID: templateID, Strategy: strings.TrimSpace(security.SanitizeUserContent(strategy)),
		Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := models.DB.Create(a).Error; err != nil {
		return nil, err
	}
	return a, nil
}

// UpdateAgent 调整 AI 角色（模型/策略/启停/预算）
func UpdateAgent(groupID, id uint, upd map[string]interface{}) error {
	var a Agent
	if err := models.DB.Where("id = ? AND group_id = ?", id, groupID).First(&a).Error; err != nil {
		return errors.New("AI 角色不存在")
	}
	allowed := map[string]bool{"model_id": true, "template_id": true, "strategy": true, "enabled": true, "budget": true}
	patch := map[string]interface{}{"updated_at": time.Now()}
	for k, v := range upd {
		if allowed[k] {
			patch[k] = v
		}
	}
	return models.DB.Model(&Agent{}).Where("id = ?", id).Updates(patch).Error
}

// RemoveAgent 移除群内 AI 角色
func RemoveAgent(groupID, id uint) error {
	return models.DB.Where("id = ? AND group_id = ?", id, groupID).Delete(&Agent{}).Error
}

// ListAgents 群内 AI 角色（带角色名/模型名）
func ListAgents(groupID uint) ([]Agent, error) {
	var list []Agent
	if err := models.DB.Where("group_id = ?", groupID).Order("id ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	ids := map[uint]bool{}
	modelIDs := map[uint]bool{}
	for _, a := range list {
		ids[a.CharacterID] = true
		if a.ModelID > 0 {
			modelIDs[a.ModelID] = true
		}
	}
	// 角色名
	if len(ids) > 0 {
		keys := make([]uint, 0, len(ids))
		for id := range ids {
			keys = append(keys, id)
		}
		var chars []Character
		models.DB.Select("id, name").Where("id IN ?", keys).Find(&chars)
		nameOf := map[uint]string{}
		for _, c := range chars {
			nameOf[c.ID] = c.Name
		}
		for i := range list {
			list[i].CharacterName = nameOf[list[i].CharacterID]
		}
	}
	// 模型名
	if len(modelIDs) > 0 {
		keys := make([]uint, 0, len(modelIDs))
		for id := range modelIDs {
			keys = append(keys, id)
		}
		var ms []models.AiModel
		models.DB.Select("id, name").Where("id IN ?", keys).Find(&ms)
		nameOf := map[uint]string{}
		for _, m := range ms {
			nameOf[m.ID] = m.Name
		}
		for i := range list {
			list[i].ModelName = nameOf[list[i].ModelID]
		}
	}
	return list, nil
}
