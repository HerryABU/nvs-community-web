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
	userID := currentUserID(c)

	var req struct {
		Name         string `json:"name" binding:"required"`
		Description  string `json:"description"`
		NovelID      *uint  `json:"novel_id"`
		BlogID       *uint  `json:"blog_id"`
		Content      string `json:"content" binding:"required"`
		IsPublic     bool   `json:"is_public"`
		Tags         string `json:"tags"`
		HasControls  bool   `json:"has_controls"`
		UsesNovelAPI bool   `json:"uses_novel_api"`
		SandboxLevel string `json:"sandbox_level"`
		FrameType    string `json:"frame_type"`
		RunMode      string `json:"run_mode"`
		FrameWidth   *int   `json:"frame_width"`
		FrameHeight  *int   `json:"frame_height"`
		Position     *string `json:"position"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	// 绑定互斥：一部模板只能绑定小说或博客其一（或均空=全局模板），避免归属歧义
	if req.NovelID != nil && req.BlogID != nil {
		utils.BadRequest(c, "模板只能绑定小说或博客其中之一")
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
	// run_mode 校验：默认 sandbox，仅允许 sandbox / inline
	if req.RunMode == "" {
		req.RunMode = "sandbox"
	}
	if req.RunMode != "sandbox" && req.RunMode != "inline" {
		req.RunMode = "sandbox"
	}
	// position 校验：默认 header，仅允许 header / footer
	if req.Position == nil || (*req.Position != "header" && *req.Position != "footer") {
		req.Position = nil
	}

	userDir := filepath.Join(config.UserFrameDir, fmt.Sprintf("%d", userID))
	os.MkdirAll(userDir, 0755)

	timestamp := time.Now().UnixMilli()
	filename := fmt.Sprintf("%s_%d.html", safeName, timestamp)
	filePath := filepath.Join(userDir, filename)

	// 🔍 宽松自动化安全审查：critical/high 阻断，medium/low 仅告警（转人工复核）
	blocked, _, warnings := security.ClassifyScan(req.Content, safeName+".html")
	if blocked {
		utils.BadRequest(c, "内容安全扫描未通过：检测到高危代码（提权/木马/窃取等），已拒绝保存")
		return
	}
	reviewStatus := "approved"
	scanWarnings := ""
	if len(warnings) > 0 {
		reviewStatus = "pending"
		scanWarnings = security.ThreatsToJSON(warnings)
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
		BlogID:       req.BlogID,
		Name:         req.Name,
		Description:  req.Description,
		FilePath:     filePath,
		IsPublic:     req.IsPublic,
		Tags:         req.Tags,
		HasControls:  req.HasControls,
		UsesNovelAPI: req.UsesNovelAPI,
		SandboxLevel: req.SandboxLevel,
		FrameType:    req.FrameType,
		RunMode:      req.RunMode,
		ReviewStatus: reviewStatus,
		ScanWarnings: scanWarnings,
	}
	// iframe 预览尺寸：未传则保持零值，由列默认值（960/600）兜底
	if req.FrameWidth != nil {
		frame.FrameWidth = *req.FrameWidth
	}
	if req.FrameHeight != nil {
		frame.FrameHeight = *req.FrameHeight
	}
	// position：空值走 DB 默认 header，显式 header/footer 写入
	if req.Position != nil {
		frame.Position = *req.Position
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
	userID := currentUserID(c)
	var frames []models.UserFrame
	models.DB.Where("user_id = ?", userID).Order("updated_at DESC").Find(&frames)
	utils.Success(c, frames)
}

// GetFrame GET /api/userframes/:id
func GetFrame(c *gin.Context) {
	userID := currentUserID(c)
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
	userID := currentUserID(c)
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
		BlogID       *uint   `json:"blog_id"`
		Content      *string `json:"content"`
		IsActive     *bool   `json:"is_active"`
		IsPublic     *bool   `json:"is_public"`
		Tags         *string `json:"tags"`
		HasControls  *bool   `json:"has_controls"`
		UsesNovelAPI *bool   `json:"uses_novel_api"`
		SandboxLevel *string `json:"sandbox_level"`
		FrameType    *string `json:"frame_type"`
		RunMode      *string `json:"run_mode"`
		FrameWidth   *int    `json:"frame_width"`
		FrameHeight  *int    `json:"frame_height"`
		Position     *string `json:"position"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	// 绑定互斥校验（合并 novel_id / blog_id 的最终值后判断）
	finalNovelID := frame.NovelID
	if req.NovelID != nil {
		finalNovelID = req.NovelID
	}
	finalBlogID := frame.BlogID
	if req.BlogID != nil {
		finalBlogID = req.BlogID
	}
	if finalNovelID != nil && finalBlogID != nil {
		utils.BadRequest(c, "模板只能绑定小说或博客其中之一")
		return
	}

	// 🔒 用 Updates(map) 精确写列：map 不会跳过零值，允许把布尔/数值显式设为 0/false，
	// 规避 GORM 「带 default 标签的列遇到零值会被跳过」的坑。
	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.NovelID != nil {
		updates["novel_id"] = *req.NovelID
	}
	if req.BlogID != nil {
		updates["blog_id"] = *req.BlogID
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.IsPublic != nil {
		updates["is_public"] = *req.IsPublic
	}
	if req.Tags != nil {
		updates["tags"] = *req.Tags
	}
	if req.HasControls != nil {
		updates["has_controls"] = *req.HasControls
	}
	if req.UsesNovelAPI != nil {
		updates["uses_novel_api"] = *req.UsesNovelAPI
	}
	if req.SandboxLevel != nil && (*req.SandboxLevel == "strict" || *req.SandboxLevel == "interactive") {
		updates["sandbox_level"] = *req.SandboxLevel
	}
	if req.FrameType != nil && (*req.FrameType == "reader" || *req.FrameType == "author") {
		updates["frame_type"] = *req.FrameType
	}
	if req.RunMode != nil && (*req.RunMode == "sandbox" || *req.RunMode == "inline") {
		updates["run_mode"] = *req.RunMode
	}
	// position：map 写入，显式 header/footer 不被 GORM 零值跳过逻辑丢弃
	if req.Position != nil && (*req.Position == "header" || *req.Position == "footer") {
		updates["position"] = *req.Position
	}
	if req.FrameWidth != nil {
		updates["frame_width"] = *req.FrameWidth
	}
	if req.FrameHeight != nil {
		updates["frame_height"] = *req.FrameHeight
	}

	if req.Content != nil {
		// 🔍 宽松自动化安全审查：critical/high 阻断，medium/low 告警转人工复核
		blocked, _, warnings := security.ClassifyScan(*req.Content, frame.Name+".html")
		if blocked {
			utils.BadRequest(c, "内容安全扫描未通过：检测到高危代码，已拒绝更新")
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
		updates["version"] = frame.Version + 1
		// 更新审查状态与告警
		if len(warnings) > 0 {
			updates["review_status"] = "pending"
			updates["scan_warnings"] = security.ThreatsToJSON(warnings)
			updates["review_note"] = ""
		} else {
			updates["review_status"] = "approved"
			updates["scan_warnings"] = ""
		}
	}

	if len(updates) > 0 {
		if err := models.DB.Model(&frame).Updates(updates).Error; err != nil {
			utils.InternalError(c, "更新模板失败")
			return
		}
	}
	models.DB.Where("id = ?", frame.ID).First(&frame)
	utils.Success(c, frame)
}

// DeleteFrame DELETE /api/userframes/:id
func DeleteFrame(c *gin.Context) {
	userID := currentUserID(c)
	frameID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	var frame models.UserFrame
	if err := models.DB.Where("id = ? AND user_id = ?", frameID, userID).First(&frame).Error; err != nil {
		utils.NotFound(c, "模板不存在")
		return
	}

	// 合规留存：软删除，保留模板文件（不物理移除），可追溯原始内容
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

// GetBlogFrames GET /api/blogs/:id/frames
// 获取博客关联的阅读模版（仅作者本人的模版，用户级隔离；与 GetNovelFrames 对称）
func GetBlogFrames(c *gin.Context) {
	blogID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	// 先查作者
	blog, err := models.GetBlogByID(uint(blogID))
	if err != nil {
		utils.Success(c, []models.UserFrame{})
		return
	}
	var frames []models.UserFrame
	models.DB.Where("user_id = ? AND blog_id = ? AND frame_type = ? AND is_active = ?",
		blog.AuthorID, uint(blogID), "reader", true).
		Order("updated_at DESC").Find(&frames)
	utils.Success(c, frames)
}

// GetAuthorFrames GET /api/author/:id/frames
// 获取作者展现模版（用于作者主页）；:id 兼容数字主键 id 与用户名
func GetAuthorFrames(c *gin.Context) {
	user, err := ResolveUserKey(c.Param("id"))
	if err != nil || user == nil {
		utils.NotFound(c, "作者不存在")
		return
	}
	var frames []models.UserFrame
	models.DB.Where("user_id = ? AND frame_type = ? AND is_active = ?",
		user.ID, "author", true).
		Order("updated_at DESC").Find(&frames)
	utils.Success(c, frames)
}

// ==================== 站内内容引用检索（CSDN式相对链接） ====================

// ContentRef 站内可引用内容（供模板内插入相对链接）
type ContentRef struct {
	Type     string `json:"type"`  // novel / blog / thread
	ID       uint   `json:"id"`
	Title    string `json:"title"`
	AuthorID uint   `json:"author_id,omitempty"` // 博客详情路由为 /author/:id/blogs/:blogId，须带作者 id 才能拼出正确相对路径
}

// listContentRefs 跨小说 / 博客 / 论坛帖检索标题，返回可插入的相对链接目标。
// 仅返回软删除未被标记的记录（GORM 对带 gorm.DeletedAt 的模型自动过滤）。
func listContentRefs(query string) ([]ContentRef, error) {
	q := strings.TrimSpace(query)
	like := "%" + q + "%"
	const perType = 8
	refs := make([]ContentRef, 0, perType*3)

	// 小说
	var novels []models.Novel
	nq := models.DB.Select("id, title").Limit(perType)
	if q != "" {
		nq = nq.Where("title LIKE ?", like)
	}
	if err := nq.Find(&novels).Error; err == nil {
		for _, n := range novels {
			refs = append(refs, ContentRef{Type: "novel", ID: n.ID, Title: n.Title})
		}
	}

	// 博客（详情路由为 /author/:authorId/blogs/:blogId，须带 author_id）
	var blogs []models.AuthorBlog
	bq := models.DB.Select("id, title, author_id").Limit(perType)
	if q != "" {
		bq = bq.Where("title LIKE ?", like)
	}
	if err := bq.Find(&blogs).Error; err == nil {
		for _, b := range blogs {
			refs = append(refs, ContentRef{Type: "blog", ID: b.ID, Title: b.Title, AuthorID: b.AuthorID})
		}
	}

	// 论坛帖
	var threads []models.Thread
	tq := models.DB.Select("id, title").Limit(perType)
	if q != "" {
		tq = tq.Where("title LIKE ?", like)
	}
	if err := tq.Find(&threads).Error; err == nil {
		for _, t := range threads {
			refs = append(refs, ContentRef{Type: "thread", ID: t.ID, Title: t.Title})
		}
	}

	// 总量限制 ~20（前端 picker 展示足够）
	if len(refs) > 20 {
		refs = refs[:20]
	}
	return refs, nil
}

// ListContentRefs GET /api/frame/refs?q=
// 供前端 ContentRefPicker 检索站内内容，返回 {type,id,title}。
func ListContentRefs(c *gin.Context) {
	refs, err := listContentRefs(c.Query("q"))
	if err != nil {
		utils.InternalError(c, "检索站内内容失败")
		return
	}
	utils.Success(c, refs)
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
