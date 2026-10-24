package handlers

import (
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// RecordLeakHint POST /api/security/leak-hint
// 只接收「命中类型」（phone/email/idcard/apikey/bankcard/semantic），
// **绝不接收用户输入原文**，也不落用户 ID —— 检测不留痕。
func RecordLeakHint(c *gin.Context) {
	var req struct {
		Types []string `json:"types"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if len(req.Types) > 6 {
		req.Types = req.Types[:6]
	}
	if err := models.RecordLeakHint(req.Types); err != nil {
		utils.InternalError(c, "统计失败")
		return
	}
	utils.Success(c, gin.H{"message": "ok"})
}

// AdminLeakHintStats GET /api/admin/security/leak-hint/stats
// 给站长看趋势（只有类型计数，没有内容，无法反查用户）
func AdminLeakHintStats(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	utils.Success(c, models.GetLeakHintStats())
}
