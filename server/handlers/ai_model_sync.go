package handlers

import (
	"encoding/json"
	"strconv"

	"nvs-server/aiapi"
	"nvs-server/aiapi/vendor"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// AdminPullAiModels POST /api/admin/ai/models/pull
// 从 OpenAI 兼容渠道一次性拉取全部模型，返回服务端推断的分类标签，站长按需勾选导入。
func AdminPullAiModels(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		BaseURL string `json:"base_url" binding:"required"`
		APIKey  string `json:"api_key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	sugs, err := aiapi.PullProviderModels(req.BaseURL, req.APIKey)
	if err != nil {
		utils.InternalError(c, "拉取失败："+err.Error())
		return
	}
	utils.Success(c, gin.H{"items": sugs})
}

// AdminImportAiModels POST /api/admin/ai/models/import
// 批量导入从渠道拉取的模型（站长勾选的子集）。默认对用户隐藏，由站长按需开放。
func AdminImportAiModels(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		BaseURL string `json:"base_url" binding:"required"`
		APIKey  string `json:"api_key"`
		Channel string `json:"channel"`
		Items   []struct {
			RawID       string `json:"raw_id" binding:"required"`
			Alias       string `json:"alias"`
			Vendor      string `json:"vendor"`
			Category    string `json:"category"`
			UserVisible bool   `json:"user_visible"`
		} `json:"items" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if len(req.Items) == 0 {
		utils.BadRequest(c, "未选择任何模型")
		return
	}
	created := 0
	for _, it := range req.Items {
		category := it.Category
		if category == "" {
			category = "chat"
		}
		kind := models.AiKindChat
		switch category {
		case "image":
			kind = models.AiKindImage
		case "video":
			kind = models.AiKindVideo
		}
		alias := it.Alias
		if alias == "" {
			alias = it.RawID
		}
		uv := it.UserVisible
		// 解析「前缀-版本-亚版本」（如 deepseek-v3.1-turbo）→ 落库 version/sub_version，
		// 便于管理端按版本矩阵比较「谁最新」；用户侧始终只看主名字（不暴露亚版本噪声）。
		parsed := vendor.ParseModelID(it.RawID)
		m := &models.AiModel{
			Name:         alias,
			Kind:         kind,
			ModelID:      it.RawID,
			BaseURL:      req.BaseURL,
			ApiKind:      models.AiApiOpenAI,
			Vendor:       it.Vendor,
			Channel:      req.Channel,
			Alias:        alias,
			CategoryTags: mustJSONStr([]string{category}),
			UserVisible:  &uv,
			Enabled:      true,
			MaxTokens:    2048,
			Temperature:  0.7,
			Version:      parsed.Version,
			SubVersion:   parsed.SubVersion,
		}
		// 未显式给价时套用分类默认价（管理员在「聚合分类定价」里维护）
		if m.PriceIn == 0 && m.PriceOut == 0 && m.PriceCall == 0 {
			if cp := models.GetCategoryPrice(kind); cp.Kind != "" {
				m.PriceIn = cp.PriceIn
				m.PriceOut = cp.PriceOut
				m.PriceCall = cp.PriceCall
			}
		}
		if err := models.CreateAiModel(m, req.APIKey); err != nil {
			continue
		}
		created++
	}
	utils.Success(c, gin.H{"created": created})
}

// AdminSetModelVisibility POST /api/admin/ai/models/:id/visibility
// 开放/收回某模型对用户的可见性（站长按需选择开放哪些渠道/模型，严禁硬编码名单）。
func AdminSetModelVisibility(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		UserVisible bool `json:"user_visible"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	uv := req.UserVisible
	if err := models.DB.Model(&models.AiModel{}).Where("id = ?", id).
		Updates(map[string]interface{}{"user_visible": &uv}).Error; err != nil {
		utils.InternalError(c, "操作失败")
		return
	}
	utils.Success(c, gin.H{"message": "已更新可见性"})
}

// AdminMergeModels POST /api/admin/ai/models/merge
// 按渠道合并：将指定模型归入同一渠道分组（前端按渠道聚合展示给用户）。
func AdminMergeModels(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		IDs     []uint `json:"ids" binding:"required"`
		Channel string `json:"channel" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := models.DB.Model(&models.AiModel{}).Where("id IN ?", req.IDs).
		Update("channel", req.Channel).Error; err != nil {
		utils.InternalError(c, "合并失败")
		return
	}
	utils.Success(c, gin.H{"message": "已合并到渠道 " + req.Channel})
}

// AdminSplitModels POST /api/admin/ai/models/split
// 拆分：清除指定模型的渠道分组，各自独立展示。
func AdminSplitModels(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		IDs []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := models.DB.Model(&models.AiModel{}).Where("id IN ?", req.IDs).
		Update("channel", "").Error; err != nil {
		utils.InternalError(c, "拆分失败")
		return
	}
	utils.Success(c, gin.H{"message": "已拆分（清除渠道分组）"})
}

// mustJSONStr 把字符串切片序列化为 JSON 数组字符串（分类标签落库）。
func mustJSONStr(s []string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return "[]"
	}
	return string(b)
}
