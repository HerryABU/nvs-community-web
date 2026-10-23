package models

import "time"

// ==================== AI 数据审计（管理端） ====================
// 只做只读查询：会话 / 消息 / 媒体任务 / 调用流水 / 汇总统计。
// 分页一律带上限，避免管理端把大表一次拉穿。

// AdminListAiSessions 全部会话（可按用户、智能体过滤）
func AdminListAiSessions(userID uint, page, size int) ([]AiChatSession, int64, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 20
	}
	q := DB.Model(&AiChatSession{})
	if userID > 0 {
		q = q.Where("user_id = ?", userID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []AiChatSession
	err := q.Order("updated_at DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

// AdminListAiMedia 全部媒体任务（kind/status 过滤）
func AdminListAiMedia(kind, status string, page, size int) ([]AiMediaTask, int64, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 20
	}
	q := DB.Model(&AiMediaTask{})
	if kind != "" {
		q = q.Where("kind = ?", NormalizeAiKind(kind))
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []AiMediaTask
	err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

// AdminListAiCallLogs 全部调用流水（status 过滤）
func AdminListAiCallLogs(status string, page, size int) ([]AiCallLog, int64, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 20
	}
	q := DB.Model(&AiCallLog{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []AiCallLog
	err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

// AiAuditStats 审计概览（近 24h + 累计）
func AiAuditStats() map[string]int64 {
	since := time.Now().Add(-24 * time.Hour)
	cnt := func(model interface{}, where string, args ...interface{}) int64 {
		var n int64
		q := DB.Model(model)
		if where != "" {
			q = q.Where(where, args...)
		}
		q.Count(&n)
		return n
	}
	var cost int64
	DB.Model(&AiCallLog{}).Select("COALESCE(SUM(cost),0)").Scan(&cost)
	return map[string]int64{
		"sessions":       cnt(&AiChatSession{}, ""),
		"messages":       cnt(&AiChatMessage{}, ""),
		"sessions_24h":   cnt(&AiChatSession{}, "updated_at >= ?", since),
		"images":         cnt(&AiMediaTask{}, "kind = ?", AiKindImage),
		"videos":         cnt(&AiMediaTask{}, "kind = ?", AiKindVideo),
		"media_failed":   cnt(&AiMediaTask{}, "status = ?", AiMediaFailed),
		"calls":          cnt(&AiCallLog{}, ""),
		"calls_24h":      cnt(&AiCallLog{}, "created_at >= ?", since),
		"calls_failed":   cnt(&AiCallLog{}, "status = ?", "failed"),
		"cost_total":     cost,
		"free_calls":     cnt(&AiCallLog{}, "free_quota = ?", true),
		"gallery_public": cnt(&AiMediaTask{}, "is_public = ?", true),
	}
}
