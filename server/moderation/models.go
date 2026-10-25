// Package moderation —— AI 内容审核中枢（嵌套子包，刻意拆小，避免上帝类）。
//
// 职责切分（每个文件只做一件事）：
//   - models.go    审核相关 DB 模型 + 配置（单行表）
//   - store.go     DB 读写（零值安全、按业务字段精确写回）
//   - content.go   从各类内容源抽取待审文本（小说/博客/论坛/评论，含话本气泡体解析）
//   - reviewer.go  调用上游 AI 做中立判定（直接走 aiclient，不走计费）
//   - report.go    举报触发审核 + 复检 + 恶意举报者追踪
//   - batch.go     论坛/评论区按配置间隔的定时批量审核 + 调度器
//   - service.go   对外公开 API（handler 只调这里）
//
// 所有策略（批次间隔、举报阈值、审核模型、是否对举报者隐藏）一律落在
// moderation_config 配置表，由管理端设置，代码中严禁硬编码字面量策略值。
package moderation

import "time"

// 内容作用域（与举报/内容类型对齐）
const (
	ScopeNovel   = "novel"
	ScopeChapter = "chapter"
	ScopeBlog    = "blog"
	ScopeForum   = "forum"
	ScopeThread  = "thread"
	ScopePost    = "post"
	ScopeComment = "comment"
)

// 审核触发来源
const (
	TriggerWrite  = "write"  // 发布/提交时送检（小说/博客）
	TriggerReport = "report" // 被举报时立即送检（评论区等）
	TriggerBatch  = "batch"  // 定时批量送检（论坛/评论区）
	TriggerManual = "manual" // 管理员手动送检
)

// 审核结论
const (
	VerdictPass    = "pass"    // 合规
	VerdictFail    = "fail"    // 明确违规
	VerdictReview  = "review"  // 存疑，需人工复核
	VerdictSkipped = "skipped" // 举报数已达上限，停止 AI 复检（内容仍存续）
)

// ModerationRecord 一条 AI 审核记录
type ModerationRecord struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Scope       string    `gorm:"size:16;index:idx_mod_target,priority:1" json:"scope"`
	TargetType  string    `gorm:"size:16;index:idx_mod_target,priority:2" json:"target_type"`
	TargetID    uint      `gorm:"index:idx_mod_target,priority:3" json:"target_id"`
	Trigger     string    `gorm:"size:16" json:"trigger"`
	ModelID     uint      `json:"model_id"`
	ModelName   string    `gorm:"size:64" json:"model_name"`
	Verdict     string    `gorm:"size:16" json:"verdict"`
	Compliant   bool      `json:"compliant"` // 是否合规（fail=false，pass/review=true）
	Reason      string    `gorm:"type:text" json:"reason"`
	Categories  string    `gorm:"type:text" json:"categories"` // JSON 数组：命中的违规类别
	ReportCount int       `json:"report_count"`                // 触发本次审核时该内容累计举报数（用于复检判定）
	ReTriggered bool      `json:"re_triggered"`                // 是否因「重复举报达阈值」复检
	CreatedAt   time.Time `json:"created_at"`
}

func (ModerationRecord) TableName() string { return "moderation_records" }

// ModerationConfig 审核配置（单行表，id 固定为 1）
//
// 注意：以下字段全部由管理端设置，默认值仅在首次建表时写入 DB，
// 运行期逻辑一律读此表，绝不从代码常量取策略值。
type ModerationConfig struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	Enabled            bool      `json:"enabled"`              // 总开关
	ModelID            uint      `json:"model_id"`             // 用于判定的 chat 模型（0=自动选第一个启用）
	NovelBlogOnWrite   bool      `json:"novel_blog_on_write"`  // 小说/博客发布时送检
	ForumCommentBatch  bool      `json:"forum_comment_batch"`  // 论坛/评论区定时批量送检
	BatchIntervalHours int       `json:"batch_interval_hours"` // 批量送检间隔（小时）—— 2/6/12/24 由后台选
	AutoHideOnFail     bool      `json:"auto_hide_on_fail"`    // 判定 fail 自动隐藏内容（全局）
	MaliciousThreshold int       `json:"malicious_threshold"`  // 举报者「不实举报」达此数 → 没收举报功能（5/10 由后台选）
	RecheckReportCount int       `json:"recheck_report_count"` // 同一内容被举报达此数且此前通过 → 复检（5/10 由后台选）
	StopReviewAfter    int       `json:"stop_review_after"`    // 同一内容被举报达此数后停止 AI 复检（防刷爆成本，默认 20，由后台选）
	HideFromReporter   bool      `json:"hide_from_reporter"`   // 内容合规却遭大量不实举报 → 对该举报者隐藏（内容仍存续）
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func (ModerationConfig) TableName() string { return "moderation_config" }

// MaliciousReporter 恶意举报者画像
type MaliciousReporter struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	UserID          uint       `gorm:"uniqueIndex" json:"user_id"`
	TotalReports    int        `json:"total_reports"`    // 累计举报数
	MaliciousCount  int        `json:"malicious_count"`  // 不实举报数（被举报内容最终判定合规）
	ReportingBanned bool       `json:"reporting_banned"` // 是否已没收举报功能
	BanReason       string     `gorm:"size:128" json:"ban_reason"`
	BannedAt        *time.Time `json:"banned_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (MaliciousReporter) TableName() string { return "malicious_reporters" }

// HiddenContent 对某举报者隐藏的内容（内容本身仍存续，仅对该用户不可见）
type HiddenContent struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"index:idx_hidden_user,priority:1" json:"user_id"`
	TargetType string    `gorm:"size:16;index:idx_hidden_user,priority:2" json:"target_type"`
	TargetID   uint      `gorm:"index:idx_hidden_user,priority:3" json:"target_id"`
	CreatedAt  time.Time `json:"created_at"`
}

func (HiddenContent) TableName() string { return "hidden_contents" }
