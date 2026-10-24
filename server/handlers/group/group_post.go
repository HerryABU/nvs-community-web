package grouphandler

import (
	"nvs-server/groups"
	"nvs-server/middleware"
	"nvs-server/security"
	"strconv"
	"strings"
	"time"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== 群内帖子与回复 ====================
// 审核链路与论坛一致：违禁词库（processBadWords）→ mark 命中写待审标记（flagBadWords）；
// 群组自己的「先审后发」由 Group.ReviewFirst 控制（pending → 群管审核）。
// AI 辅助创作：帖子带 ai_assisted 时落 ai_content_marks（与章节/博客同一套）。

// GroupPostList GET /api/groups/:id/posts?pending=1&page=
func GroupPostList(c *gin.Context) {
	gid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	ctx, ok := loadGroupCtx(c, uint(gid))
	if !ok {
		return
	}
	if !ctx.CanRead && ctx.Status != groups.GroupMemberActive {
		utils.Forbidden(c, "该群内容仅成员可见")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 50 {
		size = 20
	}
	includePending := c.Query("pending") == "1"
	list, total, err := groups.ListGroupPosts(ctx.Group.ID, middleware.UserID(c), ctx.Role, includePending, page, size)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{"list": list, "total": total, "page": page, "size": size})
}

// GroupPostCreate POST /api/groups/:id/posts { title, content, ai_assisted }
func GroupPostCreate(c *gin.Context) {
	gid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	uid, ok := requireGroupLogin(c)
	if !ok {
		return
	}
	ctx, ok := loadGroupCtx(c, uint(gid))
	if !ok {
		return
	}
	user, err := models.GetUserByID(uid)
	if err != nil {
		utils.Unauthorized(c, "用户不存在")
		return
	}
	var req struct {
		Title      string `json:"title"`
		Content    string `json:"content"`
		AiAssisted bool   `json:"ai_assisted"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	// 违禁词：替换/删除动作在保存前生效，mark 动作留待审标记
	cleaned, badHits := security.ApplyBadWords(req.Content, "group")
	p, perr := groups.CreateGroupPost(ctx.Group, user, req.Title, cleaned, req.AiAssisted)
	if perr != nil {
		utils.BadRequest(c, perr.Error())
		return
	}
	markBadWords(badHits, "group_post", p.ID, uid, cleaned, p.Title)
	if p.AiAssisted {
		_ = models.MarkAiContent(models.AiMarkGroupPost, p.ID, "")
	}
	// 实时消息：通知在线成员新帖（pending 由前端按状态提示「待审」）
	publishGroupEvent(ctx.Group.ID, GroupEventPostCreated, gin.H{
		"post_id": p.ID, "title": p.Title, "status": p.Status,
		"author_id": uid, "author": displayNameOf(user), "at": time.Now().Unix(),
	})
	// 群内 @ai：异步生成机器人回复
	maybeTriggerGroupAi(ctx.Group, p.ID, uid, p.Title, p.Content)

	msg := "发布成功"
	if p.Status == groups.GroupPostPending {
		msg = "已提交，等待群主审核后可见"
	}
	utils.SuccessMessage(c, msg, gin.H{"post": p})
}

// GroupPostDetail GET /api/groups/:id/posts/:pid
func GroupPostDetail(c *gin.Context) {
	gid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	pid, _ := strconv.ParseUint(c.Param("pid"), 10, 64)
	ctx, ok := loadGroupCtx(c, uint(gid))
	if !ok {
		return
	}
	p, err := groups.GetGroupPost(uint(pid))
	if err != nil || p.GroupID != ctx.Group.ID {
		utils.NotFound(c, "帖子不存在")
		return
	}
	uid := middleware.UserID(c)
	isManager := ctx.Role == groups.GroupRoleOwner || ctx.Role == groups.GroupRoleAdmin || ctx.IsAdmin
	if p.Status == groups.GroupPostPending && p.UserID != uid && !isManager {
		utils.NotFound(c, "帖子不存在")
		return
	}
	if !ctx.CanRead && ctx.Status != groups.GroupMemberActive && !isManager {
		utils.Forbidden(c, "该群内容仅成员可见")
		return
	}
	if author, aerr := models.GetUserByID(p.UserID); aerr == nil {
		p.AuthorName = author.Nickname
		if strings.TrimSpace(p.AuthorName) == "" {
			p.AuthorName = author.Username
		}
		p.AuthorAvatar = author.AvatarURL
	}
	utils.Success(c, gin.H{
		"post":     p,
		"group":    gin.H{"id": ctx.Group.ID, "name": ctx.Group.Name, "group_no": ctx.Group.GroupNo},
		"my_role":  ctx.Role,
		"can_edit": p.UserID == uid || isManager,
	})
}

// GroupPostReview POST /api/groups/:id/posts/:pid/review { approve }
func GroupPostReview(c *gin.Context) {
	pid, _ := strconv.ParseUint(c.Param("pid"), 10, 64)
	uid, ok := requireGroupLogin(c)
	if !ok {
		return
	}
	var req struct {
		Approve bool `json:"approve"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := groups.ReviewGroupPost(uint(pid), uid, middleware.UserRole(c), req.Approve); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if req.Approve {
		utils.SuccessMessage(c, "已通过", nil)
		return
	}
	utils.SuccessMessage(c, "已拒绝", nil)
}

// GroupPostDelete DELETE /api/groups/:id/posts/:pid
func GroupPostDelete(c *gin.Context) {
	pid, _ := strconv.ParseUint(c.Param("pid"), 10, 64)
	uid, ok := requireGroupLogin(c)
	if !ok {
		return
	}
	if err := groups.DeleteGroupPost(uint(pid), uid, middleware.UserRole(c)); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "已删除", nil)
}

// GroupReplyList GET /api/groups/:id/posts/:pid/replies
func GroupReplyList(c *gin.Context) {
	gid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	pid, _ := strconv.ParseUint(c.Param("pid"), 10, 64)
	ctx, ok := loadGroupCtx(c, uint(gid))
	if !ok {
		return
	}
	if !ctx.CanRead && ctx.Status != groups.GroupMemberActive {
		utils.Forbidden(c, "该群内容仅成员可见")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "50"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 50
	}
	list, total, err := groups.ListGroupReplies(ctx.Group.ID, uint(pid), page, size)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{"list": list, "total": total, "page": page, "size": size})
}

// GroupReplyCreate POST /api/groups/:id/posts/:pid/replies { content }
func GroupReplyCreate(c *gin.Context) {
	gid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	pid, _ := strconv.ParseUint(c.Param("pid"), 10, 64)
	uid, ok := requireGroupLogin(c)
	if !ok {
		return
	}
	ctx, ok := loadGroupCtx(c, uint(gid))
	if !ok {
		return
	}
	user, err := models.GetUserByID(uid)
	if err != nil {
		utils.Unauthorized(c, "用户不存在")
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	cleaned, badHits := security.ApplyBadWords(req.Content, "group")
	r, rerr := groups.CreateGroupReply(ctx.Group, uint(pid), user, cleaned)
	if rerr != nil {
		utils.BadRequest(c, rerr.Error())
		return
	}
	markBadWords(badHits, "group_reply", r.ID, uid, cleaned, "")
	publishGroupEvent(ctx.Group.ID, GroupEventReplyCreated, gin.H{
		"post_id": uint(pid), "reply_id": r.ID, "author_id": uid,
		"author": displayNameOf(user), "at": time.Now().Unix(),
	})
	maybeTriggerGroupAiReply(ctx.Group, uint(pid), uid, cleaned, r.ID)
	utils.SuccessMessage(c, "回复成功", gin.H{"reply": r})
}

// GroupReplyDelete DELETE /api/groups/:id/posts/:pid/replies/:rid
func GroupReplyDelete(c *gin.Context) {
	rid, _ := strconv.ParseUint(c.Param("rid"), 10, 64)
	uid, ok := requireGroupLogin(c)
	if !ok {
		return
	}
	if err := groups.DeleteGroupReply(uint(rid), uid, middleware.UserRole(c)); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "已删除", nil)
}
