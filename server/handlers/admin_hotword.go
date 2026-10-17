package handlers

import (
	"strconv"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// GET /api/admin/hotwords?type=content|search|tag — 热词统计（内容治理/舆情观测）。
//
//	type=content（默认）：评论/回帖/帖题高频词（models.AggregateHotWords）
//	type=search          ：搜索热词——原始搜索短语 queries + 分词 words + 搜索总次数
//	type=tag             ：标签热词（博客结构化标签 + 作品 JSON 标签）
//
// days 默认 7（可 7/30/90，>365 收敛 365）；limit 默认 50（>200 收敛 200）。
func AdminHotWords(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	typ := c.DefaultQuery("type", "content")

	switch typ {
	case "search":
		words, err := models.AggregateSearchWords(days, limit)
		if err != nil {
			utils.InternalError(c, "搜索热词统计失败")
			return
		}
		queries, err := models.AggregateSearchQueries(days, limit)
		if err != nil {
			utils.InternalError(c, "搜索热词统计失败")
			return
		}
		total, _ := models.CountSearchLogs(days)
		utils.Success(c, gin.H{
			"type":         "search",
			"words":        words,
			"queries":      queries,
			"search_total": total,
			"days":         days,
			"total":        len(words),
		})
	case "tag":
		words, err := models.AggregateTagHotWords(days, limit)
		if err != nil {
			utils.InternalError(c, "标签热词统计失败")
			return
		}
		utils.Success(c, gin.H{
			"type":  "tag",
			"words": words,
			"days":  days,
			"total": len(words),
		})
	default: // content
		words, err := models.AggregateHotWords(days, limit)
		if err != nil {
			utils.InternalError(c, "热词统计失败")
			return
		}
		utils.Success(c, gin.H{
			"type":  "content",
			"words": words,
			"days":  days,
			"total": len(words),
		})
	}
}

// POST /api/admin/search-logs/cleanup?days=0 — 维护搜索埋点日志。
//
//	days>0：仅清理 N 天前的记录（保留近期数据，返回删除行数）
//	days=0：清空全部搜索日志
func AdminSearchLogCleanup(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	days, _ := strconv.Atoi(c.DefaultQuery("days", "0"))
	if days > 0 {
		removed, err := models.SearchLogRetentionCleanup(days)
		if err != nil {
			utils.InternalError(c, "清理搜索日志失败")
			return
		}
		utils.Success(c, gin.H{"removed": removed})
		return
	}
	if err := models.SearchLogCleanup(); err != nil {
		utils.InternalError(c, "清空搜索日志失败")
		return
	}
	utils.Success(c, gin.H{"removed": -1})
}
