package handlers

// ==================== 发文/评论门槛的落地hook（gap-audit #14）====================
// gate 包只做「判定」，不查业务表；所以这里负责把「今日已发条数」等业务数据喂给它。
// 口径统一在这一个文件里，避免发文/评论/回帖各写一份「新号一天能发几条」。

import (
	"time"

	"nvs-server/gate"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// checkPostGate 发文/评论前调用；被拦截时已写好响应，调用方 return 即可。
// kind: post（作品章节）/ comment（评论与回帖）
func checkPostGate(c *gin.Context, kind string) bool {
	if !gate.Load().Enabled {
		return true
	}
	uid := currentUserID(c)
	if uid == 0 {
		return true // 未登录由各 handler 自己拦
	}
	user, err := models.GetUserByID(uid)
	if err != nil || user == nil {
		return true
	}
	today := todayCountByUser(uid, kind)
	if gerr := gate.CanPost(gate.PostGateInput{
		UserID: uid, CreatedAt: user.CreatedAt, Kind: kind, TodayCount: today,
	}); gerr != nil {
		utils.Forbidden(c, gerr.Error())
		return false
	}
	return true
}

// todayStart 今日零点（本地时区，与业务「每日限额」口径一致）
func todayStart() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

// todayCountByUser 今日已发条数（只统计未软删除的）
func todayCountByUser(userID uint, kind string) int {
	if models.DB == nil || userID == 0 {
		return 0
	}
	var n int64
	switch kind {
	case "comment":
		_ = models.DB.Raw(
			"SELECT COUNT(1) FROM comments WHERE user_id = ? AND created_at >= ?", userID, todayStart()).
			Scan(&n).Error
	default:
		_ = models.DB.Raw(
			"SELECT COUNT(1) FROM chapters WHERE author_id = ? AND created_at >= ?", userID, todayStart()).
			Scan(&n).Error
	}
	return int(n)
}
