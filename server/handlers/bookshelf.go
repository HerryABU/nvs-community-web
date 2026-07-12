package handlers

import (
	"nvs-server/models"
	"nvs-server/utils"
	"strconv"

	"github.com/gin-gonic/gin"
)

// AddToShelf POST /api/bookshelf
func AddToShelf(c *gin.Context) {
	userID := c.GetUint("userID")

	var req struct {
		NovelID uint `json:"novel_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请提供作品ID")
		return
	}

	if err := models.AddToShelf(userID, req.NovelID); err != nil {
		utils.InternalError(c, "添加书架失败")
		return
	}

	utils.Success(c, gin.H{"on_shelf": true})
}

// RemoveFromShelf DELETE /api/bookshelf/:id  (id = novel_id)
func RemoveFromShelf(c *gin.Context) {
	userID := c.GetUint("userID")
	novelID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "无效的作品ID")
		return
	}

	if err := models.RemoveFromShelf(userID, uint(novelID)); err != nil {
		utils.InternalError(c, "移出书架失败")
		return
	}

	utils.Success(c, gin.H{"on_shelf": false})
}

// CheckShelf GET /api/bookshelf/check/:id  (id = novel_id)
func CheckShelf(c *gin.Context) {
	userID := c.GetUint("userID")
	novelID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "无效的作品ID")
		return
	}

	onShelf := models.IsOnShelf(userID, uint(novelID))
	utils.Success(c, gin.H{"on_shelf": onShelf})
}

// ListShelf GET /api/bookshelf
func ListShelf(c *gin.Context) {
	userID := c.GetUint("userID")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}

	items, total, err := models.GetShelfList(userID, page, pageSize)
	if err != nil {
		utils.InternalError(c, "获取书架列表失败")
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

	utils.Success(c, gin.H{
		"list":  result,
		"total": total,
	})
}

// UpdateShelfProgress POST /api/bookshelf/progress
func UpdateShelfProgress(c *gin.Context) {
	userID := c.GetUint("userID")

	var req struct {
		NovelID         uint `json:"novel_id" binding:"required"`
		LastReadChapter int  `json:"last_read_chapter"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请提供作品ID和章节号")
		return
	}

	if err := models.UpdateShelfProgress(userID, req.NovelID, req.LastReadChapter); err != nil {
		// 书架中不存在也静默成功——用户可能没加书架但读了
	}

	utils.Success(c, nil)
}
