package models

// ==================== Web 项目发布额度 + 超额审核 ====================
//
// 平台对「用户自定义 HTML 扩展（UserHTML）」与「用户自定义模板（UserFrame）」
// 的公开发布数量做额度管控（UserHTML + UserFrame 中 is_public=true 的合计）：
//
//   - 全局默认额度：存 platform_configs.web_publish_quota（默认 3）；
//   - 按人覆盖：user_web_quotas（user_id 唯一索引，quota int，quota=-1 表示跟随全局）；
//   - 超额审核：用户把某项目设为公开时，若已达/超过本人额度，项目进入「超额待审核」
//     （review_status = quota_review，不进广场），并生成一条 WebQuotaRequest 审核单，
//     管理员批准后才 is_public=true 且 review_status=approved 计入公开数。
//
// review_status 取值约定（避免与既有 approved/pending/quarantine/rejected 冲突）：
//   approved       内容审核通过（或额度内直接公开）
//   pending        安全扫描待复核
//   quarantine     安全预检命中高危、封存未解压
//   rejected       内容审核驳回
//   quota_review   超额待审核（新增，仅表示「超额、等管理员放行公开」）

import (
	"fmt"
	"strconv"
	"time"
)

// ---------- 常量 ----------

const (
	// WebQuotaTargetHTML / WebQuotaTargetFrame 审核单指向的目标类型
	WebQuotaTargetHTML  = "html"
	WebQuotaTargetFrame = "frame"

	// WebQuotaStatusPending / Approved / Rejected 审核单状态
	WebQuotaStatusPending  = "pending"
	WebQuotaStatusApproved = "approved"
	WebQuotaStatusRejected = "rejected"

	// ReviewStatusQuotaReview 目标表新增的 review_status 值：超额待审核
	// （区别于既有 approved/pending/quarantine/rejected，专指「内容无问题、但公开数量超额」）
	ReviewStatusQuotaReview = "quota_review"

	// WebPublishQuotaKey 全局默认 web 项目公开额度（存 platform_configs）
	WebPublishQuotaKey = "web_publish_quota"
	// DefaultWebPublishQuota 默认每人可公开 3 个 web 项目
	DefaultWebPublishQuota = 3

	// WebQuotaFollowGlobal 按人覆盖中表示「跟随全局默认额度」
	WebQuotaFollowGlobal = -1
)

// ---------- 模型 ----------

// UserWebQuota 管理员对单个用户「web 项目公开额度」的覆盖。
// quota 语义：-1 = 跟随全局默认；0 = 禁止公开发布；> 0 = 该用户的专属上限。
type UserWebQuota struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;uniqueIndex" json:"user_id"`
	Quota     int       `gorm:"default:-1" json:"quota"` // -1=跟随全局；0=禁止；>0=专属上限
	Note      string    `gorm:"size:256;default:''" json:"note"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Username string `gorm:"-" json:"username,omitempty"`
}

func (UserWebQuota) TableName() string { return "user_web_quotas" }

// WebQuotaRequest 超额公开发布审核单
type WebQuotaRequest struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	UserID     uint       `gorm:"not null;index" json:"user_id"`
	TargetType string     `gorm:"size:16;not null;index" json:"target_type"` // html / frame
	TargetID   uint       `gorm:"not null;index" json:"target_id"`
	Status     string     `gorm:"size:16;not null;default:pending;index" json:"status"` // pending/approved/rejected
	Reason     string     `gorm:"type:text" json:"reason"`
	ReviewNote string     `gorm:"type:text" json:"review_note"` // 管理员驳回/通过意见
	ReviewedBy uint       `gorm:"default:0" json:"reviewed_by"` // 处理的管理员 ID
	ReviewedAt *time.Time `json:"reviewed_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`

	Username   string `gorm:"-" json:"username,omitempty"`    // 申请人用户名（列表填充）
	TargetName string `gorm:"-" json:"target_name,omitempty"` // 目标名称（列表填充）
}

func (WebQuotaRequest) TableName() string { return "web_quota_requests" }

// ---------- 全局默认额度 ----------

// GetWebPublishQuotaConfig 读取全局默认 web 项目公开额度（缺省 DefaultWebPublishQuota，范围 0~1000）
func GetWebPublishQuotaConfig() int {
	val := GetPlatformConfig(WebPublishQuotaKey)
	if val == "" {
		return DefaultWebPublishQuota
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return DefaultWebPublishQuota
	}
	if n < 0 {
		n = 0
	}
	if n > 1000 {
		n = 1000
	}
	return n
}

// SetWebPublishQuotaConfig 保存全局默认 web 项目公开额度（立即生效）
func SetWebPublishQuotaConfig(value int) error {
	if value < 0 {
		value = 0
	}
	if value > 1000 {
		value = 1000
	}
	return SetPlatformConfig(WebPublishQuotaKey, strconv.Itoa(value))
}

// ---------- 按人覆盖 ----------

// GetUserWebQuotaOverride 读取某用户的额度覆盖（无记录返回 nil = 跟随全局）
func GetUserWebQuotaOverride(userID uint) *UserWebQuota {
	if userID == 0 {
		return nil
	}
	var q UserWebQuota
	if err := DB.Where("user_id = ?", userID).First(&q).Error; err != nil {
		return nil
	}
	return &q
}

// SetUserWebQuotaOverride 写入/更新某用户的额度覆盖（UPSERT）。
// quota：-1=跟随全局；0=禁止公开；>0=专属上限。note 为管理员备注。
//
// ⚠️ 必须用 map 显式 upsert：quota 的「0 = 禁止公开」是零值，若走 struct Create，
// GORM 会忽略零值字段而套用列默认值 -1，导致「剥夺公开权利」静默失效。
func SetUserWebQuotaOverride(userID uint, quota int, note string) error {
	if userID == 0 {
		return fmt.Errorf("用户 ID 无效")
	}
	if quota < WebQuotaFollowGlobal {
		quota = WebQuotaFollowGlobal
	}
	if quota > 1000 {
		quota = 1000
	}
	now := time.Now()
	cols := map[string]interface{}{
		"user_id":    userID,
		"quota":      quota,
		"note":       note,
		"updated_at": now,
	}
	if existing := GetUserWebQuotaOverride(userID); existing != nil {
		return DB.Model(&UserWebQuota{}).Where("user_id = ?", userID).Updates(cols).Error
	}
	cols["created_at"] = now
	return DB.Model(&UserWebQuota{}).Create(cols).Error
}

// DeleteUserWebQuotaOverride 撤销按人覆盖（恢复跟随全局）
func DeleteUserWebQuotaOverride(userID uint) error {
	return DB.Where("user_id = ?", userID).Delete(&UserWebQuota{}).Error
}

// ListUserWebQuotas 列出所有被单独设置过额度的用户（附用户名）
func ListUserWebQuotas() ([]UserWebQuota, error) {
	var list []UserWebQuota
	if err := DB.Order("updated_at DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return list, nil
	}
	ids := make([]uint, 0, len(list))
	for _, q := range list {
		ids = append(ids, q.UserID)
	}
	var users []User
	DB.Where("id IN ?", ids).Find(&users)
	names := map[uint]string{}
	for _, u := range users {
		names[u.ID] = u.Username
	}
	for i := range list {
		list[i].Username = names[list[i].UserID]
	}
	return list, nil
}

// GetWebPublishQuota 计算某用户实际生效的公开额度（按人覆盖优先，否则全局默认）
func GetWebPublishQuota(userID uint) int {
	if q := GetUserWebQuotaOverride(userID); q != nil && q.Quota >= 0 {
		return q.Quota
	}
	return GetWebPublishQuotaConfig()
}

// ---------- 已公开数统计 ----------

// CountPublicWeb 统计某用户已公开（is_public=true）的 web 项目数
// （UserHTML + UserFrame 合计，与广场展示口径一致）
func CountPublicWeb(userID uint) int64 {
	var htmlN, frameN int64
	DB.Model(&UserHTML{}).Where("user_id = ? AND is_public = ?", userID, true).Count(&htmlN)
	DB.Model(&UserFrame{}).Where("user_id = ? AND is_public = ?", userID, true).Count(&frameN)
	return htmlN + frameN
}

// ---------- 目标可见性辅助 ----------

// applyTargetVisibility 直接改写目标表的 is_public / review_status（精确列，避免零值坑）
func applyTargetVisibility(targetType string, targetID uint, isPublic bool, reviewStatus string) error {
	updates := map[string]interface{}{
		"is_public":     isPublic,
		"review_status": reviewStatus,
		"updated_at":    time.Now(),
	}
	switch targetType {
	case WebQuotaTargetHTML:
		return DB.Model(&UserHTML{}).Where("id = ?", targetID).Updates(updates).Error
	case WebQuotaTargetFrame:
		return DB.Model(&UserFrame{}).Where("id = ?", targetID).Updates(updates).Error
	default:
		return fmt.Errorf("未知的目标类型: %s", targetType)
	}
}

// ---------- 核心：额度判断并应用 ----------

// CheckAndApplyWebPublish 在用户把某 web 项目设为公开时调用，决定是否直接公开还是转超额审核。
// 返回：
//
//	allowed    = true  表示已直接公开（is_public=true, review_status=approved）；
//	needReview = true  表示已进入超额待审核（is_public=false, review_status=quota_review，并生成审核单）。
//
// 注意：调用方需先校验「目标存在且归属当前用户」。
func CheckAndApplyWebPublish(userID uint, targetType string, targetID uint) (allowed bool, needReview bool, err error) {
	// 读取目标当前 is_public 状态
	var isPublic bool
	switch targetType {
	case WebQuotaTargetHTML:
		var h UserHTML
		if e := DB.First(&h, targetID).Error; e != nil {
			return false, false, e
		}
		isPublic = h.IsPublic
	case WebQuotaTargetFrame:
		var f UserFrame
		if e := DB.First(&f, targetID).Error; e != nil {
			return false, false, e
		}
		isPublic = f.IsPublic
	default:
		return false, false, fmt.Errorf("未知的目标类型: %s", targetType)
	}

	// 已公开：保持不变，无需审核
	if isPublic {
		return true, false, nil
	}

	quota := GetWebPublishQuota(userID)
	used := CountPublicWeb(userID)

	// 额度内（quota<0 视为不限；否则 used < quota 才可直接公开）
	if quota < 0 || int64(quota) > used {
		if e := applyTargetVisibility(targetType, targetID, true, "approved"); e != nil {
			return false, false, e
		}
		return true, false, nil
	}

	// 超额：转入 quota_review（不进广场），并生成审核单（已存在 pending 单则复用，不重复建单）
	if e := applyTargetVisibility(targetType, targetID, false, ReviewStatusQuotaReview); e != nil {
		return false, false, e
	}
	if _, e := CreateWebQuotaRequest(userID, targetType, targetID,
		"公开发布数量已达额度上限，提交超额审核"); e != nil {
		// 建单失败不影响「已转入待审核」的事实，仅记日志返回
		return false, true, e
	}
	return false, true, nil
}

// ---------- 审核单 CRUD ----------

// CreateWebQuotaRequest 生成一条超额审核单（同一目标已存在 pending 单则复用，避免重复建单）
func CreateWebQuotaRequest(userID uint, targetType string, targetID uint, reason string) (*WebQuotaRequest, error) {
	var existing WebQuotaRequest
	if err := DB.Where("user_id = ? AND target_type = ? AND target_id = ? AND status = ?",
		userID, targetType, targetID, WebQuotaStatusPending).First(&existing).Error; err == nil {
		return &existing, nil
	}
	req := WebQuotaRequest{
		UserID:     userID,
		TargetType: targetType,
		TargetID:   targetID,
		Status:     WebQuotaStatusPending,
		Reason:     reason,
	}
	if err := DB.Create(&req).Error; err != nil {
		return nil, err
	}
	return &req, nil
}

// ListWebQuotaRequests 列出超额审核单（status 为空表示全部），并附用户名与目标名称
func ListWebQuotaRequests(status string) ([]WebQuotaRequest, error) {
	q := DB.Order("created_at DESC")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var list []WebQuotaRequest
	if err := q.Find(&list).Error; err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return list, nil
	}
	// 补充用户名
	ids := make([]uint, 0, len(list))
	for _, r := range list {
		ids = append(ids, r.UserID)
	}
	var users []User
	DB.Where("id IN ?", ids).Find(&users)
	names := map[uint]string{}
	for _, u := range users {
		names[u.ID] = u.Username
	}
	for i := range list {
		list[i].Username = names[list[i].UserID]
		list[i].TargetName = GetWebQuotaTargetName(list[i].TargetType, list[i].TargetID)
	}
	return list, nil
}

// GetWebQuotaTargetName 取审核单指向的目标名称（用于后台展示）
func GetWebQuotaTargetName(targetType string, targetID uint) string {
	switch targetType {
	case WebQuotaTargetHTML:
		var h UserHTML
		if DB.First(&h, targetID).Error == nil {
			return h.Name
		}
	case WebQuotaTargetFrame:
		var f UserFrame
		if DB.First(&f, targetID).Error == nil {
			return f.Name
		}
	}
	return ""
}

// ApproveQuotaRequest 管理员批准超额公开：目标 is_public=true 且 review_status=approved，审核单置 approved
func ApproveQuotaRequest(reqID uint, adminID uint, note string) error {
	var req WebQuotaRequest
	if err := DB.First(&req, reqID).Error; err != nil {
		return fmt.Errorf("审核单不存在")
	}
	if req.Status != WebQuotaStatusPending {
		return fmt.Errorf("该审核单已处理（%s）", req.Status)
	}
	if err := applyTargetVisibility(req.TargetType, req.TargetID, true, "approved"); err != nil {
		return err
	}
	now := time.Now()
	cols := map[string]interface{}{
		"status":      WebQuotaStatusApproved,
		"reviewed_by": adminID,
		"review_note": note,
		"reviewed_at": now,
		"updated_at":  now,
	}
	return DB.Model(&WebQuotaRequest{}).Where("id = ?", reqID).Updates(cols).Error
}

// RejectQuotaRequest 管理员驳回超额公开：目标保持私有（is_public=false），审核单置 rejected。
// 注意：驳回不改目标 review_status——目标在提交时已置为 quota_review 且 is_public=false，
// 保留该状态即可（避免误改内容安全扫描的 pending/quarantine 等语义）。
func RejectQuotaRequest(reqID uint, adminID uint, note string) error {
	var req WebQuotaRequest
	if err := DB.First(&req, reqID).Error; err != nil {
		return fmt.Errorf("审核单不存在")
	}
	if req.Status != WebQuotaStatusPending {
		return fmt.Errorf("该审核单已处理（%s）", req.Status)
	}
	now := time.Now()
	cols := map[string]interface{}{
		"status":      WebQuotaStatusRejected,
		"reviewed_by": adminID,
		"review_note": note,
		"reviewed_at": now,
		"updated_at":  now,
	}
	return DB.Model(&WebQuotaRequest{}).Where("id = ?", reqID).Updates(cols).Error
}

// ListMyWebQuotaRequests 列出某用户自己的超额审核单（含状态、目标名）
func ListMyWebQuotaRequests(userID uint) ([]WebQuotaRequest, error) {
	var list []WebQuotaRequest
	if err := DB.Where("user_id = ?", userID).Order("created_at DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	for i := range list {
		list[i].TargetName = GetWebQuotaTargetName(list[i].TargetType, list[i].TargetID)
	}
	return list, nil
}
