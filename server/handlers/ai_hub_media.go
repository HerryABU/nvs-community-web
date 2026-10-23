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
		Ratio   string `json:"ratio"` // 1:1 / 9:16 / 2:3 / 3:4 / 4:3（优先于 size）
		Style   string `json:"style"`
		N       int    `json:"n"`
		Mode    string   `json:"mode"`       // t2i（默认）/ i2i 混图 / faceswap 换脸
		RefImages []string `json:"ref_images"` // data URL 参考图（最多 3 张）
		// 自备密钥（§5.5 BYOK）
		Byok      bool   `json:"byok"`
		ByokModel string `json:"byok_model"`
		ByokKeyID uint   `json:"byok_key_id"`
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
	// ── 参考图（混图/换脸）：仅接受上传的 data URL，绝不拉外链（防 SSRF） ──
	refs, rerr := aiclient.ImageRefsFromDataURLs(req.RefImages)
	if rerr != nil {
		utils.BadRequest(c, rerr.Error())
		return
	}
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = "t2i"
	}
	flags := models.GetAiFeatureFlags()
	switch mode {
	case "i2i":
		if !flags.AllowI2I {
			utils.BadRequest(c, "站长已关闭「混图」功能")
			return
		}
		if len(refs) == 0 {
			utils.BadRequest(c, "混图需至少上传 1 张参考图")
			return
		}
	case "faceswap":
		if !flags.AllowFaceSwap {
			utils.BadRequest(c, "站长已关闭「换脸」功能")
			return
		}
		if len(refs) < 2 {
			utils.BadRequest(c, "换脸需上传 2 张图（第 1 张目标人物、第 2 张面部来源）")
			return
		}
	}
	// 比例优先：把 1:1 / 9:16 / 2:3 / 3:4 / 4:3 映射成上游 size
	size := strings.TrimSpace(req.Size)
	if r := strings.TrimSpace(req.Ratio); r != "" {
		size = sizeOfRatio(r)
	}
	promptForMode := prompt
	if mode == "faceswap" {
		promptForMode = "保持第 1 张图的人物、姿势、光线与构图不变，将面部替换为第 2 张图中的人物面部，自然融合肤色与质感。" +
			"额外要求：" + prompt
	} else if mode == "i2i" {
		promptForMode = "参考所给图片的主体、风格与配色进行再创作。" + prompt
	}

	// BYOK：用户自备密钥（伪模型，不计平台费用）
	if req.Byok {
		byokModel := strings.TrimSpace(req.ByokModel)
		if byokModel == "" {
			utils.BadRequest(c, "使用自备密钥时需填写上游模型标识")
			return
		}
		uk, uerr := models.ResolveUserAiKey(userID, req.ByokKeyID)
		if uerr != nil {
			utils.InternalError(c, "读取自备密钥失败")
			return
		}
		if uk == nil {
			utils.BadRequest(c, "自备密钥不可用：请先在「我的钱包 → 上游密钥」中添加")
			return
		}
		apiKey, kerr := uk.DecryptApiKey()
		if kerr != nil {
			utils.InternalError(c, "自备密钥解密失败，请删除后重新添加")
			return
		}
		n := req.N
		if n < 1 || n > 4 {
			n = 1
		}
		client := aiclient.NewClient(uk.BaseURL, apiKey, 180*time.Second)
		var urls []string
		var gerr error
		if len(refs) > 0 {
			urls, gerr = client.EditImages(byokModel, promptForMode, size, refs, n)
		} else {
			urls, gerr = client.GenerateImages(aiclient.ImagesRequest{
				Model: byokModel, Prompt: prompt, N: n,
				Size: size, ResponseFormat: "url",
			})
		}
		if gerr != nil {
			utils.BadRequest(c, "绘画失败："+gerr.Error()+"（未扣款）")
			return
		}
		models.TouchUserAiKey(uk.ID)
		utils.Success(c, gin.H{
			"task_id":       0,
			"images":        urls,
			"cost":          0,
			"free":          false,
			"byok":          true,
			"balance_after": models.GetTokenBalance(userID),
		})
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
		Size: size, Style: strings.TrimSpace(req.Style), N: n,
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
	urls, gerr := generateImagesByApiKind(client, m, promptForMode, size, strings.TrimSpace(req.Style), n, refs)
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
		// 自备密钥（§5.5 BYOK）
		Byok      bool   `json:"byok"`
		ByokModel string `json:"byok_model"`
		ByokKeyID uint   `json:"byok_key_id"`
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
	apiKey, kerr := m.DecryptApiKey()
	if kerr != nil {
		models.FailAiMediaTask(task.ID, "密钥解密失败", false)
		utils.InternalError(c, "模型密钥解密失败，请联系站长")
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

// ---------- 绘画协议族分发 ----------

// aspectRatioOf 把尺寸映射成 MJ 需要的 --ar 值
func aspectRatioOf(size string) string {
	switch strings.ToLower(strings.TrimSpace(size)) {
	case "1024x1792", "9:16", "2:3", "portrait":
		return "2:3"
	case "1792x1024", "16:9", "3:2", "landscape":
		return "3:2"
	default:
		return "1:1"
	}
}

// mjBotTypeOf 模型标识里含 niji 即走 Niji 机器人
func mjBotTypeOf(modelID string) aiclient.MJBotType {
	if strings.Contains(strings.ToLower(modelID), "niji") {
		return aiclient.NijiJourney
	}
	return aiclient.MidJourney
}

// generateImagesByApiKind 按上游协议族分发：
//   - mj     ：Midjourney 代理（含 Niji），同步轮询出图，单次 1 张
//   - gemini ：generateContent（nanobanana），base64 内联返回
//   - openai ：/v1/images/generations（豆包 Seedream / gpt-image-2 / 多数中转）
func generateImagesByApiKind(client *aiclient.Client, m *models.AiModel, prompt, size, style string, n int, refs []aiclient.ImgRef) ([]string, error) {
	switch models.NormalizeAiApiKind(m.ApiKind) {
	case models.AiApiMJ:
		p := prompt
		if style != "" {
			p += " --style " + style
		}
		p += " --ar " + aspectRatioOf(size)
		return client.MJImagineMulti(aiclient.MJImagineParams{
			Prompt:  p,
			BotType: mjBotTypeOf(m.ModelID),
		}, refs, 180*time.Second)
	case models.AiApiGemini:
		p := prompt
		if style != "" {
			p += "\n\nstyle: " + style
		}
		if len(refs) > 0 {
			imgs, _, err := client.GeminiImageWithRefs(m.ModelID, p, size, refs)
			return imgs, err
		}
		imgs, _, err := client.GeminiImage(m.ModelID, p, size)
		return imgs, err
	default:
		if len(refs) > 0 {
			return client.EditImages(m.ModelID, prompt, size, refs, n)
		}
		return client.GenerateImages(aiclient.ImagesRequest{
			Model: m.ModelID, Prompt: prompt, N: n,
			Size: size, ResponseFormat: "url",
		})
	}
}

// sizeOfRatio 五档比例 → 上游尺寸（MJ 会再折算成 --ar）
func sizeOfRatio(r string) string {
	switch strings.TrimSpace(r) {
	case "9:16":
		return "1024x1792"
	case "2:3":
		return "1024x1536"
	case "3:4":
		return "1024x1365"
	case "4:3":
		return "1365x1024"
	case "16:9", "3:2":
		return "1792x1024"
	default:
		return "1024x1024"
	}
}
