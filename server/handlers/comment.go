package handlers

import (
	"strconv"

	"nvs-server/models"
	"nvs-server/security"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

func GetComments(c *gin.Context) {
	novelID, _ := strconv.ParseUint(c.Query("novel_id"), 10, 64)
	blogID, _ := strconv.ParseUint(c.Query("blog_id"), 10, 64)
	chapterNum, _ := strconv.Atoi(c.DefaultQuery("chapter_number", "0"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	// 楼层模型：List = 本页楼层 + 这些楼层下的全部回复（平铺，前端按 root_id 分组成「楼」）
	var pg models.CommentsPage
	var err error
	if blogID > 0 {
		pg, err = models.GetCommentsByBlog(uint(blogID), page, pageSize)
	} else {
		pg, err = models.GetCommentsByNovel(uint(novelID), chapterNum, page, pageSize)
	}
	if err != nil {
		utils.InternalError(c, "获取评论失败")
		return
	}

	utils.Success(c, gin.H{
		"list":        pg.List,
		"total":       pg.Total,
		"floor_total": pg.FloorTotal,
		"has_more":    pg.HasMore,
		"page":        page,
	})
}

func CreateComment(c *gin.Context) {
	userID := currentUserID(c)

	// 全局禁言检查
	if checkUserMuted(c) {
		return
	}

	var req struct {
		NovelID       uint   `json:"novel_id"`
		BlogID        uint   `json:"blog_id"`
		ChapterNumber int    `json:"chapter_number"`
		Content       string `json:"content" binding:"required"`
		QuoteText     string `json:"quote_text"`
		ParentID      *uint  `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请填写评论内容")
		return
	}

	if req.NovelID == 0 && req.BlogID == 0 {
		utils.BadRequest(c, "请指定作品或博客")
		return
	}

	// XSS 净化 + 违禁词库净化（delete/replace 动作直接改写文本）
	raw := security.SanitizeUserContent(req.Content)
	cleaned, badHits := processBadWords(raw, "comment")

	comment := &models.Comment{
		UserID:        userID,
		NovelID:       req.NovelID,
		BlogID:        req.BlogID,
		ChapterNumber: req.ChapterNumber,
		Content:       cleaned,
		QuoteText:     security.SanitizePlainText(req.QuoteText),
		IsMarkdown:    true,
	}
	if req.ParentID != nil {
		comment.ParentID = req.ParentID
	}

	if err := models.CreateComment(comment); err != nil {
		utils.InternalError(c, "发表评论失败")
		return
	}

	// 违禁词 mark 动作命中 → 写入待审标记记录
	flagBadWords(badHits, "comment", comment.ID, userID, raw, "")

	if user, err := models.GetUserByID(userID); err == nil {
		comment.Username = user.Username
	}

	utils.Success(c, comment)
}

func DeleteComment(c *gin.Context) {
	userID := currentUserID(c)
	commentID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	comment, err := models.GetCommentByID(uint(commentID))
	if err != nil {
		utils.NotFound(c, "评论不存在")
		return
	}

	if comment.UserID != userID {
		utils.Forbidden(c, "无权删除此评论")
		return
	}

	if err := models.DeleteComment(uint(commentID)); err != nil {
		utils.InternalError(c, "删除评论失败")
		return
	}

	// 标记为「用户自删」（区别于管理员删除：deleted_by='admin'）
	_ = models.DB.Unscoped().Model(&models.Comment{}).Where("id = ?", commentID).Update("deleted_by", "user")
	utils.Success(c, gin.H{"message": "评论已删除"})
}
