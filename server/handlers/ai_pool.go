package handlers

import (
	"strconv"
	"strings"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== AI 模型池管理端（§5.3） ====================
// api_key 加密落库、只写不读：列表/详情不回传密钥，仅回传 has_api_key；
// 更新时留空表示不轮换。

// aiModelVO 列表视图（密钥脱敏）
func aiModelVO(m *models.AiModel) gin.H {
	return gin.H{
		"id":          m.ID,
		"name":        m.Name,
		"model_id":    m.ModelID,
		"base_url":    m.BaseURL,
		"has_api_key": m.ApiKeyEnc != "",
		"price_in":    m.PriceIn,
		"price_out":   m.PriceOut,
		"max_tokens":  m.MaxTokens,
		"temperature": m.Temperature,
		"enabled":     m.Enabled,
		"priority":    m.Priority,
		"note":        m.Note,
		"created_at":  m.CreatedAt,
		"updated_at":  m.UpdatedAt,
	}
}

// AdminGetAiConfig GET /api/admin/ai/config
func AdminGetAiConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	utils.Success(c, gin.H{"enabled": models.IsAiEnabled()})
}

// AdminSetAiConfig PUT /api/admin/ai/config
func AdminSetAiConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if req.Enabled == nil {
		utils.BadRequest(c, "缺少 enabled 字段")
		return
	}
	if err := models.SetAiEnabled(*req.Enabled); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.Success(c, gin.H{"message": "已保存（立即生效）"})
}

// AdminListAiModels GET /api/admin/ai/models
func AdminListAiModels(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	list, err := models.ListAiModels(false)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	items := make([]gin.H, 0, len(list))
	for i := range list {
		items = append(items, aiModelVO(&list[i]))
	}
	utils.Success(c, gin.H{"list": items, "total": len(items)})
}

// aiModelInput 创建/更新共用入参
type aiModelInput struct {
	Name        string  `json:"name"`
	ModelID     string  `json:"model_id"`
	BaseURL     string  `json:"base_url"`
	ApiKey      string  `json:"api_key"` // 留空=不轮换
	PriceIn     *int64  `json:"price_in"`
	PriceOut    *int64  `json:"price_out"`
	MaxTokens   *int    `json:"max_tokens"`
	Temperature *float64 `json:"temperature"`
	Enabled     *bool   `json:"enabled"`
	Priority    *int    `json:"priority"`
	Note        string  `json:"note"`
}

// AdminCreateAiModel POST /api/admin/ai/models
func AdminCreateAiModel(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req aiModelInput
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	name := strings.TrimSpace(req.Name)
	modelID := strings.TrimSpace(req.ModelID)
	baseURL := strings.TrimSpace(req.BaseURL)
	if name == "" || modelID == "" || baseURL == "" {
		utils.BadRequest(c, "展示名、模型标识、上游地址均为必填")
		return
	}
	m := &models.AiModel{
		Name: name, ModelID: modelID, BaseURL: baseURL,
		Note: strings.TrimSpace(req.Note),
	}
	if req.PriceIn != nil {
		m.PriceIn = *req.PriceIn
	}
	if req.PriceOut != nil {
		m.PriceOut = *req.PriceOut
	}
	if req.MaxTokens != nil {
		m.MaxTokens = *req.MaxTokens
	}
	if req.Temperature != nil {
		m.Temperature = *req.Temperature
	}
	if req.Enabled != nil {
		m.Enabled = *req.Enabled
	}
	if req.Priority != nil {
		m.Priority = *req.Priority
	}
	if err := models.CreateAiModel(m, req.ApiKey); err != nil {
		utils.InternalError(c, "创建失败")
		return
	}
	utils.Success(c, gin.H{"message": "已创建", "id": m.ID})
}

// AdminUpdateAiModel PUT /api/admin/ai/models/:id
func AdminUpdateAiModel(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if _, err := models.GetAiModel(uint(id)); err != nil {
		utils.NotFound(c, "模型不存在")
		return
	}
	var req aiModelInput
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	fields := map[string]interface{}{}
	if s := strings.TrimSpace(req.Name); s != "" {
		fields["name"] = s
	}
	if s := strings.TrimSpace(req.ModelID); s != "" {
		fields["model_id"] = s
	}
	if s := strings.TrimSpace(req.BaseURL); s != "" {
		fields["base_url"] = s
	}
	if req.PriceIn != nil {
		fields["price_in"] = *req.PriceIn
	}
	if req.PriceOut != nil {
		fields["price_out"] = *req.PriceOut
	}
	if req.MaxTokens != nil {
		fields["max_tokens"] = *req.MaxTokens
	}
	if req.Temperature != nil {
		fields["temperature"] = *req.Temperature
	}
	if req.Enabled != nil {
		fields["enabled"] = *req.Enabled
	}
	if req.Priority != nil {
		fields["priority"] = *req.Priority
	}
	fields["note"] = strings.TrimSpace(req.Note)
	if err := models.UpdateAiModelFields(uint(id), fields, req.ApiKey); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.Success(c, gin.H{"message": "已保存（立即生效）"})
}

// AdminDeleteAiModel DELETE /api/admin/ai/models/:id
func AdminDeleteAiModel(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := models.DeleteAiModel(uint(id)); err != nil {
		utils.InternalError(c, "删除失败")
		return
	}
	utils.Success(c, gin.H{"message": "已删除"})
}
