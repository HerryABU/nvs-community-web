package handlers

// 检查应对工具箱（176号令 5.2 检查材料清单）：
//   GET /api/admin/compliance/export-logs   按时间范围导出访问日志（ZIP）
//   GET /api/admin/compliance/export-users  导出用户注册信息清单（CSV）
//   GET /api/admin/compliance/report        一键生成合规自查报告（JSON）

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"nvs-server/config"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ComplianceReport 合规自查报告数据
type ComplianceReport struct {
	GeneratedAt       string           `json:"generated_at"`
	SiteName          string           `json:"site_name"`
	ICP               string           `json:"icp"`
	PSB               string           `json:"psb"`
	PSBLink           string           `json:"psb_link"`
	LogRetention      bool             `json:"log_retention"`      // 日志留存（176号令 第三条）
	LogRedactEnabled  bool             `json:"log_redact_enabled"` // 日志脱敏（数据安全）
	SecurityGrade     string           `json:"security_grade"`     // 等保定级
	VulnStats         map[string]int64 `json:"vuln_stats"`         // 漏洞整改统计
	AnnouncementCount int64            `json:"announcement_count"` // 公告数
	UserCount         int64            `json:"user_count"`
	Checklist         []map[string]any `json:"checklist"` // 检查应对材料清单
}

// GetComplianceReport 一键生成合规自查报告（管理后台）
// GET /api/admin/compliance/report
func GetComplianceReport(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	icp, psb, psbLink := models.GetSiteFiling()
	grade, note := models.GetSecurityGrade()
	var userCount, announceCount int64
	models.DB.Model(&models.User{}).Count(&userCount)
	models.DB.Model(&models.Announcement{}).Count(&announceCount)

	report := ComplianceReport{
		GeneratedAt:       time.Now().Format("2006-01-02 15:04:05"),
		SiteName:          models.GetSiteName(),
		ICP:               icp,
		PSB:               psb,
		PSBLink:           psbLink,
		LogRetention:      true, // 访问日志自动留存（176号令 第三条）
		LogRedactEnabled:  config.RedactMgr.IsEnabled(),
		SecurityGrade:     grade,
		VulnStats:         models.GetVulnStats(),
		AnnouncementCount: announceCount,
		UserCount:         userCount,
	}
	// 检查应对材料清单（5.2）
	report.Checklist = []map[string]any{
		{"item": "ICP备案号 + 公安备案号", "ready": icp != "" && psb != "", "detail": fmt.Sprintf("ICP=%s 公安=%s", icp, psb)},
		{"item": "近6个月访问日志", "ready": true, "detail": "日志自动留存 + 工具箱一键导出"},
		{"item": "网络安全管理制度", "ready": true, "detail": "docs/compliance 模板（管理员可上传自定义版）"},
		{"item": "信息内容审核制度", "ready": true, "detail": "docs/compliance 模板"},
		{"item": "应急响应预案", "ready": true, "detail": "docs/compliance 模板"},
		{"item": "等保定级材料", "ready": grade != "未定级", "detail": "定级=" + grade + " 备注=" + note},
		{"item": "漏洞整改记录", "ready": report.VulnStats["total"] > 0, "detail": fmt.Sprintf("共 %d 条（已整改 %d）", report.VulnStats["total"], report.VulnStats["fixed"])},
		{"item": "算法说明文档", "ready": true, "detail": "首页「📊 推荐说明」+ 后台算法管理"},
		{"item": "公告栏（安全公告/通知）", "ready": announceCount > 0, "detail": fmt.Sprintf("共 %d 条公告", announceCount)},
	}
	utils.Success(c, report)
}

// ExportComplianceLogs 按时间范围导出访问日志 ZIP（管理后台）
// GET /api/admin/compliance/export-logs?from=2026-08-01&to=2026-08-24
func ExportComplianceLogs(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	from := c.DefaultQuery("from", time.Now().AddDate(0, 0, -30).Format("2006-01-02"))
	to := c.DefaultQuery("to", time.Now().Format("2006-01-02"))
	fromT, err1 := time.Parse("2006-01-02", from)
	toT, err2 := time.Parse("2006-01-02", to)
	if err1 != nil || err2 != nil || fromT.After(toT) {
		utils.BadRequest(c, "时间范围非法（格式 YYYY-MM-DD，from ≤ to）")
		return
	}

	// 收集 log/requests/<date>/<hour>.log 中落在范围内的文件
	logRoot := filepath.Join(utils.DailyLogRoot, "requests")
	var files []string
	for d := fromT; !d.After(toT); d = d.AddDate(0, 0, 1) {
		dir := filepath.Join(logRoot, d.Format("2006-01-02"))
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".log") {
				files = append(files, filepath.Join(dir, e.Name()))
			}
		}
	}
	if len(files) == 0 {
		utils.BadRequest(c, "所选时间范围内没有访问日志")
		return
	}

	// 打包 ZIP 流式输出
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", "attachment; filename=access-logs_"+from+"_"+to+".zip")
	zw := zip.NewWriter(c.Writer)
	defer zw.Close()
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		rel, _ := filepath.Rel(logRoot, f)
		zf, err := zw.Create("logs/" + filepath.ToSlash(rel))
		if err != nil {
			continue
		}
		zf.Write(data)
	}
}

// ExportComplianceUsers 导出用户注册信息清单 CSV（管理后台）
// GET /api/admin/compliance/export-users
func ExportComplianceUsers(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var users []models.User
	models.DB.Order("id ASC").Find(&users)
	var b strings.Builder
	b.WriteString("\xEF\xBB\xBF")
	b.WriteString("用户ID,用户名,昵称,邮箱,角色,注册时间,注册IP,最后登录IP,最后登录时间,状态\n")
	for _, u := range users {
		status := "正常"
		if u.DeletedAt.Valid {
			status = "已删除"
		} else if u.MutedUntil != nil && u.MutedUntil.After(time.Now()) {
			status = "已禁言"
		}
		lastLogin := ""
		if u.LastLoginAt != nil {
			lastLogin = u.LastLoginAt.Format("2006-01-02 15:04")
		}
		line := fmt.Sprintf("%d,%s,%s,%s,%s,%s,%s,%s,%s,%s\n",
			u.ID, safeCSV(u.Username), safeCSV(u.Nickname), safeCSV(u.Email), u.Role,
			u.CreatedAt.Format("2006-01-02 15:04"), safeCSV(u.RegistrationIP), safeCSV(u.LastLoginIP), lastLogin, status)
		b.WriteString(line)
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=users_"+time.Now().Format("20060102")+".csv")
	c.String(200, b.String())
}

func safeCSV(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, ",", "，"), "\n", " ")
}
