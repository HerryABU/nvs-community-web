package author

import (
	"strconv"
	"time"

	"nvs-server/handlers/hctx"
	"nvs-server/models"
	"nvs-server/utils/infra"

	"github.com/gin-gonic/gin"
)

// ==================== 个人活跃热力图 ====================
// 汇总当前用户近 N 天的创作/互动（评论、论坛回复、发帖），按天计数，
// 供个人中心渲染「活跃日历」（GitHub 风格）。

type activityRow struct {
	Day string
	Cnt int64
}

func sumRows(dst map[string]int64, rows []activityRow) {
	for _, r := range rows {
		if r.Day != "" {
			dst[r.Day] += r.Cnt
		}
	}
}

// UserActivityHeat GET /api/users/me/activity-heat?days=365
func UserActivityHeat(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	if uid == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	days := 365
	if v := c.Query("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 3660 {
			days = n
		}
	}
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02 15:04:05")

	merged := map[string]int64{}
	// 评论
	var comments []activityRow
	_ = models.DB.Raw(
		"SELECT date(created_at) AS day, COUNT(*) AS cnt FROM comments WHERE user_id = ? AND deleted_at IS NULL AND created_at >= ? GROUP BY day",
		uid, since,
	).Scan(&comments).Error
	sumRows(merged, comments)
	// 论坛回复
	var posts []activityRow
	_ = models.DB.Raw(
		"SELECT date(created_at) AS day, COUNT(*) AS cnt FROM posts WHERE user_id = ? AND created_at >= ? GROUP BY day",
		uid, since,
	).Scan(&posts).Error
	sumRows(merged, posts)
	// 论坛主题
	var threads []activityRow
	_ = models.DB.Raw(
		"SELECT date(created_at) AS day, COUNT(*) AS cnt FROM threads WHERE user_id = ? AND created_at >= ? GROUP BY day",
		uid, since,
	).Scan(&threads).Error
	sumRows(merged, threads)
	// 群组帖子
	var groupPosts []activityRow
	_ = models.DB.Raw(
		"SELECT date(created_at) AS day, COUNT(*) AS cnt FROM group_posts WHERE user_id = ? AND created_at >= ? GROUP BY day",
		uid, since,
	).Scan(&groupPosts).Error
	sumRows(merged, groupPosts)

	// 生成连续日期序列（含 0），便于前端直接渲染日历
	out := make([]gin.H, 0, days)
	start := time.Now().AddDate(0, 0, -(days - 1))
	for i := 0; i < days; i++ {
		d := start.AddDate(0, 0, i).Format("2006-01-02")
		out = append(out, gin.H{"date": d, "count": merged[d]})
	}
	infra.Success(c, gin.H{"days": days, "items": out})
}
