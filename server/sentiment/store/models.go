// Package store 舆情子系统的持久化层（表结构与读写）。
//
// 与 aisafety（AI 对话内容安全闸）分开：那边管「模型被越狱」，
// 这边管「社区舆论场」，两套阈值、两套词表、两套处置，混在一起必然互相拖累。
package store

import (
	"time"

	"gorm.io/gorm"
)

// 内容类型（评论守卫要判断「这条评论挂在什么内容下」）
const (
	ContentNovel = "novel"
	ContentBlog  = "blog"
	ContentForum = "forum"
)

// 词条来源
const (
	SourceBaseline = "baseline" // 内置基线
	SourceAI       = "ai"       // 每日自动学习
	SourceManual   = "manual"   // 站长手工添加
)

// 词条状态
const (
	StatusActive   = "active"   // 生效
	StatusPending  = "pending"  // 待审（AI 学来的默认先进这里）
	StatusRejected = "rejected" // 已否决
)

// 话题策略覆盖
const (
	PolicyInherit = ""      // 继承全局（按风险判定）
	PolicyBlock   = "block" // 强制拦
	PolicyFlag    = "flag"  // 仅标注
	PolicyOff     = "off"   // 不处置
)

// 复核状态
const (
	ReviewAuto        = "auto"         // AI 自动，未人工复核
	ReviewConfirmed   = "confirmed"    // 已人工确认
	ReviewNeedsReview = "needs_review" // 置信度低，待人工
)

// 处置动作
const (
	ActionAllow = "allow"
	ActionFlag  = "flag"
	ActionBlock = "block"
)

// ContentTopic 内容的话题标注。由 AI 审核写入，也可人工修订。
//
// 为什么单独存表而不是直接读作品的 Tags：
// 作品的 Tags 是作者自己写的分类（玄幻/都市），与「舆论话题」不是一回事；
// AI 敲定的标签才是风险判断依据。两者都要，但语义必须分开。
type ContentTopic struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	ContentType string `gorm:"size:16;not null;uniqueIndex:idx_sentiment_topic,priority:1" json:"content_type"`
	ContentID   uint   `gorm:"not null;uniqueIndex:idx_sentiment_topic,priority:2" json:"content_id"`
	// Tags AI 敲定的话题标签（JSON 数组）
	Tags string `gorm:"type:text" json:"tags"`
	// Groups 归类后的话题组（JSON 数组，避免每次重算）
	Groups string `gorm:"size:255" json:"groups"`
	// BaseRisk 话题组合得出的基线风险
	BaseRisk string `gorm:"size:16;default:low" json:"base_risk"`
	// Risk 叠加文本升档后的最终风险（写入时的快照）
	Risk string `gorm:"size:16;default:low" json:"risk"`
	// PublicOpinionScore 高风险舆论可能性 0-100
	PublicOpinionScore int `gorm:"default:0" json:"public_opinion_score"`
	// PublicOpinionHigh 是否高风险舆论
	PublicOpinionHigh bool `gorm:"default:false;index" json:"public_opinion_high"`
	// Source ai / manual
	Source string `gorm:"size:16;default:ai" json:"source"`
	// Confidence AI 敲定的置信度 0-100。低于阈值时**只能标注、不能拦截**
	// —— 标签是拦截策略的开关，标签错一次，整段评论区的词都会被误杀。
	Confidence int `gorm:"default:0" json:"confidence"`
	// ReviewStatus 复核状态：auto（AI 自动，未复核）/ confirmed（已人工确认）/ needs_review（置信度低，待人工）
	ReviewStatus string `gorm:"size:16;default:auto;index" json:"review_status"`
	// Policy 本站对该话题的策略覆盖：空=继承全局（按风险判定）/ block=强制拦 / flag=只标注 / off=不处置
	Policy string `gorm:"size:16;default:''" json:"policy"`
	// ReviewedBy 最后修订人（0=从未人工修订）
	ReviewedBy uint       `gorm:"default:0" json:"reviewed_by"`
	ReviewedAt *time.Time `json:"reviewed_at,omitempty"`
	UpdatedAt  time.Time  `json:"updated_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

func (ContentTopic) TableName() string { return "sentiment_content_topics" }

// Term 舆情词条（后台可维护；AI 学来的先进 pending）。
type Term struct {
	ID   uint   `gorm:"primaryKey" json:"id"`
	Word string `gorm:"size:128;not null;uniqueIndex" json:"word"`
	Cat  string `gorm:"size:16;default:slang;index" json:"cat"`
	// EscalateUnder 在哪些话题组下升级为拦截（JSON 数组；空=只标注）
	EscalateUnder string `gorm:"size:255;default:''" json:"escalate_under"`
	Severity      int    `gorm:"default:1" json:"severity"`
	// ActionOverride 强制动作：空=按话题判定；block=无视话题直接拦
	ActionOverride string `gorm:"size:16;default:''" json:"action_override"`
	Source         string `gorm:"size:16;default:manual;index" json:"source"`
	Status         string `gorm:"size:16;default:active;index" json:"status"`
	// HitCount 累计命中（用于后台按热度排序、清理无用词）
	HitCount int64 `gorm:"default:0" json:"hit_count"`
	// Confidence 词条置信度 0-100（AI 学来的带；人工添加视为 100）
	Confidence int `gorm:"default:100" json:"confidence"`
	// ReviewCycleDays 复核周期（天）：黑话会漂移，绑关系不能一绑永逸
	ReviewCycleDays int `gorm:"default:90" json:"review_cycle_days"`
	// ReviewedAt 上次复核时间；据此算「到期未复核」
	ReviewedAt *time.Time `json:"reviewed_at,omitempty"`
	Note       string     `gorm:"size:255;default:''" json:"note"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (Term) TableName() string { return "sentiment_terms" }

// Hit 一次裁决记录（拦截与标注都记，便于警方台看「舆情拦截数目」）。
type Hit struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	ContentType string `gorm:"size:16;index" json:"content_type"`
	ContentID   uint   `gorm:"index" json:"content_id"`
	CommentID   uint   `gorm:"default:0;index" json:"comment_id"`
	UserID      uint   `gorm:"index" json:"user_id"`
	Username    string `gorm:"size:64;default:''" json:"username"`
	// Decision allow/flag/block
	Decision string `gorm:"size:16;index" json:"decision"`
	// Risk 本次话题风险快照
	Risk string `gorm:"size:16;index" json:"risk"`
	// Groups 本次话题组（逗号分隔，便于分组统计）
	Groups string `gorm:"size:128;default:'';index" json:"groups"`
	// MatchedWords 命中的词（逗号分隔）
	MatchedWords string `gorm:"size:255;default:''" json:"matched_words"`
	// VariantWords 其中靠变形才命中的（取证用）
	VariantWords string `gorm:"size:255;default:''" json:"variant_words"`
	// Flood 行为特征摘要（刷屏/出征）
	Flood string `gorm:"size:128;default:''" json:"flood"`
	// Reason 判定理由（后台展示）
	Reason    string    `gorm:"size:255;default:''" json:"reason"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

func (Hit) TableName() string { return "sentiment_hits" }

// TermAudit 词条变更审计：谁改的、改了什么、为什么。
//
// 绑定关系会直接影响审核结果，没有审计就没法追责——
// 尤其是「AI 自动入库」这类改动，事后必须能查到来源。
type TermAudit struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	TermID     uint      `gorm:"index" json:"term_id"`
	Word       string    `gorm:"size:128" json:"word"`
	Action     string    `gorm:"size:32" json:"action"` // create/update/bind/unbind/approve/reject/review
	OperatorID uint      `gorm:"default:0" json:"operator_id"`
	Operator   string    `gorm:"size:64;default:''" json:"operator"`
	Note       string    `gorm:"size:255;default:''" json:"note"`
	Before     string    `gorm:"type:text" json:"before"`
	After      string    `gorm:"type:text" json:"after"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
}

func (TermAudit) TableName() string { return "sentiment_term_audits" }

// Migrate 建表。由 main 在启动时调用（失败只记日志、不阻断启动）。
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&ContentTopic{}, &Term{}, &Hit{}, &Config{}, &TermAudit{})
}
