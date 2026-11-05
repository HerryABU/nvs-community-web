package ai

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"
	"nvs-server/models"
	wallet "nvs-server/models/wallet"
)

// ==================== AI 调用计费（§5.3 / §13.3） ====================
// 计费流：预检（余额 ≥ 预估费用）→ 记 pending 流水 → 调上游 → 按真实 usage 扣款
// （wallet.AppendTokenLedger consume）→ settled；上游失败仅记 failed、不扣款。
// 价格快照随流水落库（token_ledger.price_snapshot），保证历史可对账。

// ---------- 调用状态 ----------

const (
	AiCallPending  = "pending"  // 已发起，未结算
	AiCallSettled  = "settled"  // 已扣款
	AiCallFailed   = "failed"   // 上游失败，未扣款
	AiCallRefunded = "refunded" // 已退款（扣款后异常回滚）
)

// AiCallLog AI 调用记录（计费凭据）
type AiCallLog struct {
	ID            uint   `gorm:"primaryKey" json:"id"`
	UserID        uint   `gorm:"not null;index:idx_ai_call_user,priority:1" json:"user_id"`
	ModelID       uint   `gorm:"not null;index" json:"model_id"`
	ModelName     string `gorm:"size:64;default:''" json:"model_name"`
	Action        string `gorm:"size:32;not null" json:"action"` // continue/polish/expand/summarize/custom
	RefType       string `gorm:"size:24;default:''" json:"ref_type"`
	RefID         uint   `gorm:"default:0" json:"ref_id"`
	TokensIn      int    `gorm:"not null;default:0" json:"tokens_in"`
	TokensOut     int    `gorm:"not null;default:0" json:"tokens_out"`
	Cost          int64  `gorm:"not null;default:0" json:"cost"` // 实扣代币（免费额度调用为 0）
	Status        string `gorm:"size:16;not null;default:pending;index" json:"status"`
	PriceSnapshot string `gorm:"type:text" json:"price_snapshot"`          // 计费快照 JSON
	FreeQuota     bool   `gorm:"not null;default:false" json:"free_quota"` // 走免费额度（结算不扣款）
	Error         string `gorm:"size:256;default:''" json:"error"`
	// LatencyMs 上游耗时（毫秒）：路由策略 fastest 的依据；失败也记录（0=未测）
	LatencyMs int        `gorm:"not null;default:0" json:"latency_ms"`
	CreatedAt time.Time  `gorm:"index:idx_ai_call_user,priority:2" json:"created_at"`
	SettledAt *time.Time `json:"settled_at"`
}

func (AiCallLog) TableName() string { return "ai_call_logs" }

// AiPriceSnapshot 计费快照（随流水落库，可对账）
type AiPriceSnapshot struct {
	ModelID   uint   `json:"model_id"`
	ModelName string `json:"model_name"`
	PriceIn   int64  `json:"price_in"`  // 每 1K 输入 tokens 代币价
	PriceOut  int64  `json:"price_out"` // 每 1K 输出 tokens 代币价
	TokensIn  int    `json:"tokens_in"`
	TokensOut int    `json:"tokens_out"`
	Cost      int64  `json:"cost"`
}

// ---------- 预估 ----------

// EstimateTokens 粗估文本 tokens：中日韩全角约 1 字≈1.5 token 的混合估算，
// 加上消息结构开销。仅用于预检，真实计费以上游 usage 为准。
func EstimateTokens(text string) int {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return 0
	}
	n := int(math.Ceil(float64(len(runes))/1.5)) + 4
	if n < 1 {
		n = 1
	}
	return n
}

// EstimateAiCost 预估费用（代币，向上取整到整数）：
// ceil(tokensIn/1000 × priceIn) + ceil(tokensOut/1000 × priceOut)
func EstimateAiCost(m *AiModel, tokensIn, tokensOut int) int64 {
	if m == nil {
		return 0
	}
	var cost int64
	if m.PriceIn > 0 && tokensIn > 0 {
		cost += int64(math.Ceil(float64(tokensIn) / 1000.0 * float64(m.PriceIn)))
	}
	if m.PriceOut > 0 && tokensOut > 0 {
		cost += int64(math.Ceil(float64(tokensOut) / 1000.0 * float64(m.PriceOut)))
	}
	return cost
}

// PrecheckAiBalance 调用前余额预检；返回预估费用与是否可负担
func PrecheckAiBalance(userID uint, m *AiModel, tokensIn, tokensOut int) (int64, bool) {
	cost := EstimateAiCost(m, tokensIn, tokensOut)
	if cost <= 0 {
		return 0, true // 免费模型放行
	}
	return cost, wallet.GetTokenBalance(userID) >= cost
}

// ---------- 调用生命周期 ----------

// CreateAiCallLog 发起调用前记 pending 流水（预估费用入 snapshot 的 cost 字段先记预估）。
// freeQuota=true 表示本次走免费额度（结算时 cost 归零、不碰代币账本）。
func CreateAiCallLog(userID uint, m *AiModel, action, refType string, refID uint, tokensIn, tokensOut int, freeQuota bool) (*AiCallLog, error) {
	snap := AiPriceSnapshot{
		ModelID: m.ID, ModelName: m.Name, PriceIn: m.PriceIn, PriceOut: m.PriceOut,
		TokensIn: tokensIn, TokensOut: tokensOut, Cost: EstimateAiCost(m, tokensIn, tokensOut),
	}
	snapJSON, _ := json.Marshal(snap)
	log := &AiCallLog{
		UserID: userID, ModelID: m.ID, ModelName: m.Name, Action: action,
		RefType: refType, RefID: refID, TokensIn: tokensIn, TokensOut: tokensOut,
		Cost: snap.Cost, Status: AiCallPending, PriceSnapshot: string(snapJSON),
		FreeQuota: freeQuota, CreatedAt: time.Now(),
	}
	if err := models.DB.Create(log).Error; err != nil {
		return nil, err
	}
	return log, nil
}

// SettleAiCallLog 按上游真实 usage 结算扣款（consume 流水 + settled 状态）。
// 若结算时余额不足（预检后被其他并发调用耗尽），返回错误、记录落 failed 且不扣款。
//
// 状态机：重复结算一律被拒绝（已 settled / 非 pending 直接报错），调用方据此返回
// 「结算失败（未扣款）」，不会误退款。
//
// 并发安全：采用「原子 claim + 条件更新」保证不重复扣款——
// 同一条 pending 记录在并发 settle 下只会被一个事务成功 claim
// （UPDATE ... WHERE status=pending 的 RowsAffected=1），其余事务 RowsAffected=0
// 直接报错退出、绝不入账；入账（wallet.AppendTokenLedger）发生在 claim 之后且在同一事务内，
// 余额不足会随事务回滚 claim（余额未扣、无消费流水），再在事务外把记录落 failed。
func SettleAiCallLog(logID uint, tokensIn, tokensOut int) (int64, error) {
	var costOut int64
	// 扣款失败信息：事务回滚后在事务外落 failed 状态
	var ledgerErr error
	var failTokensIn, failTokensOut int
	var failCost int64
	var failSnap string
	err := models.DB.Transaction(func(tx *gorm.DB) error {
		var log AiCallLog
		if e := tx.Where("id = ?", logID).First(&log).Error; e != nil {
			return e
		}
		if log.Status != AiCallPending {
			return fmt.Errorf("调用记录状态异常：%s", log.Status)
		}
		var m AiModel
		if e := tx.Where("id = ?", log.ModelID).First(&m).Error; e != nil {
			return e
		}
		cost := EstimateAiCost(&m, tokensIn, tokensOut)
		if log.FreeQuota {
			cost = 0 // 免费额度调用：不扣代币（用量由调用方记入免费额度流水）
		}
		snap := AiPriceSnapshot{
			ModelID: m.ID, ModelName: m.Name, PriceIn: m.PriceIn, PriceOut: m.PriceOut,
			TokensIn: tokensIn, TokensOut: tokensOut, Cost: cost,
		}
		snapJSON, _ := json.Marshal(snap)
		now := time.Now()

		// 原子 claim：仅当仍处于 pending 才推进到 settled（含费用快照），
		// RowsAffected==0 表示并发事务已先行结算，本事务幂等退出、不入账。
		claim := tx.Model(&AiCallLog{}).Where("id = ? AND status = ?", logID, AiCallPending).
			Updates(map[string]interface{}{
				"status":         AiCallSettled,
				"tokens_in":      tokensIn,
				"tokens_out":     tokensOut,
				"cost":           cost,
				"price_snapshot": string(snapJSON),
				"settled_at":     now,
			})
		if claim.Error != nil {
			return claim.Error
		}
		if claim.RowsAffected == 0 {
			// 并发事务已先行结算：本事务不重复入账，按状态机拒绝重复结算
			return fmt.Errorf("调用记录已被并发结算（重复结算被拒绝）")
		}
		if cost <= 0 {
			return nil // 免费额度 / 零费用：claim 即完成
		}
		nb, e := wallet.AppendTokenLedgerTx(tx, log.UserID, wallet.TokenTypeConsume, -cost, "ai", log.ID,
			fmt.Sprintf("AI 调用：%s（%s）", aiActionLabel(log.Action), m.Name), string(snapJSON))
		if e != nil {
			// 余额不足等：返回错误让事务整体回滚（claim 撤销、余额未扣、不留消费流水）。
			// failed 状态在事务外落（见函数末尾），因为事务内的写入会被回滚。
			ledgerErr = e
			failTokensIn, failTokensOut = tokensIn, tokensOut
			failCost, failSnap = cost, string(snapJSON)
			return e
		}
		costOut = nb
		return nil
	})
	if ledgerErr != nil {
		// 事务已回滚：把记录落为 failed 以便排查与重试。
		// 条件更新 status=pending —— 若并发事务已结算则不覆盖其结果。
		models.DB.Model(&AiCallLog{}).Where("id = ? AND status = ?", logID, AiCallPending).
			Updates(map[string]interface{}{
				"status":         AiCallFailed,
				"error":          ledgerErr.Error(),
				"tokens_in":      failTokensIn,
				"tokens_out":     failTokensOut,
				"cost":           failCost,
				"price_snapshot": failSnap,
				"settled_at":     time.Now(),
			})
		return 0, fmt.Errorf("扣款失败：%w", ledgerErr)
	}
	if err != nil {
		return 0, err
	}
	return costOut, nil
}

// FailAiCallLog 上游失败：记 failed，不扣款
func FailAiCallLog(logID uint, errMsg string) {
	msg := errMsg
	if len([]rune(msg)) > 200 {
		msg = string([]rune(msg)[:200])
	}
	now := time.Now()
	_ = models.DB.Model(&AiCallLog{}).Where("id = ?", logID).Updates(map[string]interface{}{
		"status": AiCallFailed, "error": msg, "settled_at": now,
	}).Error
}

// RefundAiCall 对已结算调用退款（refund 流水 + refunded 状态），用于结算后异常回滚
func RefundAiCall(logID uint, reason string) (int64, error) {
	var log AiCallLog
	if err := models.DB.Where("id = ?", logID).First(&log).Error; err != nil {
		return 0, err
	}
	if log.Status != AiCallSettled || log.Cost <= 0 {
		return 0, fmt.Errorf("该调用不可退款")
	}
	nb, err := wallet.AppendTokenLedger(log.UserID, wallet.TokenTypeRefund, log.Cost, "ai", log.ID, reason, log.PriceSnapshot)
	if err != nil {
		return 0, err
	}
	if err := models.DB.Model(&AiCallLog{}).Where("id = ?", logID).Update("status", AiCallRefunded).Error; err != nil {
		return 0, err
	}
	return nb, nil
}

// ListAiCallLogs 用户调用历史（分页，时间倒序）
func ListAiCallLogs(userID uint, page, pageSize int) ([]AiCallLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	var total int64
	models.DB.Model(&AiCallLog{}).Where("user_id = ?", userID).Count(&total)
	var list []AiCallLog
	err := models.DB.Where("user_id = ?", userID).Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

// aiActionLabel 动作中文名（流水备注用）
func aiActionLabel(action string) string {
	switch action {
	case "continue":
		return "续写"
	case "polish":
		return "润色"
	case "expand":
		return "扩写"
	case "summarize":
		return "摘要"
	case "character":
		return "角色设定"
	case "mindmap":
		return "思维导图"
	default:
		return "生成"
	}
}

// SetAiCallLatency 记录本次调用耗时（供 fastest 路由与故障排查；失败不阻断）
func SetAiCallLatency(id uint, ms int) {
	if id == 0 || ms <= 0 {
		return
	}
	_ = models.DB.Model(&AiCallLog{}).Where("id = ?", id).Update("latency_ms", ms).Error
}
