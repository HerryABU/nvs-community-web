package handlers

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"nvs-server/aiclient"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== AI 工作台：绘画 / 视频（§5.4，按次计费） ====================

// AiHubImage POST /api/ai/image — 绘画（同步出图，成功才扣款）
func AiHubImage(c *gin.Context) {
	if !models.IsAiEnabled() {
		utils.BadRequest(c, "AI 功能未开启")
		return
	}
	userID := currentUserID(c)
	var req struct {
		ModelID uint   `json:"model_id"`
		Prompt  string `json:"prompt"`
		Size    string `json:"size"`
		Style   string `json:"style"`
		N       int    `json:"n"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		utils.BadRequest(c, "请输入绘画提示词")
		return
	}
	if len([]rune(prompt)) > 4000 {
		utils.BadRequest(c, "提示词过长（上限 4000 字）")
		return
	}
	m, err := models.GetEnabledAiModelOfKind(req.ModelID, models.AiKindImage)
	if err != nil || m == nil {
		utils.BadRequest(c, "绘画模型不可用")
		return
	}
	n := req.N
	if n < 1 || n > 4 {
		n = 1
	}
	task, terr := models.CreateAiMediaTask(userID, m, models.AiKindImage, prompt, models.AiMediaParams{
		Size: strings.TrimSpace(req.Size), Style: strings.TrimSpace(req.Style), N: n,
	})
	if terr != nil {
		utils.InternalError(c, "创建任务失败")
		return
	}
	apiKey, kerr := m.DecryptApiKey()
	if kerr != nil {
		models.FailAiMediaTask(task.ID, "密钥解密失败", false)
		utils.InternalError(c, "模型密钥解密失败，请联系站长")
		return
	}
	client := aiclient.NewClient(m.BaseURL, apiKey, 180*time.Second)
	urls, gerr := client.GenerateImages(aiclient.ImagesRequest{
		Model: m.ModelID, Prompt: prompt, N: n,
		Size: strings.TrimSpace(req.Size), ResponseFormat: "url",
	})
	if gerr != nil {
		models.FailAiMediaTask(task.ID, gerr.Error(), false)
		utils.BadRequest(c, "绘画失败："+gerr.Error()+"（未扣款）")
		return
	}
	// 免费额度优先（按次消耗 1 次）；否则代币扣款
	freeOK, _, _ := models.CheckAiFreeQuota(userID, 0)
	var nb int64
	if freeOK {
		_ = models.MarkAiMediaFree(task.ID)
		_ = models.RecordAiFreeUsage(userID, 1, 0)
		nb = models.GetTokenBalance(userID)
	} else {
		if !models.IsTokenEnabled() {
			models.FailAiMediaTask(task.ID, "代币系统未开启", false)
			utils.BadRequest(c, "代币系统未开启，无法使用付费 AI（未扣款）")
			return
		}
		var cerr error
		nb, cerr = models.ChargeAiMediaTask(task.ID)
		if cerr != nil {
			utils.Error(c, 429, 42902, cerr.Error()+"（未扣款）")
			return
		}
	}
	rj, _ := json.Marshal(gin.H{"images": urls})
	models.UpdateAiMediaResult(task.ID, string(rj))
	utils.Success(c, gin.H{
		"task_id":       task.ID,
		"images":        urls,
		"cost":          map[bool]int64{true: 0, false: m.PriceCall}[freeOK],
		"free":          freeOK,
		"balance_after": nb,
	})
}

// AiHubVideoSubmit POST /api/ai/video — 视频生成（异步：提交即扣款，失败退款）
func AiHubVideoSubmit(c *gin.Context) {
	if !models.IsAiEnabled() {
		utils.BadRequest(c, "AI 功能未开启")
		return
	}
	userID := currentUserID(c)
	var req struct {
		ModelID  uint   `json:"model_id"`
		Prompt   string `json:"prompt"`
		Duration int    `json:"duration"`
		Size     string `json:"size"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		utils.BadRequest(c, "请输入视频提示词")
		return
	}
	if len([]rune(prompt)) > 4000 {
		utils.BadRequest(c, "提示词过长（上限 4000 字）")
		return
	}
	m, err := models.GetEnabledAiModelOfKind(req.ModelID, models.AiKindVideo)
	if err != nil || m == nil {
		utils.BadRequest(c, "视频模型不可用")
		return
	}
	// 免费额度优先（按次消耗 1 次）
	freeOK, _, _ := models.CheckAiFreeQuota(userID, 0)
	// 非免费预检：余额 ≥ 单次价
	if !freeOK {
		if !models.IsTokenEnabled() {
			utils.BadRequest(c, "代币系统未开启，无法使用付费 AI")
			return
		}
		if m.PriceCall > 0 && models.GetTokenBalance(userID) < m.PriceCall {
			utils.Error(c, 429, 42902, "余额不足：本次需 "+strconv.FormatInt(m.PriceCall, 10)+
				"，当前余额 "+strconv.FormatInt(models.GetTokenBalance(userID), 10)+"，请先充值")
			return
		}
	}
	task, terr := models.CreateAiMediaTask(userID, m, models.AiKindVideo, prompt, models.AiMediaParams{
		Duration: req.Duration, Size: strings.TrimSpace(req.Size),
	})
	if terr != nil {
		utils.InternalError(c, "创建任务失败")
		return
	}
	apiKey, kerr := m.DecryptApiKey()
	if kerr != nil {
		models.FailAiMediaTask(task.ID, "密钥解密失败", false)
		utils.InternalError(c, "模型密钥解密失败，请联系站长")
		return
	}
	client := aiclient.NewClient(m.BaseURL, apiKey, 60*time.Second)
	remoteID, status, verr := client.CreateVideo(aiclient.VideosCreateRequest{
		Model: m.ModelID, Prompt: prompt,
		Duration: req.Duration, Size: strings.TrimSpace(req.Size),
	})
	if verr != nil {
		models.FailAiMediaTask(task.ID, verr.Error(), false)
		utils.BadRequest(c, "视频提交失败："+verr.Error()+"（未扣款）")
		return
	}
	// 提交成功：免费额度记用量；否则先扣款再转 running（扣款失败则任务失败）
	var nb int64
	if freeOK {
		_ = models.MarkAiMediaFree(task.ID)
		_ = models.RecordAiFreeUsage(userID, 1, 0)
		nb = models.GetTokenBalance(userID)
	} else {
		var cerr error
		nb, cerr = models.ChargeAiMediaTask(task.ID)
		if cerr != nil {
			models.FailAiMediaTask(task.ID, cerr.Error(), false)
			utils.Error(c, 429, 42902, cerr.Error()+"（未扣款，请充值后重试）")
			return
		}
	}
	models.MarkAiMediaSubmitted(task.ID, remoteID)
	utils.Success(c, gin.H{
		"task_id":       task.ID,
		"status":        status,
		"cost":          map[bool]int64{true: 0, false: m.PriceCall}[freeOK],
		"free":          freeOK,
		"balance_after": nb,
	})
}

// AiHubVideoStatus GET /api/ai/video/:id — 视频任务轮询
func AiHubVideoStatus(c *gin.Context) {
	userID := currentUserID(c)
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	task, err := models.GetAiMediaTask(uint(id), userID)
	if err != nil {
		utils.NotFound(c, "任务不存在")
		return
	}
	if task.Status == models.AiMediaSucceeded || task.Status == models.AiMediaFailed {
		utils.Success(c, gin.H{
			"task_id": task.ID, "status": task.Status,
			"result": task.Result, "error": task.Error, "cost": task.Cost,
		})
		return
	}
	if task.RemoteID == "" {
		utils.Success(c, gin.H{"task_id": task.ID, "status": task.Status, "error": task.Error, "cost": task.Cost})
		return
	}
	m, merr := models.GetAiModel(task.ModelID)
	if merr != nil {
		utils.BadRequest(c, "模型已删除，无法继续查询")
		return
	}
	apiKey, kerr := m.DecryptApiKey()
	if kerr != nil {
		utils.InternalError(c, "模型密钥解密失败")
		return
	}
	client := aiclient.NewClient(m.BaseURL, apiKey, 30*time.Second)
	status, videoURL, upErr, qerr := client.QueryVideo(task.RemoteID)
	if qerr != nil {
		// 查询本身失败：保持原状态，前端继续轮询
		utils.Success(c, gin.H{"task_id": task.ID, "status": task.Status, "error": qerr.Error(), "cost": task.Cost})
		return
	}
	switch status {
	case "succeeded", "success", "completed":
		if videoURL == "" {
			utils.Success(c, gin.H{"task_id": task.ID, "status": models.AiMediaRunning, "cost": task.Cost})
			return
		}
		rj, _ := json.Marshal(gin.H{"video": videoURL})
		models.UpdateAiMediaResult(task.ID, string(rj))
		utils.Success(c, gin.H{
			"task_id": task.ID, "status": models.AiMediaSucceeded,
			"result": task.Result, "video": videoURL, "cost": task.Cost,
		})
	case "failed", "error", "cancelled", "canceled":
		msg := upErr
		if msg == "" {
			msg = "上游任务失败"
		}
		// 已扣款任务：退款
		if task.Status == models.AiMediaRunning || task.Status == models.AiMediaPending {
			if _, rerr := models.RefundAiMediaTask(task.ID, "视频生成失败退款"); rerr == nil {
				models.FailAiMediaTask(task.ID, msg, true)
			} else {
				models.FailAiMediaTask(task.ID, msg, false)
			}
		}
		utils.Success(c, gin.H{"task_id": task.ID, "status": models.AiMediaFailed, "error": msg, "cost": task.Cost})
	default:
		utils.Success(c, gin.H{"task_id": task.ID, "status": models.AiMediaRunning, "cost": task.Cost})
	}
}

// AiHubMediaHistory GET /api/ai/media — 我的绘画/视频历史（kind=image/video）
func AiHubMediaHistory(c *gin.Context) {
	userID := currentUserID(c)
	kind := strings.TrimSpace(c.Query("kind"))
	if kind != models.AiKindImage && kind != models.AiKindVideo {
		utils.BadRequest(c, "kind 需为 image 或 video")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	list, total, err := models.ListAiMediaTasks(userID, kind, page, 20)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{"list": list, "total": total, "page": page})
}
