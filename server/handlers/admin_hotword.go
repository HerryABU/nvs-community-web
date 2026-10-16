package handlers

import (
	"strconv"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// GET /api/admin/hotwords — 热词统计（内容治理/舆情观测）。
// 按需聚合最近 N 天（默认 7，可 7/30/90）评论/回帖/帖题的高频词，取前 limit 项。
func AdminHotWords(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	words, err := models.AggregateHotWords(days, limit)
	if err != nil {
		utils.InternalError(c, "热词统计失败")
		return
	}
	utils.Success(c, gin.H{
		"words": words,
		"days":  days,
		"total": len(words),
	})
}
