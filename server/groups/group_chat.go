package groups

import (
	"time"

	"nvs-server/models"
)

// GroupChatMessage 群聊消息（类微信/QQ 的群内聊天）。
// 与 GroupPost（帖子）互补：帖子偏主题讨论，聊天偏即时交流。
type GroupChatMessage struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	GroupID   uint      `gorm:"not null;index:idx_gcm_group,priority:1" json:"group_id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	Content   string    `gorm:"type:text" json:"content"`
	ReplyToID uint      `gorm:"not null;default:0" json:"reply_to_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (GroupChatMessage) TableName() string { return "group_chat_messages" }

// ListChatMessages 取群聊消息（按时间正序，最多 limit 条）
func ListChatMessages(groupID uint, limit int) ([]GroupChatMessage, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var list []GroupChatMessage
	err := models.DB.Where("group_id = ?", groupID).Order("id DESC").Limit(limit).Find(&list).Error
	if err != nil {
		return nil, err
	}
	// 反转为正序返回
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
	return list, nil
}

// CreateChatMessage 写入群聊消息
func CreateChatMessage(m *GroupChatMessage) error {
	return models.DB.Create(m).Error
}
