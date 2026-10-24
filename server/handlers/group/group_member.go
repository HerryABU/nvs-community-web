package grouphandler

import (
	"nvs-server/groups"
	"nvs-server/middleware"
	"strconv"
	"strings"
	"time"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== 群组：成员 / 拉人 / 踢人 / 角色 / 禁言 / 申请审核 ====================

// GroupMembers GET /api/groups/:id/members?status=&page=
func GroupMembers(c *gin.Context) {
	gid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	ctx, ok := loadGroupCtx(c, uint(gid))
	if !ok {
		return
	}
	// 成员名单仅成员可见（平台管理员除外）
	if ctx.Status != groups.GroupMemberActive && !ctx.IsAdmin {
		utils.Forbidden(c, "仅群成员可查看成员名单")
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
	status := strings.TrimSpace(c.Query("status"))
	// 只有群管能查待审列表
	if status == groups.GroupMemberPending && ctx.Role != groups.GroupRoleOwner &&
		ctx.Role != groups.GroupRoleAdmin && !ctx.IsAdmin {
		utils.Forbidden(c, "仅群主/群管理员可查看申请")
		return
	}
	list, total, err := groups.ListGroupMembers(ctx.Group.ID, status, page, size)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{"list": list, "total": total, "page": page, "size": size})
}

// GroupInvite POST /api/groups/:id/invite { username } —— 拉人入群
func GroupInvite(c *gin.Context) {
	gid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	uid, ok := requireGroupLogin(c)
	if !ok {
		return
	}
	g, err := groups.GetGroup(uint(gid))
	if err != nil {
		utils.NotFound(c, "群组不存在")
		return
	}
	var req struct {
		Username string `json:"username"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	operator, _ := models.GetUserByID(uid)
	opName := "群成员"
	if operator != nil {
		opName = operator.Username
	}
	m, ierr := groups.InviteMember(g, uid, middleware.UserRole(c), opName, req.Username)
	if ierr != nil {
		utils.BadRequest(c, ierr.Error())
		return
	}
	utils.SuccessMessage(c, "已邀请入群", gin.H{"member": m})
}

// GroupMemberApprove POST /api/groups/:id/requests/:uid { approve: bool }
func GroupMemberApprove(c *gin.Context) {
	gid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	target, _ := strconv.ParseUint(c.Param("uid"), 10, 64)
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
	if err := groups.ApproveMember(uint(gid), uid, middleware.UserRole(c), uint(target), req.Approve); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if req.Approve {
		publishGroupEvent(uint(gid), GroupEventMemberJoined, gin.H{
			"user_id": uint(target), "at": time.Now().Unix(),
		})
		groups.RefreshGroupCounts(uint(gid))
	}
	msg := "已通过申请"
	if !req.Approve {
		msg = "已拒绝申请"
	}
	utils.SuccessMessage(c, msg, nil)
}

// GroupMemberSetRole PUT /api/groups/:id/members/:uid { role }
func GroupMemberSetRole(c *gin.Context) {
	gid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	target, _ := strconv.ParseUint(c.Param("uid"), 10, 64)
	uid, ok := requireGroupLogin(c)
	if !ok {
		return
	}
	var req struct {
		Role string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := groups.SetMemberRole(uint(gid), uid, middleware.UserRole(c), uint(target), req.Role); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "已更新角色", nil)
}

// GroupMemberMute PUT /api/groups/:id/members/:uid/mute { minutes }
func GroupMemberMute(c *gin.Context) {
	gid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	target, _ := strconv.ParseUint(c.Param("uid"), 10, 64)
	uid, ok := requireGroupLogin(c)
	if !ok {
		return
	}
	var req struct {
		Minutes int `json:"minutes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := groups.MuteMember(uint(gid), uid, middleware.UserRole(c), uint(target), req.Minutes); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if req.Minutes > 0 {
		utils.SuccessMessage(c, "已禁言", nil)
		return
	}
	utils.SuccessMessage(c, "已解除禁言", nil)
}

// GroupMemberRemove DELETE /api/groups/:id/members/:uid —— 踢人 / 自己退群
func GroupMemberRemove(c *gin.Context) {
	gid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	target, _ := strconv.ParseUint(c.Param("uid"), 10, 64)
	uid, ok := requireGroupLogin(c)
	if !ok {
		return
	}
	if uint(target) == uid {
		if err := groups.LeaveGroup(uint(gid), uid); err != nil {
			utils.BadRequest(c, err.Error())
			return
		}
		utils.SuccessMessage(c, "已退出群组", nil)
		return
	}
	if err := groups.RemoveMember(uint(gid), uid, middleware.UserRole(c), uint(target)); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "已移出群组", nil)
}

// GroupAnnounce PUT /api/groups/:id/announcement { content }
func GroupAnnounce(c *gin.Context) {
	gid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	uid, ok := requireGroupLogin(c)
	if !ok {
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := groups.SetGroupAnnouncement(uint(gid), uid, middleware.UserRole(c), req.Content); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "公告已更新", nil)
}

// GroupInviteCode GET /api/groups/:id/invite-code —— 查看/重置邀请码
func GroupInviteCode(c *gin.Context) {
	gid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	uid, ok := requireGroupLogin(c)
	if !ok {
		return
	}
	g, err := groups.GetGroup(uint(gid))
	if err != nil {
		utils.NotFound(c, "群组不存在")
		return
	}
	role, inGroup := groups.GroupRoleOf(g, uid)
	if !inGroup && !groups.IsGroupPlatformAdmin(middleware.UserRole(c)) {
		utils.Forbidden(c, "仅群成员可获取邀请码")
		return
	}
	if role != groups.GroupRoleOwner && role != groups.GroupRoleAdmin && !g.AllowMemberInvite &&
		!groups.IsGroupPlatformAdmin(middleware.UserRole(c)) {
		utils.Forbidden(c, "该群已关闭成员邀请")
		return
	}
	utils.Success(c, gin.H{"invite_code": g.InviteCode, "group_no": g.GroupNo})
}
