package handlers

import (
	"strconv"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ============ 精读：书签 + 批注 ============

// POST /api/reading/notes — 新建书签/批注
func CreateReadingNote(c *gin.Context) {
	userID := currentUserID(c)
	if userID == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}
	var req struct {
		NovelID       uint   `json:"novel_id" binding:"required"`
		ChapterNumber int    `json:"chapter_number"`
		Type          string `json:"type" binding:"required,oneof=bookmark annotation"`
		Content       string `json:"content"`
		SelectedText  string `json:"selected_text"`
		Anchor        int    `json:"anchor"`
		Visibility    string `json:"visibility"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if req.Type == "annotation" && req.Content == "" && req.SelectedText == "" {
		utils.BadRequest(c, "批注内容不能为空")
		return
	}
	vis := req.Visibility
	if vis != "public" {
		vis = "private"
	}
	// 校验作品存在
	var n models.Novel
	if err := models.DB.Select("id").First(&n, req.NovelID).Error; err != nil {
		utils.NotFound(c, "作品不存在")
		return
	}
	note := &models.ReadingNote{
		UserID: userID, NovelID: req.NovelID, ChapterNumber: req.ChapterNumber,
		Type: req.Type, Content: req.Content, SelectedText: req.SelectedText,
		Anchor: req.Anchor, Visibility: vis,
	}
	if err := models.CreateReadingNote(note); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.Success(c, note)
}

// GET /api/reading/notes — 我的笔记（可按 novel_id / chapter_number / type 筛选）
func ListMyReadingNotes(c *gin.Context) {
	userID := currentUserID(c)
	if userID == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}
	novelID, _ := strconv.ParseUint(c.DefaultQuery("novel_id", "0"), 10, 64)
	chapterNum, _ := strconv.Atoi(c.DefaultQuery("chapter_number", "0"))
	noteType := c.Query("type")

	list, err := models.ListMyReadingNotes(userID, uint(novelID), chapterNum, noteType)
	if err != nil {
		utils.InternalError(c, "获取笔记失败")
		return
	}
	utils.Success(c, list)
}

// GET /api/reading/notes/public — 公开笔记（novel_id 必填；游客可读）
func ListPublicNotes(c *gin.Context) {
	novelID, _ := strconv.ParseUint(c.DefaultQuery("novel_id", "0"), 10, 64)
	if novelID == 0 {
		utils.BadRequest(c, "缺少 novel_id")
		return
	}
	chapterNum, _ := strconv.Atoi(c.DefaultQuery("chapter_number", "0"))
	list, err := models.ListPublicNotes(uint(novelID), chapterNum)
	if err != nil {
		utils.InternalError(c, "获取公开批注失败")
		return
	}
	utils.Success(c, list)
}

// GET /api/reading/notes/mine — 我的全部笔记（精读页，分页，含作品名/章节名）
func ListAllMyNotes(c *gin.Context) {
	userID := currentUserID(c)
	if userID == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	list, total, err := models.ListAllMyNotes(userID, page, pageSize)
	if err != nil {
		utils.InternalError(c, "获取精读列表失败")
		return
	}
	utils.Success(c, gin.H{"list": list, "total": total, "page": page, "page_size": pageSize})
}

// PATCH /api/reading/notes/:id — 编辑内容 / 切换公开私密
func UpdateReadingNote(c *gin.Context) {
	userID := currentUserID(c)
	if userID == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	note, err := models.GetReadingNote(uint(id))
	if err != nil || note.UserID != userID {
		utils.NotFound(c, "笔记不存在")
		return
	}
	var req struct {
		Content    string `json:"content"`
		Visibility string `json:"visibility"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := models.UpdateReadingNote(uint(id), req.Content, req.Visibility); err != nil {
		utils.InternalError(c, "更新失败")
		return
	}
	utils.SuccessMessage(c, "已更新", nil)
}

// DELETE /api/reading/notes/:id — 删除（仅本人）
func DeleteReadingNote(c *gin.Context) {
	userID := currentUserID(c)
	if userID == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	note, err := models.GetReadingNote(uint(id))
	if err != nil || note.UserID != userID {
		utils.NotFound(c, "笔记不存在")
		return
	}
	if err := models.DeleteReadingNote(uint(id)); err != nil {
		utils.InternalError(c, "删除失败")
		return
	}
	utils.SuccessMessage(c, "已删除", nil)
}
