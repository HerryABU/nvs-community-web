package comment

import (
	"nvs-server/handlers/admincommon"
	"strconv"
	"strings"

	"nvs-server/crisis"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	"nvs-server/models/security"
)

// 未成年人心理危机防护（「赛博守护神」方案）的接入层（HTTP 边界）。
//
// 领域逻辑已抽离到独立包 `server/crisis`（检测 + 处置 + 报告一体，评论/论坛/AI 对话三入口统一调用）。
// 本文件只保留 gin 边界：把 HTTP 请求转交给 crisis 包，职责单一、不再膨胀成上帝文件。

// GetCrisisReport GET /api/admin/crisis-report
//
//	?user_id=N  → 生成该用户的干预报告（Markdown）
//	不传        → 返回当前处于高危观察期的用户清单（总览入口）
func GetCrisisReport(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	raw := strings.TrimSpace(c.Query("user_id"))
	if raw != "" {
		uid, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || uid == 0 {
			utils.BadRequest(c, "user_id 不合法")
			return
		}
		report, rerr := crisis.BuildCrisisReport(uint(uid))
		if rerr != nil {
			utils.NotFound(c, "用户不存在")
			return
		}
		utils.Success(c, gin.H{"user_id": uid, "report": report})
		return
	}

	users, uerr := security.ListUsersInObservation()
	if uerr != nil {
		utils.InternalError(c, "读取观察期用户失败")
		return
	}
	list := make([]gin.H, 0, len(users))
	for _, u := range users {
		n, _ := security.CountCrisisEventsByUser(u.ID)
		until := ""
		if u.RiskYouthMentalUntil != nil {
			until = u.RiskYouthMentalUntil.Format("2006-01-02 15:04")
		}
		list = append(list, gin.H{
			"user_id":       u.ID,
			"username":      u.Username,
			"nickname":      u.Nickname,
			"event_count":   n,
			"observe_until": until,
			"is_minor":      crisis.IsMinorUser(&u),
		})
	}
	utils.Success(c, gin.H{"users": list})
}
