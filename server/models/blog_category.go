package models

import "time"

// ─────────────────────────────────────────────────────────────
// 博客分类（支持三级，CSDN 式；按用户归属 + 事后审核）
//
// 设计：
//   - blog_categories 分类树：ParentID=0 为一级分类，其余为二级/三级分类。
//   - author_blogs.category_id 指向叶子分类，0 = 未分类。
//   - 普通作者可创建分类（默认 status=pending 待管理员事后审核），管理员创建 = approved。
//   - 审核通过前不对外展示（公开树/编辑器只出现 approved）；作者可编辑自己的分类。
//   - 删除分类时其下博客自动回到未分类(0)。
// ─────────────────────────────────────────────────────────────

type BlogCategory struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	ParentID     uint      `gorm:"default:0;index:idx_blog_cat_parent" json:"parent_id"` // 0 = 一级分类
	Name         string    `gorm:"size:64;not null" json:"name"`
	SortOrder    int       `gorm:"default:0" json:"sort_order"`
	CreatedBy    uint      `gorm:"default:0;index" json:"created_by"`           // 创建者用户 ID（0=平台/管理员）
	Status       string    `gorm:"size:16;default:pending;index" json:"status"` // pending=待审核 / approved=已通过 / rejected=已拒绝
	RejectReason string    `gorm:"size:256;default:''" json:"reject_reason"`    // 拒绝原因（status=rejected 时）
	CreatedAt    time.Time `json:"created_at"`
}

func (BlogCategory) TableName() string { return "blog_categories" }

// 分类状态常量
const (
	CategoryStatusPending  = "pending"  // 待审核（事后审核：先创建后审核）
	CategoryStatusApproved = "approved" // 已通过（公开可见）
	CategoryStatusRejected = "rejected" // 已拒绝
)

// BlogCategoryNode 分类树节点（含子分类与博客数量）
type BlogCategoryNode struct {
	ID        uint               `json:"id"`
	ParentID  uint               `json:"parent_id"`
	Name      string             `json:"name"`
	SortOrder int                `json:"sort_order"`
	Status    string             `json:"status,omitempty"`     // 审核状态（公开树恒为 approved，管理视图才带）
	CreatedBy uint               `json:"created_by,omitempty"` // 创建者（管理视图）
	BlogCount int64              `json:"blog_count"`
	Children  []BlogCategoryNode `json:"children,omitempty"`
}

// buildCategoryTree 通用分类树组装（含状态过滤）：
// statusFilter 为空 = 全部；否则只取该状态的分类（其父链上允许出现非目标状态但用于挂载）
func buildCategoryTree(statusFilter string) []BlogCategoryNode {
	var cats []BlogCategory
	q := DB.Order("sort_order ASC, id ASC")
	if statusFilter != "" {
		// 取目标状态分类 + 其所有祖先（祖先用于挂载，即使状态不同）
		ids := []uint{}
		var target []BlogCategory
		q.Where("status = ?", statusFilter).Find(&target)
		for _, t := range target {
			ids = append(ids, t.ID)
			pid := t.ParentID
			for pid != 0 {
				ids = append(ids, pid)
				var p BlogCategory
				if err := DB.First(&p, pid).Error; err != nil {
					break
				}
				pid = p.ParentID
			}
		}
		if len(ids) == 0 {
			return []BlogCategoryNode{}
		}
		q = DB.Where("id IN ?", ids)
	}
	q.Find(&cats)

	nodes := make([]BlogCategoryNode, 0, len(cats))
	for _, c := range cats {
		nodes = append(nodes, BlogCategoryNode{
			ID: c.ID, ParentID: c.ParentID, Name: c.Name, SortOrder: c.SortOrder,
			Status: c.Status, CreatedBy: c.CreatedBy,
		})
	}
	// 目标状态过滤（仅对外层节点生效；祖先用于挂载但不显示）
	visible := map[uint]bool{}
	if statusFilter != "" {
		for _, c := range cats {
			if c.Status == statusFilter {
				visible[c.ID] = true
			}
		}
	}

	// 统计每个分类直接挂载的博客数
	for i := range nodes {
		var cnt int64
		DB.Model(&AuthorBlog{}).Where("category_id = ?", nodes[i].ID).Count(&cnt)
		nodes[i].BlogCount = cnt
	}
	// 父子映射（可到三级）
	childrenOf := map[uint][]BlogCategoryNode{}
	for _, n := range nodes {
		if n.ParentID != 0 {
			childrenOf[n.ParentID] = append(childrenOf[n.ParentID], n)
		}
	}
	// 组装树（递归最多三层），自底向上聚合 blog_count
	var build func(id uint) []BlogCategoryNode
	build = func(id uint) []BlogCategoryNode {
		kids := childrenOf[id]
		if len(kids) == 0 {
			return nil
		}
		out := make([]BlogCategoryNode, 0, len(kids))
		for _, k := range kids {
			if len(visible) > 0 && !visible[k.ID] {
				// 祖先节点仅作挂载，不进入展示树；其子节点直接上提一层
				sub := build(k.ID)
				if sub != nil {
					out = append(out, sub...)
				}
				continue
			}
			k.Children = build(k.ID)
			if k.Children == nil {
				k.Children = []BlogCategoryNode{}
			}
			for _, g := range k.Children {
				k.BlogCount += g.BlogCount
			}
			out = append(out, k)
		}
		return out
	}
	tree := make([]BlogCategoryNode, 0, len(nodes))
	for _, n := range nodes {
		if n.ParentID == 0 && (len(visible) == 0 || visible[n.ID]) {
			n.Children = build(n.ID)
			if n.Children == nil {
				n.Children = []BlogCategoryNode{}
			}
			for _, c := range n.Children {
				n.BlogCount += c.BlogCount
			}
			tree = append(tree, n)
		}
	}
	return tree
}

// GetBlogCategoryTree 公开分类树：仅展示已审核通过（approved）的分类
func GetBlogCategoryTree() []BlogCategoryNode {
	return buildCategoryTree(CategoryStatusApproved)
}

// GetAllCategoryTree 管理视图分类树：全部状态（pending/approved/rejected）
func GetAllCategoryTree() []BlogCategoryNode {
	return buildCategoryTree("")
}

// GetMyCategories 作者视图：自己创建的分类（任意状态，平铺）
func GetMyCategories(userID uint) []BlogCategory {
	var cats []BlogCategory
	DB.Where("created_by = ?", userID).Order("sort_order ASC, id ASC").Find(&cats)
	return cats
}

// GetAuthorCategoryTree 某作者的已公开分类树（个人专栏，仅 approved，含博客数）
func GetAuthorCategoryTree(authorID uint) []BlogCategoryNode {
	var cats []BlogCategory
	DB.Where("created_by = ? AND status = ?", authorID, "approved").
		Order("sort_order ASC, id ASC").Find(&cats)
	if len(cats) == 0 {
		return []BlogCategoryNode{}
	}
	// 只保留属于自己的分类（父分类若不是自己的，则上提）
	ids := map[uint]bool{}
	for _, c := range cats {
		ids[c.ID] = true
	}
	nodes := make([]BlogCategoryNode, 0, len(cats))
	for _, c := range cats {
		pid := c.ParentID
		if pid != 0 {
			var p BlogCategory
			if err := DB.First(&p, pid).Error; err != nil || p.CreatedBy != authorID || !ids[p.ID] {
				pid = 0
			}
		}
		nodes = append(nodes, BlogCategoryNode{
			ID: c.ID, ParentID: pid, Name: c.Name, SortOrder: c.SortOrder,
			Status: c.Status, CreatedBy: c.CreatedBy,
		})
	}
	// 统计博客数 + 父子映射（可到三级）
	for i := range nodes {
		var cnt int64
		DB.Model(&AuthorBlog{}).Where("category_id = ?", nodes[i].ID).Count(&cnt)
		nodes[i].BlogCount = cnt
	}
	childrenOf := map[uint][]BlogCategoryNode{}
	for _, n := range nodes {
		if n.ParentID != 0 {
			childrenOf[n.ParentID] = append(childrenOf[n.ParentID], n)
		}
	}
	// 组装树（递归最多三层），自底向上聚合 blog_count
	var build func(id uint) []BlogCategoryNode
	build = func(id uint) []BlogCategoryNode {
		kids := childrenOf[id]
		if len(kids) == 0 {
			return nil
		}
		out := make([]BlogCategoryNode, 0, len(kids))
		for _, k := range kids {
			k.Children = build(k.ID)
			if k.Children == nil {
				k.Children = []BlogCategoryNode{}
			}
			for _, g := range k.Children {
				k.BlogCount += g.BlogCount
			}
			out = append(out, k)
		}
		return out
	}
	tree := make([]BlogCategoryNode, 0, len(nodes))
	for _, n := range nodes {
		if n.ParentID == 0 {
			n.Children = build(n.ID)
			if n.Children == nil {
				n.Children = []BlogCategoryNode{}
			}
			for _, c := range n.Children {
				n.BlogCount += c.BlogCount
			}
			tree = append(tree, n)
		}
	}
	return tree
}

// GetBlogCategoryName 取分类名（带父级前缀，如「技术/Go/并发」），不存在返回空
func GetBlogCategoryName(id uint) string {
	if id == 0 {
		return ""
	}
	var c BlogCategory
	if err := DB.First(&c, id).Error; err != nil {
		return ""
	}
	return blogCategoryFullName(&c)
}

// blogCategoryFullName 沿 parent_id 链向上拼接分类全名（支持三级，最多 3 层防环）
func blogCategoryFullName(c *BlogCategory) string {
	seg := []string{c.Name}
	cur := c
	for depth := 0; depth < 3 && cur.ParentID != 0; depth++ {
		var p BlogCategory
		if err := DB.First(&p, cur.ParentID).Error; err != nil {
			break
		}
		if p.ID == c.ID {
			break // 防环
		}
		seg = append([]string{p.Name}, seg...)
		cur = &p
	}
	out := ""
	for i, s := range seg {
		if i > 0 {
			out += "/"
		}
		out += s
	}
	return out
}

// BlogCategoryHasChildren 是否存在子分类
func BlogCategoryHasChildren(id uint) bool {
	var cnt int64
	DB.Model(&BlogCategory{}).Where("parent_id = ?", id).Count(&cnt)
	return cnt > 0
}

// GetBlogCategoryDescendants 返回某分类及其全部子孙分类 id（支持三级；用于按上级分类过滤博客）。
func GetBlogCategoryDescendants(rootID uint) []uint {
	ids := []uint{rootID}
	if rootID == 0 {
		return ids
	}
	var all []BlogCategory
	DB.Select("id", "parent_id").Find(&all)
	childMap := map[uint][]uint{}
	for _, c := range all {
		childMap[c.ParentID] = append(childMap[c.ParentID], c.ID)
	}
	queue := []uint{rootID}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, cid := range childMap[cur] {
			ids = append(ids, cid)
			queue = append(queue, cid)
		}
	}
	return ids
}

// CategoryLevel 返回分类层级：0=一级 1=二级 2=三级；不存在返回 -1
func CategoryLevel(id uint) int {
	if id == 0 {
		return 0
	}
	var c BlogCategory
	if err := DB.First(&c, id).Error; err != nil {
		return -1
	}
	level := 0
	cur := &c
	for depth := 0; depth < 3 && cur.ParentID != 0; depth++ {
		level++
		var p BlogCategory
		if err := DB.First(&p, cur.ParentID).Error; err != nil {
			break
		}
		if p.ID == c.ID {
			break // 防环
		}
		cur = &p
	}
	return level
}

// ─────────────────────────────────────────────────────────────
// 博客点赞
//
// blog_likes：blog_id + user_id 唯一，点赞/取消点赞为 toggle。
// 点赞数直接在 author_blogs.like_count 冗余计数（避免每次 count）。
// ─────────────────────────────────────────────────────────────

type BlogLike struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	BlogID    uint      `gorm:"not null;uniqueIndex:uk_blog_like,priority:1;index:idx_blog_like_blog" json:"blog_id"`
	UserID    uint      `gorm:"not null;uniqueIndex:uk_blog_like,priority:2;index" json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (BlogLike) TableName() string { return "blog_likes" }

// ToggleBlogLike 点赞/取消点赞，返回是否已点赞（true=刚点赞）
func ToggleBlogLike(blogID, userID uint) (bool, error) {
	var like BlogLike
	err := DB.Where("blog_id = ? AND user_id = ?", blogID, userID).First(&like).Error
	if err == nil {
		// 已点赞 → 取消
		if err := DB.Delete(&like).Error; err != nil {
			return false, err
		}
		DB.Model(&AuthorBlog{}).Where("id = ?", blogID).
			UpdateColumn("like_count", DB.Raw("CASE WHEN like_count > 0 THEN like_count - 1 ELSE 0 END"))
		return false, nil
	}
	// 未点赞 → 点赞
	like = BlogLike{BlogID: blogID, UserID: userID}
	if err := DB.Create(&like).Error; err != nil {
		return false, err
	}
	DB.Model(&AuthorBlog{}).Where("id = ?", blogID).
		UpdateColumn("like_count", DB.Raw("like_count + 1"))
	return true, nil
}

// UserLikedBlog 当前用户是否已点赞
func UserLikedBlog(blogID, userID uint) bool {
	if userID == 0 {
		return false
	}
	var cnt int64
	DB.Model(&BlogLike{}).Where("blog_id = ? AND user_id = ?", blogID, userID).Count(&cnt)
	return cnt > 0
}
