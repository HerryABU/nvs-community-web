package handlers

// 合规材料一键打包（等保/176 号令检查应对材料）
//
// 为什么要打包：检查方要的是「一套材料」，不是「几个按钮各导出一次」。
// 因此把报告、算法说明、定级、漏洞整改、日志清单、蜜罐摘要、用户规模汇总成
// 一个 zip，并附 SHA256 清单（证明导出后未被改动）。导出动作写一条审计。

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"nvs-server/honeypot"
	"nvs-server/middleware"
	"nvs-server/models"
	"nvs-server/utils"
	"nvs-server/version"

	"github.com/gin-gonic/gin"
)

// complianceFile 待打包文件
type complianceFile struct {
	Name string
	Data []byte
}

// ExportComplianceBundle GET /api/admin/compliance/bundle.zip
// 一键导出检查应对材料包（含 manifest.json 的 SHA256 清单）
func ExportComplianceBundle(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	files := make([]complianceFile, 0, 8)
	addJSON := func(name string, v interface{}) error {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		files = append(files, complianceFile{Name: name, Data: b})
		return nil
	}

	// 1) 自查报告（与 /compliance/report 同口径）
	if err := addJSON("01-report.json", buildComplianceReport()); err != nil {
		utils.InternalError(c, "生成报告失败")
		return
	}
	// 2) 算法推荐配置（算法透明度）
	sortRule, rankPeriod, personalized, desc := models.GetAlgoConfig()
	if err := addJSON("02-algorithm.json", gin.H{
		"sort_rule": sortRule, "rank_period": rankPeriod,
		"personalized": personalized, "description": desc,
		"public_page": "首页「推荐说明」弹窗 + /api/algorithms/description",
	}); err != nil {
		utils.InternalError(c, "生成算法说明失败")
		return
	}
	// 3) 等保定级
	grade, note := models.GetSecurityGrade()
	if err := addJSON("03-security-grade.json", gin.H{"grade": grade, "note": note}); err != nil {
		utils.InternalError(c, "生成定级材料失败")
		return
	}
	// 4) 漏洞整改统计
	if err := addJSON("04-vulnerabilities.json", models.GetVulnStats()); err != nil {
		utils.InternalError(c, "生成漏洞统计失败")
		return
	}
	// 5) 用户规模汇总（不含 PII）
	if err := addJSON("05-users-summary.json", userSummary()); err != nil {
		utils.InternalError(c, "生成用户汇总失败")
		return
	}
	// 6) 蜜罐与日志策略摘要（等级/记录字段/命中规模）
	hset := honeypot.Current()
	var sessions, events, highRisk int64
	ov := honeypot.GetOverview()
	sessions, events, highRisk = ov.SessionCount, ov.EventCount, ov.HighRisk
	if err := addJSON("06-honeypot-summary.json", gin.H{
		"level": hset.Level, "enabled": hset.Enabled,
		"isolated_db": honeypot.Isolated(),
		"record_fields": gin.H{
			"ua": hset.RecordUA, "referer": hset.RecordReferer, "xff": hset.RecordXFF,
			"query": hset.RecordQuery, "headers": hset.RecordHeaders, "body": hset.RecordBodySnippet,
		},
		"sessions": sessions, "events": events, "high_risk_sessions": highRisk,
		"retention_days": hset.RetentionDays,
	}); err != nil {
		utils.InternalError(c, "生成蜜罐摘要失败")
		return
	}
	// 7) 日志留存清单（最近 30 天，附哈希；不打包日志本体避免体积失控）
	if err := addJSON("07-logs-manifest.json", logsManifest(30)); err != nil {
		utils.InternalError(c, "生成日志清单失败")
		return
	}

	// 8) manifest（每文件 SHA256）
	manifest := map[string]interface{}{
		"exported_at": time.Now().Format(time.RFC3339),
		"operator_id": middleware.UserID(c),
		"site":        models.GetSiteName(),
		"version":     version.AppVersion,
		"files":       map[string]string{},
	}
	sums := map[string]string{}
	for _, f := range files {
		sum := sha256.Sum256(f.Data)
		sums[f.Name] = hex.EncodeToString(sum[:])
	}
	manifest["files"] = sums
	mb, _ := json.MarshalIndent(manifest, "", "  ")
	files = append(files, complianceFile{Name: "00-manifest.json", Data: mb})

	dir := filepath.Join("data", "compliance")
	_ = os.MkdirAll(dir, 0700)
	p := filepath.Join(dir, fmt.Sprintf("compliance-bundle-%s.zip", time.Now().Format("20060102-150405")))
	f, err := os.Create(p)
	if err != nil {
		utils.InternalError(c, "创建材料包失败")
		return
	}
	zw := zip.NewWriter(f)
	for _, cf := range files {
		w, werr := zw.Create(cf.Name)
		if werr != nil {
			_ = zw.Close()
			_ = f.Close()
			utils.InternalError(c, "写入材料包失败")
			return
		}
		_, _ = w.Write(cf.Data)
	}
	if err := zw.Close(); err != nil {
		_ = f.Close()
		utils.InternalError(c, "关闭材料包失败")
		return
	}
	_ = f.Close()

	// 审计：导出材料包本身要留痕
	auditAdmExport(middleware.UserID(c), "compliance_bundle", filepath.Base(p))

	c.Header("Content-Disposition", "attachment; filename="+filepath.Base(p))
	c.File(p)
	go func() { _ = os.Remove(p) }() // 下载后删除，避免堆积敏感材料
}

// userSummary 用户规模/角色分布（不含邮箱、手机号等 PII）
func userSummary() gin.H {
	type row struct {
		Role  string
		Count int64
	}
	var rows []row
	models.DB.Model(&models.User{}).Select("role, count(*) as count").Group("role").Scan(&rows)
	dist := map[string]int64{}
	var total int64
	for _, r := range rows {
		dist[r.Role] = r.Count
		total += r.Count
	}
	// 近 7 日新增
	var recent int64
	models.DB.Model(&models.User{}).Where("created_at >= ?", time.Now().AddDate(0, 0, -7)).Count(&recent)
	return gin.H{"total": total, "by_role": dist, "new_last_7d": recent}
}

// logsManifest 日志文件清单（路径 + 大小 + SHA256 前 16 位）
func logsManifest(days int) gin.H {
	root := filepath.Join(utils.DailyLogRoot, "requests")
	type fileInfo struct {
		Path   string `json:"path"`
		Bytes  int64  `json:"bytes"`
		SHA256 string `json:"sha256_16"`
	}
	files := make([]fileInfo, 0, 64)
	for i := 0; i < days; i++ {
		d := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		dir := filepath.Join(root, d)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
				continue
			}
			full := filepath.Join(dir, e.Name())
			b, rerr := os.ReadFile(full)
			if rerr != nil {
				continue
			}
			sum := sha256.Sum256(b)
			files = append(files, fileInfo{
				Path: filepath.ToSlash(full), Bytes: int64(len(b)),
				SHA256: hex.EncodeToString(sum[:8]),
			})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path > files[j].Path })
	return gin.H{
		"root": root, "days": days, "count": len(files),
		"files": files,
		"note":  "日志本体不打包（体积与敏感度），需要时用 /api/admin/compliance/export-logs 按范围导出",
	}
}

// auditAdmExport 管理端导出留痕（写用户行为审计，失败不阻断下载）
func auditAdmExport(operatorID uint, kind, target string) {
	models.RecordUserAudit(&models.UserAuditLog{
		UserID: operatorID, Action: "export_" + kind,
		Detail: target, Result: "success", CreatedAt: time.Now(),
	})
}
