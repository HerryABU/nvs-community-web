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

	// 博客评论
	if blogID > 0 {
		comments, total, err := models.GetCommentsByBlog(uint(blogID), page, pageSize)
		if err != nil {
			utils.InternalError(c, "获取评论失败")
			return
		}
		utils.Success(c, gin.H{"list": comments, "total": total})
		return
	}

	comments, total, err := models.GetCommentsByNovel(uint(novelID), chapterNum, page, pageSize)
	if err != nil {
		utils.InternalError(c, "获取评论失败")
		return
	}

	utils.Success(c, gin.H{
		"list":  comments,
		"total": total,
	})
}

func CreateComment(c *gin.Context) {
	userID := c.GetUint("userID")

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

	comment := &models.Comment{
		UserID:        userID,
		NovelID:       req.NovelID,
		BlogID:        req.BlogID,
		ChapterNumber: req.ChapterNumber,
		Content:       security.SanitizeUserContent(req.Content),
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

	if user, err := models.GetUserByID(userID); err == nil {
		comment.Username = user.Username
	}

	utils.Success(c, comment)
}

func DeleteComment(c *gin.Context) {
	userID := c.GetUint("userID")
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

	utils.Success(c, gin.H{"message": "评论已删除"})
}
