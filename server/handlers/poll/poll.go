package poll

// 议题投票接口：作者发起 / 终止，读者投票，结果统计。
// 权限：只有载体作者（或管理员）能创建与终止投票。

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"nvs-server/handlers/admincommon"
	"nvs-server/middleware"
	"nvs-server/models"
	"nvs-server/utils/infra"

	"gorm.io/gorm"
)

var allowedKinds = map[string]bool{"blog": true, "micro": true, "forum": true}

// Create POST /api/polls —— 作者发起投票（编辑时插入 或 详情页补发均走此接口）
func Create(c *gin.Context) {
	userID := middleware.UserID(c)
	if userID == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	var req struct {
		TargetKind  string   `json:"target_kind" binding:"required"`
		TargetID    uint     `json:"target_id" binding:"required"`
		Question    string   `json:"question" binding:"required"`
		Description string   `json:"description"`
		Multiple    bool     `json:"multiple"`
		Anonymous   bool     `json:"anonymous"`
		Options     []string `json:"options" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	if !allowedKinds[req.TargetKind] {
		infra.BadRequest(c, "该类型不支持投票")
		return
	}
	if len(req.Options) < 2 || len(req.Options) > 20 {
		infra.BadRequest(c, "选项须为 2-20 个")
		return
	}
	if !isOwner(req.TargetKind, req.TargetID, userID) && !admincommon.EnsureAdmin(c) {
		infra.Forbidden(c, "只有作者可以发起投票")
		return
	}
	p := models.Poll{
		TargetKind: req.TargetKind, TargetID: req.TargetID, AuthorID: userID,
		Question: req.Question, Description: req.Description,
		Multiple: req.Multiple, Anonymous: req.Anonymous, Status: models.PollStatusOpen,
	}
	if err := models.DB.Create(&p).Error; err != nil {
		infra.InternalError(c, "创建失败")
		return
	}
	for i, label := range req.Options {
		models.DB.Create(&models.PollOption{PollID: p.ID, Label: label, Sort: i})
	}
	infra.Success(c, p)
}

// Get GET /api/polls/:id —— 查询投票（含当前用户已投项）
func Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var p models.Poll
	if err := models.DB.First(&p, id).Error; err != nil {
		infra.NotFound(c, "投票不存在")
		return
	}
	var opts []models.PollOption
	models.DB.Where("poll_id = ?", id).Order("sort ASC, id ASC").Find(&opts)
	var mine []uint
	if uid := middleware.UserID(c); uid != 0 {
		models.DB.Model(&models.PollVote{}).Where("poll_id = ? AND user_id = ?", id, uid).Pluck("option_id", &mine)
	}
	voterCount := int64(0)
	models.DB.Model(&models.PollVote{}).Where("poll_id = ?", id).Distinct("user_id").Count(&voterCount)
	infra.Success(c, gin.H{"poll": p, "options": opts, "mine": mine, "voter_count": voterCount})
}

// ListByTarget GET /api/polls?target_kind=&target_id=
func ListByTarget(c *gin.Context) {
	kind := c.Query("target_kind")
	targetID, _ := strconv.ParseUint(c.Query("target_id"), 10, 64)
	if kind == "" || targetID == 0 {
		infra.BadRequest(c, "参数错误")
		return
	}
	var polls []models.Poll
	models.DB.Where("target_kind = ? AND target_id = ?", kind, targetID).Order("id DESC").Find(&polls)
	infra.Success(c, polls)
}

// Vote POST /api/polls/:id/vote —— 投票（可多选，重复投票覆盖）
func Vote(c *gin.Context) {
	userID := middleware.UserID(c)
	if userID == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var p models.Poll
	if err := models.DB.First(&p, id).Error; err != nil {
		infra.NotFound(c, "投票不存在")
		return
	}
	if p.Status != models.PollStatusOpen {
		infra.BadRequest(c, "投票已终止")
		return
	}
	var req struct {
		OptionIDs []uint `json:"option_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.OptionIDs) == 0 {
		infra.BadRequest(c, "请选择选项")
		return
	}
	if !p.Multiple && len(req.OptionIDs) > 1 {
		infra.BadRequest(c, "该投票为单选")
		return
	}
	// 校验选项归属
	var count int64
	models.DB.Model(&models.PollOption{}).Where("poll_id = ? AND id IN ?", id, req.OptionIDs).Count(&count)
	if count != int64(len(req.OptionIDs)) {
		infra.BadRequest(c, "选项非法")
		return
	}
	// 覆盖旧投票：删旧记录、扣旧计数、加新计数
	var old []models.PollVote
	models.DB.Where("poll_id = ? AND user_id = ?", id, userID).Find(&old)
	tx := models.DB.Begin()
	for _, v := range old {
		tx.Model(&models.PollOption{}).Where("id = ?", v.OptionID).UpdateColumn("votes", gorm.Expr("votes - 1"))
	}
	tx.Where("poll_id = ? AND user_id = ?", id, userID).Delete(&models.PollVote{})
	for _, oid := range req.OptionIDs {
		tx.Create(&models.PollVote{PollID: uint(id), OptionID: oid, UserID: userID})
		tx.Model(&models.PollOption{}).Where("id = ?", oid).UpdateColumn("votes", gorm.Expr("votes + 1"))
	}
	tx.Commit()
	infra.Success(c, gin.H{"message": "投票成功"})
}

// Close POST /api/polls/:id/close —— 作者终止投票并统计
func Close(c *gin.Context) {
	userID := middleware.UserID(c)
	if userID == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var p models.Poll
	if err := models.DB.First(&p, id).Error; err != nil {
		infra.NotFound(c, "投票不存在")
		return
	}
	if p.AuthorID != userID && !admincommon.EnsureAdmin(c) {
		infra.Forbidden(c, "只有作者可以终止投票")
		return
	}
	now := time.Now()
	models.DB.Model(&p).Updates(map[string]interface{}{"status": models.PollStatusClosed, "closed_at": now})
	infra.Success(c, gin.H{"message": "投票已终止", "closed_at": now})
}

func isOwner(kind string, targetID, userID uint) bool {
	switch kind {
	case "blog":
		var row struct{ AuthorID uint }
		models.DB.Table("author_blogs").Select("author_id").Where("id = ?", targetID).Scan(&row)
		return row.AuthorID == userID
	case "micro":
		var row struct{ UserID uint }
		models.DB.Table("feed_posts").Select("user_id").Where("id = ?", targetID).Scan(&row)
		return row.UserID == userID
	case "forum":
		var row struct{ UserID uint }
		models.DB.Table("threads").Select("user_id").Where("id = ?", targetID).Scan(&row)
		return row.UserID == userID
	}
	return false
}
