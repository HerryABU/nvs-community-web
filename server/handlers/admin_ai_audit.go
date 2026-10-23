package handlers

import (
	"strconv"
	"strings"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== AI 管理后台：审计 / 功能开关 / 厂商字典 ====================

// AdminAiAuditStats GET /api/admin/ai/audit/stats — 概览
func AdminAiAuditStats(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	utils.Success(c, models.AiAuditStats())
}

// AdminAiAuditSessions GET /api/admin/ai/audit/sessions?user_id=&page=
func AdminAiAuditSessions(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	uid, _ := strconv.ParseUint(c.Query("user_id"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	list, total, err := models.AdminListAiSessions(uint(uid), page, 20)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	items := make([]gin.H, 0, len(list))
	for i := range list {
		s := &list[i]
		msgCount, _ := models.CountAiChatMessages(s.ID)
		items = append(items, gin.H{
			"id": s.ID, "user_id": s.UserID, "title": s.Title,
			"agent_id": s.AgentID, "agent_name": s.AgentName,
			"model_id": s.ModelID, "messages": msgCount,
			"shared": s.ShareToken != nil, "coze_conv": s.ThirdConvID != "",
			"created_at": s.CreatedAt, "updated_at": s.UpdatedAt,
		})
	}
	utils.Success(c, gin.H{"list": items, "total": total, "page": page})
}

// AdminAiAuditSessionMessages GET /api/admin/ai/audit/sessions/:id/messages
func AdminAiAuditSessionMessages(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var s models.AiChatSession
	if err := models.DB.Where("id = ?", id).First(&s).Error; err != nil {
		utils.NotFound(c, "会话不存在")
		return
	}
	msgs, err := models.ListAiChatMessages(s.ID, 500)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{"session": s, "messages": msgs})
}

// AdminAiAuditMedia GET /api/admin/ai/audit/media?kind=&status=&page=
func AdminAiAuditMedia(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	list, total, err := models.AdminListAiMedia(c.Query("kind"), c.Query("status"), page, 20)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	items := make([]gin.H, 0, len(list))
	for i := range list {
		t := &list[i]
		items = append(items, gin.H{
			"id": t.ID, "user_id": t.UserID, "kind": t.Kind, "model_name": t.ModelName,
			"prompt": t.Prompt, "status": t.Status, "cost": t.Cost, "error": t.Error,
			"is_public": t.IsPublic, "title": t.Title, "style": t.Style, "likes": t.Likes,
			"result": t.Result, "created_at": t.CreatedAt,
		})
	}
	utils.Success(c, gin.H{"list": items, "total": total, "page": page})
}

// AdminAiAuditCalls GET /api/admin/ai/audit/calls?status=&page=
func AdminAiAuditCalls(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	list, total, err := models.AdminListAiCallLogs(c.Query("status"), page, 20)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{"list": list, "total": total, "page": page})
}

// AdminGetAiFeatures GET /api/admin/ai/features — AI 功能开关
func AdminGetAiFeatures(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	utils.Success(c, models.GetAiFeatureFlags())
}

// AdminSetAiFeatures PUT /api/admin/ai/features
func AdminSetAiFeatures(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req models.AiFeatureFlags
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := models.SetAiFeatureFlags(req); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.Success(c, gin.H{"message": "已保存（立即生效）"})
}

// ---------- 厂商字典 ----------

// AdminListAiVendors GET /api/admin/ai/vendors
func AdminListAiVendors(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	list, err := models.ListAiVendors(false)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	// 附带每个厂商下的模型数，便于判断能否删除
	type row struct {
		models.AiVendor
		Models int64 `json:"models"`
	}
	out := make([]row, 0, len(list))
	for _, v := range list {
		var n int64
		models.DB.Model(&models.AiModel{}).Where("vendor = ?", v.Name).Count(&n)
		out = append(out, row{AiVendor: v, Models: n})
	}
	utils.Success(c, gin.H{"list": out})
}

// AdminCreateAiVendor POST /api/admin/ai/vendors
func AdminCreateAiVendor(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		Name      string `json:"name"`
		SortOrder int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	v, err := models.CreateAiVendor(req.Name, req.SortOrder)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, v)
}

// AdminUpdateAiVendor PUT /api/admin/ai/vendors/:id
func AdminUpdateAiVendor(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Name      string `json:"name"`
		SortOrder int    `json:"sort_order"`
		Enabled   *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if err := models.UpdateAiVendor(uint(id), strings.TrimSpace(req.Name), req.SortOrder, enabled); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, gin.H{"message": "已保存"})
}

// AdminDeleteAiVendor DELETE /api/admin/ai/vendors/:id
func AdminDeleteAiVendor(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := models.DeleteAiVendor(uint(id)); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, gin.H{"message": "已删除"})
}
