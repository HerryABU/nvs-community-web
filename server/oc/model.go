package oc

// ==================== OC 模式：数据模型 ====================
// 规则基线见 docs/oc-mode.md。包内只做数据 + 规则，HTTP 与前端各归其位。
// 依赖方向：oc → models / security / config（不反向依赖 handlers）。

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// ─── 常量 ───

// 群组 OC 模式
const (
	ModeMixed  = "mixed"   // 人与 AI 角色同群互动（默认）
	ModeAIOnly = "ai_only" // 纯 AI 互聊，用户是导演
)

// 审核等级（站长/群主可配；框架中性，红队模式用于安全测试）
const (
	AuditStrict  = "strict"  // 严格：每个窗口都审，命中即停流并待复核
	AuditLoose   = "loose"   // 宽松：只审公开分享与被举报内容
	AuditRedTeam = "redteam" // 红队：不拦截、只记录（隔离环境，产出安全资产）
)

// 审核时机
const (
	AuditPre  = "pre"  // 先审后发
	AuditPost = "post" // 先发后审（命中则标记/隐藏/回滚）
)

// 调度策略
const (
	ScheduleRoundRobin = "round_robin"
	ScheduleRandom     = "random"
	ScheduleTrigger    = "trigger"
	ScheduleManual     = "manual"
)

// 发布审核状态
const (
	ReviewPending  = "pending"
	ReviewApproved = "approved"
	ReviewRejected = "rejected"
)

// 可见性
const (
	VisPrivate = "private"
	VisPublic  = "public"
)

// 对话流状态
const (
	FlowRunning = "running"
	FlowPaused  = "paused"
	FlowStopped = "stopped"
)

// 世界书条目分类
const (
	EntryWorld   = "world"
	EntryFaction = "faction"
	EntryPlace   = "place"
	EntryRule    = "rule"
	EntryEvent   = "event"
)

const (
	MaxNameRunes       = 40
	MaxPersonaRunes    = 8000
	MaxEntryRunes      = 8000
	MaxTurnOutput      = 4000
	DefaultMaxTurns    = 40
	DefaultWindowSize  = 5
	DefaultDailyBudget = 20000 // 群级每日预算（代币）
)

// ─── 角色卡 ───

// Character 角色卡（归用户所有，可绑定到多个群）
//
// 借鉴 SillyTavern（酒馆）V2 卡片规范的字段划分：
//
//	description=常驻人设（固定税） / scenario=场景 / first_mes=开场白 /
//	mes_example=对话示例（最强的风格锚点） / system_prompt / post_history_instructions /
//	creator_notes=创作者备注（不进提示词） / alternate_greetings=备用开场
//
// 同时借鉴「小团子/咔伴」的养成玩法：角色有状态（情绪/好感/精力），发言会改变状态。
type Character struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	UserID     uint   `gorm:"not null;index" json:"user_id"`
	Name       string `gorm:"size:64;not null;index" json:"name"`
	AvatarURL  string `gorm:"size:512;default:''" json:"avatar_url"`
	Persona    string `gorm:"type:text" json:"persona"`    // 人设/性格/背景（= description，常驻注入）
	Appearance string `gorm:"type:text" json:"appearance"` // 外貌
	Speech     string `gorm:"type:text" json:"speech"`     // 说话风格（口吻/口头禅/句长）

	// 酒馆 V2 对齐字段
	Scenario     string `gorm:"type:text" json:"scenario"`         // 场景设定（ situational frame）
	FirstMessage string `gorm:"type:text" json:"first_message"`    // 开场白（first_mes）
	MesExample   string `gorm:"type:text" json:"mes_example"`      // 对话示例（mes_example，风格锚点）
	SystemPrompt string `gorm:"type:text" json:"system_prompt"`    // 角色级系统提示（system_prompt）
	PostHistory  string `gorm:"type:text" json:"post_history"`     // 历史后指令（post_history_instructions）
	CreatorNotes string `gorm:"type:text" json:"creator_notes"`    // 创作者备注（不进提示词）
	Greetings    string `gorm:"type:text" json:"greetings"`        // 备用开场（JSON 数组字符串）
	BookID       uint   `gorm:"not null;default:0" json:"book_id"` // 角色自带世界书（character_book）

	// 小团子式养成：角色状态（0~100）
	Mood     int    `gorm:"not null;default:50" json:"mood"`    // 情绪
	Affinity int    `gorm:"not null;default:0" json:"affinity"` // 好感
	Energy   int    `gorm:"not null;default:100" json:"energy"` // 精力
	EmoteMap string `gorm:"type:text" json:"emote_map"`         // 情绪→立绘/表情 JSON（{"笑":"url"}）

	Tags string `gorm:"size:256;default:''" json:"tags"`

	Visibility string `gorm:"size:16;not null;default:private;index" json:"visibility"` // private / public
	Review     string `gorm:"size:16;not null;default:approved;index" json:"review"`    // 私有默认 approved；公开需审核
	Status     string `gorm:"size:16;not null;default:active" json:"status"`            // active / deleted（软删除留证）

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// 运行时
	AuthorName string `gorm:"-" json:"author_name,omitempty"`
}

func (Character) TableName() string { return "oc_characters" }

// ─── 世界书 ───

// WorldBook 世界书（条目化存储，等价酒馆的 Lorebook / World Info）
type WorldBook struct {
	ID      uint   `gorm:"primaryKey" json:"id"`
	UserID  uint   `gorm:"not null;index" json:"user_id"`
	GroupID uint   `gorm:"not null;default:0;index" json:"group_id"` // 0 = 个人世界书
	Title   string `gorm:"size:128;not null" json:"title"`
	Summary string `gorm:"type:text" json:"summary"`
	Version int    `gorm:"not null;default:1" json:"version"`

	// 书级检索与预算（酒馆 Lorebook 设置）
	ScanDepth     int  `gorm:"not null;default:20" json:"scan_depth"`        // 扫描最近多少条发言
	TokenBudget   int  `gorm:"not null;default:800" json:"token_budget"`     // 世界书占用的 token 预算
	RecursiveScan bool `gorm:"not null;default:false" json:"recursive_scan"` // 是否递归触发（条目触发条目）
	// 官方世界（借鉴「小团子」的官方世界玩法）：平台预置世界观，用户可复制为起点
	IsOfficial bool `gorm:"not null;default:false;index" json:"is_official"`

	Visibility string `gorm:"size:16;not null;default:private;index" json:"visibility"`
	Review     string `gorm:"size:16;not null;default:approved;index" json:"review"`
	Status     string `gorm:"size:16;not null;default:active" json:"status"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Entries []WorldEntry `gorm:"-" json:"entries,omitempty"`
}

func (WorldBook) TableName() string { return "oc_world_books" }

// WorldEntry 世界书条目（对齐酒馆 Lorebook 条目的触发机制）
//
// 关键：设定不是整本塞进上下文（那是「固定税」），而是按键触发（「条件开销」）——
// 只有最近对话里出现 key / secondary_key 时，条目才进提示词。
type WorldEntry struct {
	ID      uint   `gorm:"primaryKey" json:"id"`
	BookID  uint   `gorm:"not null;index" json:"book_id"`
	Kind    string `gorm:"size:16;not null;default:world;index" json:"kind"` // world/faction/place/rule/event
	Key     string `gorm:"size:256;not null;index" json:"key"`               // 主触发词（逗号分隔，等价 keys[]）
	Content string `gorm:"type:text" json:"content"`

	// 触发机制
	SecondaryKeys string `gorm:"size:256;default:''" json:"secondary_keys"` // 次触发词（逗号分隔）
	Selective     bool   `gorm:"not null;default:false" json:"selective"`   // 是否要求主/次键同时命中
	Logic         int    `gorm:"not null;default:0" json:"logic"`           // 0=AND ANY 1=NOT ALL 2=NOT ANY 3=AND ALL
	Constant      bool   `gorm:"not null;default:false" json:"constant"`    // 常驻插入（忽略触发词）
	Enabled       bool   `gorm:"not null;default:true" json:"enabled"`
	Probability   int    `gorm:"not null;default:100" json:"probability"` // 触发概率 0~100
	CaseSensitive bool   `gorm:"not null;default:false" json:"case_sensitive"`
	WholeWord     bool   `gorm:"not null;default:false" json:"whole_word"`

	// 排序与预算：insertion_order 小者优先；priority 小者在预算不足时先被丢弃
	InsertionOrder int `gorm:"not null;default:100" json:"insertion_order"`
	Priority       int `gorm:"not null;default:100" json:"priority"`
	// 位置：0=角色定义前 1=角色定义后 4=对话内指定深度（酒馆 position）
	Position int    `gorm:"not null;default:1" json:"position"`
	Depth    int    `gorm:"not null;default:0" json:"depth"` // 扫描覆盖（0=用书级 scan_depth）
	Group    string `gorm:"size:32;default:''" json:"group"` // 包含组：同组只取其一

	SortOrder int       `gorm:"not null;default:0" json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Position 常量（与酒馆 position 对齐）
const (
	PosBeforeChar = 0
	PosAfterChar  = 1
	PosInChat     = 4
)

func (WorldEntry) TableName() string { return "oc_world_entries" }

// ─── 群级 OC 配置 ───

// GroupSetting 群组的 OC 玩法配置
type GroupSetting struct {
	ID      uint   `gorm:"primaryKey" json:"id"`
	GroupID uint   `gorm:"not null;uniqueIndex" json:"group_id"`
	Enabled bool   `gorm:"not null;default:false" json:"enabled"`
	Mode    string `gorm:"size:16;not null;default:mixed" json:"mode"` // mixed / ai_only

	// 审核：多条一起审（窗口）
	AuditLevel   string `gorm:"size:16;not null;default:strict" json:"audit_level"` // strict / loose / redteam
	AuditTiming  string `gorm:"size:16;not null;default:post" json:"audit_timing"`  // pre / post
	WindowSize   int    `gorm:"not null;default:5" json:"window_size"`              // 每个窗口的发言条数
	WindowRounds int    `gorm:"not null;default:0" json:"window_rounds"`            // 0=按条数；>0=按轮次

	// 预算（防成本失控）
	DailyBudget int  `gorm:"not null;default:20000" json:"daily_budget"` // 群级每日预算（代币）
	FlowBudget  int  `gorm:"not null;default:0" json:"flow_budget"`      // 单流上限（0=不限）
	MaxTurns    int  `gorm:"not null;default:40" json:"max_turns"`       // 单流最大轮次
	AutoRun     bool `gorm:"not null;default:false" json:"auto_run"`     // 是否允许自动连续推进（纯 AI 群）

	// 红队模式授权留痕
	RedTeamEnabled bool   `gorm:"not null;default:false" json:"redteam_enabled"`
	RedTeamNote    string `gorm:"size:256;default:''" json:"redteam_note"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (GroupSetting) TableName() string { return "oc_group_settings" }

// ─── 群内 AI 角色 ───

// Agent 群内 AI 角色（角色卡 × 群 的绑定实例）
type Agent struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	GroupID     uint      `gorm:"not null;index" json:"group_id"`
	CharacterID uint      `gorm:"not null;index" json:"character_id"`
	ModelID     uint      `gorm:"not null;default:0" json:"model_id"` // 0 = 走池子默认
	TemplateID  uint      `gorm:"not null;default:0" json:"template_id"`
	Strategy    string    `gorm:"size:32;default:''" json:"strategy"` // 发言策略/额外提示
	Enabled     bool      `gorm:"not null;default:true" json:"enabled"`
	Budget      int       `gorm:"not null;default:0" json:"budget"` // 该角色单流预算（0=不限）
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// 运行时
	CharacterName string `gorm:"-" json:"character_name,omitempty"`
	ModelName     string `gorm:"-" json:"model_name,omitempty"`
}

func (Agent) TableName() string { return "oc_agents" }

// ─── 对话流 ───

// Flow 对话流/剧情线
type Flow struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	GroupID    uint      `gorm:"not null;index" json:"group_id"`
	StarterID  uint      `gorm:"not null" json:"starter_id"`
	Title      string    `gorm:"size:128;default:''" json:"title"`
	BookID     uint      `gorm:"not null;default:0" json:"book_id"`
	Schedule   string    `gorm:"size:16;not null;default:round_robin" json:"schedule"`
	Status     string    `gorm:"size:16;not null;default:running;index" json:"status"`
	TurnCount  int       `gorm:"not null;default:0" json:"turn_count"`
	MaxTurns   int       `gorm:"not null;default:40" json:"max_turns"`
	StopReason string    `gorm:"size:64;default:''" json:"stop_reason"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (Flow) TableName() string { return "oc_flows" }

// Turn 单轮发言
type Turn struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	FlowID      uint   `gorm:"not null;index" json:"flow_id"`
	GroupID     uint   `gorm:"not null;index" json:"group_id"`
	Seq         int    `gorm:"not null;index" json:"seq"` // 轮次序号（从 1 开始）
	CharacterID uint   `gorm:"not null;default:0" json:"character_id"`
	AgentID     uint   `gorm:"not null;default:0" json:"agent_id"`
	UserID      uint   `gorm:"not null;default:0" json:"user_id"` // 真人发言时填（0=AI）
	Content     string `gorm:"type:text" json:"content"`
	ModelID     uint   `gorm:"not null;default:0" json:"model_id"`
	ModelName   string `gorm:"size:64;default:''" json:"model_name"`
	TokensIn    int    `json:"tokens_in"`
	TokensOut   int    `json:"tokens_out"`
	Cost        int    `json:"cost"`
	// 情绪标记 [情绪:笑]（小团子式立绘/气泡表情 + 状态养成）
	Emotion string `gorm:"size:16;default:''" json:"emotion"`
	// 审核：pending/approved/rejected；hidden = 命中后被隐藏（先发后审）
	Review    string    `gorm:"size:16;not null;default:approved;index" json:"review"`
	WindowID  uint      `gorm:"not null;default:0;index" json:"window_id"`
	CreatedAt time.Time `json:"created_at"`

	CharacterName string `gorm:"-" json:"character_name,omitempty"`
	IsAI          bool   `gorm:"-" json:"is_ai"`
}

func (Turn) TableName() string { return "oc_turns" }

// ─── 审核窗口 ───

// Window 审核窗口（多条一起审）
type Window struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	FlowID    uint      `gorm:"not null;index" json:"flow_id"`
	GroupID   uint      `gorm:"not null;index" json:"group_id"`
	FromSeq   int       `json:"from_seq"`
	ToSeq     int       `json:"to_seq"`
	TurnCount int       `json:"turn_count"`
	Level     string    `gorm:"size:16" json:"level"`                           // 审核等级快照
	Result    string    `gorm:"size:16;not null;default:pending" json:"result"` // pending/pass/hit
	Hits      string    `gorm:"type:text" json:"hits"`                          // 命中规则 JSON
	Note      string    `gorm:"size:256;default:''" json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

func (Window) TableName() string { return "oc_windows" }

// ─── 预算消耗 ───

// BudgetUsage 预算消耗（按群 + 按日累计）
type BudgetUsage struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	GroupID   uint      `gorm:"not null;uniqueIndex:idx_oc_budget" json:"group_id"`
	Day       string    `gorm:"size:10;not null;uniqueIndex:idx_oc_budget" json:"day"` // YYYY-MM-DD
	Cost      int       `gorm:"not null;default:0" json:"cost"`
	Turns     int       `gorm:"not null;default:0" json:"turns"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (BudgetUsage) TableName() string { return "oc_budget_usage" }

// ─── 归一化 ───

func NormalizeMode(m string) string {
	if strings.ToLower(strings.TrimSpace(m)) == ModeAIOnly {
		return ModeAIOnly
	}
	return ModeMixed
}

func NormalizeAuditLevel(l string) string {
	switch strings.ToLower(strings.TrimSpace(l)) {
	case AuditLoose:
		return AuditLoose
	case AuditRedTeam:
		return AuditRedTeam
	default:
		return AuditStrict
	}
}

func NormalizeAuditTiming(t string) string {
	if strings.ToLower(strings.TrimSpace(t)) == AuditPre {
		return AuditPre
	}
	return AuditPost
}

func NormalizeSchedule(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case ScheduleRandom:
		return ScheduleRandom
	case ScheduleTrigger:
		return ScheduleTrigger
	case ScheduleManual:
		return ScheduleManual
	default:
		return ScheduleRoundRobin
	}
}

func NormalizeVisibility(v string) string {
	if strings.ToLower(strings.TrimSpace(v)) == VisPublic {
		return VisPublic
	}
	return VisPrivate
}

func NormalizeEntryKind(k string) string {
	switch strings.ToLower(strings.TrimSpace(k)) {
	case EntryFaction, EntryPlace, EntryRule, EntryEvent:
		return strings.ToLower(strings.TrimSpace(k))
	default:
		return EntryWorld
	}
}

// Migrate 建表（独立功能包，由 main 在主迁移后调用）
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&Character{}, &WorldBook{}, &WorldEntry{},
		&GroupSetting{}, &Agent{}, &Flow{}, &Turn{}, &Window{}, &BudgetUsage{},
	)
}
