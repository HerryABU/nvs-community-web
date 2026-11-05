package bookshelf

import (
	"nvs-server/handlers/hctx"
	"nvs-server/models"
	"nvs-server/utils/infra"
	"strconv"

	"github.com/gin-gonic/gin"
	social "nvs-server/models/social"
)

// AddToShelf POST /api/bookshelf
func AddToShelf(c *gin.Context) {
	userID := hctx.CurrentUserID(c)

	var req struct {
		NovelID uint `json:"novel_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "请提供作品ID")
		return
	}

	if err := social.AddToShelf(userID, req.NovelID); err != nil {
		infra.InternalError(c, "添加书架失败")
		return
	}

	infra.Success(c, gin.H{"on_shelf": true})
}

// RemoveFromShelf DELETE /api/bookshelf/:id  (id = novel_id)
func RemoveFromShelf(c *gin.Context) {
	userID := hctx.CurrentUserID(c)
	novelID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		infra.BadRequest(c, "无效的作品ID")
		return
	}

	if err := social.RemoveFromShelf(userID, uint(novelID)); err != nil {
		infra.InternalError(c, "移出书架失败")
		return
	}

	infra.Success(c, gin.H{"on_shelf": false})
}

// CheckShelf GET /api/bookshelf/check/:id  (id = novel_id)
func CheckShelf(c *gin.Context) {
	userID := hctx.CurrentUserID(c)
	novelID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		infra.BadRequest(c, "无效的作品ID")
		return
	}

	onShelf := social.IsOnShelf(userID, uint(novelID))
	infra.Success(c, gin.H{"on_shelf": onShelf})
}

// ListShelf GET /api/bookshelf
func ListShelf(c *gin.Context) {
	userID := hctx.CurrentUserID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}

	items, total, err := social.GetShelfList(userID, page, pageSize)
	if err != nil {
		infra.InternalError(c, "获取书架列表失败")
		return
	}

	type ShelfItem struct {
		ID              uint          `json:"id"`
		NovelID         uint          `json:"novel_id"`
		LastReadChapter int           `json:"last_read_chapter"`
		AddedAt         string        `json:"added_at"`
		Novel           *models.Novel `json:"novel"`
	}

	var result []ShelfItem
	for _, item := range items {
		result = append(result, ShelfItem{
			ID:              item.ID,
			NovelID:         item.NovelID,
			LastReadChapter: item.LastReadChapter,
			AddedAt:         item.AddedAt.Format("2006-01-02 15:04:05"),
			Novel:           item.Novel,
		})
	}

	if result == nil {
		result = []ShelfItem{}
	}

	infra.Success(c, gin.H{
		"list":  result,
		"total": total,
	})
}

// UpdateShelfProgress POST /api/bookshelf/progress
func UpdateShelfProgress(c *gin.Context) {
	userID := hctx.CurrentUserID(c)

	var req struct {
		NovelID         uint `json:"novel_id" binding:"required"`
		LastReadChapter int  `json:"last_read_chapter"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "请提供作品ID和章节号")
		return
	}

	if err := social.UpdateShelfProgress(userID, req.NovelID, req.LastReadChapter); err != nil {
		// 书架中不存在也静默成功——用户可能没加书架但读了
	}

	infra.Success(c, nil)
}
