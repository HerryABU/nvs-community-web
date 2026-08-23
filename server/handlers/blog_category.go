package handlers

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// trimSpace 去除首尾空白
func trimSpace(s string) string { return strings.TrimSpace(s) }

// collectDescendants 收集 id 分类的全部子孙 id（防止移动成环）
func collectDescendants(id uint, out *[]uint) {
	var kids []models.BlogCategory
	models.DB.Where("parent_id = ?", id).Find(&kids)
	for _, k := range kids {
		*out = append(*out, k.ID)
		collectDescendants(k.ID, out)
	}
}

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
// 博客分类管理（登录用户即可创建；普通作者创建=待审核，管理员创建=已通过）
// 权限：创建者可编辑/删除自己的分类；管理员可管理全部并审核。
// 公开树接口 ListBlogCategories 见文件顶部。
// ─────────────────────────────────────────────────────────────

// requireLogin 分类管理需要登录（作者/管理员均可）
func requireLogin(c *gin.Context) bool {
	if currentUserID(c) == 0 {
		utils.Unauthorized(c, "请先登录")
		return false
	}
	return true
}

// canManageCategory 当前用户能否管理该分类（创建者本人 或 管理员）
func canManageCategory(c *gin.Context, cat *models.BlogCategory) bool {
	uid := currentUserID(c)
	if uid == 0 {
		return false
	}
	if isAdminUser(c) {
		return true
	}
	return cat.CreatedBy == uid
}

// GET /api/blog-categories/manage — 登录用户分类管理视图
// { platform: 已通过分类树, mine: 自己创建的分类（含待审核/已拒绝，带全名与状态） }
func ListMyBlogCategories(c *gin.Context) {
	if !requireLogin(c) {
		return
	}
	uid := currentUserID(c)
	mine := models.GetMyCategories(uid)
	mineItems := make([]gin.H, 0, len(mine))
	for _, m := range mine {
		mineItems = append(mineItems, gin.H{
			"id":            m.ID,
			"parent_id":     m.ParentID,
			"name":          m.Name,
			"sort_order":    m.SortOrder,
			"status":        m.Status,
			"reject_reason": m.RejectReason,
			"full_name":     models.GetBlogCategoryName(m.ID),
		})
	}
	utils.Success(c, gin.H{
		"platform": models.GetBlogCategoryTree(),
		"mine":     mineItems,
	})
}

// POST /api/blog-categories — 新建分类 { name, parent_id?, sort_order? }
// 普通作者创建 → status=pending（待管理员事后审核）；管理员创建 → approved
func CreateBlogCategory(c *gin.Context) {
	if !requireLogin(c) {
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
	// 层级校验：最多三级（一级挂 0，二级挂一级，三级挂二级）；三级不能再挂子分类
	if req.ParentID != 0 {
		var p models.BlogCategory
		if err := models.DB.First(&p, req.ParentID).Error; err != nil {
			utils.BadRequest(c, "上级分类不存在")
			return
		}
		if p.ParentID != 0 {
			// 父分类是二级/三级：检查其父级是否为一级（父=一级→当前为三级；父=二级→父已三级，拒绝）
			var pp models.BlogCategory
			if err := models.DB.First(&pp, p.ParentID).Error; err == nil && pp.ParentID != 0 {
				utils.BadRequest(c, "分类最多支持三级，三级下不能再挂子分类")
				return
			}
		}
	}
	var cnt int64
	models.DB.Model(&models.BlogCategory{}).Where("name = ? AND parent_id = ?", req.Name, req.ParentID).Count(&cnt)
	if cnt > 0 {
		utils.BadRequest(c, "同名分类已存在")
		return
	}
	// 状态：管理员创建直接通过；普通作者按站点审核模式
	//   事后审核(post_review，默认) → pending，管理员通过后公开
	//   先公开(pre_public)         → approved 直接公开，管理员事后可封禁(拒绝)
	status := models.CategoryStatusApproved
	if !isAdminUser(c) && models.GetCategoryReviewMode() == models.CategoryReviewModePost {
		status = models.CategoryStatusPending
	}
	cat := &models.BlogCategory{
		Name: req.Name, ParentID: req.ParentID, SortOrder: req.SortOrder,
		CreatedBy: currentUserID(c), Status: status,
	}
	if err := models.DB.Create(cat).Error; err != nil {
		utils.InternalError(c, "创建失败")
		return
	}
	utils.Success(c, cat)
}

// PUT /api/blog-categories/:id — 修改分类（改名 / 挂靠新父分类 / 排序）
// 权限：创建者本人 或 管理员
func UpdateBlogCategory(c *gin.Context) {
	if !requireLogin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var cat models.BlogCategory
	if err := models.DB.First(&cat, id).Error; err != nil {
		utils.NotFound(c, "分类不存在")
		return
	}
	if !canManageCategory(c, &cat) {
		utils.Forbidden(c, "只能管理自己创建的分类")
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
	// 改父级：最多三级；不能把自己挂到自己/子孙下；不能移到会超过三级的层级
	if req.ParentID != cat.ParentID {
		if req.ParentID != 0 {
			var p models.BlogCategory
			if err := models.DB.First(&p, req.ParentID).Error; err != nil {
				utils.BadRequest(c, "上级分类不存在")
				return
			}
			// 目标父分类的深度决定新层级：父深度 ≥3 拒绝；若目标父分类已是三级，新分类会是四级
			parentDepth := 1
			if p.ParentID != 0 {
				var pp models.BlogCategory
				if err := models.DB.First(&pp, p.ParentID).Error; err == nil {
					parentDepth = 2
					if pp.ParentID != 0 {
						parentDepth = 3
					}
				}
			}
			if parentDepth >= 3 {
				utils.BadRequest(c, "分类最多支持三级，三级下不能再挂子分类")
				return
			}
			if req.ParentID == cat.ID {
				utils.BadRequest(c, "不能将分类挂到自己下面")
				return
			}
			// 不能移到自己的子孙分类下（防止成环）
			var descendants []uint
			collectDescendants(cat.ID, &descendants)
			for _, d := range descendants {
				if req.ParentID == d {
					utils.BadRequest(c, "不能将分类挂到自己的子分类下")
					return
				}
			}
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

// DELETE /api/blog-categories/:id — 删除分类（其下博客回到未分类；有子分类先删子分类）
// 权限：创建者本人 或 管理员
func DeleteBlogCategory(c *gin.Context) {
	if !requireLogin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var cat models.BlogCategory
	if err := models.DB.First(&cat, id).Error; err != nil {
		utils.NotFound(c, "分类不存在")
		return
	}
	if !canManageCategory(c, &cat) {
		utils.Forbidden(c, "只能管理自己创建的分类")
		return
	}
	if models.BlogCategoryHasChildren(cat.ID) {
		utils.BadRequest(c, "该分类下还有子分类，请先删除子分类")
		return
	}
	models.DB.Model(&models.AuthorBlog{}).Where("category_id = ?", cat.ID).Update("category_id", 0)
	if err := models.DB.Delete(&cat).Error; err != nil {
		utils.InternalError(c, "删除失败")
		return
	}
	utils.Success(c, gin.H{"message": "已删除，该分类下的博客已回到未分类"})
}

// ─────────────────────────────────────────────────────────────
// 管理员分类审核（事后审核：普通作者创建的分类先可用后审核）
// ─────────────────────────────────────────────────────────────

// GET /api/admin/blog-categories — 管理员全量分类视图（含状态与创建者，平铺）
func AdminListBlogCategories(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	type item struct {
		ID           uint      `json:"id"`
		ParentID     uint      `json:"parent_id"`
		Name         string    `json:"name"`
		FullName     string    `json:"full_name"`
		SortOrder    int       `json:"sort_order"`
		Status       string    `json:"status"`
		RejectReason string    `json:"reject_reason"`
		CreatedBy    uint      `json:"created_by"`
		CreatorName  string    `json:"creator_name"`
		BlogCount    int64     `json:"blog_count"`
		CreatedAt    time.Time `json:"created_at"`
	}
	var cats []models.BlogCategory
	models.DB.Order("created_at DESC").Find(&cats)
	list := make([]item, 0, len(cats))
	for _, c := range cats {
		it := item{
			ID: c.ID, ParentID: c.ParentID, Name: c.Name,
			FullName: models.GetBlogCategoryName(c.ID), SortOrder: c.SortOrder,
			Status: c.Status, RejectReason: c.RejectReason, CreatedBy: c.CreatedBy,
			CreatedAt: c.CreatedAt,
		}
		models.DB.Model(&models.AuthorBlog{}).Where("category_id = ?", c.ID).Count(&it.BlogCount)
		if c.CreatedBy > 0 {
			if u, err := models.GetUserByID(c.CreatedBy); err == nil && u != nil {
				name := u.Nickname
				if name == "" {
					name = u.Username
				}
				it.CreatorName = name
			}
		} else {
			it.CreatorName = "平台/管理员"
		}
		list = append(list, it)
	}

	// 按创建者分组（二级目录：用户 → 该用户的分类），突出待审核用户
	type groupItem struct {
		UserID       uint   `json:"user_id"`
		UserName     string `json:"user_name"`
		Nickname     string `json:"nickname"`
		AvatarURL    string `json:"avatar_url"`
		PendingCount int64  `json:"pending_count"` // 该用户待审核分类数（真实值，不受筛选影响）
		TotalCount   int    `json:"total_count"`   // 该用户分类总数
		Categories   []item `json:"categories"`
	}
	groupsMap := map[uint]*groupItem{}
	var groupOrder []uint
	for _, it := range list {
		key := it.CreatedBy
		g, ok := groupsMap[key]
		if !ok {
			g = &groupItem{UserID: key, Categories: []item{}}
			groupsMap[key] = g
			groupOrder = append(groupOrder, key)
			if key > 0 {
				if u, err := models.GetUserByID(key); err == nil && u != nil {
					g.UserName = u.Username
					g.Nickname = u.Nickname
					g.AvatarURL = u.AvatarURL
				}
			} else {
				g.UserName = "platform"
				g.Nickname = "平台/管理员"
			}
		}
		g.Categories = append(g.Categories, it)
		g.TotalCount++
		if it.Status == "pending" {
			g.PendingCount++
		}
	}
	groups := make([]groupItem, 0, len(groupOrder))
	for _, key := range groupOrder {
		groups = append(groups, *groupsMap[key])
	}
	// 待审核多的用户排前面（管理员一眼看到"谁还需审核"）
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].PendingCount != groups[j].PendingCount {
			return groups[i].PendingCount > groups[j].PendingCount
		}
		return groups[i].TotalCount > groups[j].TotalCount
	})

	utils.Success(c, gin.H{"list": list, "groups": groups})
}

// POST /api/admin/blog-categories/:id/review — 审核分类 { action: approve|reject, reason? }
func AdminReviewBlogCategory(c *gin.Context) {
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
		Action string `json:"action" binding:"required"`
		Reason string `json:"reason"`	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	switch req.Action {
	case "approve":
		cat.Status = models.CategoryStatusApproved
		cat.RejectReason = ""
		auditEmergency(c, models.AuditContentRestore, "blog_category", cat.ID,
			"管理员通过博客分类《"+cat.Name+"》审核")
	case "reject":
		cat.Status = models.CategoryStatusRejected
		cat.RejectReason = trimSpace(req.Reason)
		if cat.RejectReason == "" {
			cat.RejectReason = "分类名称/内容不符合平台规范"
		}
		auditEmergency(c, models.AuditContentDelete, "blog_category", cat.ID,
			"管理员拒绝博客分类《"+cat.Name+"》："+cat.RejectReason)
	default:
		utils.BadRequest(c, "action 仅支持 approve / reject")
		return
	}
	if err := models.DB.Save(&cat).Error; err != nil {
		utils.InternalError(c, "操作失败")
		return
	}
	utils.Success(c, cat)
}

// GET /api/admin/blog-categories/mode — 获取博客分类审核模式
//   post_review：事后审核（默认），普通作者创建需审核通过后才公开
//   pre_public：先公开，事后封禁，普通作者创建直接公开，管理员可随时下架
func AdminGetCategoryReviewMode(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	mode := models.GetCategoryReviewMode()
	label := "事后审核（通过后公开）"
	if mode == models.CategoryReviewModePreOpen {
		label = "先公开（事后封禁）"
	}
	utils.Success(c, gin.H{"mode": mode, "label": label})
}

// PUT /api/admin/blog-categories/mode — 设置博客分类审核模式 { mode: post_review | pre_public }
func AdminSetCategoryReviewMode(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		Mode string `json:"mode" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if req.Mode != models.CategoryReviewModePost && req.Mode != models.CategoryReviewModePreOpen {
		utils.BadRequest(c, "mode 仅支持 post_review（事后审核）/ pre_public（先公开）")
		return
	}
	if err := models.SetSetting(models.CategoryReviewModeKey, req.Mode); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	label := "事后审核（通过后公开）"
	if req.Mode == models.CategoryReviewModePreOpen {
		label = "先公开（事后封禁）"
	}
	auditEmergency(c, models.AuditContentRestore, "blog_category", 0,
		"管理员切换博客分类审核模式为："+label)
	utils.Success(c, gin.H{"mode": req.Mode, "label": label})
}

// fillBlogCategories 为博客列表填充分类名（含父级前缀）
func fillBlogCategories(blogs []models.AuthorBlog) {
	for i := range blogs {
		blogs[i].Category = models.GetBlogCategoryName(blogs[i].CategoryID)
	}
}
