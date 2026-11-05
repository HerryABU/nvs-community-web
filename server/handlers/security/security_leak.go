package security

import (
	"nvs-server/handlers/admincommon"
	"nvs-server/utils/infra"

	"github.com/gin-gonic/gin"
	securitym "nvs-server/models/security"
)

// RecordLeakHint POST /api/security/leak-hint
// 只接收「命中类型」（phone/email/idcard/apikey/bankcard/semantic），
// **绝不接收用户输入原文**，也不落用户 ID —— 检测不留痕。
func RecordLeakHint(c *gin.Context) {
	var req struct {
		Types []string `json:"types"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	if len(req.Types) > 6 {
		req.Types = req.Types[:6]
	}
	if err := securitym.RecordLeakHint(req.Types); err != nil {
		infra.InternalError(c, "统计失败")
		return
	}
	infra.Success(c, gin.H{"message": "ok"})
}

// AdminLeakHintStats GET /api/admin/security/leak-hint/stats
// 给站长看趋势（只有类型计数，没有内容，无法反查用户）
func AdminLeakHintStats(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	infra.Success(c, securitym.GetLeakHintStats())
}
