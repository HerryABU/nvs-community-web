package handlers

import (
	"strconv"
	"strings"

	"nvs-server/aiclient"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== 管理端：API 卡池（多密钥轮询） ====================

func aiModelKeyVO(k *models.AiModelKey) gin.H {
	return gin.H{
		"id": k.ID, "model_id": k.ModelID, "label": k.Label,
		"weight": k.Weight, "enabled": k.Enabled, "fail_count": k.FailCount,
		"last_fail_at": k.LastFailAt, "last_used_at": k.LastUsedAt,
		// 明文永不回传；只暴露尾部 4 位便于站长辨认
		"hint": keyHint(k),
	}
}

// keyHint 为密钥生成「尾 4 位」提示（解密失败则返回空）
func keyHint(k *models.AiModelKey) string {
	plain, err := models.DecryptAiModelKey(k)
	if err != nil || len(plain) < 4 {
		return ""
	}
	return "****" + plain[len(plain)-4:]
}

// AdminListAiModelKeys GET /api/admin/ai/models/:id/keys
func AdminListAiModelKeys(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	modelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if _, err := models.GetAiModel(uint(modelID)); err != nil {
		utils.NotFound(c, "模型不存在")
		return
	}
	list, err := models.ListAiModelKeys(uint(modelID))
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	items := make([]gin.H, 0, len(list))
	for i := range list {
		items = append(items, aiModelKeyVO(&list[i]))
	}
	utils.Success(c, gin.H{
		"list": items, "total": len(items),
		"max_fail": models.MaxFailBeforeDisable,
	})
}

// AdminCreateAiModelKey POST /api/admin/ai/models/:id/keys — 支持一次多行批量导入
func AdminCreateAiModelKey(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	modelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if _, err := models.GetAiModel(uint(modelID)); err != nil {
		utils.NotFound(c, "模型不存在")
		return
	}
	var req struct {
		Label  string `json:"label"`
		Key    string `json:"key"`
		Keys   string `json:"keys"` // 批量：每行一把
		Weight int    `json:"weight"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	raw := strings.TrimSpace(req.Keys)
	var lines []string
	if raw != "" {
		for _, l := range strings.Split(raw, "\n") {
			if s := strings.TrimSpace(l); s != "" {
				lines = append(lines, s)
			}
		}
	} else if strings.TrimSpace(req.Key) != "" {
		lines = []string{strings.TrimSpace(req.Key)}
	}
	if len(lines) == 0 {
		utils.BadRequest(c, "请填写密钥")
		return
	}
	if len(lines) > 50 {
		utils.BadRequest(c, "单次最多导入 50 把密钥")
		return
	}
	added := 0
	for i, k := range lines {
		label := strings.TrimSpace(req.Label)
		if label != "" && len(lines) > 1 {
			label = label + "-" + strconv.Itoa(i+1)
		}
		if _, err := models.AddAiModelKey(uint(modelID), label, k, req.Weight); err == nil {
			added++
		}
	}
	utils.Success(c, gin.H{"message": "已添加 " + strconv.Itoa(added) + " 把密钥", "added": added})
}

// AdminUpdateAiModelKey PUT /api/admin/ai/models/:id/keys/:keyId — 权重/启停/重置失败
func AdminUpdateAiModelKey(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	modelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	keyID, _ := strconv.ParseUint(c.Param("keyId"), 10, 64)
	var req struct {
		Label   *string `json:"label"`
		Weight  *int    `json:"weight"`
		Enabled *bool   `json:"enabled"`
		Reset   bool    `json:"reset"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	fields := map[string]interface{}{}
	if req.Label != nil {
		fields["label"] = strings.TrimSpace(*req.Label)
	}
	if req.Weight != nil {
		w := *req.Weight
		if w <= 0 {
			w = 1
		}
		if w > 100 {
			w = 100
		}
		fields["weight"] = w
	}
	if req.Enabled != nil {
		fields["enabled"] = *req.Enabled
	}
	if req.Reset {
		fields["fail_count"] = 0
	}
	if err := models.UpdateAiModelKey(uint(keyID), uint(modelID), fields); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, gin.H{"message": "已保存"})
}

// AdminDeleteAiModelKey DELETE /api/admin/ai/models/:id/keys/:keyId
func AdminDeleteAiModelKey(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	modelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	keyID, _ := strconv.ParseUint(c.Param("keyId"), 10, 64)
	if err := models.DeleteAiModelKey(uint(keyID), uint(modelID)); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, gin.H{"message": "已删除"})
}

// AdminTestAiModelKey POST /api/admin/ai/models/:id/keys/test — 连通性测试
// key_id 传池内密钥 ID；不传则测试请求体里的明文（用于"保存前先测"）
func AdminTestAiModelKey(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	modelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	m, err := models.GetAiModel(uint(modelID))
	if err != nil {
		utils.NotFound(c, "模型不存在")
		return
	}
	var req struct {
		KeyID uint   `json:"key_id"`
		Key   string `json:"key"`
	}
	_ = c.ShouldBindJSON(&req)

	var plain string
	if req.KeyID > 0 {
		k, e := models.GetAiModelKey(req.KeyID, uint(modelID))
		if e != nil {
			utils.NotFound(c, "密钥不存在")
			return
		}
		plain, e = models.DecryptAiModelKey(k)
		if e != nil {
			utils.InternalError(c, "密钥解密失败")
			return
		}
	} else {
		plain = strings.TrimSpace(req.Key)
	}
	if plain == "" {
		utils.BadRequest(c, "请提供要测试的密钥")
		return
	}
	if err := aiclient.PingAuth(m.BaseURL, plain); err != nil {
		utils.Success(c, gin.H{"ok": false, "message": err.Error()})
		return
	}
	// 池内密钥测试成功即清零失败计数
	if req.KeyID > 0 {
		models.ReportAiModelKey(req.KeyID, true)
	}
	utils.Success(c, gin.H{"ok": true, "message": "连通正常"})
}

// ==================== 管理端：扣子默认配置 ====================

// AdminGetAiCozeDefault GET /api/admin/ai/coze-default
func AdminGetAiCozeDefault(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	utils.Success(c, models.GetAiCozeDefault())
}

// AdminSetAiCozeDefault PUT /api/admin/ai/coze-default
func AdminSetAiCozeDefault(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req models.AiCozeDefault
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := models.SetAiCozeDefault(req); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.Success(c, gin.H{"message": "已保存（新会话生效）"})
}
