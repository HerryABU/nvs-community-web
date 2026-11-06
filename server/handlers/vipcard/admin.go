// Package vipcard VIP 卡密的 HTTP 接口层（#66）。
package vipcard

import (
	"strconv"
	"strings"
	"time"

	"nvs-server/handlers/admincommon"
	"nvs-server/handlers/hctx"
	"nvs-server/utils/infra"

	vc "nvs-server/vipcard"

	"github.com/gin-gonic/gin"
)

// AdminGenerate POST /api/admin/vip-cards — 批量生成（明文仅本次返回）
func AdminGenerate(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	var req struct {
		Count     int    `json:"count"`
		Level     int    `json:"level"`
		Days      int    `json:"days"`
		ExpiresIn int    `json:"expires_in"` // 卡密有效期天数，0 = 永久
		Note      string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	var expiresAt *time.Time
	if req.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(req.ExpiresIn) * 24 * time.Hour)
		expiresAt = &t
	}
	codes, err := vc.Generate(hctx.CurrentUserID(c), req.Count, req.Level, req.Days, expiresAt, strings.TrimSpace(req.Note))
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.Success(c, gin.H{
		"message": "已生成，请立即保存以下卡密（仅本次可见，刷新后无法再次查看）",
		"codes":   codes,
		"count":   len(codes),
	})
}

// AdminList GET /api/admin/vip-cards — 列表（status / batch 筛选）
func AdminList(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	list, err := vc.List(c.Query("status"), c.Query("batch"), 200)
	if err != nil {
		infra.InternalError(c, "查询失败")
		return
	}
	infra.Success(c, list)
}

// AdminVoid POST /api/admin/vip-cards/:id/void
func AdminVoid(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := vc.Void(uint(id)); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.Success(c, gin.H{"message": "已作废"})
}
