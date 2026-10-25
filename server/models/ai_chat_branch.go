package models

import (
	"time"
)

// ==================== 对话分支（树状会话） ====================
//
// 线性会话存不下「对同一条提问换个模型再答一次」这种结构。
// 这里用最经典的做法：每条消息记 parent_id，会话记 active_leaf_id。
//
//	user:问我           ← parent=0（根）
//	├─ assistant:A1     ← parent=user
//	└─ assistant:A2     ← parent=user（对 A1「重新生成」产生的兄弟分支）
//	     └─ user:追问     ← parent=A2
//
// 显示哪条分支由 active_leaf_id 决定：从叶子沿 parent 回溯就是当前线程。
// 老数据（parent_id 全 0）由 BackfillMessageParents 串成链，行为与以前一致。

// AppendAiChatMessageWithParent 落一条带父节点的消息。
func AppendAiChatMessageWithParent(sessionID uint, role, content string, parentID uint, tokensIn, tokensOut int, cost int64, callLogID uint) (*AiChatMessage, error) {
	m := &AiChatMessage{
		SessionID: sessionID, Role: role, Content: content, ParentID: parentID,
		TokensIn: tokensIn, TokensOut: tokensOut, Cost: cost, CallLogID: callLogID,
		CreatedAt: time.Now(),
	}
	if err := DB.Create(m).Error; err != nil {
		return nil, err
	}
	// ParentID 为 0 时会被 GORM 的零值规则忽略（走列默认 0，结果一样）；
	// 非 0 时上面的 Create 已经写入，这里不用额外处理。
	return m, nil
}

// AppendSafetyBlockedMessage 写一条「被安全闸拦截」的助手消息。
// 与普通消息分开，是为了让前端能把它渲染成警告块而不是普通回答。
func AppendSafetyBlockedMessage(sessionID, parentID uint, text string, callLogID uint) (*AiChatMessage, error) {
	m := &AiChatMessage{
		SessionID: sessionID, Role: "assistant", Content: text, ParentID: parentID,
		SafetyBlocked: true, CallLogID: callLogID, CreatedAt: time.Now(),
	}
	if err := DB.Create(m).Error; err != nil {
		return nil, err
	}
	return m, nil
}

// GetAiChatMessage 取一条消息（带会话归属校验，防止跨会话读取）。
func GetAiChatMessage(id, sessionID uint) (*AiChatMessage, error) {
	var m AiChatMessage
	q := DB.Where("id = ?", id)
	if sessionID > 0 {
		q = q.Where("session_id = ?", sessionID)
	}
	if err := q.First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// SetActiveLeaf 把会话的显示位置切到某个叶子。
func SetActiveLeaf(sessionID, leafID uint) error {
	return DB.Model(&AiChatSession{}).Where("id = ?", sessionID).
		UpdateColumn("active_leaf_id", leafID).Error
}

// ListAiChatThread 按 active_leaf 回溯出当前线程（正序）。
//
// 不用「全表按时间排序」是因为分支存在后，时间顺序会把兄弟分支串在一起，
// 显示出来就是「问一次答两次」的错乱对话。
func ListAiChatThread(sessionID, leafID uint) ([]AiChatMessage, error) {
	if sessionID == 0 {
		return nil, nil
	}
	// leaf 未设置（老会话 / 空会话）：退化成"最新一条"
	if leafID == 0 {
		var last AiChatMessage
		if err := DB.Where("session_id = ?", sessionID).Order("id DESC").Limit(1).Find(&last).Error; err != nil {
			return nil, err
		}
		if last.ID == 0 {
			return []AiChatMessage{}, nil
		}
		leafID = last.ID
	}

	chain := make([]AiChatMessage, 0, 32)
	seen := map[uint]bool{}
	cur := leafID
	for cur > 0 && !seen[cur] && len(chain) < 500 {
		m, err := GetAiChatMessage(cur, sessionID)
		if err != nil || m == nil {
			break // 叶子指向被删除的消息：拿到多少返回多少，不报错
		}
		seen[cur] = true
		chain = append(chain, *m)
		cur = m.ParentID
	}
	// 反转成正序
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain, nil
}

// SiblingIndex 计算某条消息在「同父同角色」兄弟中的位置（1 起）与总数。
//
// 只算同角色：对 user 消息「编辑重发」产生的是 user 兄弟，
// assistant 的重新生成产生 assistant 兄弟，两者混在一起会让「第 2/3 条」失去意义。
func SiblingIndex(m *AiChatMessage) (index int, count int) {
	if m == nil || m.ID == 0 {
		return 0, 0
	}
	var total int64
	DB.Model(&AiChatMessage{}).
		Where("session_id = ? AND parent_id = ? AND role = ?", m.SessionID, m.ParentID, m.Role).
		Count(&total)
	if total == 0 {
		return 0, 0
	}
	var before int64
	DB.Model(&AiChatMessage{}).
		Where("session_id = ? AND parent_id = ? AND role = ? AND id < ?", m.SessionID, m.ParentID, m.Role, m.ID).
		Count(&before)
	return int(before) + 1, int(total)
}

// SiblingIDs 取同父同角色兄弟的消息 ID（升序），供前端左右切换。
func SiblingIDs(m *AiChatMessage) []uint {
	if m == nil {
		return nil
	}
	var ids []uint
	DB.Model(&AiChatMessage{}).
		Where("session_id = ? AND parent_id = ? AND role = ?", m.SessionID, m.ParentID, m.Role).
		Order("id ASC").Pluck("id", &ids)
	return ids
}

// DeepestLeafFrom 从某条消息往下找到它所在分支的最新叶子（用于切换分支后定位显示位置）。
//
// 兄弟分支可能又各自发展出后续消息，切到某条兄弟时必须跟到它的末端，
// 否则会显示到一半就断了。
func DeepestLeafFrom(sessionID, msgID uint) uint {
	cur := msgID
	seen := map[uint]bool{}
	for cur > 0 && !seen[cur] {
		seen[cur] = true
		var child AiChatMessage
		// 取该节点最新的子节点（可能有多个，取最新的那条路径）
		if err := DB.Where("session_id = ? AND parent_id = ?", sessionID, cur).
			Order("id DESC").Limit(1).Find(&child).Error; err != nil || child.ID == 0 {
			break
		}
		cur = child.ID
	}
	return cur
}

// LockAiChatSession 终止会话（内容安全判定）。by=0 表示系统判定。
func LockAiChatSession(id uint, reason string, by uint) error {
	now := time.Now()
	return DB.Model(&AiChatSession{}).Where("id = ?", id).Updates(map[string]interface{}{
		"locked":      true,
		"locked_at":   now,
		"lock_reason": truncateRunes(reason, 240),
		"locked_by":   by,
	}).Error
}

// UnlockAiChatSession 解除终止（管理员复审后）。
func UnlockAiChatSession(id uint) error {
	return DB.Model(&AiChatSession{}).Where("id = ?", id).Updates(map[string]interface{}{
		"locked":      false,
		"locked_at":   nil,
		"lock_reason": "",
		"locked_by":   0,
	}).Error
}

func truncateRunes(s string, n int) string {
	if n <= 0 {
		return s
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// BackfillMessageParents 把老数据（parent_id 全 0）串成链，并补 active_leaf。
//
// 不做这一步的话，升级后所有历史会话的 parent 都是 0，
// 回溯线程只会得到最后一条消息 —— 表现为「打开旧对话只剩一句」，数据没丢但看不出。
// 幂等：只处理 parent_id=0 且前面已有消息的记录。
func BackfillMessageParents() {
	var sessionIDs []uint
	DB.Model(&AiChatMessage{}).Where("parent_id = 0").
		Distinct().Pluck("session_id", &sessionIDs)
	for _, sid := range sessionIDs {
		var msgs []AiChatMessage
		if err := DB.Where("session_id = ?", sid).Order("created_at ASC, id ASC").Find(&msgs).Error; err != nil {
			continue
		}
		if len(msgs) <= 1 {
			continue
		}
		prev := uint(0)
		for i := range msgs {
			if msgs[i].ParentID == 0 && prev != 0 {
				DB.Model(&AiChatMessage{}).Where("id = ?", msgs[i].ID).
					UpdateColumn("parent_id", prev)
				msgs[i].ParentID = prev
			}
			prev = msgs[i].ID
		}
		// 没设过活跃叶子的会话，指到最后一条
		DB.Model(&AiChatSession{}).Where("id = ? AND active_leaf_id = 0", sid).
			UpdateColumn("active_leaf_id", msgs[len(msgs)-1].ID)
	}
}

// MarkSessionContaminated 标记会话上下文已污染（中档模式：可继续但注入警告）。
func MarkSessionContaminated(id uint) error {
	now := time.Now()
	return DB.Model(&AiChatSession{}).Where("id = ?", id).Updates(map[string]interface{}{
		"contaminated":    true,
		"contaminated_at": now,
	}).Error
}
