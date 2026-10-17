package models

import (
	"time"

	"nvs-server/utils"
)

// UserHomeCustomize 用户个人主页装扮（仅本人登录态可见，允许原始 HTML+style+script）。
// 区别于 AuthorHomeCustomize（他人可见、需消毒+审核）。
// 信任模型：仅本人可见（自 XSS，无横向危害），等同 alist 个人自定义（/me 自看）。
type UserHomeCustomize struct {
	UserID    uint      `gorm:"primaryKey" json:"user_id"`
	RawHTML   string    `gorm:"type:text" json:"raw_html"` // 原始 HTML 片段（含 style/script，仅本人可见）
	UpdatedAt time.Time `json:"updated_at"`
}

func (UserHomeCustomize) TableName() string { return "user_home_customize" }

// GetUserHomeCustomize 读取用户个人主页装扮原始 HTML；无记录返回空串。
func GetUserHomeCustomize(userID uint) string {
	if userID == 0 {
		return ""
	}
	var t UserHomeCustomize
	if err := DB.Where("user_id = ?", userID).First(&t).Error; err != nil {
		return ""
	}
	return t.RawHTML
}

// SetUserHomeCustomize 保存用户个人主页装扮（原始 HTML，不消毒）：按用户 Upsert。
func SetUserHomeCustomize(userID uint, raw string) error {
	if userID == 0 {
		return nil
	}
	now := time.Now()
	var t UserHomeCustomize
	if err := DB.Where("user_id = ?", userID).First(&t).Error; err == nil {
		t.RawHTML = raw
		t.UpdatedAt = now
		return DB.Save(&t).Error
	}
	return DB.Create(&UserHomeCustomize{UserID: userID, RawHTML: raw, UpdatedAt: now}).Error
}

// AuthorHomeCustomize 作者公开主页自定义（他人可见，需消毒 + 站长/管理员审核）。
// 作者提交后：服务端立即消毒为 safe_html（去脚本/危险 CSS），状态置 pending 等待审核；
// 若自动化预检发现被移除项（removed 非空 → had_issues=true），原文 raw_html 一并送站长复核
// （不依赖硬编码，站长由 role 查询），站长检查原文后通过则 public_html = safe_html 对外可见。
type AuthorHomeCustomize struct {
	AuthorID     uint       `gorm:"primaryKey" json:"author_id"`
	RawHTML      string     `gorm:"type:text" json:"raw_html"`     // 作者提交的原始片段（送审原文，供站长复核）
	SafeHTML     string     `gorm:"type:text" json:"safe_html"`    // 服务端消毒后片段
	PublicHTML   string     `gorm:"type:text" json:"public_html"`  // 对外可见（approved 时等于 safe_html）
	ReviewStatus string     `gorm:"type:varchar(16);default:pending" json:"review_status"` // pending/approved/rejected
	HadIssues    bool       `gorm:"default:false" json:"had_issues"`                      // 自动化预检是否发现被移除项
	ReviewNote   string     `gorm:"type:text" json:"review_note"`
	ReviewedBy   uint       `gorm:"default:0" json:"reviewed_by"`
	ReviewedAt   *time.Time `json:"reviewed_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (AuthorHomeCustomize) TableName() string { return "author_home_customize" }

// GetApprovedAuthorHomeCustomize 读取已通过审核的对外可见片段；无/未通过返回空串。
func GetApprovedAuthorHomeCustomize(authorID uint) string {
	if authorID == 0 {
		return ""
	}
	var t AuthorHomeCustomize
	if err := DB.Where("author_id = ? AND review_status = ?", authorID, "approved").First(&t).Error; err != nil {
		return ""
	}
	return t.PublicHTML
}

// SubmitAuthorHomeCustomize 作者提交：消毒 + 置 pending + 记录是否预检发现问题。
// 返回 (safe_html, removed, hadIssues, error)。通知站长由 handler 负责（模型层不耦合推送）。
func SubmitAuthorHomeCustomize(authorID uint, raw string) (safe string, removed []string, hadIssues bool, err error) {
	if authorID == 0 {
		return "", nil, false, nil
	}
	safe, removed = utils.SanitizeCustomHTML(raw)
	had := len(removed) > 0
	now := time.Now()
	var t AuthorHomeCustomize
	if e := DB.Where("author_id = ?", authorID).First(&t).Error; e == nil {
		t.RawHTML = raw
		t.SafeHTML = safe
		t.PublicHTML = ""
		t.ReviewStatus = "pending"
		t.HadIssues = had
		t.ReviewNote = ""
		t.ReviewedBy = 0
		t.ReviewedAt = nil
		t.UpdatedAt = now
		err = DB.Save(&t).Error
	} else {
		err = DB.Create(&AuthorHomeCustomize{
			AuthorID: authorID, RawHTML: raw, SafeHTML: safe, PublicHTML: "",
			ReviewStatus: "pending", HadIssues: had, UpdatedAt: now,
		}).Error
	}
	return safe, removed, had, err
}

// ApproveAuthorHomeCustomize 站长/管理员审核通过：public_html = safe_html（绝不发布原文脚本）。
func ApproveAuthorHomeCustomize(authorID, adminID uint, note string) error {
	var t AuthorHomeCustomize
	if err := DB.Where("author_id = ?", authorID).First(&t).Error; err != nil {
		return err
	}
	updates := map[string]interface{}{
		"review_status": "approved",
		"public_html":   t.SafeHTML,
		"review_note":   note,
		"reviewed_by":   adminID,
		"reviewed_at":   time.Now(),
	}
	return DB.Model(&t).Where("author_id = ?", authorID).Updates(updates).Error
}

// RejectAuthorHomeCustomize 驳回。
func RejectAuthorHomeCustomize(authorID, adminID uint, note string) error {
	updates := map[string]interface{}{
		"review_status": "rejected",
		"public_html":   "",
		"review_note":   note,
		"reviewed_by":   adminID,
		"reviewed_at":   time.Now(),
	}
	return DB.Model(&AuthorHomeCustomize{}).Where("author_id = ?", authorID).Updates(updates).Error
}
