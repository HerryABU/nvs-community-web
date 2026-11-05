package aiadmin

import (
	"nvs-server/handlers/admincommon"
	"nvs-server/handlers/hctx"
	"nvs-server/supervise"
	"nvs-server/utils/infra"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ==================== 名单子系统（以疏代堵）管理端 API ====================
// 绥靖模式下被静默记入「可疑名单」的用户，在此可被站长查看、改档、清除、加注。
// 升级到「恶意名单」后同样在此管理（升级由系统按判定分自动完成，此处只做人工干预）。

// AdminListSupervised GET /api/admin/supervise/list
func AdminListSupervised(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	tier := c.Query("tier")
	status := c.Query("status")
	list, total, err := supervise.List(page, size, tier, status)
	if err != nil {
		infra.InternalError(c, "查询失败")
		return
	}
	infra.Success(c, gin.H{"items": list, "total": total, "page": page})
}

// AdminGetSupervised GET /api/admin/supervise/:id
func AdminGetSupervised(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if id == 0 {
		infra.BadRequest(c, "参数错误")
		return
	}
	su, logs, err := supervise.Get(uint(id))
	if err != nil {
		infra.NotFound(c, "记录不存在")
		return
	}
	infra.Success(c, gin.H{"record": su, "logs": logs})
}

// AdminSetTier POST /api/admin/supervise/:id/set-tier
// 管理员手动改档（含降级，必须留痕）。
func AdminSetTier(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if id == 0 {
		infra.BadRequest(c, "参数错误")
		return
	}
	var req struct {
		Tier string `json:"tier"`
		Note string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	if err := supervise.SetTier(uint(id), hctx.CurrentUserID(c), req.Tier, req.Note); err != nil {
		infra.InternalError(c, err.Error())
		return
	}
	infra.Success(c, gin.H{"message": "已改档"})
}

// AdminClearSupervised POST /api/admin/supervise/:id/clear
// 管理员清除（不在档，但保留历史原因与日志）。
func AdminClearSupervised(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if id == 0 {
		infra.BadRequest(c, "参数错误")
		return
	}
	var req struct {
		Note string `json:"note"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := supervise.Clear(uint(id), hctx.CurrentUserID(c), req.Note); err != nil {
		infra.InternalError(c, err.Error())
		return
	}
	infra.Success(c, gin.H{"message": "已清除"})
}

// AdminAddSuperviseNote POST /api/admin/supervise/:id/note
func AdminAddSuperviseNote(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if id == 0 {
		infra.BadRequest(c, "参数错误")
		return
	}
	var req struct {
		Note string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	if err := supervise.AddNote(uint(id), hctx.CurrentUserID(c), req.Note); err != nil {
		infra.InternalError(c, err.Error())
		return
	}
	infra.Success(c, gin.H{"message": "已加注"})
}

// AdminSuperviseStats GET /api/admin/supervise/stats
func AdminSuperviseStats(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	infra.Success(c, supervise.Stats())
}
