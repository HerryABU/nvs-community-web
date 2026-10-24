package oc

// ==================== 窗口审核（多条一起审） ====================
// 为什么不是「每条都审」：AI 互聊高频，逐条审成本高、上下文割裂（单条看不出对话流违规模式）、
// 延迟大。窗口审核把 N 条/N 轮当一批，审核对象 = 窗口内多条 + 相关设定 + 上下文。
//
// 审核等级（站长/群主可配，框架中性）：
//   strict  严格：每个窗口都审，命中即停流待复核
//   loose   宽松：只审公开分享与被举报内容（窗口仍记录，不拦截）
//   redteam 红队：不拦截、只记录（隔离环境，产出安全资产）
//
// 红队模式四条边界：隔离（不进社区流/搜索/推荐/分享）、授权（站长确认留痕）、
// 记录（prompt/模型/输出/命中全进审计）、不教唆（产出安全资产而非攻击教程）。

import (
	"encoding/json"
	"strings"
	"time"

	"nvs-server/models"
	"nvs-server/security"
)

// windowTrigger 判断是否需要结算窗口（按条数或按轮次）
func windowTrigger(f *Flow, set GroupSetting, turns int) bool {
	if set.WindowRounds > 0 {
		return turns%set.WindowRounds == 0
	}
	size := set.WindowSize
	if size <= 0 {
		size = DefaultWindowSize
	}
	return turns%size == 0
}

// maybeAuditWindow 累积到阈值就结算一个窗口（同步执行，便于测试；生产可放 goroutine）
func maybeAuditWindow(flowID uint) {
	f, err := GetFlow(flowID)
	if err != nil {
		return
	}
	set := GetGroupSetting(f.GroupID)
	if set.AuditLevel == AuditLoose {
		return // 宽松模式：窗口不主动审（公开分享与举报另行处理）
	}
	if f.TurnCount == 0 || !windowTrigger(f, set, f.TurnCount) {
		return
	}
	_, _ = AuditWindow(f.ID, f.TurnCount)
}

// AuditWindow 审核 [fromSeq, toSeq] 区间（缺省取最近一个窗口）
func AuditWindow(flowID uint, toSeq int) (*Window, error) {
	f, err := GetFlow(flowID)
	if err != nil {
		return nil, err
	}
	set := GetGroupSetting(f.GroupID)
	size := set.WindowSize
	if set.WindowRounds > 0 {
		size = set.WindowRounds
	}
	if size <= 0 {
		size = DefaultWindowSize
	}
	if toSeq <= 0 {
		toSeq = f.TurnCount
	}
	fromSeq := toSeq - size + 1
	if fromSeq < 1 {
		fromSeq = 1
	}
	var turns []Turn
	if err := models.DB.Where("flow_id = ? AND seq BETWEEN ? AND ?", flowID, fromSeq, toSeq).
		Order("seq ASC").Find(&turns).Error; err != nil {
		return nil, err
	}
	if len(turns) == 0 {
		return nil, nil
	}

	w := &Window{
		FlowID: f.ID, GroupID: f.GroupID, FromSeq: fromSeq, ToSeq: toSeq,
		TurnCount: len(turns), Level: set.AuditLevel,
		Result: "pending", CreatedAt: time.Now(),
	}
	_ = models.DB.Create(w).Error

	// 审核对象：窗口内多条 + 上下文（拼接后统一扫描，识别对话流违规模式）
	var body strings.Builder
	for _, t := range turns {
		body.WriteString(t.Content + "\n")
	}
	joined := body.String()
	hits := security.ScanBadWords(joined, "oc")

	hitJSON := "[]"
	if len(hits) > 0 {
		if b, e := json.Marshal(hits); e == nil {
			hitJSON = string(b)
		}
	}

	result := "pass"
	if len(hits) > 0 {
		result = "hit"
	}
	w.Result = result
	w.Hits = hitJSON

	switch set.AuditLevel {
	case AuditRedTeam:
		// 只记录、不拦截（隔离环境 + 授权 + 留痕）
		w.Note = "红队模式：只记录不拦截"
	case AuditStrict:
		if result == "hit" {
			w.Note = "命中：窗口已标记并暂停对话流，待人工复核"
			// 先审后发：隐藏窗口内发言；先发后审：标记待复核（默认保留可见但打标）
			if set.AuditTiming == AuditPre {
				_ = models.DB.Model(&Turn{}).Where("flow_id = ? AND seq BETWEEN ? AND ?", flowID, fromSeq, toSeq).
					Update("review", ReviewRejected).Error
			} else {
				_ = models.DB.Model(&Turn{}).Where("flow_id = ? AND seq BETWEEN ? AND ?", flowID, fromSeq, toSeq).
					Update("review", ReviewPending).Error
			}
			_ = SetFlowStatus(flowID, FlowPaused, "窗口审核命中，待人工复核")
		}
	default:
		if result == "hit" {
			w.Note = "命中：已标记待复核"
			_ = models.DB.Model(&Turn{}).Where("flow_id = ? AND seq BETWEEN ? AND ?", flowID, fromSeq, toSeq).
				Update("review", ReviewPending).Error
		}
	}

	// 窗口 ID 回写到轮次，便于追溯
	_ = models.DB.Model(&Turn{}).Where("flow_id = ? AND seq BETWEEN ? AND ?", flowID, fromSeq, toSeq).
		Update("window_id", w.ID).Error
	_ = models.DB.Model(&Window{}).Where("id = ?", w.ID).
		Updates(map[string]interface{}{"result": w.Result, "hits": w.Hits, "note": w.Note}).Error
	return w, nil
}

// ListWindows 对话流的审核窗口记录（可导出报告）
func ListWindows(flowID uint, limit int) ([]Window, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var list []Window
	if err := models.DB.Where("flow_id = ?", flowID).
		Order("id DESC").Limit(limit).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// ModerationReport 窗口审核报告（红队模式产出的「安全资产」）
type ModerationReport struct {
	GroupID   uint      `json:"group_id"`
	FlowID    uint      `json:"flow_id"`
	Level     string    `json:"level"`
	Windows   []Window  `json:"windows"`
	Generated time.Time `json:"generated_at"`
}

// BuildReport 导出报告（红队/审计用）
func BuildReport(flowID uint) (*ModerationReport, error) {
	f, err := GetFlow(flowID)
	if err != nil {
		return nil, err
	}
	ws, err := ListWindows(flowID, 200)
	if err != nil {
		return nil, err
	}
	return &ModerationReport{
		GroupID: f.GroupID, FlowID: flowID,
		Level:   GetGroupSetting(f.GroupID).AuditLevel,
		Windows: ws, Generated: time.Now(),
	}, nil
}
