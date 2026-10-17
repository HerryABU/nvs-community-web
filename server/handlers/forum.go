package handlers

import (
	"nvs-server/models"
	"nvs-server/security"
	"nvs-server/utils"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// GET /api/forums — 论坛列表
func ListForums(c *gin.Context) {
	userID := currentUserID(c) // 公开路由未登录时为 0
	ftype := c.DefaultQuery("type", "general")
	parentIDStr := c.Query("parent_id")

	// 如果传了 parent_id，返回该父论坛的子论坛
	if parentIDStr != "" {
		parentID, err := strconv.ParseUint(parentIDStr, 10, 64)
		if err != nil {
			utils.BadRequest(c, "无效的 parent_id")
			return
		}
		forums, err := models.GetSubForums(uint(parentID))
		if err != nil {
			utils.InternalError(c, "获取子论坛列表失败")
			return
		}
		utils.Success(c, forums)
		return
	}

	// 仅查询自建论坛
	if ftype == "custom" {
		forums := listCustomForums(userID)
		utils.Success(c, forums)
		return
	}

	var forums []models.Forum
	var err error
	if ftype == "all" {
		all, e := models.ListAllForums("")
		if e != nil {
			utils.InternalError(c, "获取论坛列表失败")
			return
		}
		// 系统论坛（非 custom）+ 自建论坛（公开非驳回 + 本人创建的）
		for _, f := range all {
			if f.Type != "custom" {
				forums = append(forums, f)
			}
		}
		forums = append(forums, listCustomForums(userID)...)
	} else if strings.Contains(ftype, ",") {
		types := strings.Split(ftype, ",")
		forums, err = models.GetForumsByTypes(types)
	} else {
		forums, err = models.GetForumsByType(ftype)
	}
	if err != nil {
		utils.InternalError(c, "获取论坛列表失败")
		return
	}
	utils.Success(c, forums)
}

// listCustomForums 返回大厅自建区可见的论坛：公开且未驳回 + 本人创建的（含隐藏）
func listCustomForums(userID uint) []models.Forum {
	var forums []models.Forum
	q := models.DB.Where("type = ?", "custom").
		Where("(is_public = ? AND review_status != ?) OR creator_id = ?", true, "rejected", userID).
		Order("id DESC")
	q.Find(&forums)
	return forums
}

// GET /api/forums/:id — 论坛详情+帖子列表+子论坛
func GetForum(c *gin.Context) {
	userID := currentUserID(c)
	forumID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}

	forum, err := models.GetForumByID(uint(forumID))
	if err != nil {
		utils.NotFound(c, "论坛不存在")
		return
	}

	// 房间号门禁：非公开（不可见）自建论坛，非创建者/管理员/警方须凭房间号进入。
	// 有密码的论坛还需同时提供正确密码（room + password）。
	// 🔒 警方取证豁免：警方（police）与管理员可越过门禁直接查看封锁/加密/隐藏论坛。
	if !forum.IsPublic && forum.CreatorID != userID && !isPoliceOrAdminUser(c) {
		room := c.Query("room")
		pass := c.Query("password")
		// 兼容请求头传递（🔒 安全审计：密码不进 URL——前端已改为 header + sessionStorage）
		if room == "" {
			room = c.GetHeader("X-Forum-Room")
		}
		if pass == "" {
			pass = c.GetHeader("X-Forum-Password")
		}
		if !strings.EqualFold(strings.TrimSpace(room), forum.RoomCode) ||
			(forum.HasPassword && !utils.CheckPassword(pass, forum.PasswordHash)) {
			// 🔒 安全审计 L5：未解锁时仅返回最小信息，不泄露论坛名称/描述等元数据
			utils.Success(c, gin.H{
				"forum": gin.H{
					"id":        forum.ID,
					"type":      forum.Type,
					"is_public": forum.IsPublic,
					"locked":    true,
				},
				"locked":     true,
				"threads":    []models.Thread{},
				"total":      0,
				"page":       page,
				"sub_forums": []models.Forum{},
			})
			return
		}
	}

	// 密码门禁：有密码且非创建者/管理员/警方时，需提供正确密码才能读取帖子
	if forum.HasPassword && forum.CreatorID != userID && !isPoliceOrAdminUser(c) {
		password := c.Query("password")
		if password == "" {
			password = c.GetHeader("X-Forum-Password")
		}
		if !utils.CheckPassword(password, forum.PasswordHash) {
			utils.Success(c, gin.H{
				"forum": gin.H{
					"id":        forum.ID,
					"type":      forum.Type,
					"is_public": forum.IsPublic,
					"locked":    true,
				},
				"locked":     true,
				"threads":    []models.Thread{},
				"total":      0,
				"page":       page,
				"sub_forums": []models.Forum{},
			})
			return
		}
	}

	threads, total, err := models.GetThreadsByForum(uint(forumID), page, 20)
	if err != nil {
		utils.InternalError(c, "获取帖子列表失败")
		return
	}

	// 黑名单 hide 模式（F13）：不看被自己屏蔽（hide）的作者发的帖子
	threads = FilterHiddenItems(currentUserID(c), threads, func(t models.Thread) uint { return t.UserID })

	// 获取子论坛列表
	subForums, _ := models.GetSubForums(uint(forumID))

	utils.Success(c, gin.H{
		"forum":      forum,
		"threads":    threads,
		"total":      total,
		"page":       page,
		"sub_forums": subForums,
	})
}

// POST /api/forums/:id/threads — 发帖
func CreateThread(c *gin.Context) {
	userID := currentUserID(c)
	forumID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || forumID == 0 {
		utils.BadRequest(c, "无效的论坛ID")
		return
	}

	// 禁言检查（全局 + 论坛级）
	if checkUserMuted(c) {
		return
	}
	if models.IsForumMuted(uint(forumID), userID) {
		utils.Forbidden(c, "您已被本论坛禁言，无法发帖")
		return
	}

	// 验证论坛是否存在
	forum, err := models.GetForumByID(uint(forumID))
	if err != nil {
		utils.NotFound(c, "论坛不存在")
		return
	}
	_ = forum

	var req struct {
		Title   string `json:"title" binding:"required,max=256"`
		Content string `json:"content" binding:"required,max=65536"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请填写标题和内容")
		return
	}

	raw := security.SanitizeUserContent(req.Content)
	cleaned, badHits := processBadWords(raw, "forum")

	thread := &models.Thread{
		ForumID: uint(forumID),
		UserID:  userID,
		Title:   strings.TrimSpace(req.Title),
		Content: cleaned,
	}

	if err := models.CreateThread(thread); err != nil {
		utils.InternalError(c, "发帖失败")
		return
	}

	// 违禁词 mark 动作命中 → 写入待审标记记录
	flagBadWords(badHits, "thread", thread.ID, userID, raw, thread.Title)

	utils.Success(c, thread)
}

// GET /api/threads/:id — 帖子详情+回复
func GetThread(c *gin.Context) {
	threadID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}

	thread, err := models.GetThreadByID(uint(threadID))
	if err != nil {
		utils.NotFound(c, "帖子不存在")
		return
	}

	models.IncrementThreadView(uint(threadID))

	pg, err := models.GetPostsByThread(uint(threadID), page, 20)
	if err != nil {
		utils.InternalError(c, "获取回复失败")
		return
	}

	// 黑名单 hide 模式（F13）：不看被自己屏蔽（hide）的作者发的回帖
	pg.List = FilterHiddenItems(currentUserID(c), pg.List, func(p models.Post) uint { return p.UserID })

	// 楼层模型：posts = 本页楼层 + 这些楼层下的全部回复（平铺，前端按 root_id 分组成「楼」）
	utils.Success(c, gin.H{
		"thread":      thread,
		"posts":       pg.List,
		"total":       pg.Total,
		"floor_total": pg.FloorTotal,
		"has_more":    pg.HasMore,
		"page":        page,
	})
}

// POST /api/threads/:id/posts — 回复帖子
func CreatePost(c *gin.Context) {
	userID := currentUserID(c)
	threadID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || threadID == 0 {
		utils.BadRequest(c, "无效的帖子ID")
		return
	}

	// 验证帖子是否存在且未被锁定
	thread, err := models.GetThreadByID(uint(threadID))
	if err != nil {
		utils.NotFound(c, "帖子不存在")
		return
	}
	if thread.IsLocked {
		utils.Forbidden(c, "帖子已被锁定，无法回复")
		return
	}

	// 禁言检查（全局 + 论坛级）
	if checkUserMuted(c) {
		return
	}
	if models.IsForumMuted(thread.ForumID, userID) {
		utils.Forbidden(c, "您已被本论坛禁言，无法回复")
		return
	}

	var req struct {
		Content  string `json:"content" binding:"required,max=65536"`
		ParentID *uint  `json:"parent_id"` // 楼层模型：空=回复楼主（开新楼）；有值=回复该条（挂靠其所属楼）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请输入回复内容")
		return
	}
	// 父回帖必须属于同一帖子，防止跨帖挂靠
	if req.ParentID != nil && *req.ParentID != 0 {
		var parent models.Post
		if err := models.DB.Select("id", "thread_id").First(&parent, *req.ParentID).Error; err != nil ||
			parent.ThreadID != uint(threadID) {
			utils.BadRequest(c, "被回复的内容不存在")
			return
		}
	}

	raw := security.SanitizeUserContent(req.Content)
	cleaned, badHits := processBadWords(raw, "forum")

	post := &models.Post{
		ThreadID: uint(threadID),
		UserID:   userID,
		Content:  cleaned,
		ParentID: req.ParentID,
	}

	if err := models.CreatePost(post); err != nil {
		utils.InternalError(c, "回复失败")
		return
	}

	// 违禁词 mark 动作命中 → 写入待审标记记录
	flagBadWords(badHits, "post", post.ID, userID, raw, "")

	if user, err := models.GetUserByID(userID); err == nil {
		post.User = user
	}

	utils.Success(c, post)
}

// GET /api/novels/:id/forum — 获取作品子论坛
func GetNovelForum(c *gin.Context) {
	novelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	novel, err := models.GetNovelByID(uint(novelID))
	if err != nil {
		utils.NotFound(c, "作品不存在")
		return
	}

	forum, err := models.GetOrCreateForum(novel.Title+" 讨论区", "sub", strconv.FormatUint(novelID, 10), "作品《"+novel.Title+"》的专属讨论区")
	if err != nil {
		utils.InternalError(c, "获取论坛失败")
		return
	}

	threads, total, _ := models.GetThreadsByForum(forum.ID, 1, 20)

	// 黑名单 hide 模式（F13）：不看被自己屏蔽（hide）的作者发的帖子
	threads = FilterHiddenItems(currentUserID(c), threads, func(t models.Thread) uint { return t.UserID })

	utils.Success(c, gin.H{
		"forum":   forum,
		"threads": threads,
		"total":   total,
	})
}

// POST /api/threads/:id/pin — 置顶帖子
func PinThread(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	thread, err := models.GetThreadByID(uint(id))
	if err != nil {
		utils.NotFound(c, "帖子不存在")
		return
	}
	thread.IsPinned = true
	models.DB.Save(thread)
	utils.Success(c, gin.H{"message": "置顶成功"})
}

// POST /api/threads/:id/unpin — 取消置顶
func UnpinThread(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	thread, err := models.GetThreadByID(uint(id))
	if err != nil {
		utils.NotFound(c, "帖子不存在")
		return
	}
	thread.IsPinned = false
	models.DB.Save(thread)
	utils.Success(c, gin.H{"message": "已取消置顶"})
}
