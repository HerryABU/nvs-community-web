package grouphandler

import (
	"nvs-server/groups"
	"nvs-server/middleware"
	"strconv"
	"strings"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== 群组：列表 / 详情 / 建群 / 改设置 ====================
// 与论坛的区别：群组有群号（可搜索、可扫码加入）、可见性、角色、拉人/踢人、群公告、群文件。
// 私密不豁免平台审核：群内帖子照样过违禁词库，平台管理员可强制介入。

// groupViewCtx 当前用户在群内的视角上下文
type groupViewCtx struct {
	Group   *groups.Group
	Role    string // owner/admin/member/""（非成员）
	Status  string // active/pending/""
	CanRead bool
	IsAdmin bool // 平台管理员/警方
}

// loadGroupCtx 载入群组并按当前用户计算权限；不存在/封禁对普通用户按 404 处理
func loadGroupCtx(c *gin.Context, groupID uint) (*groupViewCtx, bool) {
	g, err := groups.GetGroup(groupID)
	if err != nil {
		utils.NotFound(c, "群组不存在")
		return nil, false
	}
	uid := middleware.UserID(c)
	platformRole := middleware.UserRole(c)
	ctx := &groupViewCtx{Group: g, IsAdmin: groups.IsGroupPlatformAdmin(platformRole)}
	if m, _ := groups.GetMembership(g.ID, uid); m != nil {
		ctx.Role = m.Role
		ctx.Status = m.Status
	}
	ctx.CanRead = groups.CanReadGroupContent(g, ctx.Role, ctx.Status)
	if g.Status == groups.GroupStatusBanned && !ctx.IsAdmin {
		utils.NotFound(c, "群组不存在")
		return nil, false
	}
	// 半公开/私密：非成员不暴露邀请码与内部数据
	if ctx.Role == groups.GroupRoleOwner || ctx.Role == groups.GroupRoleAdmin {
		g.InviteCode2 = g.InviteCode
	}
	g.InviteCode = ""
	return ctx, true
}

// requireGroupLogin 群组写操作需要登录
func requireGroupLogin(c *gin.Context) (uint, bool) {
	uid := middleware.UserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "请先登录")
		return 0, false
	}
	return uid, true
}

// GroupList GET /api/groups?scope=discover|my&category=&q=&page=
func GroupList(c *gin.Context) {
	if err := groups.CheckGroupEnabled(); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	scope := strings.TrimSpace(c.DefaultQuery("scope", "discover"))
	if scope == "my" {
		uid, ok := requireGroupLogin(c)
		if !ok {
			return
		}
		list, err := groups.ListMyGroups(uid)
		if err != nil {
			utils.InternalError(c, "查询失败")
			return
		}
		utils.Success(c, gin.H{"list": list, "total": len(list), "categories": groups.GroupCategoryList()})
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
	list, total, err := groups.ListDiscoverGroups(middleware.UserID(c), c.Query("category"), c.Query("q"), page, size)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{
		"list": list, "total": total, "page": page, "size": size,
		"categories": groups.GroupCategoryList(),
	})
}

// GroupCreate POST /api/groups
func GroupCreate(c *gin.Context) {
	if err := groups.CheckGroupEnabled(); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	uid, ok := requireGroupLogin(c)
	if !ok {
		return
	}
	user, err := models.GetUserByID(uid)
	if err != nil {
		utils.Unauthorized(c, "用户不存在")
		return
	}
	var req groupInputReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	g, cerr := groups.CreateGroup(uid, user, req.toInput())
	if cerr != nil {
		utils.BadRequest(c, cerr.Error())
		return
	}
	utils.SuccessMessage(c, "群组已创建", gin.H{"group": g})
}

// groupInputReq 建群/改设置请求体
type groupInputReq struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Visibility  string `json:"visibility"`
	JoinMode    string `json:"join_mode"`
	CoverURL    string `json:"cover_url"`
	MemberLimit int    `json:"member_limit"`
	PostLimit   int    `json:"post_limit_per_day"`
	ReviewFirst bool   `json:"review_first"`
	// 群管理（AllowMemberInvite 用指针：未传时默认允许成员邀请）
	AllowMemberInvite *bool `json:"allow_member_invite"`
	AiEnabled         bool  `json:"ai_enabled"`
	AiAgentID         uint  `json:"ai_agent_id"`
}

func (r groupInputReq) toInput() groups.GroupInput {
	return groups.GroupInput{
		Name: r.Name, Description: r.Description, Category: r.Category,
		Visibility: r.Visibility, JoinMode: r.JoinMode, CoverURL: r.CoverURL,
		MemberLimit: r.MemberLimit, PostLimit: r.PostLimit, ReviewFirst: r.ReviewFirst,
		AllowMemberInvite: r.AllowMemberInvite, AiEnabled: r.AiEnabled, AiAgentID: r.AiAgentID,
	}
}

// GroupDetail GET /api/groups/:id —— 详情（非成员只看简介；半公开/私密不返回帖子正文）
func GroupDetail(c *gin.Context) {
	if err := groups.CheckGroupEnabled(); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	gid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	ctx, ok := loadGroupCtx(c, uint(gid))
	if !ok {
		return
	}
	g := ctx.Group
	if owner, err := models.GetUserByID(g.OwnerID); err == nil {
		g.OwnerName = owner.Nickname
		if strings.TrimSpace(g.OwnerName) == "" {
			g.OwnerName = owner.Username
		}
	}
	// 非成员看不到公告以外的内部信息
	out := gin.H{
		"group":     g,
		"can_read":  ctx.CanRead,
		"my_role":   ctx.Role,
		"my_status": ctx.Status,
		"is_admin":  ctx.IsAdmin,
		"unread":    groups.UnreadCount(g.ID, middleware.UserID(c)),
	}
	if ctx.Role != "" && ctx.Status == groups.GroupMemberActive {
		// 群管待审数提示
		if ctx.Role == groups.GroupRoleOwner || ctx.Role == groups.GroupRoleAdmin {
			var pending int64
			models.DB.Model(&groups.GroupMember{}).
				Where("group_id = ? AND status = ?", g.ID, groups.GroupMemberPending).Count(&pending)
			out["pending_requests"] = pending
		}
		out["allowed"] = gin.H{
			"invite": ctx.Role == groups.GroupRoleOwner || ctx.Role == groups.GroupRoleAdmin || g.AllowMemberInvite,
			"manage": ctx.Role == groups.GroupRoleOwner || ctx.Role == groups.GroupRoleAdmin || ctx.IsAdmin,
			"owner":  ctx.Role == groups.GroupRoleOwner,
		}
	}
	utils.Success(c, out)
}

// GroupUpdate PUT /api/groups/:id —— 群主/管理员改设置
func GroupUpdate(c *gin.Context) {
	gid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	uid, ok := requireGroupLogin(c)
	if !ok {
		return
	}
	var req groupInputReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := groups.UpdateGroupSettings(uint(gid), uid, middleware.UserRole(c), req.toInput()); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "已保存", nil)
}

// GroupDissolve DELETE /api/groups/:id —— 群主解散（软删除）
func GroupDissolve(c *gin.Context) {
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
	if g.OwnerID != uid && !groups.IsGroupPlatformAdmin(middleware.UserRole(c)) {
		utils.Forbidden(c, "仅群主可解散群组")
		return
	}
	if err := models.DB.Delete(&groups.Group{}, g.ID).Error; err != nil {
		utils.InternalError(c, "解散失败")
		return
	}
	utils.SuccessMessage(c, "群组已解散", nil)
}

// GroupJoin POST /api/groups/:id/join { code?: 邀请码或群号 }
func GroupJoin(c *gin.Context) {
	if err := groups.CheckGroupEnabled(); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	uid, ok := requireGroupLogin(c)
	if !ok {
		return
	}
	user, err := models.GetUserByID(uid)
	if err != nil {
		utils.Unauthorized(c, "用户不存在")
		return
	}
	gid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	g, gerr := groups.GetGroup(uint(gid))
	if gerr != nil {
		utils.NotFound(c, "群组不存在")
		return
	}
	var req struct {
		Code string `json:"code"` // 邀请码 / 申请理由
	}
	_ = c.ShouldBindJSON(&req)
	status, jerr := groups.JoinGroup(g, user, req.Code)
	if jerr != nil {
		utils.BadRequest(c, jerr.Error())
		return
	}
	msg := "已加入群组"
	if status == groups.GroupMemberPending {
		msg = "申请已提交，等待群主审核"
	} else {
		groups.RefreshGroupCounts(g.ID)
	}
	utils.SuccessMessage(c, msg, gin.H{"status": status})
}

// GroupJoinByNo POST /api/groups/join { group_no?, code? } —— 按群号 / 邀请码加入
// 路由刻意不用 /groups/by-no，避免与 /groups/:id 的通配段冲突（gin httprouter 限制）。
func GroupJoinByNo(c *gin.Context) {
	if err := groups.CheckGroupEnabled(); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	uid, ok := requireGroupLogin(c)
	if !ok {
		return
	}
	user, err := models.GetUserByID(uid)
	if err != nil {
		utils.Unauthorized(c, "用户不存在")
		return
	}
	var req struct {
		GroupNo string `json:"group_no"`
		Code    string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	key := strings.TrimSpace(req.GroupNo)
	if key == "" {
		utils.BadRequest(c, "请输入群号或邀请码")
		return
	}
	// 群号优先；纯数字当群号，否则当邀请码
	var g *groups.Group
	if isAllDigits(key) {
		if g, err = groups.GetGroupByNo(key); err != nil {
			// 群号不存在，再尝试把 key 当邀请码
			g = nil
		}
	}
	if g == nil {
		var found groups.Group
		if err := models.DB.Where("invite_code = ? AND invite_code <> ''", strings.ToUpper(key)).First(&found).Error; err == nil {
			g = &found
		}
	}
	if g == nil {
		utils.NotFound(c, "未找到该群组（群号或邀请码有误）")
		return
	}
	status, jerr := groups.JoinGroup(g, user, req.Code)
	if jerr != nil {
		utils.BadRequest(c, jerr.Error())
		return
	}
	msg := "已加入群组"
	if status == groups.GroupMemberPending {
		msg = "申请已提交，等待群主审核"
	}
	utils.SuccessMessage(c, msg, gin.H{"status": status, "group": g})
}

// GroupSearchByNo GET /api/groups/search?no=1234567 —— 扫码/输入群号后先预览
func GroupSearchByNo(c *gin.Context) {
	no := strings.TrimSpace(c.Query("no"))
	if no == "" {
		utils.BadRequest(c, "请输入群号")
		return
	}
	g, err := groups.GetGroupByNo(no)
	if err != nil {
		// 兼容邀请码
		var found groups.Group
		if err2 := models.DB.Where("invite_code = ? AND invite_code <> ''", strings.ToUpper(no)).First(&found).Error; err2 != nil {
			utils.NotFound(c, "未找到该群组")
			return
		}
		g = &found
	}
	g.InviteCode = ""
	ctx, _ := groups.GetMembership(g.ID, middleware.UserID(c))
	out := gin.H{"group": g, "joined": ctx != nil && ctx.Status == groups.GroupMemberActive}
	if ctx != nil {
		out["my_status"] = ctx.Status
		out["my_role"] = ctx.Role
	}
	utils.Success(c, out)
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
