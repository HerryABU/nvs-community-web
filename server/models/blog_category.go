package models

import "time"

// ─────────────────────────────────────────────────────────────
// 博客分类（支持一级/二级，CSDN 式）
//
// 设计：
//   - blog_categories 分类树：ParentID=0 为一级分类，其余为二级分类。
//   - author_blogs.category_id 指向叶子分类（一级或二级均可），0 = 未分类。
//   - 管理员可任意增删改；删除分类时其下博客自动回到未分类(0)。
// ─────────────────────────────────────────────────────────────

type BlogCategory struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ParentID  uint      `gorm:"default:0;index:idx_blog_cat_parent" json:"parent_id"` // 0 = 一级分类
	Name      string    `gorm:"size:64;not null" json:"name"`
	SortOrder int       `gorm:"default:0" json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
}

func (BlogCategory) TableName() string { return "blog_categories" }

// BlogCategoryNode 分类树节点（含子分类与博客数量）
type BlogCategoryNode struct {
	ID        uint               `json:"id"`
	ParentID  uint               `json:"parent_id"`
	Name      string             `json:"name"`
	SortOrder int                `json:"sort_order"`
	BlogCount int64              `json:"blog_count"`
	Children  []BlogCategoryNode `json:"children,omitempty"`
}

// GetBlogCategoryTree 获取完整分类树（按 sort_order, id 排序），并统计每个分类的博客数（仅未删除）。
// 一级分类的 blog_count = 直接挂载 + 其下所有二级分类的博客数（与列表按一级分类过滤的语义一致）。
func GetBlogCategoryTree() []BlogCategoryNode {
	var cats []BlogCategory
	DB.Order("sort_order ASC, id ASC").Find(&cats)

	nodes := make([]BlogCategoryNode, 0, len(cats))
	for _, c := range cats {
		nodes = append(nodes, BlogCategoryNode{
			ID: c.ID, ParentID: c.ParentID, Name: c.Name, SortOrder: c.SortOrder,
		})
	}

	// 统计每个分类直接挂载的博客数
	for i := range nodes {
		var cnt int64
		DB.Model(&AuthorBlog{}).Where("category_id = ?", nodes[i].ID).Count(&cnt)
		nodes[i].BlogCount = cnt
	}
	// 一级分类聚合其下二级分类的博客数（含直接挂载）
	childrenOf := map[uint][]BlogCategoryNode{}
	for _, n := range nodes {
		if n.ParentID != 0 {
			childrenOf[n.ParentID] = append(childrenOf[n.ParentID], n)
		}
	}
	for i := range nodes {
		if nodes[i].ParentID == 0 {
			for _, sub := range childrenOf[nodes[i].ID] {
				nodes[i].BlogCount += sub.BlogCount
			}
		}
	}

	// 组装树：一级分类 + 挂载二级分类
	tree := make([]BlogCategoryNode, 0, len(nodes))
	for _, n := range nodes {
		if n.ParentID == 0 {
			n.Children = childrenOf[n.ID]
			if n.Children == nil {
				n.Children = []BlogCategoryNode{}
			}
			tree = append(tree, n)
		}
	}
	return tree
}

// GetBlogCategoryName 取分类名（带父级前缀，如「技术/Go」），不存在返回空
func GetBlogCategoryName(id uint) string {
	if id == 0 {
		return ""
	}
	var c BlogCategory
	if err := DB.First(&c, id).Error; err != nil {
		return ""
	}
	if c.ParentID == 0 {
		return c.Name
	}
	var p BlogCategory
	if err := DB.First(&p, c.ParentID).Error; err == nil {
		return p.Name + "/" + c.Name
	}
	return c.Name
}

// BlogCategoryWithChildren 是否存在二级子分类
func BlogCategoryHasChildren(id uint) bool {
	var cnt int64
	DB.Model(&BlogCategory{}).Where("parent_id = ?", id).Count(&cnt)
	return cnt > 0
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
