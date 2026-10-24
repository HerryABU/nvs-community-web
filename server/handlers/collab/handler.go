package collabhandler

import (
	"strconv"
	"strings"

	"nvs-server/collab"
	"nvs-server/middleware"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== 多人协作：接口层 ====================
// 权限规则见 docs/novel-collaboration.md；本包只做入参校验与响应组装，
// 能力判定统一走 collab.Actor / collab.Invite 等（避免前后端两套规则）。

func isPlatform(c *gin.Context) bool {
	r := middleware.UserRole(c)
	return r == models.RoleAdmin || r == models.RolePolice
}

// List GET /api/novels/:id/collaborators —— 协作成员 + 我的权限
func List(c *gin.Context) {
	uid := middleware.UserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	novelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	novel, err := models.GetNovelByID(uint(novelID))
	if err != nil {
		utils.NotFound(c, "作品不存在")
		return
	}
	actor := collab.ActorOf(uint(novelID), uid, middleware.UserRole(c))
	if !actor.IsOwner && !actor.ActiveMember() && !actor.IsPlatform {
		utils.Forbidden(c, "仅主笔或协作成员可查看")
		return
	}
	list, err := collab.List(uint(novelID), actor.IsOwner || actor.IsPlatform)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	// 主笔展示名（前端在列表里把主笔固定在第一位）
	ownerName := ""
	if u, uerr := models.GetUserByID(novel.AuthorID); uerr == nil && u != nil {
		ownerName = u.Nickname
		if strings.TrimSpace(ownerName) == "" {
			ownerName = u.Username
		}
	}
	utils.Success(c, gin.H{
		"list":       list,
		"owner_id":   novel.AuthorID,
		"owner_name": ownerName,
		"byline":     collab.Byline(uint(novelID)),
		"me": gin.H{
			"is_owner":         actor.IsOwner,
			"role":             actor.Role,
			"status":           actor.Status,
			"can_publish":      actor.Can(collab.CapPublishChapter),
			"can_review":       actor.Can(collab.CapReviewChapter),
			"can_edit_others":  actor.Can(collab.CapEditOthers),
			"can_write":        actor.Can(collab.CapWriteChapter),
		},
	})
}

// Invite POST /api/novels/:id/collaborators { username, role }
func Invite(c *gin.Context) {
	uid := middleware.UserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	novelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Username string `json:"username"`
		Role     string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	m, err := collab.Invite(uint(novelID), uid, req.Username, req.Role)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "邀请已发送，等待对方接受", m)
}

// Update PUT /api/novels/:id/collaborators/:uid —— 角色/能力/分成/署名顺序
func Update(c *gin.Context) {
	uid := middleware.UserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	novelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	target, _ := strconv.ParseUint(c.Param("uid"), 10, 64)
	var in collab.UpdateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := collab.Update(uint(novelID), uid, uint(target), in); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "已保存", nil)
}

// Remove DELETE /api/novels/:id/collaborators/:uid
func Remove(c *gin.Context) {
	uid := middleware.UserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	novelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	target, _ := strconv.ParseUint(c.Param("uid"), 10, 64)
	if err := collab.Remove(uint(novelID), uid, uint(target)); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "已移除（历史署名保留）", nil)
}

// Respond POST /api/collab/:novelId/respond { accept }
func Respond(c *gin.Context) {
	uid := middleware.UserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	novelID, _ := strconv.ParseUint(c.Param("novelId"), 10, 64)
	var req struct {
		Accept bool `json:"accept"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := collab.Respond(uint(novelID), uid, req.Accept); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if req.Accept {
		utils.SuccessMessage(c, "已接受邀请", nil)
		return
	}
	utils.SuccessMessage(c, "已拒绝邀请", nil)
}

// Leave POST /api/collab/:novelId/leave —— 协作者主动退出
func Leave(c *gin.Context) {
	uid := middleware.UserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	novelID, _ := strconv.ParseUint(c.Param("novelId"), 10, 64)
	if err := collab.Leave(uint(novelID), uid); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "已退出协作（历史署名保留）", nil)
}

// Invites GET /api/collab/invites —— 我收到的待接受邀请
func Invites(c *gin.Context) {
	uid := middleware.UserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	list, err := collab.ListPendingByUser(uid)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	// 补作品标题
	type inviteVO struct {
		collab.Collaborator
		NovelTitle string `json:"novel_title"`
	}
	out := make([]inviteVO, 0, len(list))
	for _, it := range list {
		vo := inviteVO{Collaborator: it}
		if n, nerr := models.GetNovelByID(it.NovelID); nerr == nil && n != nil {
			vo.NovelTitle = n.Title
		}
		out = append(out, vo)
	}
	utils.Success(c, gin.H{"list": out})
}

// Works GET /api/collab/works —— 我参与协作的作品
func Works(c *gin.Context) {
	uid := middleware.UserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	list, err := collab.ListActiveByUser(uid)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	type workVO struct {
		NovelID    uint   `json:"novel_id"`
		Title      string `json:"title"`
		CoverURL   string `json:"cover_url"`
		Role       string `json:"role"`
		RoleLabel  string `json:"role_label"`
		ShareBP    int    `json:"revenue_share_bp"`
		BylineOrd  int    `json:"byline_order"`
		OwnerName  string `json:"owner_name"`
		ChapterCnt int64  `json:"chapter_count"`
	}
	out := make([]workVO, 0, len(list))
	for _, it := range list {
		n, nerr := models.GetNovelByID(it.NovelID)
		if nerr != nil || n == nil {
			continue
		}
		ownerName := ""
		if u, uerr := models.GetUserByID(n.AuthorID); uerr == nil && u != nil {
			ownerName = u.Nickname
			if strings.TrimSpace(ownerName) == "" {
				ownerName = u.Username
			}
		}
		var cnt int64
		models.DB.Model(&models.Chapter{}).
			Where("novel_id = ? AND author_id = ?", it.NovelID, uid).Count(&cnt)
		out = append(out, workVO{
			NovelID: it.NovelID, Title: n.Title, CoverURL: n.CoverURL,
			Role: it.Role, RoleLabel: collab.RoleLabel(it.Role),
			ShareBP: it.RevenueShareBP, BylineOrd: it.BylineOrder,
			OwnerName: ownerName, ChapterCnt: cnt,
		})
	}
	utils.Success(c, gin.H{"list": out})
}
