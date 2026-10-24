package honeypothandler

// ==================== 蜜罐 / 日志记录策略：管理接口 ====================
// 全部限管理员（admin 组路由 + handler 内 ensureAdmin 双保险）。
// 站长关心两件事：① 蜜罐开不开、多狠（等级）；② 日志记不记请求头这些字段。

import (
	"strconv"

	"nvs-server/honeypot"
	"nvs-server/middleware"
	"nvs-server/models"
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

// Cleanup POST /api/admin/honeypot/cleanup —— 先冷归档再删除（合规留存）
func Cleanup(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	res, err := honeypot.ArchiveAndCleanup()
	if err != nil {
		utils.InternalError(c, "归档失败，未删除任何数据")
		return
	}
	if res.Skipped {
		utils.SuccessMessage(c, "没有超期事件", res)
		return
	}
	utils.SuccessMessage(c, "已归档并清理", res)
}

// Archives GET /api/admin/honeypot/archives —— 已归档文件列表
func Archives(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	list, err := honeypot.ListArchives()
	if err != nil {
		utils.InternalError(c, "读取归档目录失败")
		return
	}
	utils.Success(c, gin.H{"list": list, "dir_hint": "data/honeypot/archive"})
}

// DBInfo GET /api/admin/honeypot/db —— 当前是内嵌库还是独立库
func DBInfo(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	utils.Success(c, gin.H{"isolated": honeypot.Isolated(), "path": honeypot.DBPath()})
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

// SQLiStats GET /api/admin/honeypot/sqli —— 注入蜜罐命中统计（gap-audit #3）
func SQLiStats(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	type row struct {
		Decoy string `json:"decoy"`
		Kind  string `json:"kind"`
		C     int64  `json:"count"`
	}
	var rows []row
	_ = models.DB.Model(&honeypot.Event{}).
		Select("decoy, kind, COUNT(1) as c").
		Where("kind IN ?", []string{honeypot.EventSQLiHit, honeypot.EventSQLiProbe}).
		Group("decoy, kind").Order("c DESC").Scan(&rows).Error
	var hits, probes int64
	_ = models.DB.Model(&honeypot.Event{}).Where("kind = ?", honeypot.EventSQLiHit).Count(&hits).Error
	_ = models.DB.Model(&honeypot.Event{}).Where("kind = ?", honeypot.EventSQLiProbe).Count(&probes).Error
	utils.Success(c, gin.H{
		"hits": hits, "probes": probes, "breakdown": rows,
		"note": "真实业务链路全部参数化，SQL 注入打不穿；这些是「钓鱼式假入口」的记录，用途是拿到完整 payload 与工具特征，并拖住攻击者。",
	})
}

// TLSStats GET /api/admin/honeypot/tls —— JA3/JA4 采集率与配置模板（gap-audit #4）
func TLSStats(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	utils.Success(c, honeypot.TLSStats())
}
