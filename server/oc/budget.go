package oc

// ==================== 预算（防成本失控） ====================
// 三道闸门：单次调用上限（沿用 AI 体系） / 群级每日预算 / 单流（单角色）上限。
// 超额即暂停对话流并标记，绝不静默继续烧。

import (
	"errors"
	"time"

	"nvs-server/models"
)

// ErrBudgetExceeded 预算超额
var ErrBudgetExceeded = errors.New("已达到预算上限，对话流已暂停")

func todayStr() string { return time.Now().Format("2006-01-02") }

// DailyUsed 群当日已消耗
func DailyUsed(groupID uint) int {
	var u BudgetUsage
	if err := models.DB.Where("group_id = ? AND day = ?", groupID, todayStr()).First(&u).Error; err != nil {
		return 0
	}
	return u.Cost
}

// AddUsage 记账（群级按日累计）
func AddUsage(groupID uint, cost int) {
	day := todayStr()
	var u BudgetUsage
	err := models.DB.Where("group_id = ? AND day = ?", groupID, day).First(&u).Error
	if err != nil {
		u = BudgetUsage{GroupID: groupID, Day: day}
		_ = models.DB.Create(&u).Error
	}
	_ = models.DB.Model(&BudgetUsage{}).Where("id = ?", u.ID).
		Updates(map[string]interface{}{
			"cost":       models.DB.Raw("cost + ?", cost),
			"turns":      models.DB.Raw("turns + 1"),
			"updated_at": time.Now(),
		}).Error
}

// FlowUsed 单流已消耗
func FlowUsed(flowID uint) int {
	type row struct{ Total int }
	var r row
	_ = models.DB.Model(&Turn{}).Select("COALESCE(SUM(cost),0) as total").
		Where("flow_id = ?", flowID).Scan(&r).Error
	return r.Total
}

// CheckBudget 生成前校验：群日预算 + 单流预算 + 角色预算
func CheckBudget(groupID, flowID, agentID uint, est int) error {
	set := GetGroupSetting(groupID)
	if set.DailyBudget > 0 && DailyUsed(groupID)+est > set.DailyBudget {
		return ErrBudgetExceeded
	}
	if flowID > 0 && set.FlowBudget > 0 && FlowUsed(flowID)+est > set.FlowBudget {
		return ErrBudgetExceeded
	}
	if agentID > 0 {
		var a Agent
		if err := models.DB.Where("id = ?", agentID).First(&a).Error; err == nil && a.Budget > 0 {
			type row struct{ Total int }
			var r row
			_ = models.DB.Model(&Turn{}).Select("COALESCE(SUM(cost),0) as total").
				Where("agent_id = ?", agentID).Scan(&r).Error
			if r.Total+est > a.Budget {
				return ErrBudgetExceeded
			}
		}
	}
	return nil
}
