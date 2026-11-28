package models

import "time"

// Poll 议题投票：作者可挂到博客/微文/论坛帖子等载体上，支持发起与终止。
// TargetKind: blog / micro / forum；TargetID 为对应内容 ID。
type Poll struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	TargetKind  string     `gorm:"size:24;not null;index:idx_poll_target" json:"target_kind"`
	TargetID    uint       `gorm:"not null;index:idx_poll_target" json:"target_id"`
	AuthorID    uint       `gorm:"not null;index" json:"author_id"`
	Question    string     `gorm:"size:512;not null" json:"question"`
	Description string     `gorm:"type:text" json:"description"`
	Multiple    bool       `gorm:"default:false" json:"multiple"`            // 是否多选
	Anonymous   bool       `gorm:"default:false" json:"anonymous"`           // 是否匿名投票（不展示投票者）
	Status      string     `gorm:"size:16;default:open;index" json:"status"` // open / closed
	ClosedAt    *time.Time `json:"closed_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`

	Options []PollOption `gorm:"foreignKey:PollID" json:"options,omitempty"`
}

func (Poll) TableName() string { return "polls" }

// PollOption 投票选项。
type PollOption struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	PollID uint   `gorm:"not null;index" json:"poll_id"`
	Label  string `gorm:"size:256;not null" json:"label"`
	Sort   int    `gorm:"default:0" json:"sort"`
	Votes  int    `gorm:"default:0" json:"votes"` // 冗余计数，避免每次 COUNT
}

func (PollOption) TableName() string { return "poll_options" }

// PollVote 用户投票记录：一用户一选项一条（多选则多条）。
type PollVote struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	PollID    uint      `gorm:"not null;index:idx_poll_vote" json:"poll_id"`
	OptionID  uint      `gorm:"not null;index" json:"option_id"`
	UserID    uint      `gorm:"not null;index:idx_poll_vote" json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (PollVote) TableName() string { return "poll_votes" }

// 状态常量
const (
	PollStatusOpen   = "open"
	PollStatusClosed = "closed"
)
