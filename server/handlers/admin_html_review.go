package handlers

// 应用扩展（UserHTML）安全预检与管理员审核：
//   - 解压前预检命中高危 → 原样封存 ZIP（不解压），状态 quarantine
//   - 预检仅有中/低危告警 → 已解压，状态 pending，等复核
//   - 管理员可放行（approve）或驳回（reject）
//     · 放行 quarantine 件：按封存的 ZIP 补一次解压，再转 approved
//     · 驳回：转 rejected；可选连封存件一起删除

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"nvs-server/config"
	"nvs-server/models"
	"nvs-server/security"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// notifyAdminsHTMLReview 给全部管理员推送站内通知（best-effort）
func notifyAdminsHTMLReview(item *models.UserHTML, title, content string) {
	var admins []models.User
	models.DB.Where("role = ?", "admin").Find(&admins)
	for _, a := range admins {
		_ = models.PushNotification(0, a.ID, "html_review", title, content,
			"/admin/html-review")
	}
}

// notifyAdminsHTMLQuarantine 预检未通过 → 通知管理员有待处置封存件
func notifyAdminsHTMLQuarantine(item *models.UserHTML) {
	reason := item.ScanBlockReason
	if reason == "" {
		reason = "安全预检命中高危代码"
	}
	notifyAdminsHTMLReview(item, "扩展上传被安全预检拦截",
		fmt.Sprintf("《%s》未通过预检（%s），已原样封存、未解压，待你审核", item.Name, reason))
}

// AdminListHTMLs GET /api/admin/userhtmls?status= — 全站扩展列表（可按审核状态过滤）
func AdminListHTMLs(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	status := strings.TrimSpace(c.Query("status"))
	q := models.DB.Order("id DESC")
	if status != "" {
		q = q.Where("review_status = ?", status)
	}
	var list []models.UserHTML
	if err := q.Find(&list).Error; err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	// 补用户名，便于管理员一眼看出是谁传的
	ids := make([]uint, 0, len(list))
	for _, it := range list {
		ids = append(ids, it.UserID)
	}
	names := map[uint]string{}
	if len(ids) > 0 {
		var users []models.User
		models.DB.Where("id IN ?", ids).Find(&users)
		for _, u := range users {
			names[u.ID] = u.Username
		}
	}
	type row struct {
		models.UserHTML
		Username string `json:"username"`
	}
	rows := make([]row, 0, len(list))
	for _, it := range list {
		rows = append(rows, row{UserHTML: it, Username: names[it.UserID]})
	}
	utils.Success(c, rows)
}

// extractQuarantined 把封存的 ZIP 解压到正式目录，返回 (extractDir, entries, error)
func extractQuarantined(item *models.UserHTML) (string, []utils.ZipEntry, error) {
	data, err := os.ReadFile(item.QuarantinePath)
	if err != nil {
		return "", nil, fmt.Errorf("读取封存件失败: %w", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", nil, fmt.Errorf("封存件不是合法 ZIP: %w", err)
	}
	// 放行时再验一次炸弹（管理员也可能误放，且封存件可被替换）
	if ok, msg := utils.ValidateZipBomb(zr, nil); !ok {
		return "", nil, fmt.Errorf("解压失败: %s", msg)
	}
	safeName := sanitizeFilename(item.Name)
	ts := item.CreatedAt.UnixMilli()
	dir := filepath.Join(config.UserHTMLDir, fmt.Sprintf("%d", item.UserID),
		fmt.Sprintf("%s_%d", safeName, ts))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", nil, err
	}
	entries, err := utils.ExtractZipSafe(zr, dir, nil)
	if err != nil {
		os.RemoveAll(dir)
		return "", nil, err
	}
	if len(entries) == 0 {
		os.RemoveAll(dir)
		return "", nil, fmt.Errorf("封存件中没有有效文件")
	}
	// 保存原始 ZIP 副本（与正常上传保持一致）
	if err := os.WriteFile(filepath.Join(dir, "_original.zip"), data, 0644); err != nil {
		// 失败不阻断：原始件仍在封存目录
		_ = err
	}
	// 锁定目录，禁止后续新增脚本文件
	if err := security.LockDirectory(dir); err == nil {
		security.MarkSandboxLocked(dir)
	}
	return dir, entries, nil
}

// AdminReviewHTML POST /api/admin/userhtmls/:id/review
// body: { action: "approve" | "reject", note?, publish?, purge_file? }
//   - approve：pending → approved（publish=true 时同步上广场）；
//     quarantine → 先补解压再 approved（未解压过的必须解压成功才能放行）
//   - reject：→ rejected；purge_file=true 时连封存的原始 ZIP 一并删除
func AdminReviewHTML(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var item models.UserHTML
	if err := models.DB.First(&item, id).Error; err != nil {
		utils.NotFound(c, "扩展不存在")
		return
	}

	var req struct {
		Action    string `json:"action"`
		Note      string `json:"note"`
		Publish   bool   `json:"publish"`
		PurgeFile bool   `json:"purge_file"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	req.Action = strings.ToLower(strings.TrimSpace(req.Action))
	if req.Action != "approve" && req.Action != "reject" {
		utils.BadRequest(c, "action 只能是 approve 或 reject")
		return
	}

	adminID := currentUserID(c)
	now := time.Now()
	note := strings.TrimSpace(req.Note)

	if req.Action == "reject" {
		updates := map[string]interface{}{
			"review_status":   "rejected",
			"review_note":     note,
			"reviewed_by":     adminID,
			"reviewed_at":     now,
			"is_public":       false,
			"is_downloadable": false,
		}
		if req.PurgeFile && item.QuarantinePath != "" {
			_ = os.Remove(item.QuarantinePath)
			updates["quarantine_path"] = ""
		}
		if err := models.DB.Model(&item).Updates(updates).Error; err != nil {
			utils.InternalError(c, "操作失败")
			return
		}
		utils.Success(c, gin.H{"message": "已驳回", "review_status": "rejected"})
		return
	}

	// ====== approve ======
	if !item.IsExtracted {
		// 封存件必须先补解压（放行即意味着内容可信，需落盘才能运行）
		dir, entries, err := extractQuarantined(&item)
		if err != nil {
			utils.BadRequest(c, "放行失败："+err.Error())
			return
		}
		entry := ""
		for _, e := range entries {
			if strings.HasSuffix(strings.ToLower(e.Name), ".html") {
				entry = e.Name
				break
			}
		}
		if entry == "" && len(entries) > 0 {
			entry = entries[0].Name
		}
		var total int64
		for _, e := range entries {
			total += e.Size
		}
		item.ExtractDir = dir
		item.EntryFile = entry
		item.FileCount = len(entries)
		item.TotalSize = total
		item.FilePath = dir
		item.IsExtracted = true
		item.IsActive = true
	}

	updates := map[string]interface{}{
		"review_status": "approved",
		"review_note":   note,
		"reviewed_by":   adminID,
		"reviewed_at":   now,
		"is_extracted":  item.IsExtracted,
		"extract_dir":   item.ExtractDir,
		"entry_file":    item.EntryFile,
		"file_count":    item.FileCount,
		"total_size":    item.TotalSize,
		"file_path":     item.FilePath,
		"is_active":     true,
	}
	if req.Publish {
		updates["is_public"] = true
	}
	if err := models.DB.Model(&item).Updates(updates).Error; err != nil {
		utils.InternalError(c, "操作失败")
		return
	}
	utils.Success(c, gin.H{
		"message":       "已通过审核",
		"review_status": "approved",
		"is_public":     req.Publish || item.IsPublic,
	})
}

// AdminGetHTMLScan GET /api/admin/userhtmls/:id/scan — 查看某扩展的预检扫描结果
func AdminGetHTMLScan(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var item models.UserHTML
	if err := models.DB.First(&item, id).Error; err != nil {
		utils.NotFound(c, "扩展不存在")
		return
	}
	utils.Success(c, gin.H{
		"id":                item.ID,
		"name":              item.Name,
		"review_status":     item.ReviewStatus,
		"scan_blocked":      item.ScanBlocked,
		"scan_block_reason": item.ScanBlockReason,
		"scan_warnings":     item.ScanWarnings,
		"is_extracted":      item.IsExtracted,
		"quarantine_path":   item.QuarantinePath,
		"review_note":       item.ReviewNote,
	})
}
