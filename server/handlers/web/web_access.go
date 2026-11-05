package web

import (
	"strconv"

	"nvs-server/handlers/admin"
	"nvs-server/handlers/auth"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	quota "nvs-server/models/quota"
)

// ListWebAccessLogs GET /api/admin/web-access
// 网络诊断 / 用户分布 / 攻击流量面板数据源：IP、UA、状态码、是否 bot 等多维筛选 + 分页。
func ListWebAccessLogs(c *gin.Context) {
	if !admin.EnsureAdmin(c) {
		return
	}
	q := quota.WebAccessQuery{
		IP:         c.Query("ip"),
		UA:         c.Query("ua"),
		UAFamily:   c.Query("ua_family"),
		Method:     c.Query("method"),
		Path:       c.Query("path"),
		Host:       c.Query("host"),
		Identity:   c.Query("identity"),
		OnlyAttack: c.Query("attack") == "1",
		Page:       AtoiDefault(c.Query("page"), 1),
		PageSize:   AtoiDefault(c.Query("page_size"), 50),
	}
	if t := auth.ParseDateParam(c.Query("from")); !t.IsZero() {
		q.From = t.Format("2006-01-02 15:04:05")
	}
	if t := auth.ParseDateParam(c.Query("to")); !t.IsZero() {
		q.To = t.Format("2006-01-02 15:04:05")
	}
	if v := c.Query("status"); v != "" {
		q.Status, _ = strconv.Atoi(v)
	}
	if v := c.Query("is_bot"); v != "" {
		b := v == "1" || v == "true"
		q.IsBot = &b
	}
	items, total, err := quota.ListWebAccess(q)
	if err != nil {
		utils.InternalError(c, "查询 Web 访问日志失败")
		return
	}
	utils.Success(c, gin.H{"total": total, "items": items})
}

// GetWebAccessStats GET /api/admin/web-access/stats
// 聚合统计：累计/今日/爬虫总数/近24h攻击流量、Top IP、Top 路径、状态码分布、UA 家族分布、爬虫原因分布。
func GetWebAccessStats(c *gin.Context) {
	if !admin.EnsureAdmin(c) {
		return
	}
	s, err := quota.GetWebAccessStats()
	if err != nil {
		utils.InternalError(c, "统计 Web 访问日志失败")
		return
	}
	utils.Success(c, s)
}

// CleanupWebAccessLogs POST /api/admin/web-access/cleanup
// 清空 Web 访问日志（合规归档证据由文件日志承担，本表仅为前台诊断用途）。
func CleanupWebAccessLogs(c *gin.Context) {
	if !admin.EnsureAdmin(c) {
		return
	}
	if err := quota.WebAccessLogCleanup(); err != nil {
		utils.InternalError(c, "清空 Web 访问日志失败")
		return
	}
	utils.Success(c, gin.H{"ok": true})
}

// atoiDefault 安全解析整型查询参数，解析失败或空返回 def。
func AtoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
