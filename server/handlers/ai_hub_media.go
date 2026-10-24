package handlers

import (
	"encoding/json"
	"log"
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
		// 自备密钥（§5.5 BYOK）
		Byok      bool   `json:"byok"`
		ByokModel string `json:"byok_model"`
		ByokKeyID uint   `json:"byok_key_id"`
		Confirm   bool   `json:"confirm"`
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
	// 模型解析：BYOK 用伪模型（0 费、不进平台池）
	var m *models.AiModel
	if req.Byok {
		m = &models.AiModel{ID: 0, Name: "自备密钥", PriceCall: 0}
	} else {
		mm, merr := models.GetEnabledAiModelOfKind(req.ModelID, models.AiKindVideo)
		if merr != nil || mm == nil {
			utils.BadRequest(c, "视频模型不可用")
			return
		}
		m = mm
	}
	// 免费额度优先（按次消耗 1 次）
	freeOK, _, _ := models.CheckAiFreeQuota(userID, 0)
	// 资金闸门：按次单价的单次上限与二次确认
	if !freeOK && !req.Byok && m.PriceCall > 0 {
		allowed, needConfirm, reason := models.CallGuard(m.PriceCall, req.Confirm)
		if !allowed {
			utils.BadRequest(c, reason+"（未扣款）")
			return
		}
		if needConfirm {
			utils.Error(c, 428, 42801, reason+"，请确认后重试")
			return
		}
	}
	// 非免费预检：余额 ≥ 单次价（BYOK 不扣平台代币）
	if !freeOK && !req.Byok {
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
	lease, kerr := models.AcquireModelKey(m)
	apiKey := ""
	if lease != nil {
		apiKey = lease.Key
	}
	if kerr != nil {
		models.FailAiMediaTask(task.ID, "密钥不可用", false)
		utils.InternalError(c, "模型密钥不可用，请联系站长")
		return
	}
	// BYOK 视频：用用户自备密钥提交，任务记 0 费、不扣平台代币
	var uk *models.UserAiKey
	baseURL, upModel := m.BaseURL, m.ModelID
	byok := req.Byok
	if byok {
		var uerr error
		uk, uerr = models.ResolveUserAiKey(userID, req.ByokKeyID)
		if uerr != nil || uk == nil {
			models.FailAiMediaTask(task.ID, "自备密钥不可用", false)
			utils.BadRequest(c, "自备密钥不可用：请先在「我的钱包 → 上游密钥」中添加")
			return
		}
		ukKey, derr := uk.DecryptApiKey()
		if derr != nil {
			models.FailAiMediaTask(task.ID, "自备密钥解密失败", false)
			utils.InternalError(c, "自备密钥解密失败，请删除后重新添加")
			return
		}
		baseURL, apiKey = uk.BaseURL, ukKey
		upModel = strings.TrimSpace(req.ByokModel)
		if upModel == "" {
			models.FailAiMediaTask(task.ID, "缺少上游模型标识", false)
			utils.BadRequest(c, "使用自备密钥时需填写上游模型标识")
			return
		}
	}
	client := aiclient.NewClient(baseURL, apiKey, 60*time.Second)
	remoteID, status, verr := client.CreateVideo(aiclient.VideosCreateRequest{
		Model: upModel, Prompt: prompt,
		Duration: req.Duration, Size: strings.TrimSpace(req.Size),
	})
	if verr != nil {
		if !byok {
			lease.Fail()
		}
		models.FailAiMediaTask(task.ID, verr.Error(), false)
		utils.BadRequest(c, "视频提交失败："+verr.Error()+"（未扣款）")
		return
	}
	// 提交成功：免费额度/BYOK 记零费；否则先扣款再转 running（扣款失败则任务失败）
	var nb int64
	switch {
	case byok:
		_ = models.MarkAiMediaFree(task.ID)
		nb = models.GetTokenBalance(userID)
	case freeOK:
		_ = models.MarkAiMediaFree(task.ID)
		_ = models.RecordAiFreeUsage(userID, 1, 0)
		nb = models.GetTokenBalance(userID)
	default:
		var cerr error
		nb, cerr = models.ChargeAiMediaTask(task.ID)
		if cerr != nil {
			models.FailAiMediaTask(task.ID, cerr.Error(), false)
			utils.Error(c, 429, 42902, cerr.Error()+"（未扣款，请充值后重试）")
			return
		}
	}
	if byok {
		models.TouchUserAiKey(uk.ID)
	}
	models.MarkAiMediaSubmitted(task.ID, remoteID)
	utils.Success(c, gin.H{
		"task_id":       task.ID,
		"status":        status,
		"cost":          map[bool]int64{true: 0, false: m.PriceCall}[freeOK || byok],
		"free":          freeOK && !byok,
		"byok":          byok,
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
	// 终态直接返回，不再打上游：已取消的任务若漏在这里，会被继续轮询到成功，
	// 已退的款又变回「已扣款」——既多花钱又对不上账。
	if models.IsAiMediaTerminal(task.Status) {
		utils.Success(c, gin.H{
			"task_id": task.ID, "status": task.Status,
			"result": task.Result, "error": task.Error, "cost": task.Cost,
			"refunded": task.Refunded,
		})
		return
	}
	if task.RemoteID == "" {
		utils.Success(c, gin.H{"task_id": task.ID, "status": task.Status, "error": task.Error, "cost": task.Cost})
		return
	}
	// BYOK 任务（ModelID=0）：用用户默认自备密钥轮询上游
	if task.ModelID == 0 {
		uk, uerr := models.ResolveUserAiKey(userID, 0)
		if uerr != nil || uk == nil {
			utils.BadRequest(c, "自备密钥不可用，无法查询任务状态")
			return
		}
		ukKey, derr := uk.DecryptApiKey()
		if derr != nil {
			utils.InternalError(c, "自备密钥解密失败")
			return
		}
		client := aiclient.NewClient(uk.BaseURL, ukKey, 30*time.Second)
		status, videoURL, upErr, qerr := client.QueryVideo(task.RemoteID)
		if qerr != nil {
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
			models.FailAiMediaTask(task.ID, msg, false)
			utils.Success(c, gin.H{"task_id": task.ID, "status": models.AiMediaFailed, "error": msg, "cost": task.Cost})
		default:
			utils.Success(c, gin.H{"task_id": task.ID, "status": models.AiMediaRunning, "cost": task.Cost})
		}
		return
	}
	m, merr := models.GetAiModel(task.ModelID)
	if merr != nil {
		utils.BadRequest(c, "模型已删除，无法继续查询")
		return
	}
	lease, kerr := models.AcquireModelKey(m)
	apiKey := ""
	if lease != nil {
		apiKey = lease.Key
	}
	if kerr != nil {
		utils.InternalError(c, "模型密钥不可用")
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
		// 已扣款任务退款。退款成功时 RefundAiMediaTask 自己会把状态落成 failed + refunded，
		// 所以这里不能再调 FailAiMediaTask（那会把 refunded 覆盖回 false，前端就报错账）。
		refunded := false
		if task.Cost > 0 && !task.Refunded {
			if _, rerr := models.RefundAiMediaTask(task.ID, "视频生成失败退款"); rerr == nil {
				refunded = true
			} else {
				models.FailAiMediaTask(task.ID, msg, false)
			}
		} else {
			models.FailAiMediaTask(task.ID, msg, false)
		}
		utils.Success(c, gin.H{
			"task_id": task.ID, "status": models.AiMediaFailed,
			"error": msg, "cost": task.Cost, "refunded": refunded,
		})
	default:
		utils.Success(c, gin.H{"task_id": task.ID, "status": models.AiMediaRunning, "cost": task.Cost})
	}
}

// AiHubVideoCancel POST /api/ai/video/:id/cancel — 用户主动取消视频任务
//
// 为什么需要它：视频是异步任务，一次常要几分钟。原来前端只能一直轮询到上游出结果，
// 想放弃就只能刷新页面（轮询还在后台跑），已扣的代币也拿不回来。
// 现在：立即置终态停止等待，已扣款原路退回。
//
// 上游账单说明：是否真的撤销了上游任务取决于上游是否支持（本实现会尝试一次 best-effort
// 取消），撤销不了的部分由站长承担——这一点在 UI 的确认文案里如实写明。
func AiHubVideoCancel(c *gin.Context) {
	userID := currentUserID(c)
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if id == 0 {
		utils.BadRequest(c, "任务 ID 无效")
		return
	}
	task, err := models.GetAiMediaTask(uint(id), userID)
	if err != nil {
		utils.NotFound(c, "任务不存在")
		return
	}
	if task.Kind != models.AiKindVideo {
		utils.BadRequest(c, "只有异步任务支持取消")
		return
	}
	// best-effort：上游若支持撤销就撤销，失败不影响本地退款
	if task.RemoteID != "" {
		cancelUpstreamVideo(task, userID)
	}
	refunded, cerr := models.CancelAiMediaTask(uint(id), userID)
	if cerr != nil {
		utils.BadRequest(c, cerr.Error())
		return
	}
	utils.Success(c, gin.H{
		"task_id":       task.ID,
		"status":        models.AiMediaCancelled,
		"refunded":      refunded > 0,
		"refund_amount": refunded,
		"balance_after": models.GetTokenBalance(userID),
		"message":       map[bool]string{true: "已取消，费用已退回", false: "已取消"}[refunded > 0],
	})
}

// cancelUpstreamVideo 尝试撤销上游任务（尽力而为，失败只记日志）
func cancelUpstreamVideo(task *models.AiMediaTask, userID uint) {
	var baseURL, apiKey string
	if task.ModelID == 0 {
		// BYOK 任务：用用户自己的密钥
		uk, err := models.ResolveUserAiKey(userID, 0)
		if err != nil || uk == nil {
			return
		}
		k, derr := uk.DecryptApiKey()
		if derr != nil {
			return
		}
		baseURL, apiKey = uk.BaseURL, k
	} else {
		m, merr := models.GetAiModel(task.ModelID)
		if merr != nil {
			return
		}
		lease, kerr := models.AcquireModelKey(m)
		if kerr != nil {
			return
		}
		baseURL, apiKey = m.BaseURL, lease.Key
	}
	client := aiclient.NewClient(baseURL, apiKey, 10*time.Second)
	if err := client.CancelVideo(task.RemoteID); err != nil {
		log.Printf("[ai-video] 上游取消失败（不影响本地退款）task=%d remote=%s err=%v",
			task.ID, task.RemoteID, err)
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

// ---------- 绘画协议族分发 ----------

// aspectRatioOf 把尺寸映射成 MJ 需要的 --ar 值
// mjBotTypeOf 模型标识里含 niji 即走 Niji 机器人
// generateImagesByApiKind 按上游协议族分发：
//   - mj     ：Midjourney 代理（含 Niji），同步轮询出图，单次 1 张
//   - gemini ：generateContent（nanobanana），base64 内联返回
//   - openai ：/v1/images/generations（豆包 Seedream / gpt-image-2 / 多数中转）
// sizeOfRatio 五档比例 → 上游尺寸（MJ 会再折算成 --ar）
