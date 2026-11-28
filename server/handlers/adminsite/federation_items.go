package adminsite

// 联邦内容输出：把本站公开的各类内容统一暴露给友站同步。
// 只输出公开字段（标题/摘要/封面/作者），不输出正文、密钥、用户信息。
// 多跳联邦：响应里带 sites（本站已互联的站点清单），友站可据此继续递归发现。

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"nvs-server/models"
	blogmodel "nvs-server/models/blog"
	feed "nvs-server/models/feed"
	platform "nvs-server/models/platform"
	"nvs-server/utils/infra"
)

// fedItem 是联邦内容条目的对外统一结构。
type fedItem struct {
	ID        uint   `json:"id"`
	Kind      string `json:"kind"`
	Title     string `json:"title"`
	Category  string `json:"category"`
	Summary   string `json:"summary"`
	CoverURL  string `json:"cover_url"`
	Author    string `json:"author"`
	SourceURL string `json:"source_url"`
}

// ListFederatedItems GET /api/federated/items
// 查询参数：kind（novel/blog/forum/micro/subculture，空=全部）、page、page_size。
func ListFederatedItems(c *gin.Context) {
	RecordInbound(c, "federated/items")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}
	kind := strings.TrimSpace(c.Query("kind"))

	items, total := collectFederatedItems(kind, page, pageSize, strings.TrimRight(selfPublicURL(c), "/"))

	// 多跳：暴露本站互联站点（只给公开的 name/url，不给 API 地址与密钥）。
	sites := make([]gin.H, 0)
	if kind == "" || kind == "sites" {
		var rows []platform.FederatedSite
		models.DB.Where("status = ?", "active").Order("id ASC").Find(&rows)
		for _, s := range rows {
			sites = append(sites, gin.H{"id": s.ID, "name": s.Name, "url": s.URL})
		}
	}

	infra.Success(c, gin.H{
		"list":  items,
		"total": total,
		"site":  gin.H{"name": models.GetSiteName(), "url": selfPublicURL(c)},
		"sites": sites,
	})
}

func collectFederatedItems(kind string, page, pageSize int, baseURL string) ([]fedItem, int64) {
	items := make([]fedItem, 0)
	var total int64

	if kind == "" || kind == "novel" {
		var rows []struct {
			ID         uint
			Title      string
			Summary    string
			CoverURL   string
			Category   string
			AuthorName string
		}
		q := models.DB.Model(&models.Novel{}).Select("novels.id, novels.title, novels.summary, novels.cover_url, novels.category, users.nickname as author_name").
			Joins("LEFT JOIN users ON users.id = novels.author_id").Where("novels.status = ?", "published")
		q.Count(&total)
		q.Order("novels.updated_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&rows)
		for _, r := range rows {
			items = append(items, fedItem{r.ID, "novel", r.Title, r.Category, r.Summary, r.CoverURL, r.AuthorName, baseURL + "/novel/" + strconv.FormatUint(uint64(r.ID), 10)})
		}
	}
	if kind == "" || kind == "blog" {
		var rows []struct {
			ID       uint
			Title    string
			Summary  string
			CoverURL string
			Category uint
			Nickname string
		}
		q := models.DB.Model(&blogmodel.AuthorBlog{}).Select("author_blogs.id, author_blogs.title, author_blogs.summary, author_blogs.cover_url, author_blogs.category_id, users.nickname").
			Joins("LEFT JOIN users ON users.id = author_blogs.author_id").Where("author_blogs.status = ?", "published")
		var cnt int64
		q.Count(&cnt)
		total += cnt
		q.Order("author_blogs.created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&rows)
		for _, r := range rows {
			items = append(items, fedItem{r.ID, "blog", r.Title, strconv.Itoa(int(r.Category)), r.Summary, r.CoverURL, r.Nickname, baseURL + "/blogs/" + strconv.FormatUint(uint64(r.ID), 10)})
		}
	}
	if kind == "" || kind == "micro" {
		posts, cnt, err := feed.ListPosts(feed.ListOptions{Page: page, PageSize: pageSize, Kind: feed.KindMicro, Scope: "all"})
		if err == nil {
			total += cnt
			for _, p := range posts {
				items = append(items, fedItem{p.ID, "micro", p.Title, "微文", p.Content, p.CoverURL, p.AuthorName, baseURL + "/micro/" + strconv.FormatUint(uint64(p.ID), 10)})
			}
		}
	}
	if kind == "" || kind == "forum" {
		var rows []struct {
			ID       uint
			Title    string
			Content  string
			Nickname string
		}
		q := models.DB.Model(&models.Thread{}).Select("threads.id, threads.title, threads.content, users.nickname").
			Joins("LEFT JOIN users ON users.id = threads.user_id")
		var cnt int64
		q.Count(&cnt)
		total += cnt
		q.Order("threads.updated_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&rows)
		for _, r := range rows {
			items = append(items, fedItem{r.ID, "forum", r.Title, "论坛", truncate(r.Content, 200), "", r.Nickname, baseURL + "/thread/" + strconv.FormatUint(uint64(r.ID), 10)})
		}
	}
	if kind == "" || kind == "subculture" {
		var rows []struct {
			ID       uint
			Name     string
			Summary  string
			CoverURL string
		}
		q := models.DB.Table("oc_projects").Select("id, name, summary, cover_url").Where("visibility = ?", "public")
		var cnt int64
		q.Count(&cnt)
		total += cnt
		q.Order("updated_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&rows)
		for _, r := range rows {
			items = append(items, fedItem{r.ID, "subculture", r.Name, "亚文化", r.Summary, r.CoverURL, "", baseURL + "/subculture/world/" + strconv.FormatUint(uint64(r.ID), 10)})
		}
	}
	return items, total
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
