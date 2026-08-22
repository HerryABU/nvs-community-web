package handlers

import (
	"os"
	"path/filepath"
	"strconv"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ─────────────────────────────────────────────────────────────
// 回收站：用户自删内容（deleted_by='user'）
//
// 软删除区分两种来源：
//   - deleted_by='user'  → 用户自己删除，进回收站，作者可自行恢复
//   - deleted_by='admin' → 管理员封禁/删除（176号令），仅管理员可恢复（admin/softdel/restore）
//
// 用户恢复仅限本人、仅限 deleted_by='user' 的记录；被封禁内容恢复权限在管理员。
// ─────────────────────────────────────────────────────────────

// GET /api/author/recycle — 当前作者的回收站（自己删除的小说/博客）
func ListAuthorRecycle(c *gin.Context) {
	userID := currentUserID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var novels []models.Novel
	var novelTotal int64
	models.DB.Unscoped().Model(&models.Novel{}).
		Where("author_id = ? AND deleted_at IS NOT NULL AND deleted_by = ?", userID, "user").
		Count(&novelTotal)
	models.DB.Unscoped().Preload("Author").
		Where("author_id = ? AND deleted_at IS NOT NULL AND deleted_by = ?", userID, "user").
		Order("deleted_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&novels)

	var blogs []models.AuthorBlog
	var blogTotal int64
	models.DB.Unscoped().Model(&models.AuthorBlog{}).
		Where("author_id = ? AND deleted_at IS NOT NULL AND deleted_by = ?", userID, "user").
		Count(&blogTotal)
	models.DB.Unscoped().Preload("Author").
		Where("author_id = ? AND deleted_at IS NOT NULL AND deleted_by = ?", userID, "user").
		Order("deleted_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&blogs)
	fillBlogCategories(blogs)

	utils.Success(c, gin.H{
		"novels":     novels,
		"novel_total": novelTotal,
		"blogs":      blogs,
		"blog_total": blogTotal,
	})
}

// POST /api/author/recycle/restore — 恢复自己删除的内容（仅 deleted_by='user'）
// body: { type: novel|blog|chapter|comment, id }
func RestoreAuthorRecycle(c *gin.Context) {
	userID := currentUserID(c)
	var req struct {
		Type string `json:"type" binding:"required"`
		ID   uint   `json:"id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}

	switch req.Type {
	case "novel":
		var n models.Novel
		if err := models.DB.Unscoped().First(&n, req.ID).Error; err != nil {
			utils.NotFound(c, "作品不存在")
			return
		}
		if n.AuthorID != userID {
			utils.Forbidden(c, "只能恢复自己的作品")
			return
		}
		if n.DeletedBy != "user" {
			utils.Forbidden(c, "该内容由管理员封禁，无法自行恢复；请联系管理员申诉")
			return
		}
		models.DB.Unscoped().Model(&models.Novel{}).Where("id = ?", req.ID).
			Updates(map[string]interface{}{"deleted_at": nil, "deleted_by": "", "ban_reason": ""})
		// 恢复其章节（用户自删时章节未软删，但保险起见一并恢复标记）
		models.DB.Unscoped().Model(&models.Chapter{}).Where("novel_id = ?", req.ID).
			Updates(map[string]interface{}{"deleted_at": nil, "deleted_by": ""})
	case "blog":
		var b models.AuthorBlog
		if err := models.DB.Unscoped().First(&b, req.ID).Error; err != nil {
			utils.NotFound(c, "博客不存在")
			return
		}
		if b.AuthorID != userID {
			utils.Forbidden(c, "只能恢复自己的博客")
			return
		}
		if b.DeletedBy != "user" {
			utils.Forbidden(c, "该内容由管理员封禁，无法自行恢复；请联系管理员申诉")
			return
		}
		models.DB.Unscoped().Model(&models.AuthorBlog{}).Where("id = ?", req.ID).
			Updates(map[string]interface{}{"deleted_at": nil, "deleted_by": "", "ban_reason": ""})
	case "chapter":
		var ch models.Chapter
		if err := models.DB.Unscoped().First(&ch, req.ID).Error; err != nil {
			utils.NotFound(c, "章节不存在")
			return
		}
		var n models.Novel
		if err := models.DB.Unscoped().First(&n, ch.NovelID).Error; err != nil {
			utils.NotFound(c, "作品不存在")
			return
		}
		if n.AuthorID != userID {
			utils.Forbidden(c, "只能恢复自己的章节")
			return
		}
		if ch.DeletedBy != "user" {
			utils.Forbidden(c, "该内容由管理员封禁，无法自行恢复；请联系管理员申诉")
			return
		}
		models.DB.Unscoped().Model(&models.Chapter{}).Where("id = ?", req.ID).
			Updates(map[string]interface{}{"deleted_at": nil, "deleted_by": "", "ban_reason": ""})
		// 正文文件从 .deleted/ 归档目录移回（如还在）
		restoreChapterFile(&ch)
		models.UpdateNovelStats(ch.NovelID)
	case "comment":
		var cm models.Comment
		if err := models.DB.Unscoped().First(&cm, req.ID).Error; err != nil {
			utils.NotFound(c, "评论不存在")
			return
		}
		if cm.UserID != userID {
			utils.Forbidden(c, "只能恢复自己的评论")
			return
		}
		if cm.DeletedBy != "user" {
			utils.Forbidden(c, "该内容由管理员封禁，无法自行恢复")
			return
		}
		models.DB.Unscoped().Model(&models.Comment{}).Where("id = ?", req.ID).
			Updates(map[string]interface{}{"deleted_at": nil, "deleted_by": ""})
	default:
		utils.BadRequest(c, "不支持的类型（支持 novel/blog/chapter/comment）")
		return
	}

	utils.Success(c, gin.H{"message": "已恢复到原位置"})
}

// ─────────────────────────────────────────────────────────────
// 管理员视角：用户自删内容列表（回收站监管）
// ─────────────────────────────────────────────────────────────

// GET /api/admin/recycle — 管理员查看全站用户自删内容（novel/blog/chapter/comment）
func AdminListRecycle(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	ctype := c.Query("type") // novel / blog / chapter / comment，空=全部
	limit := 200
	if n, err := strconv.Atoi(c.Query("limit")); err == nil && n > 0 {
		limit = n
	}

	type recycleItem struct {
		Type       string `json:"type"`
		ID         uint   `json:"id"`
		Title      string `json:"title"`
		AuthorID   uint   `json:"author_id"`
		AuthorName string `json:"author_name"`
		DeletedAt  string `json:"deleted_at"`
	}
	items := []recycleItem{}
	appendNovels := func() {
		var rows []struct {
			ID        uint   `json:"id"`
			Title     string `json:"title"`
			AuthorID  uint   `json:"author_id"`
			DeletedAt string `json:"deleted_at"`
		}
		models.DB.Unscoped().Model(&models.Novel{}).
			Where("deleted_at IS NOT NULL AND deleted_by = ?", "user").
			Order("deleted_at DESC").Limit(limit).
			Scan(&rows)
		for _, r := range rows {
			items = append(items, recycleItem{Type: "novel", ID: r.ID, Title: r.Title, AuthorID: r.AuthorID, DeletedAt: r.DeletedAt})
		}
	}
	appendBlogs := func() {
		var rows []struct {
			ID        uint   `json:"id"`
			Title     string `json:"title"`
			AuthorID  uint   `json:"author_id"`
			DeletedAt string `json:"deleted_at"`
		}
		models.DB.Unscoped().Model(&models.AuthorBlog{}).
			Where("deleted_at IS NOT NULL AND deleted_by = ?", "user").
			Order("deleted_at DESC").Limit(limit).
			Scan(&rows)
		for _, r := range rows {
			items = append(items, recycleItem{Type: "blog", ID: r.ID, Title: r.Title, AuthorID: r.AuthorID, DeletedAt: r.DeletedAt})
		}
	}
	appendChapters := func() {
		var rows []struct {
			ID        uint   `json:"id"`
			Title     string `json:"title"`
			AuthorID  uint   `json:"author_id"`
			NovelID   uint   `json:"novel_id"`
			DeletedAt string `json:"deleted_at"`
		}
		models.DB.Unscoped().Model(&models.Chapter{}).
			Where("deleted_at IS NOT NULL AND deleted_by = ?", "user").
			Order("deleted_at DESC").Limit(limit).
			Scan(&rows)
		for _, r := range rows {
			title := r.Title
			if title == "" {
				title = "第" + strconv.FormatUint(uint64(r.NovelID), 10) + "章"
			}
			items = append(items, recycleItem{Type: "chapter", ID: r.ID, Title: title, AuthorID: r.AuthorID, DeletedAt: r.DeletedAt})
		}
	}
	appendComments := func() {
		var rows []struct {
			ID        uint   `json:"id"`
			Content   string `json:"content"`
			UserID    uint   `json:"user_id"`
			DeletedAt string `json:"deleted_at"`
		}
		models.DB.Unscoped().Model(&models.Comment{}).
			Where("deleted_at IS NOT NULL AND deleted_by = ?", "user").
			Order("deleted_at DESC").Limit(limit).
			Scan(&rows)
		for _, r := range rows {
			items = append(items, recycleItem{Type: "comment", ID: r.ID, Title: truncateRunes(toPlainText(r.Content), 60), AuthorID: r.UserID, DeletedAt: r.DeletedAt})
		}
	}

	switch ctype {
	case "novel":
		appendNovels()
	case "blog":
		appendBlogs()
	case "chapter":
		appendChapters()
	case "comment":
		appendComments()
	default:
		appendNovels()
		appendBlogs()
		appendChapters()
		appendComments()
	}

	// 填充作者名
	idSet := map[uint]bool{}
	for _, it := range items {
		idSet[it.AuthorID] = true
	}
	names := map[uint]string{}
	if len(idSet) > 0 {
		var users []models.User
		models.DB.Where("id IN ?", keysOf(idSet)).Find(&users)
		for _, u := range users {
			name := u.Nickname
			if name == "" {
				name = u.Username
			}
			names[u.ID] = name
		}
	}
	for i := range items {
		items[i].AuthorName = names[items[i].AuthorID]
	}

	utils.Success(c, items)
}

func keysOf(m map[uint]bool) []uint {
	out := make([]uint, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// restoreChapterFile 从 .deleted/ 归档目录找回章节正文文件
func restoreChapterFile(ch *models.Chapter) {
	if ch.ContentPath == "" {
		return
	}
	// 若正文文件不存在（在 .deleted/ 中），尝试按命名规则找回
	if _, err := os.Stat(ch.ContentPath); err != nil {
		dir := filepath.Dir(ch.ContentPath)
		deletedDir := filepath.Join(filepath.Dir(dir), ".deleted")
		matches, _ := filepath.Glob(filepath.Join(deletedDir, itoa(ch.ChapterNumber)+"_*.html"))
		if len(matches) > 0 {
			if err := copyFile(matches[0], ch.ContentPath); err == nil {
				models.DB.Model(&models.Chapter{}).Where("id = ?", ch.ID).
					Update("content_path", ch.ContentPath)
			}
		}
	}
}
