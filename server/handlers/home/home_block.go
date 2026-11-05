// home_block.go — 自定义主页区块管理（F15）。
// 管理员：区块 CRUD / 排序 / 启停 / 一键套用布局预设；
// 公开：仅返回启用区块（自定义 HTML 已在保存时服务端消毒）。
package home

import (
	"nvs-server/handlers/admincommon"
	"nvs-server/security"
	"nvs-server/utils/infra"
	"strconv"

	"github.com/gin-gonic/gin"
	theme "nvs-server/models/theme"
)

// ListPublicHomeBlocks GET /api/home-blocks — 首页区块（仅启用，按排序）
func ListPublicHomeBlocks(c *gin.Context) {
	blocks, err := theme.ListEnabledHomeBlocks()
	if err != nil {
		infra.InternalError(c, "获取首页区块失败")
		return
	}
	// 空库时播种默认布局（混合一条龙），保证老部署升级后首页不空
	if len(blocks) == 0 && theme.CountHomeBlocks() == 0 {
		if err := theme.SeedDefaultHomeBlocks(); err == nil {
			blocks, _ = theme.ListEnabledHomeBlocks()
		}
	}
	infra.Success(c, gin.H{"items": blocks})
}

// AdminListHomeBlocks GET /api/admin/home-blocks — 全部区块（含禁用）
func AdminListHomeBlocks(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	blocks, err := theme.ListHomeBlocks()
	if err != nil {
		infra.InternalError(c, "获取区块失败")
		return
	}
	infra.Success(c, gin.H{"items": blocks})
}

// AdminListHomeBlockPresets GET /api/admin/home-blocks/presets — 布局预设列表
func AdminListHomeBlockPresets(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	presets := theme.HomeBlockPresets()
	out := make([]gin.H, 0, len(presets))
	for _, p := range presets {
		out = append(out, gin.H{"key": p.Key, "name": p.Name, "desc": p.Desc})
	}
	infra.Success(c, gin.H{"items": out})
}

// AdminCreateHomeBlock POST /api/admin/home-blocks
func AdminCreateHomeBlock(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
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
		infra.BadRequest(c, "参数错误")
		return
	}
	if !theme.IsValidHomeBlockType(req.Type) {
		infra.BadRequest(c, "无效的区块类型")
		return
	}
	cfg, err := theme.HomeBlockConfigJSON(req.Config)
	if err != nil {
		infra.BadRequest(c, "config 必须是合法 JSON")
		return
	}
	// 自定义 HTML：管理员输入也过一遍消毒（纵深防御）
	if req.Type == theme.HomeBlockCustom {
		cfg = security.SanitizeUserContent(cfg)
	}
	block := &theme.HomeBlock{
		Type:      req.Type,
		Title:     infra.ClampString(req.Title, 128),
		Config:    cfg,
		Enabled:   req.Enabled,
		SortOrder: req.SortOrder,
	}
	if err := theme.CreateHomeBlock(block); err != nil {
		infra.InternalError(c, "创建区块失败")
		return
	}
	infra.Success(c, block)
}

// AdminUpdateHomeBlock PUT /api/admin/home-blocks/:id
func AdminUpdateHomeBlock(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		infra.BadRequest(c, "无效的区块 ID")
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
		infra.BadRequest(c, "参数错误")
		return
	}
	updates := map[string]interface{}{}
	if req.Type != nil {
		if !theme.IsValidHomeBlockType(*req.Type) {
			infra.BadRequest(c, "无效的区块类型")
			return
		}
		updates["type"] = *req.Type
	}
	if req.Title != nil {
		updates["title"] = infra.ClampString(*req.Title, 128)
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}
	if req.Config != nil {
		cfg, cerr := theme.HomeBlockConfigJSON(*req.Config)
		if cerr != nil {
			infra.BadRequest(c, "config 必须是合法 JSON")
			return
		}
		blockType := ""
		if req.Type != nil {
			blockType = *req.Type
		} else if existing, gerr := theme.GetHomeBlock(uint(id)); gerr == nil {
			blockType = existing.Type
		}
		if blockType == theme.HomeBlockCustom {
			cfg = security.SanitizeUserContent(cfg)
		}
		updates["config"] = cfg
	}
	if len(updates) == 0 {
		infra.BadRequest(c, "没有需要更新的字段")
		return
	}
	if err := theme.UpdateHomeBlock(uint(id), updates); err != nil {
		infra.InternalError(c, "更新区块失败")
		return
	}
	block, _ := theme.GetHomeBlock(uint(id))
	infra.Success(c, block)
}

// AdminDeleteHomeBlock DELETE /api/admin/home-blocks/:id
func AdminDeleteHomeBlock(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		infra.BadRequest(c, "无效的区块 ID")
		return
	}
	if err := theme.DeleteHomeBlock(uint(id)); err != nil {
		infra.InternalError(c, "删除区块失败")
		return
	}
	infra.SuccessMessage(c, "已删除", nil)
}

// AdminReorderHomeBlocks POST /api/admin/home-blocks/reorder — body: {ids: [3,1,2]}
func AdminReorderHomeBlocks(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	var req struct {
		IDs []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		infra.BadRequest(c, "参数错误")
		return
	}
	if err := theme.ReorderHomeBlocks(req.IDs); err != nil {
		infra.InternalError(c, "排序失败")
		return
	}
	infra.SuccessMessage(c, "已排序", nil)
}

// AdminApplyHomeBlockPreset POST /api/admin/home-blocks/presets/:key/apply
func AdminApplyHomeBlockPreset(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	key := c.Param("key")
	if theme.FindHomeBlockPreset(key) == nil {
		infra.BadRequest(c, "无效的布局预设")
		return
	}
	if err := theme.ApplyHomeBlockPreset(key); err != nil {
		infra.InternalError(c, "套用预设失败")
		return
	}
	blocks, _ := theme.ListHomeBlocks()
	infra.SuccessMessage(c, "已套用布局预设", gin.H{"items": blocks})
}
