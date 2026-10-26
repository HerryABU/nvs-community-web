package handlers

import (
	"strconv"
	"time"

	"nvs-server/models"
	"nvs-server/sentiment/guard"
	"nvs-server/sentiment/store"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// 警方台的舆情面板接口。
//
// 权限边界（按评审要求收口）：
//   - 可以看：舆情拦截**数目**、按话题分布、按决策分布、词条待审数
//   - 不可以看：被拦截评论的正文、检索摘要原文、合规与安全配置（脱敏/实名/密钥/蜜罐）
//
// 看统计是为了判断「这个站的舆论是否失控」，不是为了逐条读内容；
// 逐条取证走「内容治理 → 留证列表」，那里有独立的权限与审计。

// ensurePolice 警方或管理员。
func ensurePoliceGate(c *gin.Context) bool {
	if models.IsAdminOrPolice(currentUserRole(c)) {
		return true
	}
	utils.Forbidden(c, "需要警方或管理员权限")
	return false
}

// PoliceSentimentStats GET /api/police/sentiment/stats
func PoliceSentimentStats(c *gin.Context) {
	if !ensurePoliceGate(c) {
		return
	}
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	s, err := store.LoadStats(days)
	if err != nil {
		utils.InternalError(c, "统计失败")
		return
	}
	// 只给聚合口径，不给任何正文/词条详情
	utils.Success(c, gin.H{
		"stats": s,
		"fold_chars": func() int {
			if cfg, e := store.GetConfig(); e == nil && cfg != nil {
				return cfg.FoldChars
			}
			return 0
		}(),
	})
}

// PoliceSentimentTrend GET /api/police/sentiment/trend?days=14
// 按天的拦截/标注曲线（警方台顶部图表用，只有计数）。
func PoliceSentimentTrend(c *gin.Context) {
	if !ensurePoliceGate(c) {
		return
	}
	days, _ := strconv.Atoi(c.DefaultQuery("days", "14"))
	if days < 1 || days > 90 {
		days = 14
	}
	type row struct {
		Day     string `json:"day"`
		Blocked int64  `json:"blocked"`
		Flagged int64  `json:"flagged"`
	}
	out := make([]row, 0, days)
	for i := days - 1; i >= 0; i-- {
		start := time.Now().AddDate(0, 0, -i).Truncate(24 * time.Hour)
		end := start.AddDate(0, 0, 1)
		var blocked, flagged int64
		store.DB.Model(&store.Hit{}).
			Where("created_at >= ? AND created_at < ? AND decision = ?", start, end, guard.ActionBlock).
			Count(&blocked)
		store.DB.Model(&store.Hit{}).
			Where("created_at >= ? AND created_at < ? AND decision = ?", start, end, guard.ActionFlag).
			Count(&flagged)
		out = append(out, row{Day: start.Format("01-02"), Blocked: blocked, Flagged: flagged})
	}
	utils.Success(c, gin.H{"items": out})
}
