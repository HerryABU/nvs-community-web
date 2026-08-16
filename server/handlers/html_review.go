package handlers

import (
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
	Type         string           `json:"type"` // frame / html
	ID           uint             `json:"id"`
	UserID       uint             `json:"user_id"`
	OwnerName    string           `json:"owner_name"`
	Name         string           `json:"name"`
	Description  string           `json:"description"`
	ReviewStatus string           `json:"review_status"`
	ReviewNote   string           `json:"review_note"`
	RunMode      string           `json:"run_mode"`
	IsActive     bool             `json:"is_active"`
	IsPublic     bool             `json:"is_public"`
	Warnings     []security.Threat `json:"warnings"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
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
				Warnings:  security.ThreatsFromJSON(f.ScanWarnings),
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
				CreatedAt: h.CreatedAt, UpdatedAt: h.UpdatedAt,
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
			"warnings": security.ThreatsFromJSON(h.ScanWarnings),
			"entry_file": h.EntryFile,
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

	var req struct {
		Action string `json:"action" binding:"required"` // approve / reject
		Note   string `json:"note"`
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
