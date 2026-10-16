package models

import (
	"time"

	"gorm.io/gorm"
)

// recallWindow 撤回时间窗口：发送后 2 分钟内可撤回
const recallWindow = 2 * time.Minute

// Message 站内私信（F20）。
// 单方删除模型：发送方删除只影响发送方视图（deleted_by_sender），
// 接收方删除只影响接收方视图（deleted_by_receiver），双方互不影响。
// 撤回（recalled）对双方同时生效，内容仍留存备查但不再展示。
type Message struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	SenderID          uint       `gorm:"not null;index:idx_msg_pair;index:idx_msg_sender" json:"sender_id"`     // 发送者
	ReceiverID        uint       `gorm:"not null;index:idx_msg_pair;index:idx_msg_receiver" json:"receiver_id"` // 接收者
	Content           string     `gorm:"type:text" json:"content"`                                              // 内容（handler 层截断到 2000 字符）
	ReadAt            *time.Time `json:"read_at"`                                                               // 接收方已读时间（null=未读）
	Recalled          bool       `gorm:"default:false" json:"recalled"`                                         // 已撤回（2 分钟内发送方可撤）
	DeletedBySender   bool       `gorm:"default:false" json:"-"`                                                // 发送方已删除（单方）
	DeletedByReceiver bool       `gorm:"default:false" json:"-"`                                                // 接收方已删除（单方）
	CreatedAt         time.Time  `gorm:"index" json:"created_at"`
}

func (Message) TableName() string { return "messages" }

// Conversation 会话列表项：对方信息 + 最后一条消息 + 未读数
type Conversation struct {
	PeerID       uint      `json:"peer_id"`
	PeerUsername string    `json:"peer_username"`
	PeerNickname string    `json:"peer_nickname"`
	PeerAvatar   string    `json:"peer_avatar_url"`
	LastMessage  *Message  `json:"last_message"`
	Unread       int64     `json:"unread"`
	LastAt       time.Time `json:"last_at"`
}

// CreateMessage 写入一条私信
func CreateMessage(m *Message) error {
	return DB.Create(m).Error
}

// GetMessageByID 按主键取消息（撤回/权限校验用）
func GetMessageByID(id uint) (*Message, error) {
	var m Message
	if err := DB.First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// visibleScope 我与某人的消息可见范围（排除「我这一侧」已删除的消息）SQL 片段：
// 我发出的未被我删 + 我收到的未被我删。
func visibleWhere(userID uint) (string, []interface{}) {
	return "(sender_id = ? AND deleted_by_sender = ?) OR (receiver_id = ? AND deleted_by_receiver = ?)",
		[]interface{}{userID, false, userID, false}
}

// ListConversations 我的会话列表：按最后一条消息时间倒序。
// 每个会话携带对方用户信息、最后一条消息与未读数。
func ListConversations(userID uint) ([]Conversation, error) {
	// 1. 聚合出所有与我通过消息的对方ID（排除我已删除的一侧），按最后消息时间倒序
	type peerRow struct {
		PeerID uint      `gorm:"column:peer_id"`
		LastAt time.Time `gorm:"column:last_at"`
	}
	cond, args := visibleWhere(userID)
	var rows []peerRow
	err := DB.Model(&Message{}).
		Select("CASE WHEN sender_id = ? THEN receiver_id ELSE sender_id END AS peer_id, MAX(created_at) AS last_at", userID).
		Where(cond, args...).
		Group("peer_id").
		Order("last_at DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []Conversation{}, nil
	}

	// 2. 批量取对方用户信息，避免 N+1
	peerIDs := make([]uint, 0, len(rows))
	for _, r := range rows {
		peerIDs = append(peerIDs, r.PeerID)
	}
	var peers []User
	if err := DB.Where("id IN ?", peerIDs).Find(&peers).Error; err != nil {
		return nil, err
	}
	peerMap := make(map[uint]User, len(peers))
	for _, u := range peers {
		peerMap[u.ID] = u
	}

	// 3. 逐个会话取最后一条消息 + 未读数
	convs := make([]Conversation, 0, len(rows))
	for _, r := range rows {
		var last Message
		// 最后一条消息同样只算「我可见」的（对方删不删不影响我）
		lastCond := "((sender_id = ? AND receiver_id = ? AND deleted_by_sender = ?) OR (sender_id = ? AND receiver_id = ? AND deleted_by_receiver = ?))"
		if err := DB.Where(lastCond, userID, r.PeerID, false, r.PeerID, userID, false).
			Order("id DESC").First(&last).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				continue // 理论上不会：聚合里存在但明细没有（并发删除），跳过
			}
			return nil, err
		}
		var unread int64
		if err := DB.Model(&Message{}).
			Where("sender_id = ? AND receiver_id = ? AND read_at IS NULL AND recalled = ?", r.PeerID, userID, false).
			Count(&unread).Error; err != nil {
			return nil, err
		}
		conv := Conversation{
			PeerID:      r.PeerID,
			LastMessage: &last,
			Unread:      unread,
			LastAt:      r.LastAt,
		}
		if u, ok := peerMap[r.PeerID]; ok {
			conv.PeerUsername = u.Username
			conv.PeerNickname = u.Nickname
			conv.PeerAvatar = u.AvatarURL
		}
		convs = append(convs, conv)
	}
	return convs, nil
}

// ListMessagesBetween 我与某人的聊天记录（分页，按 id 倒序=新消息在前）。
// 排除「我这一侧」已删除的消息；撤回的消息保留记录，前端展示「已撤回」。
func ListMessagesBetween(userID, peerID uint, page, pageSize int) ([]Message, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 30
	}
	cond := "((sender_id = ? AND receiver_id = ? AND deleted_by_sender = ?) OR (sender_id = ? AND receiver_id = ? AND deleted_by_receiver = ?))"
	q := DB.Model(&Message{}).
		Where(cond, userID, peerID, false, peerID, userID, false)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []Message
	if err := DB.Where(cond, userID, peerID, false, peerID, userID, false).
		Order("id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// MarkReadBetween 把对方发给我的未读消息标记为已读，返回受影响行数
func MarkReadBetween(receiverID, senderID uint) (int64, error) {
	now := time.Now()
	res := DB.Model(&Message{}).
		Where("sender_id = ? AND receiver_id = ? AND read_at IS NULL", senderID, receiverID).
		Update("read_at", &now)
	return res.RowsAffected, res.Error
}

// UnreadCount 我的未读私信数（排除已撤回）
func UnreadCount(userID uint) (int64, error) {
	var count int64
	err := DB.Model(&Message{}).
		Where("receiver_id = ? AND read_at IS NULL AND recalled = ?", userID, false).
		Count(&count).Error
	return count, err
}

// CountBetween 两人之间的历史消息总数（第 3 条消息规则用；
// 含撤回与删除记录，防止靠撤回/删除重置计数绕过规则）
func CountBetween(aID, bID uint) (int64, error) {
	var count int64
	err := DB.Model(&Message{}).
		Where("(sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)", aID, bID, bID, aID).
		Count(&count).Error
	return count, err
}

// RecallMessage 撤回消息：仅发送者本人在 2 分钟内可撤，返回受影响行数（0=不满足条件）
func RecallMessage(messageID, senderID uint) (int64, error) {
	deadline := time.Now().Add(-recallWindow)
	res := DB.Model(&Message{}).
		Where("id = ? AND sender_id = ? AND recalled = ? AND created_at >= ?", messageID, senderID, false, deadline).
		Update("recalled", true)
	return res.RowsAffected, res.Error
}

// DeleteConversationBetween 单方删除会话：我发出的标记 deleted_by_sender，
// 我收到的标记 deleted_by_receiver，对方视图不受影响。
func DeleteConversationBetween(userID, peerID uint) error {
	if err := DB.Model(&Message{}).
		Where("sender_id = ? AND receiver_id = ?", userID, peerID).
		Update("deleted_by_sender", true).Error; err != nil {
		return err
	}
	return DB.Model(&Message{}).
		Where("sender_id = ? AND receiver_id = ?", peerID, userID).
		Update("deleted_by_receiver", true).Error
}
