package moderation

import (
	"log"
	"time"

	"nvs-server/models"
)

// RunBatchOnce 对论坛/评论区内容做一次批量审核。
// 仅审核「距上次超过 BatchIntervalHours 且窗口内未审过」的内容；间隔来自配置，严禁硬编码。
func RunBatchOnce() (int, error) {
	cfg, err := GetConfig()
	if err != nil {
		return 0, err
	}
	if !cfg.Enabled || !cfg.ForumCommentBatch {
		return 0, nil
	}
	cutoff := time.Now().Add(-time.Duration(cfg.BatchIntervalHours) * time.Hour)

	total := 0
	for _, scope := range []string{ScopeComment, ScopePost, ScopeThread} {
		n, e := reviewScopeBatch(scope, cutoff)
		if e != nil {
			log.Printf("[moderation] batch %s error: %v", scope, e)
			continue
		}
		total += n
	}
	return total, nil
}

// reviewScopeBatch 批量审核某一作用域下、窗口内的内容（单次上限 500 条）。
func reviewScopeBatch(scope string, cutoff time.Time) (int, error) {
	var ids []uint
	switch scope {
	case ScopeComment:
		models.DB.Model(&models.Comment{}).Where("created_at >= ?", cutoff).Limit(500).Pluck("id", &ids)
	case ScopePost:
		models.DB.Model(&models.Post{}).Where("created_at >= ?", cutoff).Limit(500).Pluck("id", &ids)
	case ScopeThread:
		models.DB.Model(&models.Thread{}).Where("created_at >= ?", cutoff).Limit(500).Pluck("id", &ids)
	}
	n := 0
	for _, id := range ids {
		if reviewedRecently(scope, id, cutoff) {
			continue
		}
		content, ok := ResolveContent(scope, id)
		if !ok {
			continue
		}
		res, e := ReviewText(scope, content, false, 0)
		if e != nil {
			continue
		}
		_ = CreateRecord(&ModerationRecord{
			Scope: scope, TargetType: scope, TargetID: id, Trigger: TriggerBatch,
			ModelID: res.ModelID, ModelName: res.ModelName, Verdict: res.Verdict,
			Compliant: res.Verdict != VerdictFail, Reason: res.Reason, Categories: joinJSON(res.Categories),
		})
		n++
	}
	return n, nil
}

// reviewedRecently 判断该内容在窗口内是否已有审核记录。
func reviewedRecently(scope string, id uint, cutoff time.Time) bool {
	var cnt int64
	models.DB.Model(&ModerationRecord{}).
		Where("target_type = ? AND target_id = ? AND created_at >= ?", scope, id, cutoff).
		Count(&cnt)
	return cnt > 0
}

// StartScheduler 启动定时批量审核（每小时巡检；实际审核窗口由配置间隔控制）。
func StartScheduler() {
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for range ticker.C {
			if _, e := RunBatchOnce(); e != nil {
				log.Printf("[moderation] scheduler run error: %v", e)
			}
		}
	}()
}
