// Package readperm 阅读权限的 HTTP 接口层：
//   - /api/admin/read-permission  站长配置可用等级、VIP 门槛、付费最低价
//   - /api/read-permission/levels 作者端下拉使用的当前可用等级
package readperm

import (
	"nvs-server/handlers/admincommon"
	"nvs-server/utils/infra"

	rp "nvs-server/readperm"

	"github.com/gin-gonic/gin"
)

// AdminGetConfig GET /api/admin/read-permission
func AdminGetConfig(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	cfg := rp.Load()
	infra.Success(c, gin.H{
		"config":         cfg,
		"levels":         rp.AvailableLevels(),
		"all_levels":     rp.AllLevels,
		"default_labels": rp.DefaultLabels,
	})
}

// AdminSaveConfig PUT /api/admin/read-permission
func AdminSaveConfig(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	var in rp.Config
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	if err := rp.Save(in); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.Success(c, gin.H{"config": rp.Load(), "levels": rp.AvailableLevels()})
}

// Levels GET /api/read-permission/levels — 作者端选择阅读权限时的可选项。
// 结果与站长配置实时一致；停用某等级后作者端立刻不再展示，
// 历史内容按判定层保守降级，不会被锁死。
func Levels(c *gin.Context) {
	cfg := rp.Load()
	infra.Success(c, gin.H{
		"levels":         rp.AvailableLevels(),
		"vip_min_level":  cfg.VipMinLevel,
		"paid_min_price": cfg.PaidMinPrice,
	})
}
