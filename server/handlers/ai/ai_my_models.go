package ai

import (
	"time"

	"nvs-server/handlers/hctx"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// AiMyModels GET /api/ai/my-models
//
// 「我的」筛选：返回当前用户**最近用过**的模型 id 列表（默认 30 天内、按最近使用排序）。
// 参考主流 AI 站的模型广场：全部 / 对话 / 图像 / 视频 / 我的 —— 「我的」不是收藏夹，
// 而是「我实际调用过的」，这样新用户不用手动维护，老用户一眼看到常用模型。
//
// 数据来源是 AiCallLog（已有），不需要新表、也不需要额外埋点。
func AiMyModels(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}
	since := time.Now().AddDate(0, 0, -30)

	type row struct {
		ModelID uint
		LastAt  time.Time
	}
	var rows []row
	// 按模型分组取最近一次调用时间，再按时间倒序（最近用过的排在前面）
	models.DB.Model(&models.AiCallLog{}).
		Select("model_id, MAX(created_at) as last_at").
		Where("user_id = ? AND created_at >= ? AND model_id > 0", uid, since).
		Group("model_id").
		Order("last_at DESC").
		Limit(50).
		Scan(&rows)

	ids := make([]uint, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ModelID)
	}
	utils.Success(c, gin.H{"model_ids": ids, "days": 30})
}
