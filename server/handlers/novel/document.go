package novel

import (
	"nvs-server/handlers/hctx"
	"nvs-server/models"
	"nvs-server/models/social"
	"nvs-server/utils/infra"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// ==================== 小说文档（设定集 / 资料） ====================
// 与小说域绑定：作者可放置设定集、大纲、人物小传、时间线、故事线、资料等，
// 可控制对读者是否公开（visibility）。读者端只读 public。

// canManageNovelDoc 小说作者本人或平台管理员可维护文档
func canManageNovelDoc(c *gin.Context, novel *models.Novel) bool {
	userID := hctx.CurrentUserID(c)
	if userID == 0 {
		return false
	}
	if social.IsElevated(hctx.CurrentUserRole(c)) {
		return true
	}
	return novel != nil && novel.AuthorID == userID
}

// ListDocuments GET /api/novels/:id/documents — 文档列表
// 作者/管理员看全部；其他登录用户仅看 public。
func ListDocuments(c *gin.Context) {
	novelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	novel, err := models.GetNovelByID(uint(novelID))
	if err != nil {
		infra.NotFound(c, "小说不存在")
		return
	}
	list, err := models.ListNovelDocuments(uint(novelID))
	if err != nil {
		infra.InternalError(c, "查询失败")
		return
	}
	if !canManageNovelDoc(c, novel) {
		out := make([]models.NovelDocument, 0, len(list))
		for _, d := range list {
			if d.Visibility == "public" {
				out = append(out, d)
			}
		}
		list = out
	}
	infra.Success(c, gin.H{"list": list})
}

// CreateDocument POST /api/novels/:id/documents — 新建文档
func CreateDocument(c *gin.Context) {
	novelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	novel, err := models.GetNovelByID(uint(novelID))
	if err != nil {
		infra.NotFound(c, "小说不存在")
		return
	}
	if !canManageNovelDoc(c, novel) {
		infra.Forbidden(c, "只有作者或管理员可以维护文档")
		return
	}
	var req struct {
		Kind       string `json:"kind"`
		Title      string `json:"title"`
		Content    string `json:"content"`
		Visibility string `json:"visibility"`
		Status     string `json:"status"`
		SortOrder  *int   `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		kind = models.DocKindSetting
	}
	if !models.IsValidDocKind(kind) {
		infra.BadRequest(c, "不支持的文档类型")
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		infra.BadRequest(c, "标题不能为空")
		return
	}
	visibility := strings.TrimSpace(req.Visibility)
	if visibility == "" {
		visibility = "public"
	}
	if visibility != "public" && visibility != "private" {
		infra.BadRequest(c, "可见性只能是 public 或 private")
		return
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "published"
	}
	doc := &models.NovelDocument{
		NovelID: uint(novelID), Kind: kind, Title: title,
		Content: req.Content, Visibility: visibility, Status: status,
	}
	if req.SortOrder != nil {
		doc.SortOrder = *req.SortOrder
	}
	if err := models.CreateNovelDocument(doc); err != nil {
		infra.InternalError(c, "创建失败")
		return
	}
	infra.Success(c, doc)
}

// UpdateDocument PUT /api/novels/:id/documents/:did — 更新文档
func UpdateDocument(c *gin.Context) {
	novelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	docID, _ := strconv.ParseUint(c.Param("did"), 10, 64)
	novel, err := models.GetNovelByID(uint(novelID))
	if err != nil {
		infra.NotFound(c, "小说不存在")
		return
	}
	if !canManageNovelDoc(c, novel) {
		infra.Forbidden(c, "只有作者或管理员可以维护文档")
		return
	}
	var req struct {
		Kind       *string `json:"kind"`
		Title      *string `json:"title"`
		Content    *string `json:"content"`
		Visibility *string `json:"visibility"`
		Status     *string `json:"status"`
		SortOrder  *int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	fields := map[string]interface{}{}
	if req.Kind != nil {
		kind := strings.TrimSpace(*req.Kind)
		if !models.IsValidDocKind(kind) {
			infra.BadRequest(c, "不支持的文档类型")
			return
		}
		fields["kind"] = kind
	}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			infra.BadRequest(c, "标题不能为空")
			return
		}
		fields["title"] = title
	}
	if req.Content != nil {
		fields["content"] = *req.Content
	}
	if req.Visibility != nil {
		v := strings.TrimSpace(*req.Visibility)
		if v != "public" && v != "private" {
			infra.BadRequest(c, "可见性只能是 public 或 private")
			return
		}
		fields["visibility"] = v
	}
	if req.Status != nil {
		fields["status"] = strings.TrimSpace(*req.Status)
	}
	if req.SortOrder != nil {
		fields["sort_order"] = *req.SortOrder
	}
	if len(fields) == 0 {
		infra.BadRequest(c, "没有要更新的内容")
		return
	}
	if err := models.UpdateNovelDocumentFields(uint(docID), uint(novelID), fields); err != nil {
		infra.InternalError(c, "更新失败")
		return
	}
	doc, _ := models.GetNovelDocument(uint(docID))
	infra.Success(c, doc)
}

// DeleteDocument DELETE /api/novels/:id/documents/:did — 删除文档
func DeleteDocument(c *gin.Context) {
	novelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	docID, _ := strconv.ParseUint(c.Param("did"), 10, 64)
	novel, err := models.GetNovelByID(uint(novelID))
	if err != nil {
		infra.NotFound(c, "小说不存在")
		return
	}
	if !canManageNovelDoc(c, novel) {
		infra.Forbidden(c, "只有作者或管理员可以维护文档")
		return
	}
	if err := models.DeleteNovelDocument(uint(docID), uint(novelID)); err != nil {
		infra.InternalError(c, "删除失败")
		return
	}
	infra.Success(c, gin.H{"message": "已删除"})
}
