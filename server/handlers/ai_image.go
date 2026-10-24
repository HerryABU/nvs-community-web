package handlers

// ==================== AI 绘画：同步出图（含协议族分发 / 混图换脸） ====================
import (
	"encoding/json"
	"strings"
	"time"

	"nvs-server/aiclient"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

func AiHubImage(c *gin.Context) {
	if !models.IsAiEnabled() {
		utils.BadRequest(c, "AI 功能未开启")
		return
	}
	userID := currentUserID(c)
	var req struct {
		ModelID   uint     `json:"model_id"`
		Prompt    string   `json:"prompt"`
		Size      string   `json:"size"`
		Ratio     string   `json:"ratio"` // 1:1 / 9:16 / 2:3 / 3:4 / 4:3（优先于 size）
		Style     string   `json:"style"`
		N         int      `json:"n"`
		Mode      string   `json:"mode"`       // t2i（默认）/ i2i 混图 / faceswap 换脸
		RefImages []string `json:"ref_images"` // data URL 参考图（最多 3 张）
		// AttachURLs 已上传的附件（站内 /uploads/… 或 http(s)），会随任务落库供历史查看与导出
		AttachURLs []string `json:"attach_urls"`
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
		utils.BadRequest(c, "请输入绘画提示词")
		return
	}
	if len([]rune(prompt)) > 4000 {
		utils.BadRequest(c, "提示词过长（上限 4000 字）")
		return
	}
	// ── 附件（参考图/素材）：只存地址不做下载（导出时才安全抓取），并校验协议 ──
	attachments := make([]models.MediaAttachment, 0, len(req.AttachURLs))
	for _, raw := range req.AttachURLs {
		u := strings.TrimSpace(raw)
		if u == "" {
			continue
		}
		low := strings.ToLower(u)
		if !strings.HasPrefix(u, "/uploads/") && !strings.HasPrefix(low, "http://") && !strings.HasPrefix(low, "https://") {
			utils.BadRequest(c, "附件地址仅支持站内 /uploads/ 或 http(s)")
			return
		}
		attachments = append(attachments, models.MediaAttachment{URL: u})
		if len(attachments) >= 6 {
			break
		}
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
	// 资金闸门：按 单价 × 张数
	if est := m.PriceCall * int64(n); est > 0 {
		allowed, needConfirm, reason := models.CallGuard(est, req.Confirm)
		if !allowed {
			utils.BadRequest(c, reason+"（未扣款）")
			return
		}
		if needConfirm {
			utils.Error(c, 428, 42801, reason+"，请确认后重试")
			return
		}
	}
	task, terr := models.CreateAiMediaTask(userID, m, models.AiKindImage, prompt, models.AiMediaParams{
		Size: size, Style: strings.TrimSpace(req.Style), N: n,
	})
	if terr != nil {
		utils.InternalError(c, "创建任务失败")
		return
	}
	if len(attachments) > 0 {
		_ = models.SetMediaAttachments(task.ID, attachments)
	}
	lease, kerr := models.AcquireModelKey(m)
	apiKey := ""
	if lease != nil {
		apiKey = lease.Key
	}
	if kerr != nil {
		models.FailAiMediaTask(task.ID, "密钥不可用", false)
		utils.InternalError(c, "模型密钥不可用（密钥池已被摘除且无兜底密钥），请联系站长")
		return
	}
	client := aiclient.NewClient(m.BaseURL, apiKey, 180*time.Second)
	urls, gerr := generateImagesByApiKind(client, m, promptForMode, size, strings.TrimSpace(req.Style), n, refs)
	if gerr != nil {
		lease.Fail()
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
	lease.OK()
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

func mjBotTypeOf(modelID string) aiclient.MJBotType {
	if strings.Contains(strings.ToLower(modelID), "niji") {
		return aiclient.NijiJourney
	}
	return aiclient.MidJourney
}

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
