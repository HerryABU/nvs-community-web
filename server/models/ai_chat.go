package models

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// ==================== AI 工作台会话（§5.4） ====================
// 会话持久化：AiChatSession（会话概览）+ AiChatMessage（消息明细）。
// 助手消息带 tokens/成本，与代币流水一一对应（ref_type=ai, ref_id=call_log.id）。

// AiChatSession AI 会话
type AiChatSession struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	UserID    uint   `gorm:"not null;index:idx_ai_session_user,priority:1" json:"user_id"`
	AgentID   uint   `gorm:"not null;default:0;index" json:"agent_id"` // 0 = 无智能体
	AgentName string `gorm:"size:64;default:''" json:"agent_name"`
	ModelID   uint   `gorm:"not null;default:0" json:"model_id"`
	Title     string `gorm:"size:128;default:''" json:"title"`
	// ThirdConvID 第三方会话 ID（扣子 conversation_id；用于多轮上下文续接）
	ThirdConvID string `gorm:"size:64;default:''" json:"-"`
	// ShareToken 分享令牌（nil = 未分享）；非 nil 即公开可读（只读快照）
	ShareToken *string    `gorm:"size:32;unique" json:"share_token,omitempty"`
	SharedAt   *time.Time `json:"shared_at,omitempty"`

	// ── 内容安全终止 ──
	// 判定有害后整个会话被锁死，而不是只拦当轮。
	// 理由：一旦模型已经吐出配方，这段内容就已经进了上下文，
	// 后续任何一句「继续」「然后呢」都会在污染过的上下文里接着生成。
	Locked     bool       `gorm:"not null;default:false;index" json:"locked"`
	LockedAt   *time.Time `json:"locked_at,omitempty"`
	LockReason string     `gorm:"size:255;default:''" json:"lock_reason"`
	LockedBy   uint       `gorm:"not null;default:0" json:"locked_by"` // 0=系统审核判定，>0=管理员手动

	// Contaminated 上下文已污染（中档模式）。
	// 与 Locked 的区别：会话还能继续聊，但每轮都会向模型注入「前面有被拦截的内容，
	// 不要延续/补全/换个说法恢复那个话题」的提示。适用于「一次误判就锁死太狠、
	// 但完全不提醒又会被顺着继续」的场景。
	Contaminated   bool       `gorm:"not null;default:false;index" json:"contaminated"`
	ContaminatedAt *time.Time `json:"contaminated_at,omitempty"`

	// ── 对话分支 ──
	// 当前活跃叶子：显示哪条分支的末端。0 = 老数据/空会话。
	ActiveLeafID uint      `gorm:"not null;default:0" json:"active_leaf_id"`
	CreatedAt    time.Time `gorm:"index:idx_ai_session_user,priority:2" json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (AiChatSession) TableName() string { return "ai_chat_sessions" }

// AiChatMessage 会话消息
type AiChatMessage struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	SessionID uint   `gorm:"not null;index:idx_ai_msg_session,priority:1" json:"session_id"`
	Role      string `gorm:"size:16;not null" json:"role"` // user / assistant
	Content   string `gorm:"type:text" json:"content"`
	TokensIn  int    `gorm:"not null;default:0" json:"tokens_in"`
	TokensOut int    `gorm:"not null;default:0" json:"tokens_out"`
	Cost      int64  `gorm:"not null;default:0" json:"cost"`
	CallLogID uint   `gorm:"not null;default:0" json:"call_log_id"` // 关联 AiCallLog（对账）
	// ParentID 上一条消息（0 = 会话根）。
	// 有它才能做分支：对同一条消息「重新生成」或「编辑后重发」时，
	// 新消息与原消息共享同一个 parent，于是成为兄弟分支，可左右切换。
	ParentID uint `gorm:"not null;default:0;index" json:"parent_id"`
	// SafetyBlocked 这条助手消息是安全闸拦截后写入的占位内容（不是模型的真实回答）。
	// 前端据此渲染成「警告块」而不是普通气泡 —— 否则用户会以为 AI 真的这样回答了。
	SafetyBlocked bool      `gorm:"not null;default:false" json:"safety_blocked"`
	CreatedAt     time.Time `gorm:"index:idx_ai_msg_session,priority:2" json:"created_at"`
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

// RenameAiChatSession 重命名会话（校验归属）
func RenameAiChatSession(id, userID uint, title string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return fmt.Errorf("标题不能为空")
	}
	if len([]rune(title)) > 60 {
		title = string([]rune(title)[:60])
	}
	res := DB.Model(&AiChatSession{}).Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]interface{}{"title": title, "updated_at": time.Now()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("会话不存在")
	}
	return nil
}

// ShareAiChatSession 生成分享令牌（幂等：已分享则返回原令牌）
func ShareAiChatSession(id, userID uint) (string, error) {
	var s AiChatSession
	if err := DB.Where("id = ? AND user_id = ?", id, userID).First(&s).Error; err != nil {
		return "", fmt.Errorf("会话不存在")
	}
	if s.ShareToken != nil && s.SharedAt != nil {
		return *s.ShareToken, nil
	}
	// 唯一令牌（32 位 hex，撞库重试）
	var token string
	for i := 0; i < 3; i++ {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		token = hex.EncodeToString(b)
		var n int64
		DB.Model(&AiChatSession{}).Where("share_token = ?", token).Count(&n)
		if n == 0 {
			break
		}
		token = ""
	}
	if token == "" {
		return "", fmt.Errorf("生成分享令牌失败，请重试")
	}
	now := time.Now()
	tk := token
	if err := DB.Model(&AiChatSession{}).Where("id = ?", id).
		Updates(map[string]interface{}{"share_token": &tk, "shared_at": &now, "updated_at": now}).Error; err != nil {
		return "", err
	}
	return token, nil
}

// UnshareAiChatSession 取消分享（令牌置空，链接立即失效）
func UnshareAiChatSession(id, userID uint) error {
	res := DB.Model(&AiChatSession{}).Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]interface{}{"share_token": nil, "shared_at": nil})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("会话不存在")
	}
	return nil
}

// GetSharedSessionByToken 公开读取已分享会话（无归属校验，仅限 shared_at 非空）
func GetSharedSessionByToken(token string) (*AiChatSession, error) {
	if len(token) != 32 {
		return nil, fmt.Errorf("分享链接无效")
	}
	var s AiChatSession
	if err := DB.Where("share_token = ? AND shared_at IS NOT NULL", token).First(&s).Error; err != nil {
		return nil, fmt.Errorf("分享不存在或已取消")
	}
	return &s, nil
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

// SetSessionThirdConv 记录第三方会话 ID
func SetSessionThirdConv(sessionID uint, convID string) {
	if sessionID == 0 || strings.TrimSpace(convID) == "" {
		return
	}
	DB.Model(&AiChatSession{}).Where("id = ?", sessionID).UpdateColumn("third_conv_id", convID)
}
