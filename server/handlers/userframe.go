package handlers

import (
	"fmt"
	"os"
	"path"
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

// ==================== UserFrame 模板管理 ====================

// CreateFrame POST /api/userframes
func CreateFrame(c *gin.Context) {
	userID := c.GetUint("user_id")

	var req struct {
		Name         string `json:"name" binding:"required"`
		Description  string `json:"description"`
		NovelID      *uint  `json:"novel_id"`
		Content      string `json:"content" binding:"required"`
		IsPublic     bool   `json:"is_public"`
		Tags         string `json:"tags"`
		HasControls  bool   `json:"has_controls"`
		UsesNovelAPI bool   `json:"uses_novel_api"`
		SandboxLevel string `json:"sandbox_level"`
		FrameType    string `json:"frame_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	safeName := sanitizeFilename(req.Name)
	if safeName == "" {
		utils.BadRequest(c, "模板名称不合法")
		return
	}

	// sandbox_level 校验
	if req.SandboxLevel == "" {
		req.SandboxLevel = "strict"
	}
	if req.SandboxLevel != "strict" && req.SandboxLevel != "interactive" {
		req.SandboxLevel = "strict"
	}
	if req.FrameType != "reader" && req.FrameType != "author" {
		req.FrameType = "reader"
	}

	userDir := filepath.Join(config.UserFrameDir, fmt.Sprintf("%d", userID))
	os.MkdirAll(userDir, 0755)

	timestamp := time.Now().UnixMilli()
	filename := fmt.Sprintf("%s_%d.html", safeName, timestamp)
	filePath := filepath.Join(userDir, filename)

	// 🔍 内容安全扫描
	if passed, reason := security.ScanContentStrict(req.Content, safeName+".html"); !passed {
		utils.BadRequest(c, reason)
		return
	}

	// 注入模板API桥接代码（如果使用平台API）
	content := req.Content
	if req.UsesNovelAPI {
		content = injectTemplateAPI(content)
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		utils.InternalError(c, "保存模板文件失败")
		return
	}

	frame := &models.UserFrame{
		UserID:       userID,
		NovelID:      req.NovelID,
		Name:         req.Name,
		Description:  req.Description,
		FilePath:     filePath,
		IsPublic:     req.IsPublic,
		Tags:         req.Tags,
		HasControls:  req.HasControls,
		UsesNovelAPI: req.UsesNovelAPI,
		SandboxLevel: req.SandboxLevel,
		FrameType:    req.FrameType,
	}
	if err := models.DB.Create(frame).Error; err != nil {
		os.Remove(filePath)
		utils.InternalError(c, "创建模板记录失败")
		return
	}

	utils.Success(c, frame)
}

// ListFrames GET /api/userframes
func ListFrames(c *gin.Context) {
	userID := c.GetUint("user_id")
	var frames []models.UserFrame
	models.DB.Where("user_id = ?", userID).Order("updated_at DESC").Find(&frames)
	utils.Success(c, frames)
}

// GetFrame GET /api/userframes/:id
func GetFrame(c *gin.Context) {
	userID := c.GetUint("user_id")
	frameID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	var frame models.UserFrame
	if err := models.DB.Where("id = ? AND user_id = ?", frameID, userID).First(&frame).Error; err != nil {
		utils.NotFound(c, "模板不存在")
		return
	}

	content, err := os.ReadFile(frame.FilePath)
	if err != nil {
		utils.InternalError(c, "读取模板文件失败")
		return
	}

	utils.Success(c, gin.H{
		"frame":   frame,
		"content": string(content),
	})
}

// UpdateFrame PUT /api/userframes/:id
func UpdateFrame(c *gin.Context) {
	userID := c.GetUint("user_id")
	frameID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	var frame models.UserFrame
	if err := models.DB.Where("id = ? AND user_id = ?", frameID, userID).First(&frame).Error; err != nil {
		utils.NotFound(c, "模板不存在")
		return
	}

	var req struct {
		Name         *string `json:"name"`
		Description  *string `json:"description"`
		NovelID      *uint   `json:"novel_id"`
		Content      *string `json:"content"`
		IsActive     *bool   `json:"is_active"`
		IsPublic     *bool   `json:"is_public"`
		Tags         *string `json:"tags"`
		HasControls  *bool   `json:"has_controls"`
		UsesNovelAPI *bool   `json:"uses_novel_api"`
		SandboxLevel *string `json:"sandbox_level"`
		FrameType    *string `json:"frame_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	applyIfNotNil(req.Name, &frame.Name)
	applyIfNotNil(req.Description, &frame.Description)
	if req.NovelID != nil {
		frame.NovelID = req.NovelID
	}
	applyIfNotNil(req.IsActive, &frame.IsActive)
	applyIfNotNil(req.IsPublic, &frame.IsPublic)
	applyIfNotNil(req.Tags, &frame.Tags)
	applyIfNotNil(req.HasControls, &frame.HasControls)
	applyIfNotNil(req.UsesNovelAPI, &frame.UsesNovelAPI)
	if req.SandboxLevel != nil && (*req.SandboxLevel == "strict" || *req.SandboxLevel == "interactive") {
		frame.SandboxLevel = *req.SandboxLevel
	}
	if req.FrameType != nil && (*req.FrameType == "reader" || *req.FrameType == "author") {
		frame.FrameType = *req.FrameType
	}

	if req.Content != nil {
		// 🔍 内容安全扫描
		if passed, reason := security.ScanContentStrict(*req.Content, frame.Name+".html"); !passed {
			utils.BadRequest(c, reason)
			return
		}
		content := *req.Content
		if frame.UsesNovelAPI {
			content = injectTemplateAPI(content)
		}
		if err := os.WriteFile(frame.FilePath, []byte(content), 0644); err != nil {
			utils.InternalError(c, "更新模板文件失败")
			return
		}
		frame.Version++
	}

	if err := models.DB.Save(&frame).Error; err != nil {
		utils.InternalError(c, "更新模板失败")
		return
	}
	utils.Success(c, frame)
}

// DeleteFrame DELETE /api/userframes/:id
func DeleteFrame(c *gin.Context) {
	userID := c.GetUint("user_id")
	frameID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	var frame models.UserFrame
	if err := models.DB.Where("id = ? AND user_id = ?", frameID, userID).First(&frame).Error; err != nil {
		utils.NotFound(c, "模板不存在")
		return
	}

	os.Remove(frame.FilePath)
	models.DB.Delete(&frame)
	utils.Success(c, gin.H{"message": "模板已删除"})
}

// ListPublicFrames GET /api/userframes/public
func ListPublicFrames(c *gin.Context) {
	var frames []models.UserFrame
	models.DB.Where("is_public = ? AND is_active = ?", true, true).
		Order("updated_at DESC").Limit(50).Find(&frames)
	utils.Success(c, frames)
}

// GetNovelFrames GET /api/novels/:id/frames
// 获取作品关联的阅读模版（仅作者本人的模版，用户级隔离）
func GetNovelFrames(c *gin.Context) {
	novelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	// 先查作者
	novel, err := models.GetNovelByID(uint(novelID))
	if err != nil {
		utils.Success(c, []models.UserFrame{})
		return
	}
	var frames []models.UserFrame
	models.DB.Where("user_id = ? AND novel_id = ? AND frame_type = ? AND is_active = ?",
		novel.AuthorID, uint(novelID), "reader", true).
		Order("updated_at DESC").Find(&frames)
	utils.Success(c, frames)
}

// GetAuthorFrames GET /api/author/:id/frames
// 获取作者展现模版（用于作者主页）
func GetAuthorFrames(c *gin.Context) {
	authorID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var frames []models.UserFrame
	models.DB.Where("user_id = ? AND frame_type = ? AND is_active = ?",
		uint(authorID), "author", true).
		Order("updated_at DESC").Find(&frames)
	utils.Success(c, frames)
}

// ==================== 模板格式API ====================

// GetTemplateAPI 获取平台小说数据格式API（供模板调用）
// GET /api/template/novel/:id
// 返回小说元数据，供模板中的JS通过 postMessage 获取
func GetTemplateAPI(c *gin.Context) {
	novelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	novel, err := models.GetNovelByID(uint(novelID))
	if err != nil {
		utils.NotFound(c, "作品不存在")
		return
	}

	chapters, _ := models.GetChaptersByNovel(uint(novelID))

	utils.Success(c, gin.H{
		"novel": gin.H{
			"id":             novel.ID,
			"title":          novel.Title,
			"author":         novel.Author.Nickname,
			"author_id":      novel.AuthorID,
			"category":       novel.Category,
			"summary":        novel.Summary,
			"cover_url":      novel.CoverURL,
			"total_chapters": novel.TotalChapters,
			"total_words":    novel.TotalWords,
			"view_count":     novel.ViewCount,
			"status":         novel.Status,
			"created_at":     novel.CreatedAt.Format(time.RFC3339),
			"updated_at":     novel.UpdatedAt.Format(time.RFC3339),
		},
		"chapters": chapters,
	})
}

// ==================== 辅助 ====================

// injectTemplateAPI 向模板HTML中注入平台API桥接脚本
// 提供 window.NVS.getNovelData() 和 postMessage 通信机制
func injectTemplateAPI(html string) string {
	apiScript := `
<!-- NVS 平台模板 API 桥接（自动注入） -->
<script>
(function() {
  // NVS 平台API命名空间
  window.NVS = window.NVS || {};

  // 获取小说数据（自动从URL参数中提取 novel=xxx）
  window.NVS.getNovelData = function(novelId) {
    var id = novelId || (new URLSearchParams(location.search)).get('novel') || '';
    return fetch('/api/template/novel/' + id)
      .then(function(r) { return r.json(); })
      .then(function(d) { return d.data; });
  };

  // 向父页面发送消息
  window.NVS.sendMessage = function(type, data) {
    window.parent.postMessage({ source: 'nvs-frame', type: type, data: data }, '*');
  };

  // 监听父页面消息
  window.addEventListener('message', function(e) {
    if (e.data && e.data.source === 'nvs-parent') {
      window.dispatchEvent(new CustomEvent('nvs-message', { detail: e.data }));
    }
  });

  // 获取当前阅读章节
  window.NVS.getCurrentChapter = function() {
    var m = location.search.match(/chapter=(\d+)/);
    return m ? parseInt(m[1]) : 1;
  };

  console.log('[NVS] 模板API已就绪 · 沙盒模式');
})();
</script>`

	// 注入到 </head> 之前或 <body> 之后
	if idx := strings.Index(html, "</head>"); idx != -1 {
		return html[:idx] + apiScript + html[idx:]
	}
	if idx := strings.Index(html, "<body"); idx != -1 {
		return html[:idx] + apiScript + html[idx:]
	}
	return apiScript + html
}

func applyIfNotNil[T any](src *T, dst *T) {
	if src != nil {
		*dst = *src
	}
}

func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_",
		"?", "_", "\"", "_", "<", "_", ">", "_",
		"|", "_", " ", "_",
	)
	safe := replacer.Replace(name)
	safe = strings.Trim(safe, "._")
	if safe == "" {
		safe = "untitled"
	}
	if len(safe) > 64 {
		safe = safe[:64]
	}
	return safe
}

// Ensure these are used
var _ = path.Base
var _ = strings.HasPrefix
