package handlers

import (
	"strconv"
	"strings"

	"nvs-server/aiapi/vendor"
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
		Prefix    string `json:"prefix"`
		SortOrder int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	v, err := models.CreateAiVendor(req.Name, req.SortOrder, req.Prefix)
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
		Prefix    string `json:"prefix"`
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
	if err := models.UpdateAiVendor(uint(id), strings.TrimSpace(req.Name), req.SortOrder, enabled, req.Prefix); err != nil {
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

// ---------- 聚合分类定价（§厂商管理：按 chat/image/video 设价） ----------

// AdminGetCategoryPrices GET /api/admin/ai/category-prices — 3 种聚合分类的默认单价
func AdminGetCategoryPrices(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	list, err := models.AllCategoryPrices()
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{"list": list})
}

// AdminSetCategoryPrice PUT /api/admin/ai/category-prices — 保存某分类默认价
func AdminSetCategoryPrice(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		Kind      string `json:"kind"`
		PriceIn   int64  `json:"price_in"`
		PriceOut  int64  `json:"price_out"`
		PriceCall int64  `json:"price_call"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	req.Kind = models.NormalizeAiKind(req.Kind)
	if req.Kind != models.AiKindChat && req.Kind != models.AiKindImage && req.Kind != models.AiKindVideo {
		utils.BadRequest(c, "kind 必须是 chat/image/video 之一")
		return
	}
	if req.PriceIn < 0 || req.PriceOut < 0 || req.PriceCall < 0 {
		utils.BadRequest(c, "价格不能为负")
		return
	}
	if err := models.SetCategoryPrice(req.Kind, req.PriceIn, req.PriceOut, req.PriceCall); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.Success(c, gin.H{"message": "已保存"})
}

// ---------- 厂商→版本→亚版本矩阵（管理端专用，含 newest 标记） ----------

// AdminAiVendorMatrix GET /api/admin/ai/vendors/matrix
// 把模型按「厂商→版本→亚版本」三级聚合，并在同一版本下标出最新亚版本（newest）。
// 该「谁最新」比较仅管理端可见，用户侧只看厂商主名字（不暴露亚版本噪声）。
func AdminAiVendorMatrix(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	vendors, err := models.ListAiVendors(false)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	allModels, err := models.ListAiModels(false)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	// 按厂商名归并模型（空厂商名归到「未分类」）
	byVendor := map[string][]models.AiModel{}
	for i := range allModels {
		name := allModels[i].Vendor
		if name == "" {
			name = "未分类"
		}
		byVendor[name] = append(byVendor[name], allModels[i])
	}
	out := make([]gin.H, 0, len(vendors))
	for _, v := range vendors {
		group := byVendor[v.Name]
		// 按版本分组（空版本用占位符）
		byVer := map[string][]models.AiModel{}
		for _, m := range group {
			ver := m.Version
			if ver == "" {
				ver = "—"
			}
			byVer[ver] = append(byVer[ver], m)
		}
		versions := make([]gin.H, 0, len(byVer))
		for ver, items := range byVer {
			subs := make([]string, 0, len(items))
			for _, m := range items {
				if m.SubVersion != "" {
					subs = append(subs, m.SubVersion)
				}
			}
			newest := ""
			if len(subs) > 0 {
				newest = vendor.NewestSubVersion(subs)
			}
			subItems := make([]gin.H, 0, len(items))
			for _, m := range items {
				uv := m.UserVisible == nil || *m.UserVisible
				subItems = append(subItems, gin.H{
					"id": m.ID, "name": m.Name, "model_id": m.ModelID,
					"sub_version": m.SubVersion, "newest": m.SubVersion == newest,
					"enabled": m.Enabled, "user_visible": uv,
				})
			}
			versions = append(versions, gin.H{
				"version": ver, "newest_sub_version": newest,
				"count": len(items), "items": subItems,
			})
		}
		out = append(out, gin.H{
			"id": v.ID, "name": v.Name, "prefix": v.Prefix, "enabled": v.Enabled,
			"models": len(group), "versions": versions,
		})
	}
	// 未被厂商字典覆盖的模型（厂商名不在字典里）单独成组，避免遗漏
	for name, group := range byVendor {
		found := false
		for _, v := range vendors {
			if v.Name == name {
				found = true
				break
			}
		}
		if found {
			continue
		}
		byVer := map[string][]models.AiModel{}
		for _, m := range group {
			ver := m.Version
			if ver == "" {
				ver = "—"
			}
			byVer[ver] = append(byVer[ver], m)
		}
		versions := make([]gin.H, 0, len(byVer))
		for ver, items := range byVer {
			subs := make([]string, 0, len(items))
			for _, m := range items {
				if m.SubVersion != "" {
					subs = append(subs, m.SubVersion)
				}
			}
			newest := ""
			if len(subs) > 0 {
				newest = vendor.NewestSubVersion(subs)
			}
			subItems := make([]gin.H, 0, len(items))
			for _, m := range items {
				uv := m.UserVisible == nil || *m.UserVisible
				subItems = append(subItems, gin.H{
					"id": m.ID, "name": m.Name, "model_id": m.ModelID,
					"sub_version": m.SubVersion, "newest": m.SubVersion == newest,
					"enabled": m.Enabled, "user_visible": uv,
				})
			}
			versions = append(versions, gin.H{
				"version": ver, "newest_sub_version": newest,
				"count": len(items), "items": subItems,
			})
		}
		out = append(out, gin.H{
			"id": 0, "name": name, "prefix": "", "enabled": true,
			"models": len(group), "versions": versions,
		})
	}
	utils.Success(c, gin.H{"vendors": out})
}
