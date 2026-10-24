package collab

import (
	"fmt"
	"strings"
	"time"

	"nvs-server/models"

	"gorm.io/gorm"
)

// ==================== 多人协作小说：协作成员 ====================
// 规则基线见 docs/novel-collaboration.md（先定规则、再落代码）。
// 角色：主笔 owner（novels.author_id，不存本表） / 协作作者 co_author / 编辑 editor / 审稿 reviewer。
// 状态：pending（邀请未接受，不授予任何权限）→ active（生效）。
//
// 细粒度能力（默认值由角色决定，主笔可逐项覆盖）：
//   can_publish      直接发布章节（跳过审核）
//   can_review       审核他人章节（通过/驳回）
//   can_edit_others  编辑他人章节
//
// 收益与署名：
//   RevenueShareBP 万分比（0~10000），所有协作者总和不得超过 10000（主笔拿剩余）
//   BylineOrder    署名顺序，越小越前（主笔固定第一）

const (
	RoleCoAuthor = "co_author"
	RoleEditor   = "editor"
	RoleReviewer = "reviewer"

	StatusPending = "pending"
	StatusActive  = "active"

	// MaxCollaborators 单作品协作人数上限（不含主笔）
	MaxCollaborators = 10
)

// Collaborator 协作成员
type Collaborator struct {
	ID      uint   `gorm:"primaryKey" json:"id"`
	NovelID uint   `gorm:"not null;uniqueIndex:idx_novel_collab" json:"novel_id"`
	UserID  uint   `gorm:"not null;uniqueIndex:idx_novel_collab;index" json:"user_id"`
	Role    string `gorm:"size:16;not null;default:co_author" json:"role"`
	Status  string `gorm:"size:16;not null;default:pending;index" json:"status"`

	CanPublish    bool `gorm:"not null;default:false" json:"can_publish"`
	CanReview     bool `gorm:"not null;default:false" json:"can_review"`
	CanEditOthers bool `gorm:"not null;default:false" json:"can_edit_others"`

	RevenueShareBP int    `gorm:"not null;default:0" json:"revenue_share_bp"` // 万分比
	RevenueNote    string `gorm:"size:256;default:''" json:"revenue_note"`
	BylineOrder    int    `gorm:"not null;default:100" json:"byline_order"`

	InvitedBy uint      `gorm:"not null;default:0" json:"invited_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// 运行时
	Username string `gorm:"-" json:"username,omitempty"`
	Nickname string `gorm:"-" json:"nickname,omitempty"`
	Avatar   string `gorm:"-" json:"avatar_url,omitempty"`
}

func (Collaborator) TableName() string { return "novel_collaborators" }

// Audit 协作审计（append-only）
type Audit struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	NovelID      uint      `gorm:"not null;index" json:"novel_id"`
	ActorID      uint      `gorm:"not null;index" json:"actor_id"`
	Action       string    `gorm:"size:24;not null;index" json:"action"`
	TargetUserID uint      `gorm:"not null;default:0" json:"target_user_id"`
	Before       string    `gorm:"type:text" json:"before"`
	After        string    `gorm:"type:text" json:"after"`
	CreatedAt    time.Time `json:"created_at"`
}

func (Audit) TableName() string { return "novel_collab_audits" }

// Migrate 建表（独立功能包，由 main 在主导航迁移后调用）
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&Collaborator{}, &Audit{})
}

// NormalizeRole 归一化角色（非法 → co_author）
func NormalizeRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case RoleEditor:
		return RoleEditor
	case RoleReviewer:
		return RoleReviewer
	default:
		return RoleCoAuthor
	}
}

// roleDefaults 角色默认能力（文档 §2.2 默认值）
func roleDefaults(role string) (canPublish, canReview, canEditOthers bool) {
	switch role {
	case RoleEditor:
		return false, false, true
	case RoleReviewer:
		return false, true, false
	default: // co_author
		return false, false, false
	}
}

// writeAudit 写审计（失败不阻断业务，但记录到日志由调用方决定）
func writeAudit(novelID, actorID uint, action string, targetUserID uint, before, after string) {
	_ = models.DB.Create(&Audit{
		NovelID: novelID, ActorID: actorID, Action: action,
		TargetUserID: targetUserID, Before: before, After: after, CreatedAt: time.Now(),
	}).Error
}

// LogChapterAction 章节级协作审计（发布/驳回/撤回/提审），供 handlers 调用
func LogChapterAction(novelID, actorID uint, action string, chapterID uint, note string) {
	writeAudit(novelID, actorID, action, 0, fmt.Sprintf("chapter=%d", chapterID), note)
}

// List 作品的协作成员（默认只返回 active；includePending=true 时含待接受）
func List(novelID uint, includePending bool) ([]Collaborator, error) {
	q := models.DB.Where("novel_id = ?", novelID)
	if !includePending {
		q = q.Where("status = ?", StatusActive)
	}
	var list []Collaborator
	if err := q.Order("byline_order ASC, created_at ASC").Limit(50).Find(&list).Error; err != nil {
		return nil, err
	}
	fillProfiles(list)
	return list, nil
}

// fillProfiles 批量补充用户名/昵称/头像
func fillProfiles(list []Collaborator) {
	if len(list) == 0 {
		return
	}
	ids := make([]uint, 0, len(list))
	for _, c := range list {
		ids = append(ids, c.UserID)
	}
	var users []models.User
	models.DB.Select("id, username, nickname, avatar_url").Where("id IN ?", ids).Find(&users)
	byID := map[uint]models.User{}
	for _, u := range users {
		byID[u.ID] = u
	}
	for i := range list {
		if u, ok := byID[list[i].UserID]; ok {
			list[i].Username = u.Username
			list[i].Nickname = u.Nickname
			list[i].Avatar = u.AvatarURL
		}
	}
}

// Get 取单个协作成员
func Get(novelID, userID uint) (*Collaborator, error) {
	var c Collaborator
	if err := models.DB.Where("novel_id = ? AND user_id = ?", novelID, userID).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

// ListPendingByUser 我收到的待接受邀请
func ListPendingByUser(userID uint) ([]Collaborator, error) {
	var list []Collaborator
	if err := models.DB.Where("user_id = ? AND status = ?", userID, StatusPending).
		Order("created_at DESC").Limit(50).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// ListActiveByUser 我参与的协作作品
func ListActiveByUser(userID uint) ([]Collaborator, error) {
	var list []Collaborator
	if err := models.DB.Where("user_id = ? AND status = ?", userID, StatusActive).
		Order("created_at DESC").Limit(100).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// Invite 邀请协作者（仅主笔）
func Invite(novelID, inviterID uint, username, role string) (*Collaborator, error) {
	novel, err := models.GetNovelByID(novelID)
	if err != nil {
		return nil, fmt.Errorf("作品不存在")
	}
	if novel.AuthorID != inviterID {
		return nil, fmt.Errorf("仅主笔可邀请协作者")
	}
	username = strings.TrimSpace(strings.TrimPrefix(username, "@"))
	if username == "" {
		return nil, fmt.Errorf("请填写要邀请的用户名")
	}
	var user models.User
	if err := models.DB.Where("username = ?", username).First(&user).Error; err != nil {
		if err2 := models.DB.Where("nickname = ?", username).First(&user).Error; err2 != nil {
			return nil, fmt.Errorf("用户 %s 不存在", username)
		}
	}
	if user.ID == inviterID {
		return nil, fmt.Errorf("不能邀请自己")
	}

	// 已在协作名单（含待接受）
	if exist, _ := Get(novelID, user.ID); exist != nil {
		if exist.Status == StatusActive {
			return nil, fmt.Errorf("%s 已是协作者", username)
		}
		return nil, fmt.Errorf("%s 已有待接受的邀请", username)
	}
	var count int64
	models.DB.Model(&Collaborator{}).Where("novel_id = ?", novelID).Count(&count)
	if int(count) >= MaxCollaborators {
		return nil, fmt.Errorf("协作成员已达上限（%d 人）", MaxCollaborators)
	}

	role = NormalizeRole(role)
	cp, cr, ce := roleDefaults(role)
	c := &Collaborator{
		NovelID: novelID, UserID: user.ID, Role: role, Status: StatusPending,
		CanPublish: cp, CanReview: cr, CanEditOthers: ce,
		BylineOrder: 100, InvitedBy: inviterID,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := models.DB.Create(c).Error; err != nil {
		return nil, fmt.Errorf("邀请失败")
	}
	// 布尔零值/默认值需要 map 精确写回
	_ = models.DB.Model(&Collaborator{}).Where("id = ?", c.ID).Updates(map[string]interface{}{
		"role": role, "status": StatusPending,
		"can_publish": cp, "can_review": cr, "can_edit_others": ce,
		"byline_order": 100,
	}).Error
	writeAudit(novelID, inviterID, "invite", user.ID, "", role)

	_ = models.PushNotification(inviterID, user.ID, "novel_collab",
		fmt.Sprintf("《%s》邀请你参与协作", novel.Title),
		fmt.Sprintf("角色：%s。接受后即可按权限参与创作。", roleLabel(role)),
		"/author/collab")

	c.Username = user.Username
	c.Nickname = user.Nickname
	c.Avatar = user.AvatarURL
	return c, nil
}

// Respond 被邀请人接受/拒绝
func Respond(novelID, userID uint, accept bool) error {
	c, err := Get(novelID, userID)
	if err != nil {
		return fmt.Errorf("没有待处理的邀请")
	}
	if c.Status != StatusPending {
		return fmt.Errorf("该邀请已处理")
	}
	if !accept {
		if err := models.DB.Where("id = ?", c.ID).Delete(&Collaborator{}).Error; err != nil {
			return err
		}
		writeAudit(novelID, userID, "decline", userID, StatusPending, "")
		return nil
	}
	if err := models.DB.Model(&Collaborator{}).Where("id = ?", c.ID).Updates(map[string]interface{}{
		"status": StatusActive, "updated_at": time.Now(),
	}).Error; err != nil {
		return err
	}
	writeAudit(novelID, userID, "accept", userID, StatusPending, StatusActive)
	return nil
}

// UpdateInput 协作设置修改（主笔）
type UpdateInput struct {
	Role           string  `json:"role"`
	CanPublish     *bool   `json:"can_publish"`
	CanReview      *bool   `json:"can_review"`
	CanEditOthers  *bool   `json:"can_edit_others"`
	RevenueShareBP *int    `json:"revenue_share_bp"`
	RevenueNote    *string `json:"revenue_note"`
	BylineOrder    *int    `json:"byline_order"`
}

// Update 修改协作成员（角色/能力/分成/署名顺序），仅主笔
func Update(novelID, operatorID, targetUserID uint, in UpdateInput) error {
	novel, err := models.GetNovelByID(novelID)
	if err != nil {
		return fmt.Errorf("作品不存在")
	}
	if novel.AuthorID != operatorID {
		return fmt.Errorf("仅主笔可修改协作设置")
	}
	c, err := Get(novelID, targetUserID)
	if err != nil {
		return fmt.Errorf("该用户不是协作者")
	}
	before := fmt.Sprintf("role=%s caps=%v/%v/%v share=%d byline=%d",
		c.Role, c.CanPublish, c.CanReview, c.CanEditOthers, c.RevenueShareBP, c.BylineOrder)

	fields := map[string]interface{}{"updated_at": time.Now()}
	role := c.Role
	if strings.TrimSpace(in.Role) != "" {
		role = NormalizeRole(in.Role)
		cp, cr, ce := roleDefaults(role)
		fields["role"] = role
		// 换角色时先套默认能力，再让显式传值覆盖
		fields["can_publish"] = cp
		fields["can_review"] = cr
		fields["can_edit_others"] = ce
	}
	if in.CanPublish != nil {
		fields["can_publish"] = *in.CanPublish
	}
	if in.CanReview != nil {
		fields["can_review"] = *in.CanReview
	}
	if in.CanEditOthers != nil {
		fields["can_edit_others"] = *in.CanEditOthers
	}
	if in.RevenueNote != nil {
		fields["revenue_note"] = strings.TrimSpace(*in.RevenueNote)
	}
	if in.RevenueShareBP != nil {
		share := *in.RevenueShareBP
		if share < 0 || share > 10000 {
			return fmt.Errorf("分成比例需在 0~10000（万分比）之间")
		}
		// 总和校验：其他 active 成员之和 + 本次 ≤ 10000
		var others int64
		models.DB.Model(&Collaborator{}).
			Where("novel_id = ? AND status = ? AND user_id <> ?", novelID, StatusActive, targetUserID).
			Select("COALESCE(SUM(revenue_share_bp),0)").Scan(&others)
		if int(others)+share > 10000 {
			return fmt.Errorf("分成总和超过 100%%，当前其他成员已占 %d‱", others)
		}
		fields["revenue_share_bp"] = share
	}
	if in.BylineOrder != nil {
		order := *in.BylineOrder
		if order < 0 || order > 999 {
			return fmt.Errorf("署名顺序需在 0~999 之间")
		}
		fields["byline_order"] = order
	}
	if err := models.DB.Model(&Collaborator{}).Where("id = ?", c.ID).Updates(fields).Error; err != nil {
		return fmt.Errorf("保存失败")
	}
	after := fmt.Sprintf("role=%v caps=%v/%v/%v share=%v byline=%v",
		fields["role"], fields["can_publish"], fields["can_review"],
		fields["can_edit_others"], fields["revenue_share_bp"], fields["byline_order"])
	writeAudit(novelID, operatorID, "update", targetUserID, before, after)
	return nil
}

// Remove 移除协作者（主笔）
func Remove(novelID, operatorID, targetUserID uint) error {
	novel, err := models.GetNovelByID(novelID)
	if err != nil {
		return fmt.Errorf("作品不存在")
	}
	if novel.AuthorID != operatorID {
		return fmt.Errorf("仅主笔可移除协作者")
	}
	c, err := Get(novelID, targetUserID)
	if err != nil {
		return fmt.Errorf("该用户不是协作者")
	}
	if err := models.DB.Where("id = ?", c.ID).Delete(&Collaborator{}).Error; err != nil {
		return err
	}
	// 草稿转交主笔（已发布章节署名保留）
	models.DB.Model(&models.Chapter{}).Where("novel_id = ? AND author_id = ? AND status <> ?",
		novelID, targetUserID, "published").Update("author_id", novel.AuthorID)
	writeAudit(novelID, operatorID, "remove", targetUserID, c.Role, "")
	return nil
}

// Leave 协作者主动退出（草稿转交主笔，历史署名保留）
func Leave(novelID, userID uint) error {
	novel, err := models.GetNovelByID(novelID)
	if err != nil {
		return fmt.Errorf("作品不存在")
	}
	if novel.AuthorID == userID {
		return fmt.Errorf("主笔不能退出自己的作品")
	}
	c, err := Get(novelID, userID)
	if err != nil {
		return fmt.Errorf("你不是该作品的协作者")
	}
	if err := models.DB.Where("id = ?", c.ID).Delete(&Collaborator{}).Error; err != nil {
		return err
	}
	models.DB.Model(&models.Chapter{}).Where("novel_id = ? AND author_id = ? AND status <> ?",
		novelID, userID, "published").Update("author_id", novel.AuthorID)
	writeAudit(novelID, userID, "leave", userID, c.Role, "")
	return nil
}

// Byline 作品署名文本：「主笔：X · 协作：A、B」
func Byline(novelID uint) string {
	novel, err := models.GetNovelByID(novelID)
	if err != nil {
		return ""
	}
	ownerName := ""
	if u, uerr := models.GetUserByID(novel.AuthorID); uerr == nil && u != nil {
		ownerName = collectDisplayName(u)
	}
	list, _ := List(novelID, false)
	names := make([]string, 0, len(list))
	for _, c := range list {
		if c.Username == "" && c.Nickname == "" {
			continue
		}
		if c.Nickname != "" {
			names = append(names, c.Nickname)
		} else {
			names = append(names, c.Username)
		}
	}
	if len(names) == 0 {
		return "主笔：" + ownerName
	}
	return "主笔：" + ownerName + " · 协作：" + strings.Join(names, "、")
}

// RoleLabel 角色中文名
func RoleLabel(role string) string { return roleLabel(role) }

func roleLabel(role string) string {
	switch role {
	case RoleEditor:
		return "编辑"
	case RoleReviewer:
		return "审稿"
	default:
		return "协作作者"
	}
}

func collectDisplayName(u *models.User) string {
	if u == nil {
		return ""
	}
	if strings.TrimSpace(u.Nickname) != "" {
		return u.Nickname
	}
	return u.Username
}
