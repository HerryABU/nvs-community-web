package handlers

import (
	"strconv"
	"strings"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== 用户自备 API Key（§5.5：BYOK） ====================
// 密钥 AES-256-GCM 加密落库、只写不读；命中自备密钥的调用不扣平台代币。

// AiMyKeys GET /api/ai/my-keys — 我的自备密钥列表（脱敏）
func AiMyKeys(c *gin.Context) {
	if !models.IsAiEnabled() {
		utils.BadRequest(c, "AI 功能未开启")
		return
	}
	list, err := models.ListUserAiKeys(currentUserID(c))
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	out := make([]gin.H, 0, len(list))
	for i := range list {
		k := &list[i]
		out = append(out, gin.H{
			"id": k.ID, "label": k.Label, "base_url": k.BaseURL,
			"model_id": k.ModelID, "enabled": k.Enabled, "is_default": k.IsDefault,
			"has_key": k.KeyEnc != "", "last_used_at": k.LastUsedAt,
			"created_at": k.CreatedAt,
		})
	}
	utils.Success(c, gin.H{"list": out, "max": models.MaxUserAiKeys})
}

// AiMyKeyCreate POST /api/ai/my-keys — 新增自备密钥
func AiMyKeyCreate(c *gin.Context) {
	if !models.IsAiEnabled() {
		utils.BadRequest(c, "AI 功能未开启")
		return
	}
	var req struct {
		Label   string `json:"label"`
		BaseURL string `json:"base_url"`
		ApiKey  string `json:"api_key"`
		ModelID string `json:"model_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if strings.TrimSpace(req.BaseURL) == "" || strings.TrimSpace(req.ApiKey) == "" {
		utils.BadRequest(c, "上游地址与密钥均为必填")
		return
	}
	k, err := models.CreateUserAiKey(currentUserID(c), req.Label, req.BaseURL, req.ApiKey, req.ModelID)
	if err != nil {
		utils.Error(c, 429, 42903, err.Error())
		return
	}
	utils.Success(c, gin.H{"message": "已添加（密钥加密存储，仅可添加不可查看）", "id": k.ID})
}

// AiMyKeyDelete DELETE /api/ai/my-keys/:id — 删除
func AiMyKeyDelete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if id == 0 {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := models.DeleteUserAiKey(uint(id), currentUserID(c)); err != nil {
		utils.NotFound(c, err.Error())
		return
	}
	utils.Success(c, gin.H{"message": "已删除"})
}

// AiMyKeySetDefault POST /api/ai/my-keys/:id/default — 设为默认
func AiMyKeySetDefault(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if id == 0 {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := models.SetDefaultUserAiKey(uint(id), currentUserID(c)); err != nil {
		utils.NotFound(c, err.Error())
		return
	}
	utils.Success(c, gin.H{"message": "已设为默认"})
}
