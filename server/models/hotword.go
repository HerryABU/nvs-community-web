package models

import (
	"encoding/json"
	"strings"
	"time"
)

// ==================== 内容热词 + 标签热词（F9：热词统计） ====================
//
// 三类热词统一由管理端 /api/admin/hotwords?type= 提供：
//   type=content（默认）——AggregateHotWords：评论/回帖/帖题高频词
//   type=search           ——AggregateSearchWords/AggregateSearchQueries（见 searchlog.go）
//   type=tag              ——AggregateTagHotWords：博客结构化标签 + 作品 JSON 标签
// 切词能力统一收敛到 tokenizer.go 的 Tokenize。

// HotWord 热词统计项（管理员内容治理/舆情观测）
type HotWord struct {
	Word  string `json:"word"`
	Count int    `json:"count"`
}

// AggregateHotWords 统计最近 N 天评论/回帖/帖题中的高频词（按需聚合，无持久化表）。
// 切词走 Tokenize（英文整词 + 中文短段整取/长段二元切分，过滤停用词），
// 取出现次数≥2 的前 limit 项。
// days<=0 默认 7，>365 收敛到 365；limit<=0 默认 50，>200 收敛到 200。
func AggregateHotWords(days, limit int) ([]HotWord, error) {
	days, limit = clampHotWordWindow(days, limit)
	since := time.Now().AddDate(0, 0, -days)
	counts := map[string]int{}

	// 单表扫描上限，防止超大库一次拉全表
	const scanCap = 5000

	var comments []Comment
	DB.Select("content").Where("created_at >= ?", since).
		Order("created_at DESC").Limit(scanCap).Find(&comments)
	for _, c := range comments {
		countHotWords(c.Content, counts)
	}

	var posts []Post
	DB.Select("content").Where("created_at >= ?", since).
		Order("created_at DESC").Limit(scanCap).Find(&posts)
	for _, p := range posts {
		countHotWords(p.Content, counts)
	}

	var threads []Thread
	DB.Select("title").Where("created_at >= ?", since).
		Order("created_at DESC").Limit(scanCap).Find(&threads)
	for _, t := range threads {
		countHotWords(t.Title, counts)
	}

	return topHotWords(counts, 2, limit), nil
}

// countHotWords 将一段文本切词并累加到 counts（薄封装，保留历史调用点）
func countHotWords(text string, counts map[string]int) {
	for _, w := range Tokenize(text) {
		counts[w]++
	}
}

// AggregateTagHotWords 标签热词：聚合最近 N 天新建内容上作者打的标签。
//  1. 博客结构化标签：blog_tags ⋈ blog_tag_links ⋈ author_blogs（窗口内、未删除）；
//  2. 作品 JSON 标签：novels.tags 为 JSON 数组字符串，逐条解析。
//
// 标签是作者人工 curation 的，单次出现即计入（≥1），按使用次数降序。
func AggregateTagHotWords(days, limit int) ([]HotWord, error) {
	days, limit = clampHotWordWindow(days, limit)
	since := time.Now().AddDate(0, 0, -days)
	counts := map[string]int{}

	// 1) 博客结构化标签（SQL 侧聚合）
	type tagRow struct {
		Name  string
		Count int64
	}
	var rows []tagRow
	DB.Table("blog_tags").
		Select("blog_tags.name AS name, COUNT(blog_tag_links.blog_id) AS count").
		Joins("JOIN blog_tag_links ON blog_tag_links.tag_id = blog_tags.id").
		Joins("JOIN author_blogs ON author_blogs.id = blog_tag_links.blog_id").
		Where("author_blogs.deleted_at IS NULL AND author_blogs.created_at >= ?", since).
		Group("blog_tags.id").
		Scan(&rows)
	for _, r := range rows {
		name := strings.TrimSpace(r.Name)
		if name != "" {
			counts[name] += int(r.Count)
		}
	}

	// 2) 作品 JSON 标签（扫描上限同内容热词）
	const scanCap = 5000
	var novels []Novel
	DB.Select("tags").Where("created_at >= ?", since).
		Order("created_at DESC").Limit(scanCap).Find(&novels)
	for _, n := range novels {
		for _, tag := range parseNovelTags(n.Tags) {
			if tag != "" {
				counts[tag]++
			}
		}
	}

	return topHotWords(counts, 1, limit), nil
}

// parseNovelTags 解析 novels.tags（JSON 数组字符串）为去空标签列表。
// 非法 JSON 时退化为按逗号/中文逗号切分，保证老数据/脏数据不丢标签。
func parseNovelTags(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err == nil {
		out := make([]string, 0, len(tags))
		for _, t := range tags {
			t = strings.TrimSpace(t)
			if t != "" {
				out = append(out, t)
			}
		}
		return out
	}
	for _, t := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '，' || r == ';' || r == '；'
	}) {
		if t = strings.TrimSpace(t); t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}
