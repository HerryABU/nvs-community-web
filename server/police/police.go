// Package police 警方联动工单（gap-audit #6）。
//
// 已有能力是「police 角色能看软删除、能下载取证包」，缺的是**流转**：
// 谁在办、办到哪一步、什么时候到期、证据挂在哪。
// 本包只做三件事，不做「案件侦破」：工单、状态流转、证据引用。
//
// 边界（写进代码而不是只写文档）：
//   - 工单里**不存原始证据内容**，只存引用（蜜罐会话 ID / 取证包文件名 / 帖子 ID），
//     证据本体在各自模块里按原留存策略过期——避免工单变成「永久证据仓库」绕过 TTL；
//   - 平台管理员可建单/派单/结单，police 角色可处理自己名下的单；
//   - 结单必须写结论（不留结论的结单等于没办），并留操作日志。

package police

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"nvs-server/models"

	"gorm.io/gorm"
)

// 工单类型
const (
	KindReport   = "report"   // 站内举报升级
	KindEvidence = "evidence" // 取证协助（蜜罐/攻击流量）
	KindAssist   = "assist"   // 协查（账号/内容）
	KindAbuse    = "abuse"    // 滥用/风控
)

// 优先级
const (
	PriLow    = "low"
	PriNormal = "normal"
	PriHigh   = "high"
	PriUrgent = "urgent"
)

// 状态
const (
	StOpen       = "open"       // 待受理
	StProcessing = "processing" // 处理中
	StWaiting    = "waiting"    // 待补充材料
	StClosed     = "closed"     // 已结单
)

// Ticket 工单
type Ticket struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	No       string `gorm:"size:32;uniqueIndex;not null" json:"no"`
	Kind     string `gorm:"size:16;not null;default:report;index" json:"kind"`
	Priority string `gorm:"size:8;not null;default:normal;index" json:"priority"`
	Status   string `gorm:"size:16;not null;default:open;index" json:"status"`
	Title    string `gorm:"size:128;not null" json:"title"`
	Summary  string `gorm:"type:text" json:"summary"`

	// 对象：哪个用户/作品/帖子/蜜罐会话（全部可空）
	SubjectUserID  uint `gorm:"not null;default:0;index" json:"subject_user_id"`
	SubjectNovelID uint `gorm:"not null;default:0" json:"subject_novel_id"`
	SubjectPostID  uint `gorm:"not null;default:0" json:"subject_post_id"`
	SessionID      uint `gorm:"not null;default:0" json:"session_id"` // 蜜罐会话

	// 证据引用（JSON 数组：{type,ref,note}；不存内容本体）
	Evidence string `gorm:"type:text" json:"evidence"`

	RequesterID uint `gorm:"not null;index" json:"requester_id"`
	HandlerID   uint `gorm:"not null;default:0;index" json:"handler_id"`

	DueAt    *time.Time `json:"due_at"`
	ClosedAt *time.Time `json:"closed_at"`
	Result   string     `gorm:"size:512;default:''" json:"result"` // 结单结论（必填）

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Ticket) TableName() string { return "police_tickets" }

// Log 工单操作日志（谁在什么时候改了什么）
type Log struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TicketID  uint      `gorm:"not null;index" json:"ticket_id"`
	ActorID   uint      `gorm:"not null" json:"actor_id"`
	Action    string    `gorm:"size:24;not null" json:"action"` // create/assign/status/comment/close
	FromTo    string    `gorm:"size:64;default:''" json:"from_to"`
	Note      string    `gorm:"size:1024;default:''" json:"note"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`

	ActorName string `gorm:"-" json:"actor_name,omitempty"`
}

func (Log) TableName() string { return "police_ticket_logs" }

// CreateInput 建单入参
type CreateInput struct {
	Kind           string
	Priority       string
	Title          string
	Summary        string
	SubjectUserID  uint
	SubjectNovelID uint
	SubjectPostID  uint
	SessionID      uint
	Evidence       string
	DueHours       int
}

// Create 建单
func Create(requesterID uint, in CreateInput) (*Ticket, error) {
	if strings.TrimSpace(in.Title) == "" {
		return nil, errors.New("标题不能为空")
	}
	kind := normalizeKind(in.Kind)
	pri := normalizePriority(in.Priority)
	t := &Ticket{
		No:   genNo(),
		Kind: kind, Priority: pri, Status: StOpen,
		Title: truncate(in.Title, 128), Summary: truncate(in.Summary, 4000),
		SubjectUserID: in.SubjectUserID, SubjectNovelID: in.SubjectNovelID,
		SubjectPostID: in.SubjectPostID, SessionID: in.SessionID,
		Evidence: truncate(in.Evidence, 4000), RequesterID: requesterID,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	// DueHours != 0 才设截止：>0 为未来，<0 用于补录「已超期」的单
	if in.DueHours != 0 {
		d := time.Now().Add(time.Duration(in.DueHours) * time.Hour)
		t.DueAt = &d
	}
	if err := models.DB.Create(t).Error; err != nil {
		return nil, err
	}
	_ = addLog(t.ID, requesterID, "create", "", "创建工单："+t.Title)
	return t, nil
}

// genNo 工单号：日期 + 6 位随机十六进制。
// 早期用 UnixNano%10000，连续建单会撞（唯一索引直接失败），这类"偶发失败"最难查。
func genNo() string {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		// 极端情况下退化为纳秒尾数（仍有唯一索引兜底）
		return fmt.Sprintf("PT-%s-%04d", time.Now().Format("20060102"), time.Now().UnixNano()%10000)
	}
	return fmt.Sprintf("PT-%s-%s", time.Now().Format("20060102"), strings.ToUpper(hex.EncodeToString(b)))
}

func normalizeKind(k string) string {
	switch k {
	case KindEvidence, KindAssist, KindAbuse:
		return k
	}
	return KindReport
}

func normalizePriority(p string) string {
	switch p {
	case PriLow, PriHigh, PriUrgent:
		return p
	case PriNormal, "":
		return PriNormal
	}
	return PriNormal
}

// List 列表（按状态/优先级/类型筛 + 只看我处理的）
func List(page, size int, status, priority, kind string, handlerID uint) ([]Ticket, int64, error) {
	if page < 1 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 20
	}
	q := models.DB.Model(&Ticket{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if priority != "" {
		q = q.Where("priority = ?", priority)
	}
	if kind != "" {
		q = q.Where("kind = ?", kind)
	}
	if handlerID > 0 {
		q = q.Where("handler_id = ?", handlerID)
	}
	var total int64
	q.Count(&total)
	var list []Ticket
	// 排序：未结单在前，紧急/高优在前，再按创建时间倒序
	if err := q.Order("CASE WHEN status = 'closed' THEN 1 ELSE 0 END, " +
		"CASE priority WHEN 'urgent' THEN 0 WHEN 'high' THEN 1 WHEN 'normal' THEN 2 ELSE 3 END, " +
		"id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// Get 详情 + 日志
func Get(id uint) (*Ticket, []Log, error) {
	var t Ticket
	if err := models.DB.Where("id = ?", id).First(&t).Error; err != nil {
		return nil, nil, err
	}
	var logs []Log
	_ = models.DB.Where("ticket_id = ?", id).Order("id ASC").Find(&logs).Error
	return &t, logs, nil
}

// Assign 派单（同时把待受理改成处理中）
func Assign(id, handlerID, operatorID uint) error {
	var t Ticket
	if err := models.DB.Where("id = ?", id).First(&t).Error; err != nil {
		return err
	}
	from := fmt.Sprintf("handler:%d", t.HandlerID)
	updates := map[string]interface{}{"handler_id": handlerID, "updated_at": time.Now()}
	if t.Status == StOpen {
		updates["status"] = StProcessing
	}
	if err := models.DB.Model(&Ticket{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return err
	}
	return addLog(id, operatorID, "assign", from, fmt.Sprintf("指派给用户 %d", handlerID))
}

// SetStatus 状态流转（结单必须给结论）
func SetStatus(id, operatorID uint, status, note string) error {
	var t Ticket
	if err := models.DB.Where("id = ?", id).First(&t).Error; err != nil {
		return err
	}
	switch status {
	case StOpen, StProcessing, StWaiting:
	case StClosed:
		if strings.TrimSpace(note) == "" {
			return errors.New("结单必须填写结论（便于复盘与责任追溯）")
		}
	default:
		return errors.New("非法状态")
	}
	updates := map[string]interface{}{"status": status, "updated_at": time.Now()}
	if status == StClosed {
		now := time.Now()
		updates["closed_at"] = now
		updates["result"] = truncate(note, 512)
	}
	if err := models.DB.Model(&Ticket{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return err
	}
	action := "status"
	if status == StClosed {
		action = "close"
	}
	return addLog(id, operatorID, action, t.Status+"→"+status, note)
}

// AddNote 追加进展（不改状态）
func AddNote(id, actorID uint, note string) error {
	if strings.TrimSpace(note) == "" {
		return errors.New("内容不能为空")
	}
	return addLog(id, actorID, "comment", "", truncate(note, 1000))
}

func addLog(ticketID, actorID uint, action, fromTo, note string) error {
	return models.DB.Create(&Log{
		TicketID: ticketID, ActorID: actorID, Action: action,
		FromTo: fromTo, Note: note, CreatedAt: time.Now(),
	}).Error
}

// Stats 概览
func Stats() map[string]interface{} {
	out := map[string]int64{}
	type row struct {
		Status string
		C      int64
	}
	var rows []row
	_ = models.DB.Model(&Ticket{}).Select("status, COUNT(1) as c").Group("status").Scan(&rows).Error
	for _, r := range rows {
		out[r.Status] = r.C
	}
	// 超期未结：due_at 已过且未结单（催办口径）
	var overdue int64
	_ = models.DB.Model(&Ticket{}).
		Where("due_at IS NOT NULL AND due_at < ? AND status <> ?", time.Now(), StClosed).
		Count(&overdue).Error
	var unassigned int64
	_ = models.DB.Model(&Ticket{}).
		Where("handler_id = 0 AND status <> ?", StClosed).Count(&unassigned).Error
	return map[string]interface{}{
		"by_status": out, "overdue": overdue, "unassigned": unassigned,
	}
}

// Migrate 建表
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&Ticket{}, &Log{})
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
