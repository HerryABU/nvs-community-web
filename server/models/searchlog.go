package models

import (
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ==================== 搜索热词埋点（站内搜索 query 记录） ====================
//
// SearchLog 记录三处公开搜索入口的 query：
//   - unified：GET /api/search         统一搜索
//   - novels ：GET /api/novels?search= 作品列表搜索
//   - authors：GET /api/search/authors 作者搜索
//
// 写入策略与 WebAccessLog 一致：异步 channel + 后台 worker 批量 INSERT
// （每 100 条或 500ms flush），channel 满则丢弃（背压保护，绝不因日志拖垮搜索）。
// 表按 created_at 索引；聚合走 AggregateSearchQueries / AggregateSearchWords。

type SearchLog struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Query       string    `gorm:"size:128;not null;index" json:"query"` // 原始搜索词（截断到 128 字符）
	UserID      uint      `gorm:"index" json:"user_id"`                 // 未登录为 0
	ResultCount int       `json:"result_count"`                         // 命中条数（粗略）
	Source      string    `gorm:"size:16" json:"source"`                // unified / novels / authors
	CreatedAt   time.Time `gorm:"index" json:"created_at"`
}

func (SearchLog) TableName() string { return "search_logs" }

// MaxSearchQueryLen 搜索词入库上限（与 Query 列 size 对齐）
const MaxSearchQueryLen = 128

// RecordSearchQuery 便捷埋点入口：规整 query（trim + 截断）后异步投递。
// 空 query 直接忽略；userID 可为 0（匿名搜索）。
func RecordSearchQuery(query string, userID uint, source string, resultCount int) {
	q := strings.TrimSpace(query)
	if q == "" {
		return
	}
	if rs := []rune(q); len(rs) > MaxSearchQueryLen {
		q = string(rs[:MaxSearchQueryLen])
	}
	EnqueueSearchLog(&SearchLog{Query: q, UserID: userID, Source: source, ResultCount: resultCount})
}

// ─────────────────────────────────────────────────────────────
// 异步批量写入（复用 WebAccessLog 的背压模式）
// ─────────────────────────────────────────────────────────────

var searchLogChan = make(chan *SearchLog, 2048)

func init() {
	go searchLogWorker()
}

// searchLogWorker 后台批量刷盘 worker（DB 未就绪时静默丢弃，不 panic）
func searchLogWorker() {
	const batchSize = 100
	batch := make([]*SearchLog, 0, batchSize)
	ticker := time.NewTicker(500 * time.Millisecond)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		if DB == nil {
			batch = batch[:0]
			return
		}
		if err := DB.CreateInBatches(batch, batchSize).Error; err == nil {
			batch = batch[:0]
		}
	}
	for {
		select {
		case e := <-searchLogChan:
			batch = append(batch, e)
			if len(batch) >= batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// EnqueueSearchLog 投递一条搜索记录（异步写入；channel 满则丢弃，保护主流程）
func EnqueueSearchLog(e *SearchLog) {
	if e == nil {
		return
	}
	select {
	case searchLogChan <- e:
	default:
	}
}

// ─────────────────────────────────────────────────────────────
// 聚合查询（管理员热词面板）
// ─────────────────────────────────────────────────────────────

// clampHotWordWindow 收敛 days/limit 参数（days<=0 默认 7、>365 收敛 365；
// limit<=0 默认 50、>200 收敛 200）
func clampHotWordWindow(days, limit int) (int, int) {
	if days <= 0 {
		days = 7
	}
	if days > 365 {
		days = 365
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return days, limit
}

// AggregateSearchQueries 原始搜索短语 Top N（管理员最关心「用户到底搜了什么」）。
// 按小写化 query 分组计数，保留大小写不敏感的重复；单次出现即计入（≥1）。
func AggregateSearchQueries(days, limit int) ([]HotWord, error) {
	days, limit = clampHotWordWindow(days, limit)
	since := time.Now().AddDate(0, 0, -days)
	var logs []SearchLog
	DB.Select("query").Where("created_at >= ?", since).
		Order("created_at DESC").Limit(searchScanCap).Find(&logs)
	counts := map[string]int{}
	for _, l := range logs {
		q := strings.TrimSpace(l.Query)
		if q == "" {
			continue
		}
		counts[strings.ToLower(q)]++
	}
	return topHotWords(counts, 1, limit), nil
}

// AggregateSearchWords 搜索词分词 Top N（Tokenizer 切分，中英混合）。
// 出现 ≥2 次才视为「搜索热词」，避免单次病态 query 的长尾噪声。
func AggregateSearchWords(days, limit int) ([]HotWord, error) {
	days, limit = clampHotWordWindow(days, limit)
	since := time.Now().AddDate(0, 0, -days)
	var logs []SearchLog
	DB.Select("query").Where("created_at >= ?", since).
		Order("created_at DESC").Limit(searchScanCap).Find(&logs)
	counts := map[string]int{}
	for _, l := range logs {
		for _, w := range Tokenize(l.Query) {
			counts[w]++
		}
	}
	return topHotWords(counts, 2, limit), nil
}

// CountSearchLogs 统计窗口内搜索总次数（管理端概览卡片）
func CountSearchLogs(days int) (int64, error) {
	if days <= 0 {
		days = 7
	}
	if days > 365 {
		days = 365
	}
	since := time.Now().AddDate(0, 0, -days)
	var n int64
	err := DB.Model(&SearchLog{}).Where("created_at >= ?", since).Count(&n).Error
	return n, err
}

// searchScanCap 单次聚合的扫描上限（与内容热词一致）
const searchScanCap = 5000

// topHotWords 从计数 map 生成排序后的热词列表（threshold 以下丢弃，按计数降序、词序升序）
func topHotWords(counts map[string]int, threshold, limit int) []HotWord {
	words := make([]HotWord, 0, len(counts))
	for w, n := range counts {
		if n < threshold {
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
	return words
}

// ─────────────────────────────────────────────────────────────
// 维护入口（管理员手动）
// ─────────────────────────────────────────────────────────────

// SearchLogCleanup 清空全部搜索日志
func SearchLogCleanup() error {
	return DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Where("1 = 1").Delete(&SearchLog{}).Error
}

// SearchLogRetentionCleanup 清理 N 天前的搜索日志（防表无限增长），返回删除行数。
// days<=0 时按 90 天兜底。
func SearchLogRetentionCleanup(days int) (int64, error) {
	if days <= 0 {
		days = 90
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	res := DB.Where("created_at < ?", cutoff).Delete(&SearchLog{})
	return res.RowsAffected, res.Error
}
