package handlers

import (
	"strconv"

	"nvs-server/collab"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== 章节工作流（多人协作） ====================
// 状态机见 docs/novel-collaboration.md §3：
//   draft --提审--> pending --通过--> published
//                      └--驳回--> rejected（作者改完可再提审）
//   published --撤回--> draft
//
// 权限：
//   提审：能编辑本章的人（主笔/本章作者/有 edit_others）
//   审核：主笔、审稿角色、can_review、平台
//   发布：主笔、can_publish；审核通过同样落到 published
//   撤回：主笔、本章作者

// loadChapterForWorkflow 载入作品/章节并解析协作身份
func loadChapterForWorkflow(c *gin.Context) (*models.Novel, *models.Chapter, collab.Actor, bool) {
	novelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	chapterNum, _ := strconv.Atoi(c.Param("num"))
	novel, err := models.GetNovelByID(uint(novelID))
	if err != nil {
		utils.NotFound(c, "作品不存在")
		return nil, nil, collab.Actor{}, false
	}
	chapter, err := models.GetChapterByNovelAndNumber(uint(novelID), chapterNum)
	if err != nil {
		utils.NotFound(c, "章节不存在")
		return nil, nil, collab.Actor{}, false
	}
	actor := collab.ActorOf(novel.ID, currentUserID(c), currentUserRole(c))
	return novel, chapter, actor, true
}

// SubmitChapterReview POST /api/novels/:id/chapters/:num/submit
func SubmitChapterReview(c *gin.Context) {
	novel, chapter, actor, ok := loadChapterForWorkflow(c)
	if !ok {
		return
	}
	if !actor.CanEditChapter(chapter) {
		utils.Forbidden(c, "无权操作本章")
		return
	}
	if chapter.Status == models.ChapterStatusPublished {
		utils.BadRequest(c, "本章已发布，无需提审")
		return
	}
	if err := models.DB.Model(&models.Chapter{}).Where("id = ?", chapter.ID).
		Updates(map[string]interface{}{"status": models.ChapterStatusPending, "review_note": ""}).Error; err != nil {
		utils.InternalError(c, "提审失败")
		return
	}
	collab.LogChapterAction(novel.ID, currentUserID(c), "submit", chapter.ID, "")
	utils.SuccessMessage(c, "已提交审核", nil)
}

// ReviewChapter POST /api/novels/:id/chapters/:num/review { approve, note }
func ReviewChapter(c *gin.Context) {
	novel, chapter, actor, ok := loadChapterForWorkflow(c)
	if !ok {
		return
	}
	if !actor.Can(collab.CapReviewChapter) {
		utils.Forbidden(c, "没有审稿权限")
		return
	}
	var req struct {
		Approve bool   `json:"approve"`
		Note    string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	status := models.ChapterStatusRejected
	if req.Approve {
		status = models.ChapterStatusPublished
	}
	note := req.Note
	if len([]rune(note)) > 200 {
		note = string([]rune(note)[:200])
	}
	if err := models.DB.Model(&models.Chapter{}).Where("id = ?", chapter.ID).
		Updates(map[string]interface{}{"status": status, "review_note": note}).Error; err != nil {
		utils.InternalError(c, "审核失败")
		return
	}
	if req.Approve {
		models.RecordPublish(chapter.AuthorID, models.PublishKindChapter, chapter.ID, chapter.WordCount)
		models.UpdateNovelStats(novel.ID)
		collab.LogChapterAction(novel.ID, currentUserID(c), "publish", chapter.ID, "审核通过")
		utils.SuccessMessage(c, "已通过并发布", nil)
		return
	}
	collab.LogChapterAction(novel.ID, currentUserID(c), "reject", chapter.ID, note)
	utils.SuccessMessage(c, "已驳回", nil)
}

// PublishChapter POST /api/novels/:id/chapters/:num/publish
func PublishChapter(c *gin.Context) {
	novel, chapter, actor, ok := loadChapterForWorkflow(c)
	if !ok {
		return
	}
	if !actor.Can(collab.CapPublishChapter) {
		utils.Forbidden(c, "没有直接发布权限（可提审后由审稿发布）")
		return
	}
	if chapter.Status == models.ChapterStatusPublished {
		utils.SuccessMessage(c, "本章已处于发布状态", nil)
		return
	}
	if err := models.DB.Model(&models.Chapter{}).Where("id = ?", chapter.ID).
		Updates(map[string]interface{}{"status": models.ChapterStatusPublished, "review_note": ""}).Error; err != nil {
		utils.InternalError(c, "发布失败")
		return
	}
	models.RecordPublish(chapter.AuthorID, models.PublishKindChapter, chapter.ID, chapter.WordCount)
	models.UpdateNovelStats(novel.ID)
	collab.LogChapterAction(novel.ID, currentUserID(c), "publish", chapter.ID, "直接发布")
	utils.SuccessMessage(c, "已发布", nil)
}

// WithdrawChapter POST /api/novels/:id/chapters/:num/withdraw
func WithdrawChapter(c *gin.Context) {
	novel, chapter, actor, ok := loadChapterForWorkflow(c)
	if !ok {
		return
	}
	if !actor.IsOwner && chapter.AuthorID != currentUserID(c) && !actor.IsPlatform {
		utils.Forbidden(c, "只有主笔或本章作者可以撤回")
		return
	}
	if chapter.Status != models.ChapterStatusPublished {
		utils.BadRequest(c, "本章尚未发布")
		return
	}
	if err := models.DB.Model(&models.Chapter{}).Where("id = ?", chapter.ID).
		Update("status", models.ChapterStatusDraft).Error; err != nil {
		utils.InternalError(c, "撤回失败")
		return
	}
	models.UpdateNovelStats(novel.ID)
	collab.LogChapterAction(novel.ID, currentUserID(c), "withdraw", chapter.ID, "")
	utils.SuccessMessage(c, "已撤回为草稿", nil)
}
