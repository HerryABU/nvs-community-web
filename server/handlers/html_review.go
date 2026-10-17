package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"nvs-server/models"
	"nvs-server/security"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// HTMLReviewItem 统一的管理员审查条目（UserFrame + UserHTML）
type HTMLReviewItem struct {
	Type         string            `json:"type"` // frame / html
	ID           uint              `json:"id"`
	UserID       uint              `json:"user_id"`
	OwnerName    string            `json:"owner_name"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	ReviewStatus string            `json:"review_status"`
	ReviewNote   string            `json:"review_note"`
	RunMode      string            `json:"run_mode"`
	IsActive     bool              `json:"is_active"`
	IsPublic     bool              `json:"is_public"`
	Warnings     []security.Threat `json:"warnings"`
	// WASM：允许加载 WASM 的条目为不可读二进制，审查时需重点关注
	AllowWasm bool      `json:"allow_wasm"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// 封存态（仅 html：预检未通过 → 原样封存 ZIP，未解压）
	IsExtracted     bool   `json:"is_extracted"`
	ScanBlocked     bool   `json:"scan_blocked"`
	ScanBlockReason string `json:"scan_block_reason"`
	QuarantinePath  string `json:"quarantine_path"`
}

// ListHTMLReviews GET /api/admin/html-reviews — 列出所有自定义 HTML 内容（含审查状态）
func ListHTMLReviews(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	status := c.Query("status")
	kind := c.Query("type") // frame / html / 空=全部

	var items []HTMLReviewItem

	if kind == "" || kind == "frame" {
		var frames []models.UserFrame
		q := models.DB.Preload("User").Order("updated_at DESC")
		if status != "" {
			q = q.Where("review_status = ?", status)
		}
		q.Find(&frames)
		for _, f := range frames {
			items = append(items, HTMLReviewItem{
				Type: "frame", ID: f.ID, UserID: f.UserID,
				OwnerName: ownerName(f.User), Name: f.Name, Description: f.Description,
				ReviewStatus: f.ReviewStatus, ReviewNote: f.ReviewNote, RunMode: f.RunMode,
				IsActive: f.IsActive, IsPublic: f.IsPublic,
				Warnings: security.ThreatsFromJSON(f.ScanWarnings),
				// 主题模板为单文件渲染，无 WASM 概念，AllowWasm 保持 false
				CreatedAt: f.CreatedAt, UpdatedAt: f.UpdatedAt,
			})
		}
	}

	if kind == "" || kind == "html" {
		var htmls []models.UserHTML
		q := models.DB.Preload("User").Order("updated_at DESC")
		if status != "" {
			q = q.Where("review_status = ?", status)
		}
		q.Find(&htmls)
		for _, h := range htmls {
			items = append(items, HTMLReviewItem{
				Type: "html", ID: h.ID, UserID: h.UserID,
				OwnerName: ownerName(h.User), Name: h.Name, Description: h.Description,
				ReviewStatus: h.ReviewStatus, ReviewNote: h.ReviewNote, RunMode: h.RunMode,
				IsActive: h.IsActive, IsPublic: h.IsPublic,
				Warnings:  security.ThreatsFromJSON(h.ScanWarnings),
				AllowWasm: h.AllowWasm,
				CreatedAt: h.CreatedAt, UpdatedAt: h.UpdatedAt,
				IsExtracted: h.IsExtracted, ScanBlocked: h.ScanBlocked,
				ScanBlockReason: h.ScanBlockReason, QuarantinePath: h.QuarantinePath,
			})
		}
	}

	utils.Success(c, items)
}

// GetHTMLReview GET /api/admin/html-reviews/:type/:id — 获取待审查内容全文
func GetHTMLReview(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	kind := c.Param("type")
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	var content string
	var meta gin.H

	switch kind {
	case "frame":
		var f models.UserFrame
		if err := models.DB.First(&f, id).Error; err != nil {
			utils.NotFound(c, "模板不存在")
			return
		}
		b, err := os.ReadFile(f.FilePath)
		if err != nil {
			utils.InternalError(c, "读取内容失败")
			return
		}
		content = string(b)
		meta = gin.H{
			"type": "frame", "id": f.ID, "name": f.Name, "user_id": f.UserID,
			"review_status": f.ReviewStatus, "review_note": f.ReviewNote, "run_mode": f.RunMode,
			"is_active": f.IsActive, "is_public": f.IsPublic,
			"warnings": security.ThreatsFromJSON(f.ScanWarnings),
		}
	case "html":
		var h models.UserHTML
		if err := models.DB.First(&h, id).Error; err != nil {
			utils.NotFound(c, "扩展不存在")
			return
		}
		// 封存件（预检未通过、未解压）：不读入口文件，改为回显阻断原因，
		// 让管理员在放行前就能看到为什么被拦、是否误判。
		if !h.IsExtracted {
			content = fmt.Sprintf(
				"[该扩展未解压]\n\n安全预检未通过，ZIP 已原样封存、未解压。\n阻断原因：%s\n封存路径：%s\n\n放行后将按封存的 ZIP 补一次解压（会再验一次 ZIP 炸弹）。",
				h.ScanBlockReason, h.QuarantinePath)
			meta = gin.H{
				"type": "html", "id": h.ID, "name": h.Name, "user_id": h.UserID,
				"review_status": h.ReviewStatus, "review_note": h.ReviewNote, "run_mode": h.RunMode,
				"is_active": h.IsActive, "is_public": h.IsPublic,
				"warnings":          security.ThreatsFromJSON(h.ScanWarnings),
				"entry_file":        h.EntryFile,
				"is_extracted":      false,
				"scan_blocked":      h.ScanBlocked,
				"scan_block_reason": h.ScanBlockReason,
				"quarantine_path":   h.QuarantinePath,
			}
			utils.Success(c, gin.H{"meta": meta, "content": content})
			return
		}
		b, err := os.ReadFile(filepath.Join(h.ExtractDir, h.EntryFile))
		if err != nil {
			utils.InternalError(c, "读取入口文件失败")
			return
		}
		content = string(b)
		meta = gin.H{
			"type": "html", "id": h.ID, "name": h.Name, "user_id": h.UserID,
			"review_status": h.ReviewStatus, "review_note": h.ReviewNote, "run_mode": h.RunMode,
			"is_active": h.IsActive, "is_public": h.IsPublic,
			"warnings":          security.ThreatsFromJSON(h.ScanWarnings),
			"entry_file":        h.EntryFile,
			"is_extracted":      true,
			"scan_blocked":      h.ScanBlocked,
			"scan_block_reason": h.ScanBlockReason,
		}
	default:
		utils.BadRequest(c, "无效的内容类型")
		return
	}

	utils.Success(c, gin.H{"meta": meta, "content": content})
}

// ReviewHTML POST /api/admin/html-reviews/:type/:id/review — 人工复核（通过/驳回）
func ReviewHTML(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	kind := c.Param("type")
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	// 封存件（html，预检未通过、未解压）直接委托专用流程。
	// ⚠️ 必须在下方 ShouldBindJSON 之前判断并委托：请求体只能被消费一次，
	// 若先在此解析再调用 AdminReviewHTML，后者再读 body 只会拿到空内容 → 报「参数错误」。
	if kind == "html" {
		var h models.UserHTML
		if err := models.DB.First(&h, id).Error; err == nil && !h.IsExtracted {
			AdminReviewHTML(c)
			return
		}
	}

	var req struct {
		Action    string `json:"action" binding:"required"` // approve / reject
		Note      string `json:"note"`
		Publish   bool   `json:"publish"`    // 放行时是否同步上广场
		PurgeFile bool   `json:"purge_file"` // 驳回时是否连封存件一起删除
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if req.Action != "approve" && req.Action != "reject" {
		utils.BadRequest(c, "无效的操作")
		return
	}

	status := "approved"
	if req.Action == "reject" {
		status = "rejected"
	}

	note := security.SanitizePlainText(req.Note)

	switch kind {
	case "frame":
		updates := map[string]interface{}{
			"review_status": status,
			"review_note":   note,
		}
		if req.Action == "reject" {
			updates["is_active"] = false
		}
		if err := models.DB.Model(&models.UserFrame{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			utils.InternalError(c, "更新失败")
			return
		}
	case "html":
		updates := map[string]interface{}{
			"review_status": status,
			"review_note":   note,
		}
		if req.Action == "reject" {
			updates["is_active"] = false
		}
		if err := models.DB.Model(&models.UserHTML{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			utils.InternalError(c, "更新失败")
			return
		}
	default:
		utils.BadRequest(c, "无效的内容类型")
		return
	}

	utils.Success(c, gin.H{"message": "已复核"})
}

// ownerName 获取用户显示名
func ownerName(u *models.User) string {
	if u == nil {
		return "未知"
	}
	if u.Nickname != "" {
		return u.Nickname
	}
	return u.Username
}
