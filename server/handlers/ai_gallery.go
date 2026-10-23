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

// ==================== 作品广场（灵感广场） ====================

// galleryItem 公开作品视图（不含 cost/user_id 等隐私字段）
func galleryItem(t *models.AiMediaTask) gin.H {
	images := []string{}
	video := ""
	var payload map[string]interface{}
	if json.Unmarshal([]byte(t.Result), &payload) == nil {
		if arr, ok := payload["images"].([]interface{}); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok {
					images = append(images, s)
				}
			}
		}
		if s, ok := payload["video"].(string); ok {
			video = s
		}
	}
	return gin.H{
		"id": t.ID, "kind": t.Kind, "title": t.Title, "prompt": t.Prompt,
		"model_name": t.ModelName, "style": t.Style, "likes": t.Likes,
		"images": images, "video": video, "created_at": t.CreatedAt,
	}
}

// AiGalleryList GET /api/ai/gallery?kind=image|video&style=&page= — 公开作品流
func AiGalleryList(c *gin.Context) {
	// 开关：关闭「公开」后仅登录用户可见
	if !models.GetAiFeatureFlags().PublicGallery && currentUserID(c) == 0 {
		utils.BadRequest(c, "灵感广场仅登录可见")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	kind := strings.TrimSpace(c.Query("kind"))
	style := strings.TrimSpace(c.Query("style"))
	list, total, err := models.ListPublicMedia(kind, style, page, 24)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	items := make([]gin.H, 0, len(list))
	for i := range list {
		items = append(items, galleryItem(&list[i]))
	}
	utils.Success(c, gin.H{
		"list": items, "total": total, "page": page,
		"styles": models.GalleryStyles(kind),
	})
}

// AiGalleryPublish POST /api/ai/gallery/:id/publish — 发布/取消发布我的作品
func AiGalleryPublish(c *gin.Context) {
	userID := currentUserID(c)
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Public bool   `json:"public"`
		Title  string `json:"title"`
		Style  string `json:"style"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := models.PublishAiMedia(uint(id), userID, req.Public, req.Title, req.Style); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, gin.H{"message": map[bool]string{true: "已发布到灵感广场", false: "已取消发布"}[req.Public]})
}

// AiGalleryLike POST /api/ai/gallery/:id/like — 点赞
func AiGalleryLike(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := models.LikeAiMedia(uint(id)); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, gin.H{"message": "ok"})
}

// AiMyMediaPublic GET /api/ai/media/:id/public — 查我的作品发布状态（发布弹窗回填）
func AiMyMediaPublic(c *gin.Context) {
	userID := currentUserID(c)
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	t, err := models.GetAiMediaTask(uint(id), userID)
	if err != nil {
		utils.NotFound(c, "作品不存在")
		return
	}
	utils.Success(c, gin.H{"id": t.ID, "is_public": t.IsPublic, "title": t.Title, "style": t.Style})
}

// ==================== 思维导图 ====================

// AiMindmap POST /api/ai/mindmap — 需求 → 层级大纲（Markdown），前端渲染为导图
func AiMindmap(c *gin.Context) {
	if !models.IsAiEnabled() {
		utils.BadRequest(c, "AI 功能未开启")
		return
	}
	userID := currentUserID(c)
	var req struct {
		Topic   string `json:"topic"`
		Depth   int    `json:"depth"`
		ModelID uint   `json:"model_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	topic := strings.TrimSpace(req.Topic)
	if topic == "" {
		utils.BadRequest(c, "请输入需求描述")
		return
	}
	if len([]rune(topic)) > 2000 {
		utils.BadRequest(c, "描述过长（上限 2000 字）")
		return
	}
	depth := req.Depth
	if depth < 2 || depth > 4 {
		depth = 3
	}
	var m *models.AiModel
	if req.ModelID > 0 {
		mm, err := models.GetEnabledAiModelOfKind(req.ModelID, models.AiKindChat)
		if err != nil || mm == nil {
			utils.BadRequest(c, "模型不可用")
			return
		}
		m = mm
	} else {
		list, err := models.ListAiModelsByKind(models.AiKindChat, true)
		if err != nil || len(list) == 0 {
			utils.BadRequest(c, "暂无可用对话模型")
			return
		}
		m = &list[0]
	}
	systemPrompt := "你是思维导图结构专家。请把用户需求整理为层级化 Markdown 大纲：" +
		"第一行用 `# ` 作为中心主题（不超过 20 字），随后用 `## ` 表示一级分支、" +
		"`### ` 表示二级、`- ` 表示叶子要点。层级不超过 " + strconv.Itoa(depth) +
		" 级，每个叶子要点不超过 18 字，只输出大纲，不要任何解释或代码块围栏。"
	messages := []aiclient.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: topic},
	}
	tokensIn := models.EstimateTokens(systemPrompt) + models.EstimateTokens(topic)
	tokensOut := 1200
	freeOK, _, _ := models.CheckAiFreeQuota(userID, tokensIn+tokensOut)
	if !freeOK {
		if !models.IsTokenEnabled() {
			utils.BadRequest(c, "代币系统未开启，无法使用付费 AI")
			return
		}
		if _, ok := models.PrecheckAiBalance(userID, m, tokensIn, tokensOut); !ok {
			utils.Error(c, 429, 42902, "余额不足，请先充值")
			return
		}
	}
	callLog, lerr := models.CreateAiCallLog(userID, m, "chat", "mindmap", 0, tokensIn, tokensOut, freeOK)
	if lerr != nil {
		utils.InternalError(c, "记录调用失败")
		return
	}
	apiKey, kerr := m.DecryptApiKey()
	if kerr != nil {
		models.FailAiCallLog(callLog.ID, "密钥解密失败")
		utils.InternalError(c, "模型密钥解密失败，请联系站长")
		return
	}
	client := aiclient.NewClient(m.BaseURL, apiKey, 120*time.Second)
	chatReq := aiclient.ChatRequest{Model: m.ModelID, Messages: messages, MaxTokens: tokensOut, Temperature: 0.4}
	var out string
	var realIn, realOut int
	if models.NormalizeAiApiKind(m.ApiKind) == models.AiApiClaude {
		out, realIn, realOut, kerr = client.CompleteClaude(chatReq)
	} else {
		out, realIn, realOut, kerr = client.Complete(chatReq)
	}
	if kerr != nil {
		models.FailAiCallLog(callLog.ID, kerr.Error())
		utils.BadRequest(c, "生成失败："+kerr.Error()+"（未扣款）")
		return
	}
	if realIn <= 0 {
		realIn = tokensIn
	}
	if realOut <= 0 {
		realOut = models.EstimateTokens(out)
	}
	cost, serr := models.SettleAiCallLog(callLog.ID, realIn, realOut)
	if serr != nil {
		models.FailAiCallLog(callLog.ID, serr.Error())
	}
	nb := models.GetTokenBalance(userID)
	utils.Success(c, gin.H{
		"outline": strings.TrimSpace(out),
		"cost":    cost, "balance_after": nb, "free": freeOK,
		"tokens_in": realIn, "tokens_out": realOut,
	})
}
