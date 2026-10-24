package groups

import (
	"nvs-server/models"

	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ==================== 群内帖子与回复 ====================
// 可见性由 Group.Visibility 决定：
//   public   —— 任何人都能读（未登录也可，用于 SEO/公开社群）
//   semi     —— 非成员只看到「帖子数/标题脱敏」，正文需成员
//   private  —— 全部内容仅成员
// 先审后发：Group.ReviewFirst = true 时新帖 status = pending，仅作者与群管可见。

const (
	GroupPostPending   = "pending"
	GroupPostPublished = "published"
	GroupPostRemoved   = "removed"
)

// GroupPost 群内帖子
type GroupPost struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	GroupID  uint   `gorm:"not null;index" json:"group_id"`
	UserID   uint   `gorm:"not null;index" json:"user_id"`
	Title    string `gorm:"size:128;not null" json:"title"`
	Content  string `gorm:"type:text" json:"content"`
	Status   string `gorm:"size:16;not null;default:published;index" json:"status"`
	ReplyCnt int64  `gorm:"not null;default:0" json:"reply_count"`
	// AI 辅助生成标识（与章节/博客同一套侧表，target_type = group_post）
	AiAssisted bool           `gorm:"not null;default:false" json:"ai_assisted"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	AuthorName   string `gorm:"-" json:"author_name,omitempty"`
	AuthorAvatar string `gorm:"-" json:"author_avatar,omitempty"`
	GroupName    string `gorm:"-" json:"group_name,omitempty"`
}

func (GroupPost) TableName() string { return "group_posts" }

// GroupPostReply 群内回帖（楼层模型：不做楼中楼，先做一层平铺）
type GroupPostReply struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	GroupID   uint           `gorm:"not null;index" json:"group_id"`
	PostID    uint           `gorm:"not null;index" json:"post_id"`
	UserID    uint           `gorm:"not null;index" json:"user_id"`
	Content   string         `gorm:"type:text" json:"content"`
	Status    string         `gorm:"size:16;not null;default:published;index" json:"status"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	AuthorName   string `gorm:"-" json:"author_name,omitempty"`
	AuthorAvatar string `gorm:"-" json:"author_avatar,omitempty"`
}

func (GroupPostReply) TableName() string { return "group_post_replies" }

// CreateGroupPost 发帖：成员校验 + 群状态 + 禁言 + 每日额度 + 先审后发
func CreateGroupPost(g *Group, user *models.User, title, content string, aiAssisted bool) (*GroupPost, error) {
	if g == nil || user == nil {
		return nil, fmt.Errorf("参数错误")
	}
	if g.Status != GroupStatusActive {
		return nil, fmt.Errorf("该群组当前不可发帖（状态：%s）", g.Status)
	}
	m, _ := GetMembership(g.ID, user.ID)
	if m == nil || m.Status != GroupMemberActive {
		return nil, fmt.Errorf("只有群成员可以发帖")
	}
	if muted, until := IsMutedInGroup(g.ID, user.ID); muted {
		return nil, fmt.Errorf("你已被群内禁言，解除时间：%s", until.Format("2006-01-02 15:04"))
	}
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	if len([]rune(title)) < 2 || len([]rune(title)) > 80 {
		return nil, fmt.Errorf("标题需 2~80 个字符")
	}
	if content == "" {
		return nil, fmt.Errorf("正文不能为空")
	}
	if len([]rune(content)) > 20000 {
		return nil, fmt.Errorf("正文过长（上限 20000 字）")
	}

	// 群内每日额度（0 = 不限）
	if g.PostLimit > 0 {
		var today int64
		models.DB.Model(&GroupPost{}).Where("group_id = ? AND user_id = ? AND created_at >= ?",
			g.ID, user.ID, startOfDay()).Count(&today)
		if int(today) >= g.PostLimit {
			return nil, fmt.Errorf("你在本群今日已发 %d 帖（上限 %d），明日恢复", today, g.PostLimit)
		}
	}
	// 全站群组每日额度（GroupQuotaConfig.DailyPosts，按用户跨群累计）
	cfg := GetGroupQuotaConfig()
	if cfg.DailyPosts > 0 {
		var todayAll int64
		models.DB.Model(&GroupPost{}).Where("user_id = ? AND created_at >= ?", user.ID, startOfDay()).Count(&todayAll)
		if int(todayAll) >= cfg.DailyPosts {
			return nil, fmt.Errorf("你今日群组发帖已达全站上限（%d 帖），明日恢复", cfg.DailyPosts)
		}
	}
	// 与全站每日发文额度联动（防止绕过限额批量铺内容）
	if allowed, used, limit, _, _ := models.CheckDailyPublish(user.ID, user.Role, models.PublishKindGroupPost, len([]rune(content))); !allowed {
		return nil, fmt.Errorf("今日发布额度已用完（%d/%d），群组发帖同样计入", used, limit)
	}

	status := GroupPostPublished
	if g.ReviewFirst {
		status = GroupPostPending
	}
	p := &GroupPost{
		GroupID: g.ID, UserID: user.ID, Title: title, Content: content,
		Status: status, AiAssisted: aiAssisted,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := models.DB.Create(p).Error; err != nil {
		return nil, fmt.Errorf("发帖失败")
	}
	// 精确写回（status 有默认值，struct Create 会被默认值覆盖）
	if err := models.DB.Model(&GroupPost{}).Where("id = ?", p.ID).Updates(map[string]interface{}{
		"status": status, "ai_assisted": aiAssisted,
	}).Error; err != nil {
		return nil, fmt.Errorf("发帖失败")
	}
	if status == GroupPostPublished {
		_ = models.RecordPublish(user.ID, models.PublishKindGroupPost, p.ID, len([]rune(content)))
		RefreshGroupCounts(g.ID)
	}
	return p, nil
}

// GetGroupPost 取帖子（不含软删）
func GetGroupPost(id uint) (*GroupPost, error) {
	var p GroupPost
	if err := models.DB.Where("id = ?", id).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// ListGroupPosts viewer 视角读帖：scope = published（默认）/ pending（群管审核队列）
func ListGroupPosts(groupID uint, viewerID uint, role string, includePending bool, page, size int) ([]GroupPost, int64, error) {
	q := models.DB.Model(&GroupPost{}).Where("group_id = ?", groupID)
	if includePending {
		if role == GroupRoleOwner || role == GroupRoleAdmin {
			// 群管可见 pending + published
			q = q.Where("status IN ?", []string{GroupPostPending, GroupPostPublished})
		} else {
			// 普通成员只看到自己的待审帖
			q = q.Where("status = ? OR (status = ? AND user_id = ?)",
				GroupPostPublished, GroupPostPending, viewerID)
		}
	} else {
		q = q.Where("status = ?", GroupPostPublished)
	}
	var total int64
	q.Count(&total)
	var list []GroupPost
	if err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	fillPostAuthors(list)
	return list, total, nil
}

// fillPostAuthors 批量填作者信息
func fillPostAuthors(list []GroupPost) {
	if len(list) == 0 {
		return
	}
	ids := make([]uint, 0, len(list))
	for _, p := range list {
		ids = append(ids, p.UserID)
	}
	var users []models.User
	models.DB.Select("id, username, nickname, avatar_url").Where("id IN ?", ids).Find(&users)
	byID := map[uint]models.User{}
	for _, u := range users {
		byID[u.ID] = u
	}
	for i := range list {
		if u, ok := byID[list[i].UserID]; ok {
			list[i].AuthorName = displayName(u)
			list[i].AuthorAvatar = u.AvatarURL
		}
	}
}

// displayName 展示名：昵称优先，其次用户名
func displayName(u models.User) string {
	if strings.TrimSpace(u.Nickname) != "" {
		return u.Nickname
	}
	return u.Username
}

// ReviewGroupPost 群管审核（通过/拒绝）；拒绝置 removed
func ReviewGroupPost(postID, operatorID uint, operatorRole string, approve bool) error {
	p, err := GetGroupPost(postID)
	if err != nil {
		return fmt.Errorf("帖子不存在")
	}
	g, err := GetGroup(p.GroupID)
	if err != nil {
		return fmt.Errorf("群组不存在")
	}
	role, ok := GroupRoleOf(g, operatorID)
	platform := IsGroupPlatformAdmin(operatorRole)
	if !ok && !platform {
		return fmt.Errorf("无权操作")
	}
	if role != GroupRoleOwner && role != GroupRoleAdmin && !platform {
		return fmt.Errorf("仅群主/群管理员可审核")
	}
	status := GroupPostRemoved
	if approve {
		status = GroupPostPublished
	}
	if err := models.DB.Model(&GroupPost{}).Where("id = ?", p.ID).Update("status", status).Error; err != nil {
		return err
	}
	if approve {
		_ = models.RecordPublish(p.UserID, models.PublishKindGroupPost, p.ID, len([]rune(p.Content)))
	}
	RefreshGroupCounts(p.GroupID)
	return nil
}

// DeleteGroupPost 删帖：作者本人 / 群管 / 平台管理员
func DeleteGroupPost(postID, operatorID uint, operatorRole string) error {
	p, err := GetGroupPost(postID)
	if err != nil {
		return fmt.Errorf("帖子不存在")
	}
	g, _ := GetGroup(p.GroupID)
	role := ""
	if g != nil {
		role, _ = GroupRoleOf(g, operatorID)
	}
	if p.UserID != operatorID && role != GroupRoleOwner && role != GroupRoleAdmin && !IsGroupPlatformAdmin(operatorRole) {
		return fmt.Errorf("无权删除该帖")
	}
	if err := models.DB.Delete(&GroupPost{}, postID).Error; err != nil {
		return err
	}
	RefreshGroupCounts(p.GroupID)
	return nil
}

// CreateGroupReply 回帖
func CreateGroupReply(g *Group, postID uint, user *models.User, content string) (*GroupPostReply, error) {
	if g == nil || user == nil {
		return nil, fmt.Errorf("参数错误")
	}
	if g.Status != GroupStatusActive {
		return nil, fmt.Errorf("该群组当前不可回帖")
	}
	if !IsGroupMemberActive(g.ID, user.ID) {
		return nil, fmt.Errorf("只有群成员可以回帖")
	}
	if muted, until := IsMutedInGroup(g.ID, user.ID); muted {
		return nil, fmt.Errorf("你已被群内禁言，解除时间：%s", until.Format("2006-01-02 15:04"))
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("回复内容不能为空")
	}
	if len([]rune(content)) > 2000 {
		return nil, fmt.Errorf("回复过长（上限 2000 字）")
	}
	p, err := GetGroupPost(postID)
	if err != nil || p.GroupID != g.ID {
		return nil, fmt.Errorf("帖子不存在")
	}
	if p.Status != GroupPostPublished {
		return nil, fmt.Errorf("该帖尚未通过审核")
	}
	r := &GroupPostReply{
		GroupID: g.ID, PostID: p.ID, UserID: user.ID, Content: content,
		Status: GroupPostPublished, CreatedAt: time.Now(),
	}
	if err := models.DB.Create(r).Error; err != nil {
		return nil, fmt.Errorf("回复失败")
	}
	models.DB.Model(&GroupPostReply{}).Where("id = ?", r.ID).Update("status", GroupPostPublished)
	models.DB.Model(&GroupPost{}).Where("id = ?", p.ID).UpdateColumn("reply_cnt", models.DB.Raw("reply_cnt + 1"))
	return r, nil
}

// ListGroupReplies 回帖列表
func ListGroupReplies(groupID, postID uint, page, size int) ([]GroupPostReply, int64, error) {
	q := models.DB.Model(&GroupPostReply{}).Where("group_id = ? AND post_id = ? AND status = ?",
		groupID, postID, GroupPostPublished)
	var total int64
	q.Count(&total)
	var list []GroupPostReply
	if err := q.Order("id ASC").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	if len(list) > 0 {
		ids := make([]uint, 0, len(list))
		for _, r := range list {
			ids = append(ids, r.UserID)
		}
		var users []models.User
		models.DB.Select("id, username, nickname, avatar_url").Where("id IN ?", ids).Find(&users)
		byID := map[uint]models.User{}
		for _, u := range users {
			byID[u.ID] = u
		}
		for i := range list {
			if u, ok := byID[list[i].UserID]; ok {
				list[i].AuthorName = displayName(u)
				list[i].AuthorAvatar = u.AvatarURL
			}
		}
	}
	return list, total, nil
}

// DeleteGroupReply 删除回复（作者/群管/平台）
func DeleteGroupReply(replyID, operatorID uint, operatorRole string) error {
	var r GroupPostReply
	if err := models.DB.Where("id = ?", replyID).First(&r).Error; err != nil {
		return fmt.Errorf("回复不存在")
	}
	g, _ := GetGroup(r.GroupID)
	role := ""
	if g != nil {
		role, _ = GroupRoleOf(g, operatorID)
	}
	if r.UserID != operatorID && role != GroupRoleOwner && role != GroupRoleAdmin && !IsGroupPlatformAdmin(operatorRole) {
		return fmt.Errorf("无权删除该回复")
	}
	if err := models.DB.Delete(&GroupPostReply{}, replyID).Error; err != nil {
		return err
	}
	var left int64
	models.DB.Model(&GroupPostReply{}).Where("post_id = ?", r.PostID).Count(&left)
	models.DB.Model(&GroupPost{}).Where("id = ?", r.PostID).UpdateColumn("reply_cnt", left)
	return nil
}
