package handlers

import (
	"strconv"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// PoliceGovernanceOverview GET /api/police/governance/overview
// 警方内容治理总览：按类型汇总「删除/封禁」留证数量。
// 数据来自 softdel_* 留证表（仅含操作者用户名，不含真实姓名/手机/IP 等敏感信息），
// 与站长内容治理板块口径一致；合规与安全（含 PII）板块不在此暴露。
func PoliceGovernanceOverview(c *gin.Context) {
	if !ensurePolice(c) {
		return
	}
	counts := map[string]int64{}
	var total int64
	for ctype, table := range models.SoftDelTables {
		var n int64
		models.DB.Table(table).Count(&n)
		counts[ctype] = n
		total += n
	}
	utils.Success(c, gin.H{
		"total":   total,
		"by_type": counts,
	})
}

// PoliceGovernanceList GET /api/police/governance/banned
// 警方内容治理封禁清单（与站长内容治理板块一致）：支持按类型过滤、分页。
// 返回 sanitized 留证（reason/operator 用户名/summary），不含敏感 PII。
func PoliceGovernanceList(c *gin.Context) {
	if !ensurePolice(c) {
		return
	}
	ctype := c.Query("type")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	items, err := models.ListSoftDel(ctype, limit)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	// 敏感信息过滤：留证快照(Snapshot)可能含真实姓名/手机/邮箱等 PII，
	// 合规与安全敏感字段警方不可见，统一置空。
	safe := make([]map[string]interface{}, 0, len(items))
	for _, it := range items {
		safe = append(safe, map[string]interface{}{
			"type":        it.Type,
			"id":          it.ID,
			"original_id": it.OriginalID,
			"deleted_by":  it.DeletedBy,
			"operator":    it.Operator,
			"reason":      it.Reason,
			"source":      it.Source,
			"summary":     it.Summary,
			"deleted_at":  it.DeletedAt,
		})
	}
	utils.Success(c, gin.H{"items": safe})
}

// ensurePolice 要求 admin 或 police（与中间件 PoliceRequired 口径一致）。
func ensurePolice(c *gin.Context) bool {
	if !models.IsAdminOrPolice(currentUserRole(c)) {
		utils.Forbidden(c, "无权限")
		return false
	}
	return true
}
