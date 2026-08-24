package handlers

// 漏洞整改管理 API（公安部令第176号 第七条第六项：漏洞隐患整改闭环留痕）：
//   GET    /api/admin/vulnerabilities          漏洞列表（支持 status/severity 筛选 + 统计）
//   POST   /api/admin/vulnerabilities          新建漏洞记录
//   PUT    /api/admin/vulnerabilities/:id      更新漏洞（状态/整改时间等）
//   DELETE /api/admin/vulnerabilities/:id      删除漏洞记录
//   GET    /api/admin/vulnerabilities/export   导出漏洞报告（CSV，供配合公安检查）

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ListVulnerabilities 漏洞列表 + 统计
// GET /api/admin/vulnerabilities?status=pending&severity=high
func ListVulnerabilities(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	status := c.Query("status")
	severity := c.Query("severity")
	q := models.DB.Model(&models.Vulnerability{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if severity != "" {
		q = q.Where("severity = ?", severity)
	}
	var list []models.Vulnerability
	q.Order("created_at DESC").Find(&list)
	utils.Success(c, gin.H{"list": list, "stats": models.GetVulnStats()})
}

// CreateVulnerability 新建漏洞记录
// POST /api/admin/vulnerabilities
func CreateVulnerability(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Severity    string `json:"severity" binding:"required"`
		Affected    string `json:"affected"`
		ReportedBy  string `json:"reported_by"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请填写漏洞名称与严重程度")
		return
	}
	severity := strings.ToLower(strings.TrimSpace(req.Severity))
	switch severity {
	case models.VulnSeverityCritical, models.VulnSeverityHigh, models.VulnSeverityMedium, models.VulnSeverityLow:
	default:
		utils.BadRequest(c, "严重程度仅支持 critical / high / medium / low")
		return
	}
	v := &models.Vulnerability{
		Name:        req.Name,
		Description: req.Description,
		Severity:    severity,
		Status:      models.VulnStatusPending,
		Affected:    req.Affected,
		ReportedBy:  req.ReportedBy,
	}
	if err := models.DB.Create(v).Error; err != nil {
		utils.InternalError(c, "创建失败")
		return
	}
	utils.Success(c, v)
}

// UpdateVulnerability 更新漏洞记录（状态流转：pending → fixing → fixed/ignored）
// PUT /api/admin/vulnerabilities/:id
func UpdateVulnerability(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var v models.Vulnerability
	if err := models.DB.First(&v, id).Error; err != nil {
		utils.NotFound(c, "漏洞记录不存在")
		return
	}
	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Severity    *string `json:"severity"`
		Status      *string `json:"status"`
		Affected    *string `json:"affected"`
		ReportedBy  *string `json:"reported_by"`
		FixedAt     *string `json:"fixed_at"` // RFC3339 或留空
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	updates := map[string]interface{}{}
	if req.Name != nil && *req.Name != "" {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Affected != nil {
		updates["affected"] = *req.Affected
	}
	if req.ReportedBy != nil {
		updates["reported_by"] = *req.ReportedBy
	}
	if req.Severity != nil {
		s := strings.ToLower(strings.TrimSpace(*req.Severity))
		switch s {
		case models.VulnSeverityCritical, models.VulnSeverityHigh, models.VulnSeverityMedium, models.VulnSeverityLow:
			updates["severity"] = s
		}
	}
	if req.Status != nil {
		s := strings.TrimSpace(*req.Status)
		switch s {
		case models.VulnStatusPending, models.VulnStatusFixing, models.VulnStatusFixed, models.VulnStatusIgnored:
			updates["status"] = s
			if s == models.VulnStatusFixed {
				now := time.Now()
				updates["fixed_at"] = &now
			} else {
				updates["fixed_at"] = nil
			}
		}
	}
	if req.FixedAt != nil && *req.FixedAt != "" {
		if t, err := time.Parse(time.RFC3339, *req.FixedAt); err == nil {
			updates["fixed_at"] = &t
		}
	}
	if len(updates) > 0 {
		if err := models.DB.Model(&v).Updates(updates).Error; err != nil {
			utils.InternalError(c, "更新失败")
			return
		}
	}
	models.DB.First(&v, id)
	utils.Success(c, v)
}

// DeleteVulnerability 删除漏洞记录
// DELETE /api/admin/vulnerabilities/:id
func DeleteVulnerability(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	models.DB.Delete(&models.Vulnerability{}, id)
	utils.Success(c, gin.H{"message": "漏洞记录已删除"})
}

// ExportVulnerabilities 导出漏洞报告 CSV（供配合公安检查）
// GET /api/admin/vulnerabilities/export
func ExportVulnerabilities(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var list []models.Vulnerability
	models.DB.Order("created_at DESC").Find(&list)
	var b strings.Builder
	b.WriteString("\xEF\xBB\xBF") // UTF-8 BOM（Excel 兼容）
	b.WriteString("漏洞名称,严重程度,状态,影响范围,报告者,创建时间,整改时间,描述\n")
	sevCN := map[string]string{"critical": "严重", "high": "高危", "medium": "中危", "low": "低危"}
	statusCN := map[string]string{"pending": "待整改", "fixing": "整改中", "fixed": "已整改", "ignored": "已忽略"}
	for _, v := range list {
		fixedAt := ""
		if v.FixedAt != nil {
			fixedAt = v.FixedAt.Format("2006-01-02 15:04")
		}
		line := fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s,%s\n",
			strings.ReplaceAll(v.Name, ",", "，"),
			sevCN[v.Severity], statusCN[v.Status],
			strings.ReplaceAll(v.Affected, ",", "，"),
			strings.ReplaceAll(v.ReportedBy, ",", "，"),
			v.CreatedAt.Format("2006-01-02 15:04"), fixedAt,
			strings.ReplaceAll(strings.ReplaceAll(v.Description, "\n", " "), ",", "，"))
		b.WriteString(line)
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=vulnerabilities_"+time.Now().Format("20060102")+".csv")
	c.String(200, b.String())
}
