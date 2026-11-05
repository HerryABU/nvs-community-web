package ai

import (
	"strconv"
	"strings"

	"nvs-server/handlers/hctx"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	aim "nvs-server/models/ai"
)

// ==================== 用户投稿智能体 ====================
//
// 流程（站长可配）：
//   需要审核（默认）：提交 → 待审 → 站长通过 → 出现在智能体广场
//   免审核：提交 → 直接上架（仍然留一条 approved 投稿记录，便于追溯作者）
//
// 「审核开关由站长设置」= AgentSubmitConfig{Enabled, NeedReview, MaxPending}。

// AiSubmitAgent POST /api/ai/agents/submit
func AiSubmitAgent(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}
	var in aim.AgentSubmission
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}

	u, _ := models.GetUserByID(uid)
	userName := ""
	if u != nil {
		userName = u.Username
	}

	sub, err := aim.CreateAgentSubmission(uid, userName, in)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	cfg := aim.GetAgentSubmitConfig()
	if !cfg.NeedReview {
		// 免审核：直接上架（reviewer 记 0 = 系统自动）
		if err := aim.ApproveAgentSubmission(sub, 0, "免审核直上（站长设置）"); err != nil {
			utils.InternalError(c, "上架失败："+err.Error())
			return
		}
		utils.Success(c, gin.H{"status": aim.AgentSubApproved, "message": "已上架，现在就能在智能体广场看到"})
		return
	}
	utils.Success(c, gin.H{"status": aim.AgentSubPending, "message": "已提交，等站长审核后上架"})
}

// AiMyAgentSubmissions GET /api/ai/agents/my-submissions
func AiMyAgentSubmissions(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}
	var list []aim.AgentSubmission
	models.DB.Where("user_id = ?", uid).Order("id DESC").Limit(50).Find(&list)
	utils.Success(c, gin.H{"list": list})
}

// AiAgentSubmitConfig GET /api/ai/agents/submit-config
// 前台据此决定「投稿」入口是否显示、文案怎么写（需要审核 / 直上）。
func AiAgentSubmitConfig(c *gin.Context) {
	cfg := aim.GetAgentSubmitConfig()
	utils.Success(c, gin.H{
		"enabled":     cfg.Enabled,
		"need_review": cfg.NeedReview,
		"max_pending": cfg.MaxPending,
	})
}

// ==================== 站长审核 ====================

// AdminListAgentSubmissions GET /api/admin/agent-submissions?status=pending&page=1&size=20
func AdminListAgentSubmissions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	list, total := aim.ListAgentSubmissions(strings.TrimSpace(c.Query("status")), page, size)
	utils.Success(c, gin.H{
		"list":    list,
		"total":   total,
		"pending": aim.CountPendingAgentSubmissions(),
	})
}

// AdminReviewAgentSubmission POST /api/admin/agent-submissions/:id/review
// body: { action: "approve" | "reject", note: "..." }
func AdminReviewAgentSubmission(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var sub aim.AgentSubmission
	if err := models.DB.First(&sub, id).Error; err != nil {
		utils.NotFound(c, "投稿不存在")
		return
	}
	var req struct {
		Action string `json:"action"`
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	switch strings.ToLower(strings.TrimSpace(req.Action)) {
	case "approve":
		if err := aim.ApproveAgentSubmission(&sub, uid, req.Note); err != nil {
			utils.BadRequest(c, err.Error())
			return
		}
		utils.Success(c, gin.H{"message": "已通过，已出现在智能体广场"})
	case "reject":
		if strings.TrimSpace(req.Note) == "" {
			utils.BadRequest(c, "拒绝时请写明原因，用户才知道怎么改")
			return
		}
		if err := aim.RejectAgentSubmission(&sub, uid, req.Note); err != nil {
			utils.BadRequest(c, err.Error())
			return
		}
		utils.Success(c, gin.H{"message": "已拒绝"})
	default:
		utils.BadRequest(c, "action 只能是 approve / reject")
	}
}

// AdminGetAgentSubmitConfig GET /api/admin/agent-submit-config
func AdminGetAgentSubmitConfig(c *gin.Context) {
	cfg := aim.GetAgentSubmitConfig()
	utils.Success(c, gin.H{"config": cfg, "pending": aim.CountPendingAgentSubmissions()})
}

// AdminSaveAgentSubmitConfig PUT /api/admin/agent-submit-config
func AdminSaveAgentSubmitConfig(c *gin.Context) {
	var cfg aim.AgentSubmitConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := aim.SetAgentSubmitConfig(cfg); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, gin.H{"message": "已保存", "config": aim.GetAgentSubmitConfig()})
}
