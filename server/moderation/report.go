package moderation

import (
	"log"
	"time"

	"nvs-server/models"
)

// OnReport 举报触发审核：被举报立刻送检 AI，并落「合规结论」。
//
// 实现用户要求的完整链路：
//  1. 被举报立刻送检；2. 存储是否合规；
//  3. 第 5/15…次举报仍正常送检，但达 RecheckReportCount 且此前通过 → 复检（提示词强制客观中立）；
//  4. 达 StopReviewAfter 后不再 AI 复检（防刷爆成本），内容仍存续；
//  5. 内容合规却遭不实举报 → 对该举报者隐藏；
//  6. 举报者不实举报累计达 MaliciousThreshold → 没收其举报功能。
func OnReport(reporterID uint, targetType string, targetID uint) {
	cfg, err := GetConfig()
	if err != nil || !cfg.Enabled {
		return
	}

	reportCount, _ := CountTargetReports(targetType, targetID)

	// 达上限：停止 AI 复检（仍记录一条 skipped，便于后台观测）
	if cfg.StopReviewAfter > 0 && int(reportCount) >= cfg.StopReviewAfter {
		_ = CreateRecord(&ModerationRecord{
			Scope: targetType, TargetType: targetType, TargetID: targetID,
			Trigger: TriggerReport, Verdict: VerdictSkipped, Compliant: true,
			Reason:      "举报数已达上限，停止 AI 复检",
			ReportCount: int(reportCount),
		})
		// 仍更新举报者画像（仅累计举报数，不因跳过而改变违规判定）
		bumpReporterCount(reporterID)
		return
	}

	content, ok := ResolveContent(targetType, targetID)
	if !ok {
		return
	}

	// 复检判定：达 RecheckReportCount 且此前有过「通过」记录才视为复检
	reTrigger := int(reportCount) >= cfg.RecheckReportCount
	if reTrigger {
		if last, le := LatestRecord(targetType, targetID); le != nil || (last != nil && !last.Compliant) {
			// last 为 nil（查询无记录）或上次未通过 → 不升级为复检口径
			if last == nil || !last.Compliant {
				reTrigger = false
			}
		}
	}

	res, rerr := ReviewText(targetType, content, reTrigger, int(reportCount))
	if rerr != nil {
		log.Printf("[moderation] OnReport review failed target=%s/%d: %v", targetType, targetID, rerr)
		return
	}

	rec := &ModerationRecord{
		Scope: targetType, TargetType: targetType, TargetID: targetID,
		Trigger: TriggerReport, ModelID: res.ModelID, ModelName: res.ModelName,
		Verdict: res.Verdict, Compliant: res.Verdict != VerdictFail,
		Reason: res.Reason, Categories: joinJSON(res.Categories),
		ReportCount: int(reportCount), ReTriggered: reTrigger,
	}
	_ = CreateRecord(rec)

	// 判定违规且站长开启了「自动处置」→ 交给注入的处置回调（软删留证）
	if res.Verdict == VerdictFail && cfg.AutoHideOnFail {
		runFailHook(targetType, targetID, res.Reason)
	}

	// 更新举报者画像
	if rec.Compliant {
		// 内容被判合规 → 本次举报大概率不实
		incr, _ := GetOrCreateMaliciousReporter(reporterID)
		incr.TotalReports++
		incr.MaliciousCount++
		// 内容合规却遭大量不实举报 → 对该举报者隐藏（内容仍存续，仅其不可见）
		if cfg.HideFromReporter && int(reportCount) >= cfg.RecheckReportCount {
			_ = AddHiddenContent(reporterID, targetType, targetID)
		}
		// 不实举报累计达阈值 → 没收举报功能
		if cfg.MaliciousThreshold > 0 && incr.MaliciousCount >= cfg.MaliciousThreshold {
			now := time.Now()
			incr.ReportingBanned = true
			incr.BanReason = "不实举报累计达阈值，举报功能已被没收"
			incr.BannedAt = &now
		}
		_ = SaveMaliciousReporter(incr)
	} else {
		bumpReporterCount(reporterID)
	}
}

// bumpReporterCount 仅累计举报总数（用于 AI 未出结论或跳过的情形）。
func bumpReporterCount(reporterID uint) {
	incr, err := GetOrCreateMaliciousReporter(reporterID)
	if err != nil {
		return
	}
	incr.TotalReports++
	_ = SaveMaliciousReporter(incr)
}

// IsReportingBanned 判断某用户是否已被没收举报功能。
func IsReportingBanned(userID uint) bool {
	m, err := GetOrCreateMaliciousReporter(userID)
	if err != nil {
		return false
	}
	return m.ReportingBanned
}

// HiddenReportTargets 返回某用户被隐藏的内容（type:id 列表），供内容读取层过滤。
func HiddenReportTargets(userID uint) map[string]bool {
	out := map[string]bool{}
	if userID == 0 {
		return out
	}
	var list []HiddenContent
	models.DB.Where("user_id = ?", userID).Find(&list)
	for _, h := range list {
		out[h.TargetType+":"+itoa(int(h.TargetID))] = true
	}
	return out
}
