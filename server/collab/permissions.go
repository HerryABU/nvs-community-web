package collab

import (
	"nvs-server/models"
)

// ==================== 协作权限矩阵（与 docs/novel-collaboration.md §2 对应） ====================

// Capability 能力标识
type Capability string

const (
	CapEditNovel      Capability = "edit_novel"      // 改作品资料/封面/简介/分类
	CapSwitchMode     Capability = "switch_mode"     // 切换散文体/话本模式
	CapManageCollab   Capability = "manage_collab"   // 管理协作者
	CapWriteChapter   Capability = "write_chapter"   // 新建章节
	CapEditOthers     Capability = "edit_others"     // 编辑他人章节
	CapReviewChapter  Capability = "review_chapter"  // 审核章节
	CapPublishChapter Capability = "publish_chapter" // 直接发布/撤回
	CapViewStats      Capability = "view_stats"      // 看作品数据
	CapExport         Capability = "export"          // 导出作品
	CapUseAiContext   Capability = "use_ai_context"  // 用作品上下文调 AI
)

// Actor 当前用户在某作品上的协作身份
type Actor struct {
	NovelID       uint
	UserID        uint
	IsOwner       bool
	IsPlatform    bool
	Role          string
	Status        string
	CanPublish    bool
	CanReview     bool
	CanEditOthers bool
}

// ActorOf 解析协作身份。platformRole 传 middleware.UserRole(c)（admin/police）。
func ActorOf(novelID, userID uint, platformRole string) Actor {
	a := Actor{NovelID: novelID, UserID: userID}
	a.IsPlatform = platformRole == models.RoleAdmin || platformRole == models.RolePolice
	if novel, err := models.GetNovelByID(novelID); err == nil && novel != nil {
		a.IsOwner = userID != 0 && novel.AuthorID == userID
	}
	if userID == 0 {
		return a
	}
	if c, err := Get(novelID, userID); err == nil && c != nil {
		a.Role = c.Role
		a.Status = c.Status
		a.CanPublish = c.CanPublish
		a.CanReview = c.CanReview
		a.CanEditOthers = c.CanEditOthers
	}
	return a
}

// ActiveMember 是否为生效中的协作成员
func (a Actor) ActiveMember() bool {
	return a.Status == StatusActive && a.Role != ""
}

// Can 能力判定（严格按文档矩阵）
func (a Actor) Can(cap Capability) bool {
	// 平台角色只做合规处置，不获得创作能力（导出/查看数据除外）
	if a.IsPlatform {
		switch cap {
		case CapExport, CapViewStats, CapReviewChapter:
			return true
		default:
			return false
		}
	}
	if a.IsOwner {
		return true
	}
	if !a.ActiveMember() {
		return false
	}
	switch cap {
	case CapExport:
		return true
	case CapWriteChapter:
		return a.Role == RoleCoAuthor // 编辑/审稿不新建章节（文档 §2.2）
	case CapEditOthers:
		return a.Role == RoleEditor || a.CanEditOthers
	case CapReviewChapter:
		return a.Role == RoleReviewer || a.CanReview
	case CapPublishChapter:
		return a.CanPublish
	case CapUseAiContext:
		return a.Role == RoleCoAuthor || a.Role == RoleEditor || a.Role == RoleReviewer
	default:
		// 作品级设置、协作者管理、看数据：仅主笔（平台已在上面处理）
		return false
	}
}

// CanEditChapter 章节编辑：主笔/平台；本人章节（协作作者/编辑）；有 edit_others 的成员
func (a Actor) CanEditChapter(ch *models.Chapter) bool {
	if ch == nil {
		return false
	}
	if a.IsPlatform || a.IsOwner {
		return true
	}
	if !a.ActiveMember() {
		return false
	}
	if ch.AuthorID == a.UserID && (a.Role == RoleCoAuthor || a.Role == RoleEditor) {
		return true
	}
	return a.Can(CapEditOthers)
}

// CanDeleteChapter 章节删除：主笔/平台；本人且非已发布
func (a Actor) CanDeleteChapter(ch *models.Chapter) bool {
	if ch == nil {
		return false
	}
	if a.IsPlatform || a.IsOwner {
		return true
	}
	if !a.ActiveMember() {
		return false
	}
	if ch.AuthorID == a.UserID && a.Role == RoleCoAuthor {
		return ch.Status != "published" // 已发布章节只能撤回，不能直接删
	}
	return false
}

// CanUseAiContext 是否可用作品上下文（AI 模板注入）：主笔或任一 active 协作者
func CanUseAiContext(novelID, userID uint, platformRole string) bool {
	a := ActorOf(novelID, userID, platformRole)
	return a.IsOwner || a.IsPlatform || a.Can(CapUseAiContext)
}

// IsCollaborator 是否 active 协作者（便于前端显示角色徽标）
func IsCollaborator(novelID, userID uint) bool {
	c, err := Get(novelID, userID)
	return err == nil && c != nil && c.Status == StatusActive
}
