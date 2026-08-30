package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ensurePoliceOrAdmin 警方取管台鉴权：仅 admin 或 police 可访问。
// police 角色是「准管理员」，只能进取管台，看不到普通 /api/admin 管理功能。
func ensurePoliceOrAdmin(c *gin.Context) bool {
	if !models.IsAdminOrPolice(currentUserRole(c)) {
		utils.Forbidden(c, "需要警方取管台或管理员权限")
		c.Abort()
		return false
	}
	return true
}

// GET /api/police/softdel — 软删除留证列表（警方可取证查看，管理员亦可）
func PoliceListSoftDel(c *gin.Context) {
	if !ensurePoliceOrAdmin(c) {
		return
	}
	ctype := c.Query("type")
	if ctype != "" {
		if _, ok := models.SoftDelTables[ctype]; !ok {
			utils.BadRequest(c, "无效的内容类型: "+ctype)
			return
		}
	}
	limit := 500
	if n, err := strconv.Atoi(c.Query("limit")); err == nil && n > 0 && n <= 2000 {
		limit = n
	}
	items, err := models.ListSoftDel(ctype, limit)
	if err != nil {
		utils.InternalError(c, err.Error())
		return
	}
	utils.Success(c, items)
}

// GET /api/police/evidence/:type/:id — 单条取证凭证详情（完整快照）
func PoliceGetEvidence(c *gin.Context) {
	if !ensurePoliceOrAdmin(c) {
		return
	}
	ctype := c.Param("type")
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	item, err := models.GetSoftDelByID(ctype, uint(id))
	if err != nil {
		utils.NotFound(c, "取证记录不存在")
		return
	}
	utils.Success(c, item)
}

// GET /api/police/evidence/:type/:id/download — 下载单条取证凭证（JSON 文件，取证留证）
func PoliceDownloadEvidence(c *gin.Context) {
	if !ensurePoliceOrAdmin(c) {
		return
	}
	ctype := c.Param("type")
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	item, err := models.GetSoftDelByID(ctype, uint(id))
	if err != nil {
		utils.NotFound(c, "取证记录不存在")
		return
	}
	serveEvidenceDownload(c, fmt.Sprintf("evidence_%s_%d.json", ctype, id), item)
}

// GET /api/police/export — 导出全部取证包（软删除留证 + 隔离墙访问 + Web 访问 + 用户审计）
// 满足「一系列凭证下载 / 提前 2-3 天通知准备」的取证导出需求。
func PoliceExportPackage(c *gin.Context) {
	if !ensurePoliceOrAdmin(c) {
		return
	}
	pkg := gin.H{
		"exported_at": time.Now().Format("2006-01-02 15:04:05"),
		"exported_by": currentUserRole(c),
	}
	if soft, err := models.ListSoftDel("", 2000); err == nil {
		pkg["soft_deleted"] = soft
	}
	if za, _, err := models.ListZoneAccessPage(models.ZoneAccessFilter{Page: 1, PageSize: 500}); err == nil {
		pkg["zone_access"] = za
	}
	if wa, _, err := models.ListWebAccess(models.WebAccessQuery{Page: 1, PageSize: 500}); err == nil {
		pkg["web_access"] = wa
	}
	if audit, err := models.ListUserAuditLogs(0, "", 500); err == nil {
		pkg["user_audit"] = audit
	}
	serveEvidenceDownload(c, fmt.Sprintf("police_evidence_package_%s.json", time.Now().Format("20060102_150405")), pkg)
}

// serveEvidenceDownload 以附件形式下载 JSON（取证凭证）
func serveEvidenceDownload(c *gin.Context, filename string, data interface{}) {
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		utils.InternalError(c, "序列化失败")
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/json; charset=utf-8")
	c.Data(http.StatusOK, "application/json; charset=utf-8", raw)
}
