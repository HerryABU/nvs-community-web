package forum

import (
	"nvs-server/handlers/badword"
	"nvs-server/handlers/blacklist"
	"nvs-server/handlers/hctx"
	"nvs-server/handlers/publish"
	"nvs-server/models"
	"nvs-server/security"
	"nvs-server/utils/infra"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	quota "nvs-server/models/quota"
)

// GET /api/forums — 论坛列表
func ListForums(c *gin.Context) {
	userID := hctx.CurrentUserID(c) // 公开路由未登录时为 0
	ftype := c.DefaultQuery("type", "general")
	parentIDStr := c.Query("parent_id")

	// 如果传了 parent_id，返回该父论坛的子论坛
	if parentIDStr != "" {
		parentID, err := strconv.ParseUint(parentIDStr, 10, 64)
		if err != nil {
			infra.BadRequest(c, "无效的 parent_id")
			return
		}
		forums, err := models.GetSubForums(uint(parentID))
		if err != nil {
			infra.InternalError(c, "获取子论坛列表失败")
			return
		}
		infra.Success(c, forums)
		return
	}

	// 仅查询自建论坛
	if ftype == "custom" {
		forums := listCustomForums(userID)
		infra.Success(c, forums)
		return
	}

	var forums []models.Forum
	var err error
	if ftype == "all" {
		all, e := models.ListAllForums("")
		if e != nil {
			infra.InternalError(c, "获取论坛列表失败")
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
		infra.InternalError(c, "获取论坛列表失败")
		return
	}
	infra.Success(c, forums)
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
	userID := hctx.CurrentUserID(c)
	forumID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}

	forum, err := models.GetForumByID(uint(forumID))
	if err != nil {
		infra.NotFound(c, "论坛不存在")
		return
	}

	// 统一门禁：创建者/警方/管理员豁免；敏感区需 24h 内确认落库；
	// 非公开需正确房间号；有密码需正确密码。未解锁仅返回最小信息。
	if ok, payload := checkForumAccess(c, forum, userID); !ok {
		if payload == nil {
			infra.NotFound(c, "论坛不存在")
			return
		}
		payload["page"] = page
		infra.Success(c, payload)
		return
	}

	threads, total, err := models.GetThreadsByForum(uint(forumID), page, 20)
	if err != nil {
		infra.InternalError(c, "获取帖子列表失败")
		return
	}

	// 黑名单 hide 模式（F13）：不看被自己屏蔽（hide）的作者发的帖子
	threads = blacklist.FilterHiddenItems(hctx.CurrentUserID(c), threads, func(t models.Thread) uint { return t.UserID })

	// 获取子论坛列表
	subForums, _ := models.GetSubForums(uint(forumID))

	infra.Success(c, gin.H{
		"forum":      forum,
		"threads":    threads,
		"total":      total,
		"page":       page,
		"sub_forums": subForums,
	})
}

// POST /api/forums/:id/threads — 发帖
func CreateThread(c *gin.Context) {
	userID := hctx.CurrentUserID(c)
	forumID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || forumID == 0 {
		infra.BadRequest(c, "无效的论坛ID")
		return
	}

	// 禁言检查（全局 + 论坛级）
	if CheckUserMuted(c) {
		return
	}
	if models.IsForumMuted(uint(forumID), userID) {
		infra.Forbidden(c, "您已被本论坛禁言，无法发帖")
		return
	}

	// 验证论坛是否存在 + 统一门禁（房间号 / 密码 / 敏感区确认），
	// 防止绕过论坛锁定直接发帖。
	if ok, _, payload := checkForumAccessByID(c, uint(forumID), userID); !ok {
		if payload == nil {
			infra.NotFound(c, "论坛不存在")
			return
		}
		infra.Forbidden(c, "该论坛已锁定，需先验证访问权限")
		return
	}

	var req struct {
		Title   string `json:"title" binding:"required,max=256"`
		Content string `json:"content" binding:"required,max=65536"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "请填写标题和内容")
		return
	}

	raw := security.SanitizeUserContent(req.Content)
	cleaned, badHits := badword.ProcessBadWords(raw, "forum")

	// 每日发文限额预检（§13.4；后台关闭时不生效）
	threadWords := len([]rune(cleaned))
	if !publish.EnforceDailyPublish(c, quota.PublishKindThread, threadWords) {
		return
	}

	thread := &models.Thread{
		ForumID: uint(forumID),
		UserID:  userID,
		Title:   strings.TrimSpace(req.Title),
		Content: cleaned,
	}

	if err := models.CreateThread(thread); err != nil {
		infra.InternalError(c, "发帖失败")
		return
	}
	// 发布流水记账（计入每日发文限额）
	quota.RecordPublish(userID, quota.PublishKindThread, thread.ID, threadWords)

	// 违禁词 mark 动作命中 → 写入待审标记记录
	badword.FlagBadWords(badHits, "thread", thread.ID, userID, raw, thread.Title)

	infra.Success(c, thread)
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
		infra.NotFound(c, "帖子不存在")
		return
	}

	// 🔒 路径穿透防护：帖子所属论坛须通过统一门禁（房间号 / 密码 / 敏感区确认），
	// 否则改 URL 数字即可绕过论坛门禁读取内容。
	if ok, _, payload := checkForumAccessByID(c, thread.ForumID, hctx.CurrentUserID(c)); !ok {
		if payload == nil {
			infra.NotFound(c, "帖子不存在")
			return
		}
		payload["posts"] = []models.Post{}
		payload["total"] = 0
		payload["floor_total"] = 0
		payload["has_more"] = false
		payload["thread"] = gin.H{"id": thread.ID, "forum_id": thread.ForumID, "locked": true}
		payload["page"] = page
		infra.Success(c, payload)
		return
	}
	// 脱敏：不向外泄露所属论坛的房间号 / 密码标记
	redactThreadForum(thread)

	models.IncrementThreadView(uint(threadID))

	pg, err := models.GetPostsByThread(uint(threadID), page, 20)
	if err != nil {
		infra.InternalError(c, "获取回复失败")
		return
	}

	// 黑名单 hide 模式（F13）：不看被自己屏蔽（hide）的作者发的回帖
	pg.List = blacklist.FilterHiddenItems(hctx.CurrentUserID(c), pg.List, func(p models.Post) uint { return p.UserID })

	// 楼层模型：posts = 本页楼层 + 这些楼层下的全部回复（平铺，前端按 root_id 分组成「楼」）
	infra.Success(c, gin.H{
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
	userID := hctx.CurrentUserID(c)
	threadID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || threadID == 0 {
		infra.BadRequest(c, "无效的帖子ID")
		return
	}

	// 验证帖子是否存在且未被锁定
	thread, err := models.GetThreadByID(uint(threadID))
	if err != nil {
		infra.NotFound(c, "帖子不存在")
		return
	}
	if thread.IsLocked {
		infra.Forbidden(c, "帖子已被锁定，无法回复")
		return
	}

	// 🔒 路径穿透防护：所属论坛须通过统一门禁（房间号 / 密码 / 敏感区确认）才能回复
	if ok, _, payload := checkForumAccessByID(c, thread.ForumID, userID); !ok {
		if payload == nil {
			infra.NotFound(c, "帖子不存在")
			return
		}
		infra.Forbidden(c, "该论坛已锁定，需先验证访问权限")
		return
	}

	// 禁言检查（全局 + 论坛级）
	if CheckUserMuted(c) {
		return
	}
	if models.IsForumMuted(thread.ForumID, userID) {
		infra.Forbidden(c, "您已被本论坛禁言，无法回复")
		return
	}

	var req struct {
		Content  string `json:"content" binding:"required,max=65536"`
		ParentID *uint  `json:"parent_id"` // 楼层模型：空=回复楼主（开新楼）；有值=回复该条（挂靠其所属楼）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "请输入回复内容")
		return
	}
	// 父回帖必须属于同一帖子，防止跨帖挂靠
	if req.ParentID != nil && *req.ParentID != 0 {
		var parent models.Post
		if err := models.DB.Select("id", "thread_id").First(&parent, *req.ParentID).Error; err != nil ||
			parent.ThreadID != uint(threadID) {
			infra.BadRequest(c, "被回复的内容不存在")
			return
		}
	}

	raw := security.SanitizeUserContent(req.Content)
	cleaned, badHits := badword.ProcessBadWords(raw, "forum")

	post := &models.Post{
		ThreadID: uint(threadID),
		UserID:   userID,
		Content:  cleaned,
		ParentID: req.ParentID,
	}

	if err := models.CreatePost(post); err != nil {
		infra.InternalError(c, "回复失败")
		return
	}

	// 违禁词 mark 动作命中 → 写入待审标记记录
	badword.FlagBadWords(badHits, "post", post.ID, userID, raw, "")

	if user, err := models.GetUserByID(userID); err == nil {
		post.User = user
	}

	infra.Success(c, post)
}

// GET /api/novels/:id/forum — 获取作品子论坛
func GetNovelForum(c *gin.Context) {
	novelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	novel, err := models.GetNovelByID(uint(novelID))
	if err != nil {
		infra.NotFound(c, "作品不存在")
		return
	}

	forum, err := models.GetOrCreateForum(novel.Title+" 讨论区", "sub", strconv.FormatUint(novelID, 10), "作品《"+novel.Title+"》的专属讨论区")
	if err != nil {
		infra.InternalError(c, "获取论坛失败")
		return
	}

	threads, total, _ := models.GetThreadsByForum(forum.ID, 1, 20)

	// 黑名单 hide 模式（F13）：不看被自己屏蔽（hide）的作者发的帖子
	threads = blacklist.FilterHiddenItems(hctx.CurrentUserID(c), threads, func(t models.Thread) uint { return t.UserID })

	infra.Success(c, gin.H{
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
		infra.NotFound(c, "帖子不存在")
		return
	}
	thread.IsPinned = true
	models.DB.Save(thread)
	infra.Success(c, gin.H{"message": "置顶成功"})
}

// POST /api/threads/:id/unpin — 取消置顶
func UnpinThread(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	thread, err := models.GetThreadByID(uint(id))
	if err != nil {
		infra.NotFound(c, "帖子不存在")
		return
	}
	thread.IsPinned = false
	models.DB.Save(thread)
	infra.Success(c, gin.H{"message": "已取消置顶"})
}
