package models

import (
	"encoding/json"
	"fmt"
	"time"
)

// ==================== AI 绘画 / 视频任务（§5.4） ====================
// 绘画为同步调用（提交即出图，成功才扣款）；视频为异步任务（提交即扣款，
// 上游失败退款）。均按「每次调用」计费（AiModel.PriceCall），价格快照随流水落库。

// ---------- 任务状态 ----------

const (
	AiMediaPending   = "pending"   // 已创建未提交
	AiMediaRunning   = "running"   // 已提交上游执行中（视频）
	AiMediaSucceeded = "succeeded" // 成功（已扣款）
	AiMediaFailed    = "failed"    // 失败（未扣款或已退款）
)

// AiMediaTask 绘画/视频任务
type AiMediaTask struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"not null;index:idx_ai_media_user,priority:1" json:"user_id"`
	Kind         string    `gorm:"size:16;not null;index" json:"kind"` // image / video
	ModelID      uint      `gorm:"not null" json:"model_id"`
	ModelName    string    `gorm:"size:64;default:''" json:"model_name"`
	Prompt       string    `gorm:"type:text" json:"prompt"`
	Params       string    `gorm:"type:text" json:"params"`     // 生成参数 JSON（尺寸/风格/时长）
	Status       string    `gorm:"size:16;not null;default:pending;index" json:"status"`
	RemoteID     string    `gorm:"size:128;default:''" json:"remote_id"` // 上游任务 ID（视频）
	Result       string    `gorm:"type:text" json:"result"`     // 结果 JSON（图片 URL/b64 列表 / 视频 URL）
	Cost         int64     `gorm:"not null;default:0" json:"cost"`
	CallLogID    uint      `gorm:"not null;default:0" json:"call_log_id"`
	Error        string    `gorm:"size:256;default:''" json:"error"`
	CreatedAt    time.Time `gorm:"index:idx_ai_media_user,priority:2" json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (AiMediaTask) TableName() string { return "ai_media_tasks" }

// AiMediaParams 生成参数（前端传入，原样存档）
type AiMediaParams struct {
	Size     string `json:"size,omitempty"`     // 图片尺寸，如 1024x1024
	Style    string `json:"style,omitempty"`    // 风格标签
	Duration int    `json:"duration,omitempty"` // 视频时长（秒）
	N        int    `json:"n,omitempty"`        // 图片张数
}

// ---------- 生命周期 ----------

// CreateAiMediaTask 创建任务（pending，未扣款）
func CreateAiMediaTask(userID uint, m *AiModel, kind, prompt string, params AiMediaParams) (*AiMediaTask, error) {
	pj, _ := json.Marshal(params)
	t := &AiMediaTask{
		UserID: userID, Kind: kind, ModelID: m.ID, ModelName: m.Name,
		Prompt: prompt, Params: string(pj), Status: AiMediaPending,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := DB.Create(t).Error; err != nil {
		return nil, err
	}
	return t, nil
}

// mediaPriceSnapshot 按次计费快照
func mediaPriceSnapshot(m *AiModel, cost int64) string {
	snap := AiPriceSnapshot{
		ModelID: m.ID, ModelName: m.Name, PriceIn: 0, PriceOut: 0, Cost: cost,
	}
	sj, _ := json.Marshal(snap)
	return string(sj)
}

// ChargeAiMediaTask 按次扣款（consume 流水）并标记成功；免费模型不碰账本。
// 返回扣款后余额。重复调用安全（状态非 pending/running 直接返回当前成本）。
func ChargeAiMediaTask(taskID uint) (int64, error) {
	var t AiMediaTask
	if err := DB.Where("id = ?", taskID).First(&t).Error; err != nil {
		return 0, err
	}
	if t.Status == AiMediaSucceeded {
		return GetTokenBalance(t.UserID), nil
	}
	if t.Status != AiMediaPending && t.Status != AiMediaRunning {
		return 0, fmt.Errorf("任务状态不可扣款：%s", t.Status)
	}
	var m AiModel
	if err := DB.Where("id = ?", t.ModelID).First(&m).Error; err != nil {
		return 0, err
	}
	cost := m.PriceCall
	now := time.Now()
	if cost <= 0 {
		_ = DB.Model(&AiMediaTask{}).Where("id = ?", taskID).Updates(map[string]interface{}{
			"status": AiMediaSucceeded, "cost": 0, "updated_at": now,
		}).Error
		return 0, nil
	}
	nb, err := AppendTokenLedger(t.UserID, TokenTypeConsume, -cost, "ai", t.ID,
		fmt.Sprintf("AI %s：%s", mediaKindLabel(t.Kind), m.Name), mediaPriceSnapshot(&m, cost))
	if err != nil {
		return 0, err
	}
	_ = DB.Model(&AiMediaTask{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"status": AiMediaSucceeded, "cost": cost, "updated_at": now,
	}).Error
	return nb, nil
}

// MarkAiMediaSubmitted 视频：记录上游任务 ID 并转 running
func MarkAiMediaSubmitted(taskID uint, remoteID string) {
	_ = DB.Model(&AiMediaTask{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"status": AiMediaRunning, "remote_id": remoteID, "updated_at": time.Now(),
	}).Error
}

// UpdateAiMediaResult 回写结果（成功）
func UpdateAiMediaResult(taskID uint, resultJSON string) {
	_ = DB.Model(&AiMediaTask{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"status": AiMediaSucceeded, "result": resultJSON, "error": "", "updated_at": time.Now(),
	}).Error
}

// FailAiMediaTask 标记失败（未扣款任务直接失败；已扣款任务由调用方决定是否退款）
func FailAiMediaTask(taskID uint, errMsg string, charged bool) {
	msg := errMsg
	if len([]rune(msg)) > 200 {
		msg = string([]rune(msg)[:200])
	}
	status := AiMediaFailed
	if charged {
		status = AiMediaSucceeded // 已扣款但上游失败：保持 succeeded 待退款，或调用方先退款
	}
	_ = DB.Model(&AiMediaTask{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"status": status, "error": msg, "updated_at": time.Now(),
	}).Error
}

// RefundAiMediaTask 退款已扣款任务（视频上游失败场景）
func RefundAiMediaTask(taskID uint, reason string) (int64, error) {
	var t AiMediaTask
	if err := DB.Where("id = ?", taskID).First(&t).Error; err != nil {
		return 0, err
	}
	if t.Status != AiMediaSucceeded || t.Cost <= 0 {
		return 0, fmt.Errorf("该任务不可退款")
	}
	nb, err := AppendTokenLedger(t.UserID, TokenTypeRefund, t.Cost, "ai", t.ID, reason, mediaPriceSnapshot(&AiModel{ID: t.ModelID, Name: t.ModelName}, t.Cost))
	if err != nil {
		return 0, err
	}
	_ = DB.Model(&AiMediaTask{}).Where("id = ?", taskID).Update("status", AiMediaFailed).Error
	return nb, nil
}

// ---------- 查询 ----------

// GetAiMediaTask 取任务（校验归属）
func GetAiMediaTask(id, userID uint) (*AiMediaTask, error) {
	var t AiMediaTask
	if err := DB.Where("id = ? AND user_id = ?", id, userID).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// ListAiMediaTasks 我的任务（分页，时间倒序）
func ListAiMediaTasks(userID uint, kind string, page, pageSize int) ([]AiMediaTask, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	q := DB.Model(&AiMediaTask{}).Where("user_id = ?", userID)
	if kind != "" {
		q = q.Where("kind = ?", kind)
	}
	var total int64
	q.Count(&total)
	var list []AiMediaTask
	err := DB.Where("user_id = ?", userID).Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	if kind != "" {
		// Find 未带 kind 条件时重新过滤（上面 Count 已带）
		filtered := make([]AiMediaTask, 0, len(list))
		for _, t := range list {
			if t.Kind == kind {
				filtered = append(filtered, t)
			}
		}
		list = filtered
	}
	return list, total, err
}

// mediaKindLabel 任务类型中文名
func mediaKindLabel(kind string) string {
	switch kind {
	case AiKindImage:
		return "绘画"
	case AiKindVideo:
		return "视频"
	default:
		return "生成"
	}
}
