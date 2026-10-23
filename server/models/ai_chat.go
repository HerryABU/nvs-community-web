package models

import (
	"strings"
	"time"
)

// ==================== AI 工作台会话（§5.4） ====================
// 会话持久化：AiChatSession（会话概览）+ AiChatMessage（消息明细）。
// 助手消息带 tokens/成本，与代币流水一一对应（ref_type=ai, ref_id=call_log.id）。

// AiChatSession AI 会话
type AiChatSession struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index:idx_ai_session_user,priority:1" json:"user_id"`
	AgentID   uint      `gorm:"not null;default:0;index" json:"agent_id"` // 0 = 无智能体
	AgentName string    `gorm:"size:64;default:''" json:"agent_name"`
	ModelID   uint      `gorm:"not null;default:0" json:"model_id"`
	Title     string    `gorm:"size:128;default:''" json:"title"`
	CreatedAt time.Time `gorm:"index:idx_ai_session_user,priority:2" json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (AiChatSession) TableName() string { return "ai_chat_sessions" }

// AiChatMessage 会话消息
type AiChatMessage struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	SessionID uint      `gorm:"not null;index:idx_ai_msg_session,priority:1" json:"session_id"`
	Role      string    `gorm:"size:16;not null" json:"role"` // user / assistant
	Content   string    `gorm:"type:text" json:"content"`
	TokensIn  int       `gorm:"not null;default:0" json:"tokens_in"`
	TokensOut int       `gorm:"not null;default:0" json:"tokens_out"`
	Cost      int64     `gorm:"not null;default:0" json:"cost"`
	CallLogID uint      `gorm:"not null;default:0" json:"call_log_id"` // 关联 AiCallLog（对账）
	CreatedAt time.Time `gorm:"index:idx_ai_msg_session,priority:2" json:"created_at"`
}

func (AiChatMessage) TableName() string { return "ai_chat_messages" }

// ---------- 会话 ----------

// CreateAiChatSession 建会话（标题可空，首条消息后回填）
func CreateAiChatSession(userID, agentID uint, agentName string, modelID uint) (*AiChatSession, error) {
	s := &AiChatSession{
		UserID: userID, AgentID: agentID, AgentName: agentName, ModelID: modelID,
		Title: "新对话", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := DB.Create(s).Error; err != nil {
		return nil, err
	}
	return s, nil
}

// ListAiChatSessions 我的会话（倒序，限流保护）
func ListAiChatSessions(userID uint, limit int) ([]AiChatSession, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	var list []AiChatSession
	err := DB.Where("user_id = ?", userID).Order("updated_at DESC").Limit(limit).Find(&list).Error
	return list, err
}

// GetAiChatSession 取会话（校验归属，防越权）
func GetAiChatSession(id, userID uint) (*AiChatSession, error) {
	var s AiChatSession
	if err := DB.Where("id = ? AND user_id = ?", id, userID).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// DeleteAiChatSession 删除会话（消息一并删）
func DeleteAiChatSession(id, userID uint) error {
	if err := DB.Where("id = ? AND user_id = ?", id, userID).Delete(&AiChatSession{}).Error; err != nil {
		return err
	}
	return DB.Where("session_id = ?", id).Delete(&AiChatMessage{}).Error
}

// TouchAiChatSession 刷新会话时间；首条用户消息时顺带定标题
func TouchAiChatSession(id uint, firstUserText string) {
	updates := map[string]interface{}{"updated_at": time.Now()}
	var s AiChatSession
	if err := DB.Where("id = ?", id).First(&s).Error; err == nil && (s.Title == "" || s.Title == "新对话") {
		title := strings.TrimSpace(firstUserText)
		if len([]rune(title)) > 24 {
			title = string([]rune(title)[:24]) + "…"
		}
		if title != "" {
			updates["title"] = title
		}
	}
	DB.Model(&AiChatSession{}).Where("id = ?", id).Updates(updates)
}

// ---------- 消息 ----------

// AppendAiChatMessage 落一条消息
func AppendAiChatMessage(sessionID uint, role, content string, tokensIn, tokensOut int, cost int64, callLogID uint) (*AiChatMessage, error) {
	m := &AiChatMessage{
		SessionID: sessionID, Role: role, Content: content,
		TokensIn: tokensIn, TokensOut: tokensOut, Cost: cost, CallLogID: callLogID,
		CreatedAt: time.Now(),
	}
	if err := DB.Create(m).Error; err != nil {
		return nil, err
	}
	return m, nil
}

// ListAiChatMessages 会话消息（正序，limit 保护）
func ListAiChatMessages(sessionID uint, limit int) ([]AiChatMessage, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var list []AiChatMessage
	err := DB.Where("session_id = ?", sessionID).Order("created_at ASC, id ASC").Limit(limit).Find(&list).Error
	return list, err
}

// CountAiChatMessages 会话消息数（判断是否首条）
func CountAiChatMessages(sessionID uint) (int64, error) {
	var n int64
	err := DB.Model(&AiChatMessage{}).Where("session_id = ?", sessionID).Count(&n).Error
	return n, err
}
