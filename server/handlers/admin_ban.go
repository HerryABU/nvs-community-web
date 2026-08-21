package handlers

import (
	"strconv"
	"time"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ─────────────────────────────────────────────────────────────
// 管理员内容封禁管理（小说 / 章节）
// 提供：管理员视角的小说/章节列表（含软删除/封禁状态）、封禁（复用 softdel）、解封。
// ─────────────────────────────────────────────────────────────

// AdminListNovels 管理员查小说列表（含已封禁/软删除），支持标题/作者搜索。
// GET /api/admin/novels?search=&page=&page_size=
func AdminListNovels(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	search := c.Query("search")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}

	query := models.DB.Unscoped().Model(&models.Novel{})
	if search != "" {
		like := "%" + search + "%"
		query = query.Joins("LEFT JOIN users ON users.id = novels.author_id").
			Where("novels.title LIKE ? OR users.nickname LIKE ? OR users.username LIKE ?", like, like, like)
	}

	var total int64
	query.Count(&total)

	var novels []models.Novel
	query.Preload("Author").
		Order("novels.deleted_at ASC, novels.updated_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&novels)

	type item struct {
		ID            uint       `json:"id"`
		Title         string     `json:"title"`
		AuthorID      uint       `json:"author_id"`
		AuthorName    string     `json:"author_name"`
		TotalWords    int        `json:"total_words"`
		TotalChapters int        `json:"total_chapters"`
		Status        string     `json:"status"`
		BanReason     string     `json:"ban_reason"`
		Banned        bool       `json:"banned"`
		BannedAt      *time.Time `json:"banned_at"`
		CreatedAt     time.Time  `json:"created_at"`
	}
	list := make([]item, 0, len(novels))
	for _, n := range novels {
		authorName := ""
		if n.Author != nil {
			authorName = n.Author.Nickname
			if authorName == "" {
				authorName = n.Author.Username
			}
		}
		it := item{
			ID:            n.ID,
			Title:         n.Title,
			AuthorID:      n.AuthorID,
			AuthorName:    authorName,
			TotalWords:    n.TotalWords,
			TotalChapters: n.TotalChapters,
			Status:        n.Status,
			BanReason:     n.BanReason,
			Banned:        n.DeletedAt.Valid,
			CreatedAt:     n.CreatedAt,
		}
		if n.DeletedAt.Valid {
			t := n.DeletedAt.Time
			it.BannedAt = &t
		}
		list = append(list, it)
	}

	utils.Success(c, gin.H{"list": list, "total": total, "page": page, "page_size": pageSize})
}

// AdminListChapters 管理员查某小说的章节列表（含已封禁/软删除）。
// GET /api/admin/novels/:id/chapters
func AdminListChapters(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	novelID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "无效的小说ID")
		return
	}

	var chapters []models.Chapter
	models.DB.Unscoped().
		Where("novel_id = ?", novelID).
		Order("chapter_number ASC").
		Find(&chapters)

	type item struct {
		ID            uint       `json:"id"`
		NovelID       uint       `json:"novel_id"`
		ChapterNumber int        `json:"chapter_number"`
		Title         string     `json:"title"`
		WordCount     int        `json:"word_count"`
		Status        string     `json:"status"`
		BanReason     string     `json:"ban_reason"`
		Banned        bool       `json:"banned"`
		BannedAt      *time.Time `json:"banned_at"`
		CreatedAt     time.Time  `json:"created_at"`
	}
	list := make([]item, 0, len(chapters))
	for _, ch := range chapters {
		it := item{
			ID:            ch.ID,
			NovelID:       ch.NovelID,
			ChapterNumber: ch.ChapterNumber,
			Title:         ch.Title,
			WordCount:     ch.WordCount,
			Status:        ch.Status,
			BanReason:     ch.BanReason,
			Banned:        ch.DeletedAt.Valid,
			CreatedAt:     ch.CreatedAt,
		}
		if ch.DeletedAt.Valid {
			t := ch.DeletedAt.Time
			it.BannedAt = &t
		}
		list = append(list, it)
	}

	utils.Success(c, list)
}

// AdminRestoreContent 解封/恢复软删除内容（清除 deleted_at 与 ban_reason）。
// POST /api/admin/softdel/restore  body: { type, id }
func AdminRestoreContent(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		Type string `json:"type" binding:"required"`
		ID   uint   `json:"id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	clearDeleted := func(table interface{}, id uint) error {
		return models.DB.Unscoped().Model(table).Where("id = ?", id).
			Updates(map[string]interface{}{"deleted_at": nil, "ban_reason": ""}).Error
	}

	switch req.Type {
	case "novel":
		// 恢复小说及其全部章节
		if err := clearDeleted(&models.Novel{}, req.ID); err != nil {
			utils.InternalError(c, "解封失败")
			return
		}
		_ = models.DB.Unscoped().Model(&models.Chapter{}).Where("novel_id = ?", req.ID).
			Updates(map[string]interface{}{"deleted_at": nil, "ban_reason": ""}).Error
	case "chapter":
		if err := clearDeleted(&models.Chapter{}, req.ID); err != nil {
			utils.InternalError(c, "解封失败")
			return
		}
	case "comment":
		_ = models.DB.Unscoped().Model(&models.Comment{}).Where("id = ?", req.ID).
			Update("deleted_at", nil).Error
	case "thread":
		_ = models.DB.Unscoped().Model(&models.Thread{}).Where("id = ?", req.ID).
			Update("deleted_at", nil).Error
	case "post":
		_ = models.DB.Unscoped().Model(&models.Post{}).Where("id = ?", req.ID).
			Update("deleted_at", nil).Error
	case "blog":
		_ = models.DB.Unscoped().Model(&models.AuthorBlog{}).Where("id = ?", req.ID).
			Update("deleted_at", nil).Error
	case "forum":
		_ = models.DB.Unscoped().Model(&models.Forum{}).Where("id = ?", req.ID).
			Update("deleted_at", nil).Error
	case "user":
		_ = models.DB.Unscoped().Model(&models.User{}).Where("id = ?", req.ID).
			Update("deleted_at", nil).Error
	default:
		utils.BadRequest(c, "不支持的类型（支持 novel/chapter/comment/thread/post/blog/forum/user）")
		return
	}

	auditEmergency(c, models.AuditContentRestore, req.Type, req.ID,
		"管理员解封/恢复 "+req.Type+" #"+strconv.FormatUint(uint64(req.ID), 10))
	utils.Success(c, gin.H{"message": "已解封"})
}
