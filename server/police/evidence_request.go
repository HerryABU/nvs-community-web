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

// EvidenceRequest 警方取证导出申请（审批流）。
//
// 设计：警方要导出某用户的取证包，必须先向站长提交申请；站长批准后才能导出。
// 这是「以疏代堵」对接警方的口径——警方不默认拥有导出权，须经平台管理员授权，
// 每一步都留痕、可审计。
type EvidenceRequest struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	No           string     `gorm:"size:32;uniqueIndex;not null" json:"no"`
	RequesterID  uint       `gorm:"not null;index" json:"requester_id"`   // 发起申请的警方用户
	TargetUserID uint       `gorm:"not null;index" json:"target_user_id"` // 被取证的目标用户
	TargetName   string     `gorm:"size:64;default:''" json:"target_name"`
	Reason       string     `gorm:"type:text" json:"reason"`
	Status       string     `gorm:"size:16;not null;default:pending;index" json:"status"`
	ReviewerID   uint       `gorm:"not null;default:0" json:"reviewer_id"`
	ReviewNote   string     `gorm:"size:512;default:''" json:"review_note"`
	ReviewedAt   *time.Time `json:"reviewed_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (EvidenceRequest) TableName() string { return "police_evidence_requests" }

// EvidenceRequestLog 审批/操作日志（谁在什么时候批了/拒了）。
type EvidenceRequestLog struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	EvidenceRequestID uint      `gorm:"not null;index" json:"evidence_request_id"`
	ActorID           uint      `gorm:"not null" json:"actor_id"`
	Action            string    `gorm:"size:24;not null" json:"action"` // create / approve / reject
	Note              string    `gorm:"size:1024;default:''" json:"note"`
	CreatedAt         time.Time `json:"created_at"`
}

func (EvidenceRequestLog) TableName() string { return "police_evidence_request_logs" }

// 申请状态
const (
	ERPending  = "pending"
	ERApproved = "approved"
	ERRejected = "rejected"
)

// CreateEvidenceRequest 警方发起取证导出申请。
func CreateEvidenceRequest(policeUserID, targetUserID uint, targetName, reason string) (*EvidenceRequest, error) {
	if policeUserID == 0 || targetUserID == 0 {
		return nil, errors.New("参数非法")
	}
	if strings.TrimSpace(reason) == "" {
		return nil, errors.New("申请理由不能为空")
	}
	er := &EvidenceRequest{
		No:           genEvidenceNo(),
		RequesterID:  policeUserID,
		TargetUserID: targetUserID,
		TargetName:   targetName,
		Reason:       truncate(reason, 2000),
		Status:       ERPending,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := models.DB.Create(er).Error; err != nil {
		return nil, err
	}
	_ = models.DB.Create(&EvidenceRequestLog{
		EvidenceRequestID: er.ID, ActorID: policeUserID, Action: "create",
		Note: "提交取证导出申请", CreatedAt: time.Now(),
	})
	return er, nil
}

func genEvidenceNo() string {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("ER-%s-%04d", time.Now().Format("20060102"), time.Now().UnixNano()%10000)
	}
	return fmt.Sprintf("ER-%s-%s", time.Now().Format("20060102"), strings.ToUpper(hex.EncodeToString(b)))
}

// ListEvidenceRequests 管理员查看申请（按状态筛，待审在前）。
func ListEvidenceRequests(page, size int, status string) ([]EvidenceRequest, int64, error) {
	if page < 1 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 20
	}
	q := models.DB.Model(&EvidenceRequest{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	q.Count(&total)
	var list []EvidenceRequest
	if err := q.Order("CASE status WHEN 'pending' THEN 0 ELSE 1 END, id DESC").
		Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// ListMyEvidenceRequests 警方看自己的申请。
func ListMyEvidenceRequests(policeUserID uint, page, size int) ([]EvidenceRequest, int64, error) {
	if page < 1 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 20
	}
	q := models.DB.Model(&EvidenceRequest{}).Where("requester_id = ?", policeUserID)
	var total int64
	q.Count(&total)
	var list []EvidenceRequest
	if err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// GetEvidenceRequest 详情。
func GetEvidenceRequest(id uint) (*EvidenceRequest, error) {
	var er EvidenceRequest
	if err := models.DB.Where("id = ?", id).First(&er).Error; err != nil {
		return nil, err
	}
	return &er, nil
}

// ReviewEvidenceRequest 管理员审批（approve=true 批准 / false 驳回）。
func ReviewEvidenceRequest(id, adminID uint, approve bool, note string) error {
	var er EvidenceRequest
	if err := models.DB.Where("id = ?", id).First(&er).Error; err != nil {
		return err
	}
	if er.Status != ERPending {
		return errors.New("该申请已审批，不能重复处理")
	}
	now := time.Now()
	action := "reject"
	updates := map[string]interface{}{
		"status": ERApproved, "updated_at": now, "reviewed_at": &now, "reviewer_id": adminID,
		"review_note": truncate(note, 512),
	}
	if !approve {
		action = "reject"
		updates["status"] = ERRejected
	}
	if err := models.DB.Model(&EvidenceRequest{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return err
	}
	_ = models.DB.Create(&EvidenceRequestLog{
		EvidenceRequestID: id, ActorID: adminID, Action: action,
		Note: truncate(note, 1000), CreatedAt: now,
	})
	return nil
}

// HasApprovedRequest 该警方用户是否有针对目标用户的已批准申请（导出前校验）。
func HasApprovedRequest(policeUserID, targetUserID uint) bool {
	var n int64
	models.DB.Model(&EvidenceRequest{}).
		Where("requester_id = ? AND target_user_id = ? AND status = ?", policeUserID, targetUserID, ERApproved).
		Count(&n)
	return n > 0
}

// HasAnyApproved 该警方用户是否有任意已批准申请（用于「导出全部」这类不指定目标的场景）。
func HasAnyApproved(policeUserID uint) bool {
	var n int64
	models.DB.Model(&EvidenceRequest{}).
		Where("requester_id = ? AND status = ?", policeUserID, ERApproved).Count(&n)
	return n > 0
}

// MigrateEvidenceRequest 建表（在 police.Migrate 内一并调用）。
func MigrateEvidenceRequest(db *gorm.DB) error {
	return db.AutoMigrate(&EvidenceRequest{}, &EvidenceRequestLog{})
}
