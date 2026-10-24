package honeypothandler

// ==================== 蜜罐 / 日志记录策略：管理接口 ====================
// 全部限管理员（admin 组路由 + handler 内 ensureAdmin 双保险）。
// 站长关心两件事：① 蜜罐开不开、多狠（等级）；② 日志记不记请求头这些字段。

import (
	"strconv"

	"nvs-server/honeypot"
	"nvs-server/middleware"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

func ensureAdmin(c *gin.Context) bool {
	role := middleware.UserRole(c)
	if role != "admin" {
		utils.Forbidden(c, "仅管理员可操作")
		return false
	}
	return true
}

// GetConfig GET /api/admin/honeypot/config —— 概览 + 当前策略
func GetConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	utils.Success(c, honeypot.GetOverview())
}

// SaveConfig PUT /api/admin/honeypot/config
func SaveConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var in honeypot.Setting
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	set, err := honeypot.SaveSetting(in)
	if err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.SuccessMessage(c, "已保存（等级："+set.Level+"）", set)
}

// ListSessions GET /api/admin/honeypot/sessions?page=&page_size=&keyword=&min_risk=
func ListSessions(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	ps, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	minRisk, _ := strconv.Atoi(c.Query("min_risk"))
	list, total, err := honeypot.ListSessions(page, ps, c.Query("keyword"), minRisk)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{"list": list, "total": total})
}

// SessionDetail GET /api/admin/honeypot/sessions/:sid
func SessionDetail(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("sid"), 10, 64)
	s, events, err := honeypot.SessionDetail(uint(id))
	if err != nil {
		utils.NotFound(c, err.Error())
		return
	}
	utils.Success(c, gin.H{"session": s, "events": events})
}

// Export GET /api/admin/honeypot/export?session_id=
// 导出取证数据；导出动作本身留痕（记一条 export 事件到该会话）
func Export(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	sid, _ := strconv.ParseUint(c.Query("session_id"), 10, 64)
	data, err := honeypot.Export(uint(sid), middleware.UserID(c))
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	honeypot.LogExport(uint(sid), middleware.UserID(c))
	utils.Success(c, data)
}

// Cleanup POST /api/admin/honeypot/cleanup —— 按留存天数清理
func Cleanup(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	n := honeypot.Cleanup()
	utils.SuccessMessage(c, "已清理过期事件", gin.H{"deleted": n})
}

// Bans GET /api/admin/honeypot/bans
func Bans(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	utils.Success(c, gin.H{"list": honeypot.BannedList()})
}

// Unban POST /api/admin/honeypot/unban { ip }
func Unban(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var in struct {
		IP string `json:"ip"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.IP == "" {
		utils.BadRequest(c, "参数错误")
		return
	}
	honeypot.Unban(in.IP)
	utils.SuccessMessage(c, "已解除封禁", nil)
}

// Ban POST /api/admin/honeypot/ban { ip, minutes }
func Ban(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var in struct {
		IP      string `json:"ip"`
		Minutes int    `json:"minutes"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.IP == "" {
		utils.BadRequest(c, "参数错误")
		return
	}
	if in.Minutes <= 0 {
		in.Minutes = 60
	}
	honeypot.Ban(in.IP, in.Minutes)
	utils.SuccessMessage(c, "已封禁", nil)
}
