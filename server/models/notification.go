package models

import (
	"time"
)

// Notification 用户消息通知（站内信）。
// 触发场景：帖子被回复、评论被回复、作品/博客收到新评论等。
// 接收者 UserID 必填；ActorID 为触发者（可为 0，如系统通知）。
type Notification struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index:idx_notifications_user_read_created,priority:1" json:"user_id"` // 接收者
	ActorID   uint      `gorm:"default:0" json:"actor_id"`                          // 触发者（0=系统）
	ActorName string    `gorm:"size:64;default:''" json:"actor_name"`               // 触发者昵称（冗余，免联表）
	Type      string    `gorm:"size:32;not null;index" json:"type"`                 // thread_reply / comment_reply / novel_comment / blog_comment
	Title     string    `gorm:"size:256;default:''" json:"title"`
	Content   string    `gorm:"size:512;default:''" json:"content"` // 摘要
	Link      string    `gorm:"size:512;default:''" json:"link"`    // 跳转地址
	Read      bool      `gorm:"default:false;index:idx_notifications_user_read_created,priority:2" json:"read"`
	CreatedAt time.Time `gorm:"index:idx_notifications_user_read_created,priority:3" json:"created_at"`
}

func (Notification) TableName() string { return "notifications" }

// PushNotification 创建一条站内通知（best-effort，失败不影响主流程）。
// 自动排除：接收者为 0、触发者就是接收者（自己不通知自己）。
func PushNotification(actorID, targetUserID uint, notifType, title, content, link string) error {
	if targetUserID == 0 || (actorID != 0 && actorID == targetUserID) {
		return nil
	}
	actorName := ""
	if actorID != 0 {
		var u User
		if err := DB.Select("nickname", "username").First(&u, actorID).Error; err == nil {
			actorName = u.Nickname
			if actorName == "" {
				actorName = u.Username
			}
		}
	}
	n := &Notification{
		UserID: targetUserID, ActorID: actorID, ActorName: actorName,
		Type: notifType, Title: title, Content: content, Link: link,
	}
	return DB.Create(n).Error
}

// CountUnreadNotifications 未读通知数
func CountUnreadNotifications(userID uint) (int64, error) {
	var count int64
	err := DB.Model(&Notification{}).Where("user_id = ? AND read = ?", userID, false).Count(&count).Error
	return count, err
}

// ListNotifications 分页列出通知（未读优先，再按时间倒序）
func ListNotifications(userID uint, page, pageSize int) ([]Notification, int64, error) {
	var list []Notification
	var total int64
	query := DB.Model(&Notification{}).Where("user_id = ?", userID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	err := query.Order("read ASC, created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

// MarkNotificationRead 标记单条已读；id=0 时标记该用户全部已读。返回受影响行数。
func MarkNotificationRead(userID uint, id uint) (int64, error) {
	query := DB.Model(&Notification{}).Where("user_id = ?", userID)
	if id != 0 {
		query = query.Where("id = ?", id)
	}
	res := query.Update("read", true)
	return res.RowsAffected, res.Error
}

// CleanupOldNotifications 清理超过 90 天的已读通知（保留未读与近期）。
func CleanupOldNotifications() {
	cutoff := time.Now().AddDate(0, 0, -90)
	DB.Where("read = ? AND created_at < ?", true, cutoff).Delete(&Notification{})
}
