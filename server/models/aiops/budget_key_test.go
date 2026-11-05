package aiops

import "nvs-server/models"

// aiBudgetKey 与根包 ai_budget.go 中的平台配置键保持一致。
// 该常量未导出、无法跨包复用，故在本子包测试内本地定义一份（值相同，行为不变）。
const aiBudgetKey = "ai_budget"

// normalizeBudget 与根包 ai_budget.go 的实现等价：未导出函数无法跨包复用，
// 这里复制一份供本子包测试验证预算配置的钳位规则（逻辑逐条对齐）。
func normalizeBudget(c models.AiBudgetConfig) models.AiBudgetConfig {
	if c.DailyCap < 0 {
		c.DailyCap = 0
	}
	if c.MonthlyCap < 0 {
		c.MonthlyCap = 0
	}
	if c.MaxConcurrent < 0 {
		c.MaxConcurrent = 0
	}
	if c.QueueTimeoutSec <= 0 {
		c.QueueTimeoutSec = 20
	}
	if c.QueueTimeoutSec > 300 {
		c.QueueTimeoutSec = 300
	}
	if c.AlertAtRatio <= 0 || c.AlertAtRatio > 1 {
		c.AlertAtRatio = 0.8
	}
	// 峰时上限不能超过全站上限
	if c.PeakConcurrent > c.MaxConcurrent && c.MaxConcurrent > 0 {
		c.PeakConcurrent = c.MaxConcurrent
	}
	hours := make([]int, 0, len(c.PeakHours))
	seen := map[int]bool{}
	for _, h := range c.PeakHours {
		if h >= 0 && h <= 23 && !seen[h] {
			seen[h] = true
			hours = append(hours, h)
		}
	}
	c.PeakHours = hours
	return c
}
