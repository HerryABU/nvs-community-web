package models

import (
	"regexp"
	"sort"
	"strings"
	"time"
)

// HotWord 热词统计项（管理员内容治理/舆情观测）
type HotWord struct {
	Word  string `json:"word"`
	Count int    `json:"count"`
}

var (
	// 英文/数字词（长度≥2）
	enWordRe = regexp.MustCompile(`[a-zA-Z][a-zA-Z']{1,}`)
	// 连续中文段（长度≥2），再做二元切分
	cjkRunRe = regexp.MustCompile(`[\p{Han}]{2,}`)
)

// 中文高频虚字：二元切分后，含这些字的二元组一律丢弃，降噪。
const cjkStopChars = "的了是在我有和就都不与及着或一个也没你说他她它们这那什么怎么为因所但还就可已很于上下里外被把让从往对着呢吧啊吗之其或并且如果那么这个人们年月日个次条种些"

// 英文停用词（小写）
var enStopwords = map[string]bool{
	"the": true, "and": true, "for": true, "are": true, "but": true, "not": true,
	"you": true, "all": true, "any": true, "can": true, "had": true, "her": true,
	"was": true, "one": true, "our": true, "out": true, "day": true, "get": true,
	"has": true, "him": true, "his": true, "how": true, "man": true, "new": true,
	"now": true, "old": true, "see": true, "two": true, "way": true, "who": true,
	"boy": true, "did": true, "its": true, "let": true, "put": true, "say": true,
	"she": true, "too": true, "use": true, "that": true, "with": true, "this": true,
	"from": true, "they": true, "have": true, "will": true, "your": true, "said": true,
}

// AggregateHotWords 统计最近 N 天评论/回帖/帖题中的高频词（按需聚合，无持久化表）。
// 采用轻量分词：英文整词 + 中文二元切分（bigram），过滤停用词，取出现次数≥2 的前 limit 项。
// days<=0 默认 7，>365 收敛到 365；limit<=0 默认 50，>200 收敛到 200。
func AggregateHotWords(days, limit int) ([]HotWord, error) {
	if days <= 0 {
		days = 7
	}
	if days > 365 {
		days = 365
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
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

	words := make([]HotWord, 0, len(counts))
	for w, n := range counts {
		if n < 2 { // 至少出现 2 次才视为「热词」
			continue
		}
		words = append(words, HotWord{Word: w, Count: n})
	}
	sort.Slice(words, func(i, j int) bool {
		if words[i].Count != words[j].Count {
			return words[i].Count > words[j].Count
		}
		return words[i].Word < words[j].Word
	})
	if len(words) > limit {
		words = words[:limit]
	}
	return words, nil
}

// countHotWords 将一段文本切词并累加到 counts。
func countHotWords(text string, counts map[string]int) {
	if strings.TrimSpace(text) == "" {
		return
	}
	// 英文/数字整词
	for _, w := range enWordRe.FindAllString(strings.ToLower(text), -1) {
		if len(w) < 2 || enStopwords[w] {
			continue
		}
		counts[w]++
	}
	// 中文二元切分
	for _, run := range cjkRunRe.FindAllString(text, -1) {
		rs := []rune(run)
		for i := 0; i+2 <= len(rs); i++ {
			a, b := rs[i], rs[i+1]
			if strings.ContainsRune(cjkStopChars, a) || strings.ContainsRune(cjkStopChars, b) {
				continue
			}
			counts[string([]rune{a, b})]++
		}
	}
}
