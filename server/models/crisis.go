package models

import "time"

// CrisisEvent 未成年人心理危机事件留痕（供站长生成《高危用户心理干预报告》）。
//
// 与 BadWordFlag / SafetyEvent 的区别：这里只聚焦「心理危机」这一类信号，
// 字段更贴近干预场景（来源、动作、片段），便于站长快速判断是否需要人工介入。
type CrisisEvent struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"not null;index:idx_crisis_user_created,priority:1" json:"user_id"`
	Username    string    `gorm:"size:64" json:"username"`
	Source      string    `gorm:"size:16;index" json:"source"` // keyword（关键词命中）/ ai（模型语义判危）
	ContentType string    `gorm:"size:32" json:"content_type"` // comment / forum / ...
	ContentID   uint      `gorm:"index" json:"content_id"`     // 触发内容 id（如评论 id）
	Snippet     string    `gorm:"type:text" json:"snippet"`    // 触发片段（截断）
	Action      string    `gorm:"size:16" json:"action"`       // intercept（自动折叠）/ review（先审后发）
	Reason      string    `gorm:"type:text" json:"reason"`     // AI 判危说明
	CreatedAt   time.Time `gorm:"index:idx_crisis_user_created,priority:2" json:"created_at"`
}

func (CrisisEvent) TableName() string { return "crisis_events" }

// CreateCrisisEvent 写一条危机事件留痕。
func CreateCrisisEvent(e *CrisisEvent) error {
	return DB.Create(e).Error
}

// ListCrisisEventsByUser 取某用户的全部危机事件（倒序）。
func ListCrisisEventsByUser(userID uint) ([]CrisisEvent, error) {
	var list []CrisisEvent
	err := DB.Where("user_id = ?", userID).Order("created_at DESC").Find(&list).Error
	return list, err
}

// CountCrisisEventsByUser 统计某用户危机事件总数。
func CountCrisisEventsByUser(userID uint) (int64, error) {
	var n int64
	err := DB.Model(&CrisisEvent{}).Where("user_id = ?", userID).Count(&n).Error
	return n, err
}

// ListUsersInObservation 取当前处于高危观察期的用户（供站长总览）。
func ListUsersInObservation() ([]User, error) {
	var users []User
	err := DB.Where("risk_youth_mental = ?", true).Order("risk_youth_mental_until DESC").Find(&users).Error
	return users, err
}
