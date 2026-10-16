// home_block.go — 自定义主页区块管理（F15）。
// 管理员：区块 CRUD / 排序 / 启停 / 一键套用布局预设；
// 公开：仅返回启用区块（自定义 HTML 已在保存时服务端消毒）。
package handlers

import (
	"strconv"

	"nvs-server/models"
	"nvs-server/security"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ListPublicHomeBlocks GET /api/home-blocks — 首页区块（仅启用，按排序）
func ListPublicHomeBlocks(c *gin.Context) {
	blocks, err := models.ListEnabledHomeBlocks()
	if err != nil {
		utils.InternalError(c, "获取首页区块失败")
		return
	}
	// 空库时播种默认布局（混合一条龙），保证老部署升级后首页不空
	if len(blocks) == 0 && models.CountHomeBlocks() == 0 {
		if err := models.SeedDefaultHomeBlocks(); err == nil {
			blocks, _ = models.ListEnabledHomeBlocks()
		}
	}
	utils.Success(c, gin.H{"items": blocks})
}

// AdminListHomeBlocks GET /api/admin/home-blocks — 全部区块（含禁用）
func AdminListHomeBlocks(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	blocks, err := models.ListHomeBlocks()
	if err != nil {
		utils.InternalError(c, "获取区块失败")
		return
	}
	utils.Success(c, gin.H{"items": blocks})
}

// AdminListHomeBlockPresets GET /api/admin/home-blocks/presets — 布局预设列表
func AdminListHomeBlockPresets(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	presets := models.HomeBlockPresets()
	out := make([]gin.H, 0, len(presets))
	for _, p := range presets {
		out = append(out, gin.H{"key": p.Key, "name": p.Name, "desc": p.Desc})
	}
	utils.Success(c, gin.H{"items": out})
}

// AdminCreateHomeBlock POST /api/admin/home-blocks
func AdminCreateHomeBlock(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		Type      string `json:"type" binding:"required"`
		Title     string `json:"title"`
		Config    string `json:"config"`
		Enabled   bool   `json:"enabled"`
		SortOrder int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if !models.IsValidHomeBlockType(req.Type) {
		utils.BadRequest(c, "无效的区块类型")
		return
	}
	cfg, err := models.HomeBlockConfigJSON(req.Config)
	if err != nil {
		utils.BadRequest(c, "config 必须是合法 JSON")
		return
	}
	// 自定义 HTML：管理员输入也过一遍消毒（纵深防御）
	if req.Type == models.HomeBlockCustom {
		cfg = security.SanitizeUserContent(cfg)
	}
	block := &models.HomeBlock{
		Type:      req.Type,
		Title:     utils.ClampString(req.Title, 128),
		Config:    cfg,
		Enabled:   req.Enabled,
		SortOrder: req.SortOrder,
	}
	if err := models.CreateHomeBlock(block); err != nil {
		utils.InternalError(c, "创建区块失败")
		return
	}
	utils.Success(c, block)
}

// AdminUpdateHomeBlock PUT /api/admin/home-blocks/:id
func AdminUpdateHomeBlock(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		utils.BadRequest(c, "无效的区块 ID")
		return
	}
	var req struct {
		Title     *string `json:"title"`
		Config    *string `json:"config"`
		Enabled   *bool   `json:"enabled"`
		SortOrder *int    `json:"sort_order"`
		Type      *string `json:"type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	updates := map[string]interface{}{}
	if req.Type != nil {
		if !models.IsValidHomeBlockType(*req.Type) {
			utils.BadRequest(c, "无效的区块类型")
			return
		}
		updates["type"] = *req.Type
	}
	if req.Title != nil {
		updates["title"] = utils.ClampString(*req.Title, 128)
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}
	if req.Config != nil {
		cfg, cerr := models.HomeBlockConfigJSON(*req.Config)
		if cerr != nil {
			utils.BadRequest(c, "config 必须是合法 JSON")
			return
		}
		blockType := ""
		if req.Type != nil {
			blockType = *req.Type
		} else if existing, gerr := models.GetHomeBlock(uint(id)); gerr == nil {
			blockType = existing.Type
		}
		if blockType == models.HomeBlockCustom {
			cfg = security.SanitizeUserContent(cfg)
		}
		updates["config"] = cfg
	}
	if len(updates) == 0 {
		utils.BadRequest(c, "没有需要更新的字段")
		return
	}
	if err := models.UpdateHomeBlock(uint(id), updates); err != nil {
		utils.InternalError(c, "更新区块失败")
		return
	}
	block, _ := models.GetHomeBlock(uint(id))
	utils.Success(c, block)
}

// AdminDeleteHomeBlock DELETE /api/admin/home-blocks/:id
func AdminDeleteHomeBlock(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		utils.BadRequest(c, "无效的区块 ID")
		return
	}
	if err := models.DeleteHomeBlock(uint(id)); err != nil {
		utils.InternalError(c, "删除区块失败")
		return
	}
	utils.SuccessMessage(c, "已删除", nil)
}

// AdminReorderHomeBlocks POST /api/admin/home-blocks/reorder — body: {ids: [3,1,2]}
func AdminReorderHomeBlocks(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		IDs []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := models.ReorderHomeBlocks(req.IDs); err != nil {
		utils.InternalError(c, "排序失败")
		return
	}
	utils.SuccessMessage(c, "已排序", nil)
}

// AdminApplyHomeBlockPreset POST /api/admin/home-blocks/presets/:key/apply
func AdminApplyHomeBlockPreset(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	key := c.Param("key")
	if models.FindHomeBlockPreset(key) == nil {
		utils.BadRequest(c, "无效的布局预设")
		return
	}
	if err := models.ApplyHomeBlockPreset(key); err != nil {
		utils.InternalError(c, "套用预设失败")
		return
	}
	blocks, _ := models.ListHomeBlocks()
	utils.SuccessMessage(c, "已套用布局预设", gin.H{"items": blocks})
}
