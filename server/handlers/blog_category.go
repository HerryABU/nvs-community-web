package handlers

import (
	"strconv"
	"strings"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

func trimSpace(s string) string { return strings.TrimSpace(s) }

// ─────────────────────────────────────────────────────────────
// 博客分类（一级/二级，CSDN 式）+ 博客点赞
// ─────────────────────────────────────────────────────────────

// GET /api/blog-categories — 公开分类树（含博客数）
func ListBlogCategories(c *gin.Context) {
	utils.Success(c, models.GetBlogCategoryTree())
}

// GET /api/blogs/like/status — 兼容：博客点赞状态（供详情页使用）
// GET /api/blogs/:id/likes — 点赞数 + 当前用户是否已赞
func GetBlogLikes(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if id == 0 {
		utils.BadRequest(c, "无效的博客ID")
		return
	}
	userID := currentUserID(c)
	var count int64
	models.DB.Model(&models.BlogLike{}).Where("blog_id = ?", id).Count(&count)
	utils.Success(c, gin.H{
		"count": count,
		"liked": models.UserLikedBlog(uint(id), userID),
	})
}

// POST /api/blogs/:id/like — 点赞/取消点赞（toggle）
func ToggleBlogLike(c *gin.Context) {
	userID := currentUserID(c)
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if id == 0 {
		utils.BadRequest(c, "无效的博客ID")
		return
	}
	blog, err := models.GetBlogByID(uint(id))
	if err != nil {
		utils.NotFound(c, "博客不存在")
		return
	}
	liked, err := models.ToggleBlogLike(blog.ID, userID)
	if err != nil {
		utils.InternalError(c, "操作失败")
		return
	}
	var count int64
	models.DB.Model(&models.BlogLike{}).Where("blog_id = ?", blog.ID).Count(&count)
	utils.Success(c, gin.H{"liked": liked, "count": count})
}

// ─────────────────────────────────────────────────────────────
// 管理员分类管理
// ─────────────────────────────────────────────────────────────

// GET /api/admin/blog-categories — 分类树（管理员）
func AdminListBlogCategories(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	utils.Success(c, models.GetBlogCategoryTree())
}

// POST /api/admin/blog-categories — 新建分类 { name, parent_id?, sort_order? }
func AdminCreateBlogCategory(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		Name      string `json:"name" binding:"required,max=64"`
		ParentID  uint   `json:"parent_id"`
		SortOrder int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请填写分类名称")
		return
	}
	req.Name = trimSpace(req.Name)
	if req.Name == "" {
		utils.BadRequest(c, "请填写分类名称")
		return
	}
	// 只允许二级：parent_id 必须是已存在的一级分类
	if req.ParentID != 0 {
		var p models.BlogCategory
		if err := models.DB.First(&p, req.ParentID).Error; err != nil || p.ParentID != 0 {
			utils.BadRequest(c, "二级分类只能挂在已有的一级分类下")
			return
		}
	}
	var cnt int64
	models.DB.Model(&models.BlogCategory{}).Where("name = ? AND parent_id = ?", req.Name, req.ParentID).Count(&cnt)
	if cnt > 0 {
		utils.BadRequest(c, "同名分类已存在")
		return
	}
	cat := &models.BlogCategory{Name: req.Name, ParentID: req.ParentID, SortOrder: req.SortOrder}
	if err := models.DB.Create(cat).Error; err != nil {
		utils.InternalError(c, "创建失败")
		return
	}
	utils.Success(c, cat)
}

// PUT /api/admin/blog-categories/:id — 修改分类
func AdminUpdateBlogCategory(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var cat models.BlogCategory
	if err := models.DB.First(&cat, id).Error; err != nil {
		utils.NotFound(c, "分类不存在")
		return
	}
	var req struct {
		Name      string `json:"name"`
		ParentID  uint   `json:"parent_id"`
		SortOrder *int   `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	updates := map[string]interface{}{}
	if req.Name != "" {
		name := trimSpace(req.Name)
		if name == "" {
			utils.BadRequest(c, "分类名称不能为空")
			return
		}
		var cnt int64
		models.DB.Model(&models.BlogCategory{}).
			Where("name = ? AND parent_id = ? AND id <> ?", name, cat.ParentID, cat.ID).Count(&cnt)
		if cnt > 0 {
			utils.BadRequest(c, "同名分类已存在")
			return
		}
		updates["name"] = name
	}
	// 改父级：仅允许在 0 与一级分类之间切换，且不能把自己挂到自己/子分类下
	if req.ParentID != cat.ParentID {
		if req.ParentID != 0 {
			var p models.BlogCategory
			if err := models.DB.First(&p, req.ParentID).Error; err != nil || p.ParentID != 0 {
				utils.BadRequest(c, "二级分类只能挂在已有的一级分类下")
				return
			}
			if req.ParentID == cat.ID {
				utils.BadRequest(c, "不能将分类挂到自己下面")
				return
			}
		}
		// 一级分类不能降为二级
		if cat.ParentID == 0 && req.ParentID != 0 {
			utils.BadRequest(c, "一级分类下已有子分类，不能改为二级；请先移动子分类")
			return
		}
		updates["parent_id"] = req.ParentID
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}
	if len(updates) == 0 {
		utils.BadRequest(c, "没有需要更新的内容")
		return
	}
	if err := models.DB.Model(&cat).Updates(updates).Error; err != nil {
		utils.InternalError(c, "更新失败")
		return
	}
	models.DB.First(&cat, id)
	utils.Success(c, cat)
}

// DELETE /api/admin/blog-categories/:id — 删除分类（其下博客回到未分类）
func AdminDeleteBlogCategory(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var cat models.BlogCategory
	if err := models.DB.First(&cat, id).Error; err != nil {
		utils.NotFound(c, "分类不存在")
		return
	}
	if models.BlogCategoryHasChildren(cat.ID) {
		utils.BadRequest(c, "该一级分类下还有二级分类，请先删除子分类")
		return
	}
	models.DB.Model(&models.AuthorBlog{}).Where("category_id = ?", cat.ID).Update("category_id", 0)
	if err := models.DB.Delete(&cat).Error; err != nil {
		utils.InternalError(c, "删除失败")
		return
	}
	utils.Success(c, gin.H{"message": "已删除，该分类下的博客已回到未分类"})
}

// fillBlogCategories 为博客列表填充分类名（含父级前缀）
func fillBlogCategories(blogs []models.AuthorBlog) {
	for i := range blogs {
		blogs[i].Category = models.GetBlogCategoryName(blogs[i].CategoryID)
	}
}
