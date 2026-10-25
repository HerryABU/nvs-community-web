// Package aisafety 是人工智能对话的内容安全闸。
//
// 背景：AI 审核此前只覆盖小说/博客/论坛/评论，**AI 对话本身没有任何审核**，
// 真实站点上模型确实答出过 TATP 合成条件、冰毒合成路线（即典型越狱）。
// 本包为此提供三层拦截：
//
//  1. 输入侧（提示词层）：扫描用户提问 + 近几轮上下文，命中即拒答，不调上游、不扣费；
//     同时在系统提示词前置不可协商的安全约束（配置化文案）。
//  2. 输出侧（流式层）：增量先入缓冲，整段扫描通过后才放行；命中即断流并整体替换，
//     保证危险内容一个字都不落到浏览器里。
//  3. 用户特征层：累计风险分 → 观察期（阈值收紧）→ 冷却（暂停 AI 功能），
//     让「换个说法反复问」的账号自己撞到墙。
//
// 所有阈值、间期、文案、额外词表一律来自数据库配置（后台可改），代码里不留策略字面量。
package aisafety

import "time"

// 方向
const (
	DirIn  = "in"  // 用户输入
	DirOut = "out" // 模型输出
)

// 处置动作
const (
	ActionBlock    = "block"    // 本次拦截
	ActionWatch    = "watch"    // 命中并计入观察期
	ActionCooldown = "cooldown" // 冷却中拒绝
	ActionReview   = "review"   // 仅记录（可疑但未达拦截线）
	ActionReleased = "released" // 规则命中但被监管模型 B 复核为安全 → 放行（避免误伤）
)

// 监管模型 B 的触发原因
const (
	JudgeTriggerSchedule    = "schedule"     // 批次到期
	JudgeTriggerSuspect     = "suspect"      // 批次内出现规则可疑，提前触发
	JudgeTriggerReviewBlock = "review_block" // 批次内规则命中，提前触发复核（救误伤）
)

// 风险等级
const (
	LevelLow  = "low"
	LevelMid  = "mid"
	LevelHigh = "high"
)

// SafetyConfig 安全闸配置（单行，id=1）。
// 策略值全部可调：默认值仅作为首次种子，运行期一律以后台保存的为准。
type SafetyConfig struct {
	ID uint `gorm:"primaryKey" json:"id"`

	Enabled bool `json:"enabled"` // 总开关

	GuardInput  bool `json:"guard_input"`  // 输入侧拦截
	GuardOutput bool `json:"guard_output"` // 输出侧拦截

	// 阈值（分越高越危险）
	InputBlockScore  int `json:"input_block_score"`  // 输入侧硬拦线
	OutputBlockScore int `json:"output_block_score"` // 输出侧硬拦线
	ReviewScore      int `json:"review_score"`       // 可疑线（只记录不拦）
	WatchBlockScore  int `json:"watch_block_score"`  // 观察期内收紧后的硬拦线
	WatchReviewScore int `json:"watch_review_score"` // 观察期内收紧后的可疑线

	// 输出侧缓冲：先攒够这么多字、扫过才放行。
	// 这是「危险内容不落地」的关键——边流边扫的话，前 300 字早就进浏览器了。
	PrebufferChars int `json:"prebuffer_chars"`
	FlushChars     int `json:"flush_chars"` // 每次放行多少字（留重叠窗口防跨块拼接）

	// 用户特征
	WatchAfterHits  int `json:"watch_after_hits"`  // 累计硬拦几次进入观察期
	WatchHours      int `json:"watch_hours"`       // 观察期时长
	CooldownHits    int `json:"cooldown_hits"`     // 观察期内再命中几次触发冷却
	CooldownMinutes int `json:"cooldown_minutes"`  // 冷却时长
	ScoreDecayHours int `json:"score_decay_hours"` // 风险分半衰期（小时），0=不衰减

	ScanContextTurns int  `json:"scan_context_turns"` // 输入扫描带几轮历史（多轮追问是关键攻击形态）
	SkipFirstTurn    bool `json:"skip_first_turn"`    // 首轮不扫上下文（避免新会话误伤）
	ProtectSystem    bool `json:"protect_system"`     // 是否前置安全系统提示词

	AllowFictionRelief bool `json:"allow_fiction_relief"` // 创作语境减免
	AllowHarmRelief    bool `json:"allow_harm_relief"`    // 危害教育语境减免
	RecordExcerpt      bool `json:"record_excerpt"`       // 事件里是否留存片段（供人工复核）
	ExcerptChars       int  `json:"excerpt_chars"`        // 片段长度上限

	// 文案（可编辑，不要写死在代码里）
	SystemGuardPrompt string `gorm:"type:text" json:"system_guard_prompt"`
	RefusalText       string `gorm:"type:text" json:"refusal_text"`
	// 越狱专用文案。指令覆盖 / 纯话术类拦截不该说「涉及危险品制备」——
	// 用户问的是别的东西却被回一句危险品，观感极差且不像人话。
	RefusalJailbreakText string `gorm:"type:text" json:"refusal_jailbreak_text"`
	CooldownText         string `gorm:"type:text" json:"cooldown_text"`
	WatchNoticeText      string `gorm:"type:text" json:"watch_notice_text"`

	ExtraTerms string `gorm:"type:text" json:"extra_terms"` // 额外禁止词，换行或逗号分隔

	// ── 监管模型 B（LLM 裁判）──
	// 规则引擎只能判「这一句像不像」，判不出「这人正在一步步把我带出去」。
	// 所以每若干轮把整段对话交给一个模型问「这算不算越狱」。
	// 模型可复用主模型（judge_model_id = 主模型 ID），也可另配便宜的 / 免费模型。
	JudgeEnabled    bool `json:"judge_enabled"`
	JudgeModelID    uint `json:"judge_model_id"`    // 0 = 自动挑一个启用的 chat 模型
	JudgeEveryTurns int  `json:"judge_every_turns"` // 每 N 轮用户消息体检一次
	JudgeOnSuspect  bool `json:"judge_on_suspect"`  // 规则判定「可疑未达线」时立即送审（不等轮次）
	// JudgeReviewBlocks 规则命中时先让 B 复核，判为安全就放行。
	//
	// 这是「避免误伤」的关键开关：规则引擎是宁枉勿纵的（高召回、低精度），
	// 直接按规则拒答会把「写小说查资料」「化学老师备课」一起打死。
	// 让 B 做二次裁定，规则负责兜住明显的，B 负责捞回错杀的。
	JudgeReviewBlocks bool `json:"judge_review_blocks"`
	JudgeBlockScore   int  `json:"judge_block_score"`   // B 判定分 ≥ 此值即拦截
	JudgeReleaseScore int  `json:"judge_release_score"` // B 判安全的分数上限（≤ 此值且未判越狱 → 放行）
	JudgeMaxChars     int  `json:"judge_max_chars"`     // 送审文本总长上限（省 token）
	JudgeTimeoutSec   int  `json:"judge_timeout_sec"`   // 单次判定超时（免费网关慢，别拖死主流程）
	// FailOpen：判定超时/失败时放行（默认，B 挂了不能让全站 AI 不可用），
	// 但会把该用户置入观察期并记事件；FailClosed 则直接拒答。
	JudgeFailOpen bool `json:"judge_fail_open"`
	JudgeMaxTurns int  `json:"judge_max_turns"` // 送审带最近几轮对话

	JudgePrompt string `gorm:"type:text" json:"judge_prompt"` // B 的判定提示词（可改）

	// SchemaVersion 配置结构版本。
	// 作用：新版本加了列之后，老行里这些列是零值（bool=false / int=0），
	// 站长打开后台看着「已保存」，实际新功能从未启用过——这种静默失效必须靠版本号回填解决。
	SchemaVersion int `json:"schema_version"`

	UpdatedAt time.Time `json:"updated_at"`
}

// JudgeLog 监管模型 B 的判定留痕（与规则命中分开存，便于评估 B 的准确率）。
type JudgeLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"index" json:"user_id"`
	Username   string    `gorm:"size:64" json:"username"`
	SessionID  uint      `gorm:"index" json:"session_id"`
	Turn       int       `json:"turn"`                   // 触发时的第几轮
	Trigger    string    `gorm:"size:16" json:"trigger"` // schedule / suspect / review_block
	ModelID    uint      `json:"model_id"`
	ModelName  string    `gorm:"size:64" json:"model_name"`
	Jailbroken bool      `gorm:"index" json:"jailbroken"`
	Score      int       `json:"score"`
	Categories string    `gorm:"size:255" json:"categories"`
	Reason     string    `gorm:"type:text" json:"reason"`
	Blocked    bool      `json:"blocked"`  // 判定导致拦截
	Released   bool      `json:"released"` // 判定推翻了规则拦截（避免误伤）
	LatencyMS  int       `json:"latency_ms"`
	Failed     bool      `json:"failed"` // 调用失败/超时（未取到有效结论）
	RawReply   string    `gorm:"type:text" json:"raw_reply"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
}

// SafetyProfile 用户安全画像（每人一行）。
type SafetyProfile struct {
	UserID     uint   `gorm:"primaryKey" json:"user_id"`
	Username   string `gorm:"size:64" json:"username"`
	RiskScore  int    `json:"risk_score"`  // 累积风险分（按半衰期衰减）
	BlockCount int    `json:"block_count"` // 累计硬拦次数
	HitCount   int    `json:"hit_count"`   // 观察期内命中次数

	WatchUntil    *time.Time `json:"watch_until"`
	CooldownUntil *time.Time `json:"cooldown_until"`
	LastHitAt     *time.Time `json:"last_hit_at"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SafetyEvent 一次拦截/可疑记录（后台可查、可复核）。
type SafetyEvent struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"index" json:"user_id"`
	Username   string    `gorm:"size:64" json:"username"`
	Direction  string    `gorm:"size:8;index" json:"direction"` // in / out
	Action     string    `gorm:"size:16;index" json:"action"`   // block / watch / cooldown / review
	Level      string    `gorm:"size:8" json:"level"`
	Score      int       `json:"score"`
	Categories string    `gorm:"size:255" json:"categories"` // 命中类别（逗号分隔）
	Hits       string    `gorm:"type:text" json:"hits"`      // 命中词（逗号分隔）
	Excerpt    string    `gorm:"type:text" json:"excerpt"`   // 触发片段（截断）
	ModelID    uint      `json:"model_id"`
	SessionID  uint      `json:"session_id"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
}

// 表名统一加 ai_ 前缀。
// GORM 默认会生成 safety_configs / judge_logs 这种过宽的通用名，
// 这个项目里已经有一堆同前缀的业务表，通用名以后必然撞车。
func (SafetyConfig) TableName() string  { return "ai_safety_configs" }
func (SafetyProfile) TableName() string { return "ai_safety_profiles" }
func (SafetyEvent) TableName() string   { return "ai_safety_events" }
func (JudgeLog) TableName() string      { return "ai_judge_logs" }
