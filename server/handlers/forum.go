package handlers

import (
	"strconv"
	"strings"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// GET /api/forums — 论坛列表
func ListForums(c *gin.Context) {
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

	var forums []models.Forum
	var err error
	if ftype == "all" {
		forums, err = models.ListAllForums("")
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

// GET /api/forums/:id — 论坛详情+帖子列表+子论坛
func GetForum(c *gin.Context) {
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

	threads, total, err := models.GetThreadsByForum(uint(forumID), page, 20)
	if err != nil {
		utils.InternalError(c, "获取帖子列表失败")
		return
	}

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
	userID := c.GetUint("userID")
	forumID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	var req struct {
		Title   string `json:"title" binding:"required"`
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请填写标题和内容")
		return
	}

	thread := &models.Thread{
		ForumID: uint(forumID),
		UserID:  userID,
		Title:   req.Title,
		Content: req.Content, // 直接存储原始 Markdown，XSS 由前端 Cherry Markdown 防护
	}

	if err := models.CreateThread(thread); err != nil {
		utils.InternalError(c, "发帖失败")
		return
	}

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

	posts, total, err := models.GetPostsByThread(uint(threadID), page, 30)
	if err != nil {
		utils.InternalError(c, "获取回复失败")
		return
	}

	utils.Success(c, gin.H{
		"thread": thread,
		"posts":  posts,
		"total":  total,
		"page":   page,
	})
}

// POST /api/threads/:id/posts — 回复帖子
func CreatePost(c *gin.Context) {
	userID := c.GetUint("userID")
	threadID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请输入回复内容")
		return
	}

	post := &models.Post{
		ThreadID: uint(threadID),
		UserID:   userID,
		Content:  req.Content, // 直接存储原始 Markdown，XSS 由前端 Cherry Markdown 防护
	}

	if err := models.CreatePost(post); err != nil {
		utils.InternalError(c, "回复失败")
		return
	}

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