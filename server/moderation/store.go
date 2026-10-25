package moderation

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"nvs-server/models"
)

// GetConfig 读取审核配置；无配置行则写入默认配置（默认值仅作首次种子，运行期以后台为准）。
func GetConfig() (*ModerationConfig, error) {
	var cfg ModerationConfig
	err := models.DB.Where("id = ?", 1).First(&cfg).Error
	if err == nil {
		return &cfg, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return &cfg, err
	}
	seed := defaultConfig()
	if e := models.DB.Create(&seed).Error; e != nil {
		return &seed, e
	}
	return &seed, nil
}

// defaultConfig 仅作为 DB 首次种子，运行期逻辑不得引用这些字面量。
func defaultConfig() ModerationConfig {
	return ModerationConfig{
		ID:                 1,
		Enabled:            false,
		NovelBlogOnWrite:   true,
		ForumCommentBatch:  true,
		BatchIntervalHours: 6, // 默认 6 小时，后台可改 2/6/12/24
		AutoHideOnFail:     false,
		MaliciousThreshold: 5,  // 不实举报 5 次没收举报功能，后台可改 5/10
		RecheckReportCount: 5,  // 同内容被举报 5 次且此前通过 → 复检，后台可改 5/10
		StopReviewAfter:    20, // 同内容被举报 20 次后停止 AI 复检，防刷爆成本
		HideFromReporter:   true,
	}
}

// SaveConfig 精确写回配置（按字段 map，零值安全）。
func SaveConfig(cfg *ModerationConfig) error {
	cfg.ID = 1
	fields := map[string]interface{}{
		"enabled":              cfg.Enabled,
		"model_id":             cfg.ModelID,
		"novel_blog_on_write":  cfg.NovelBlogOnWrite,
		"forum_comment_batch":  cfg.ForumCommentBatch,
		"batch_interval_hours": cfg.BatchIntervalHours,
		"auto_hide_on_fail":    cfg.AutoHideOnFail,
		"malicious_threshold":  cfg.MaliciousThreshold,
		"recheck_report_count": cfg.RecheckReportCount,
		"stop_review_after":    cfg.StopReviewAfter,
		"hide_from_reporter":   cfg.HideFromReporter,
		"updated_at":           time.Now(),
	}
	return models.DB.Model(&ModerationConfig{}).Where("id = ?", 1).Updates(fields).Error
}

// CreateRecord 写入一条审核记录。
func CreateRecord(r *ModerationRecord) error {
	r.CreatedAt = time.Now()
	return models.DB.Create(r).Error
}

// LatestRecord 取某内容最新一条审核记录。
func LatestRecord(targetType string, targetID uint) (*ModerationRecord, error) {
	var r ModerationRecord
	err := models.DB.Where("target_type = ? AND target_id = ?", targetType, targetID).
		Order("created_at DESC").First(&r).Error
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// CountTargetReports 统计某内容累计举报数（含全部状态）。
func CountTargetReports(targetType string, targetID uint) (int64, error) {
	var n int64
	err := models.DB.Model(&models.Report{}).
		Where("target_type = ? AND target_id = ?", targetType, targetID).Count(&n).Error
	return n, err
}

// RecordFilter 审核记录查询条件。
type RecordFilter struct {
	Scope    string
	Verdict  string
	Trigger  string
	Page     int
	PageSize int
}

// ListRecords 分页查询审核记录。
func ListRecords(f RecordFilter) ([]ModerationRecord, int64, error) {
	q := models.DB.Model(&ModerationRecord{})
	if f.Scope != "" {
		q = q.Where("scope = ?", f.Scope)
	}
	if f.Verdict != "" {
		q = q.Where("verdict = ?", f.Verdict)
	}
	if f.Trigger != "" {
		q = q.Where("trigger = ?", f.Trigger)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = 50
	}
	var list []ModerationRecord
	off := (f.Page - 1) * f.PageSize
	if err := q.Order("created_at DESC").Offset(off).Limit(f.PageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// GetOrCreateMaliciousReporter 取或建举报者画像。
func GetOrCreateMaliciousReporter(userID uint) (*MaliciousReporter, error) {
	var m MaliciousReporter
	err := models.DB.Where("user_id = ?", userID).First(&m).Error
	if err == nil {
		return &m, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	m = MaliciousReporter{UserID: userID}
	if e := models.DB.Create(&m).Error; e != nil {
		return nil, e
	}
	return &m, nil
}

// SaveMaliciousReporter 精确写回举报者画像。
func SaveMaliciousReporter(m *MaliciousReporter) error {
	m.UpdatedAt = time.Now()
	return models.DB.Model(&MaliciousReporter{}).Where("user_id = ?", m.UserID).Updates(map[string]interface{}{
		"total_reports":    m.TotalReports,
		"malicious_count":  m.MaliciousCount,
		"reporting_banned": m.ReportingBanned,
		"ban_reason":       m.BanReason,
		"banned_at":        m.BannedAt,
		"updated_at":       m.UpdatedAt,
	}).Error
}

// AddHiddenContent 对某举报者隐藏某内容（幂等）。
func AddHiddenContent(userID uint, targetType string, targetID uint) error {
	var n int64
	models.DB.Model(&HiddenContent{}).
		Where("user_id = ? AND target_type = ? AND target_id = ?", userID, targetType, targetID).
		Count(&n)
	if n > 0 {
		return nil
	}
	return models.DB.Create(&HiddenContent{UserID: userID, TargetType: targetType, TargetID: targetID}).Error
}

// ListMaliciousReporters 列出全部举报者画像（按不实举报数降序）。
func ListMaliciousReporters() ([]MaliciousReporter, error) {
	var list []MaliciousReporter
	err := models.DB.Order("malicious_count DESC, total_reports DESC").Find(&list).Error
	return list, err
}

// UnbanReporter 恢复某用户举报功能。
func UnbanReporter(userID uint) error {
	now := time.Now()
	return models.DB.Model(&MaliciousReporter{}).Where("user_id = ?", userID).Updates(map[string]interface{}{
		"reporting_banned": false,
		"ban_reason":       "",
		"banned_at":        nil,
		"updated_at":       now,
	}).Error
}

// ModerationStats 审核概览。
type ModerationStats struct {
	TotalRecords       int64 `json:"total_records"`
	PassCount          int64 `json:"pass_count"`
	FailCount          int64 `json:"fail_count"`
	ReviewCount        int64 `json:"review_count"`
	SkippedCount       int64 `json:"skipped_count"`
	MaliciousReporters int64 `json:"malicious_reporters"`
	BannedReporters    int64 `json:"banned_reporters"`
	HiddenCount        int64 `json:"hidden_count"`
}

// GetStats 汇总审核数据。
func GetStats() (*ModerationStats, error) {
	s := &ModerationStats{}
	if err := models.DB.Model(&ModerationRecord{}).Count(&s.TotalRecords).Error; err != nil {
		return nil, err
	}
	countByVerdict := func(v string) (int64, error) {
		var n int64
		e := models.DB.Model(&ModerationRecord{}).Where("verdict = ?", v).Count(&n).Error
		return n, e
	}
	var e error
	if s.PassCount, e = countByVerdict(VerdictPass); e != nil {
		return nil, e
	}
	if s.FailCount, e = countByVerdict(VerdictFail); e != nil {
		return nil, e
	}
	if s.ReviewCount, e = countByVerdict(VerdictReview); e != nil {
		return nil, e
	}
	if s.SkippedCount, e = countByVerdict(VerdictSkipped); e != nil {
		return nil, e
	}
	if e = models.DB.Model(&MaliciousReporter{}).Where("malicious_count > 0").Count(&s.MaliciousReporters).Error; e != nil {
		return nil, e
	}
	if e = models.DB.Model(&MaliciousReporter{}).Where("reporting_banned = ?", true).Count(&s.BannedReporters).Error; e != nil {
		return nil, e
	}
	if e = models.DB.Model(&HiddenContent{}).Count(&s.HiddenCount).Error; e != nil {
		return nil, e
	}
	return s, nil
}
