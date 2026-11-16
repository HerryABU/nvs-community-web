package blog

// ==================== 高级博客主题（独立主题页数据聚合） ====================
// 参考 Halo Chirpy / Docusaurus / Hexo 等成熟博客主题：
// 首页 + 分类 + 标签 + 归档 + 友链 + 关于，侧栏作者卡片/统计/近期文章/标签云。
// 本接口只读，不改现有 /api/blogs 列表与 BlogHome。

import (
	"sort"
	"strings"

	"nvs-server/handlers/hctx"
	"nvs-server/models"
	blogm "nvs-server/models/blog"
	"nvs-server/utils/infra"

	"github.com/gin-gonic/gin"
)

// GET /api/theme-blog/:key/overview
// 一次返回主题页所需的全部数据（最多 200 篇），分类/标签/归档/搜索在前端本地过滤。
func ThemeBlogOverview(c *gin.Context) {
	var author models.User
	key := strings.TrimSpace(c.Param("key"))
	if key == "" {
		infra.NotFound(c, "作者不存在")
		return
	}
	// 兼容数字 id 与用户名（与 /api/author/:id 一致）
	q := models.DB.Model(&models.User{}).Where("username = ?", key)
	if n, ok := parsePositive(key); ok {
		q = q.Or("id = ?", n)
	}
	if err := q.First(&author).Error; err != nil {
		infra.NotFound(c, "作者不存在")
		return
	}
	author.PasswordHash = ""
	author.Email = ""

	var blogs []blogm.AuthorBlog
	if err := models.DB.Model(&blogm.AuthorBlog{}).Where("author_id = ?", author.ID).
		Preload("Author").Order("is_pinned DESC, created_at DESC").Limit(200).Find(&blogs).Error; err != nil {
		infra.InternalError(c, "查询失败")
		return
	}
	blogs, _ = filterListableBlogs(c, blogs)
	FillBlogCategories(blogs)

	ids := make([]uint, 0, len(blogs))
	for _, b := range blogs {
		ids = append(ids, b.ID)
	}

	// 评论数聚合
	commentCounts := map[uint]int64{}
	if len(ids) > 0 {
		type cc struct {
			BlogID uint
			N      int64
		}
		var rows []cc
		models.DB.Model(&models.Comment{}).Select("blog_id, COUNT(*) AS n").
			Where("blog_id IN ?", ids).Group("blog_id").Find(&rows)
		for _, r := range rows {
			commentCounts[r.BlogID] = r.N
		}
	}

	// 标签（作者维度）+ 每标签文章数
	type tagRow struct {
		Name  string `json:"name"`
		Count int64  `json:"count"`
	}
	tagRows := []tagRow{}
	if len(ids) > 0 {
		models.DB.Model(&blogm.BlogTag{}).
			Select("blog_tags.name AS name, COUNT(*) AS count").
			Joins("JOIN blog_tag_links ON blog_tag_links.tag_id = blog_tags.id").
			Where("blog_tag_links.blog_id IN ?", ids).
			Group("blog_tags.name").Order("count DESC").Find(&tagRows)
	}

	// 分类（作者创建且已通过）+ 文章数
	type catRow struct {
		ID       uint   `json:"id"`
		ParentID uint   `json:"parent_id"`
		Name     string `json:"name"`
		Count    int64  `json:"count"`
	}
	catRows := []catRow{}
	models.DB.Model(&blogm.BlogCategory{}).
		Select("blog_categories.id, blog_categories.parent_id, blog_categories.name, COUNT(author_blogs.id) AS count").
		Joins("LEFT JOIN author_blogs ON author_blogs.category_id = blog_categories.id AND author_blogs.author_id = ?", author.ID).
		Where("blog_categories.created_by = ? AND blog_categories.status = ?", author.ID, blogm.CategoryStatusApproved).
		Group("blog_categories.id").Order("blog_categories.sort_order ASC, blog_categories.id ASC").Find(&catRows)

	// 归档（按年月，Go 端聚合以兼容不同数据库）
	archiveMap := map[string]int64{}
	var totalView, totalLike, totalFav int64
	posts := make([]gin.H, 0, len(blogs))
	for _, b := range blogs {
		month := b.CreatedAt.Format("2006-01")
		archiveMap[month]++
		totalView += int64(b.ViewCount)
		totalLike += int64(b.LikeCount)
		totalFav += int64(b.FavoriteCount)
		tags := blogm.GetBlogTags(b.ID)
		tagNames := make([]string, 0, len(tags))
		for _, t := range tags {
			tagNames = append(tagNames, t.Name)
		}
		item := gin.H{
			"id": b.ID, "title": b.Title, "summary": b.Summary,
			"category_id": b.CategoryID, "category": b.Category,
			"cover_url": b.CoverURL, "tags": tagNames,
			"view_count": b.ViewCount, "like_count": b.LikeCount,
			"favorite_count": b.FavoriteCount, "comment_count": commentCounts[b.ID],
			"is_pinned": b.IsPinned, "created_at": b.CreatedAt.Format("2006-01-02"),
			"month": month,
		}
		posts = append(posts, item)
	}
	archives := make([]gin.H, 0, len(archiveMap))
	for m, n := range archiveMap {
		archives = append(archives, gin.H{"month": m, "count": n})
	}
	sort.Slice(archives, func(i, j int) bool {
		return archives[i]["month"].(string) > archives[j]["month"].(string)
	})

	// 友链走现有公开接口 /api/federated/sites，由前端自行加载

	infra.Success(c, gin.H{
		"author": author,
		"stats": gin.H{
			"posts": len(posts), "categories": len(catRows), "tags": len(tagRows),
			"views": totalView, "likes": totalLike, "favorites": totalFav,
			"comments": func() int64 {
				var n int64
				for _, v := range commentCounts {
					n += v
				}
				return n
			}(),
		},
		"categories": catRows,
		"tags":       tagRows,
		"archives":   archives,
		"posts":      posts,
		"viewer_id":  hctx.CurrentUserID(c),
	})
}

func parsePositive(s string) (uint, bool) {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
		if n > 1<<31 {
			return 0, false
		}
	}
	return uint(n), n > 0
}
