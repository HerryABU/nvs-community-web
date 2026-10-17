package handlers

import (
	"nvs-server/security/slidecaptcha"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// POST /api/auth/captcha/issue — 下发滑块验证码挑战（服务端随机缺口位置 + 渲染背景/拼图 PNG）。
// 前端据此渲染拼图，用户拖动后上报 x 坐标。
func IssueCaptcha(c *gin.Context) {
	id, bg, piece, err := slidecaptcha.Issue()
	if err != nil {
		utils.InternalError(c, "生成验证码失败: "+err.Error())
		return
	}
	utils.Success(c, gin.H{
		"challenge_id": id,
		"bg_image":     bg,
		"piece_image":  piece,
		"width":        320,
		"height":       160,
		"tip":          "拖动拼图到缺口位置完成验证",
	})
}

// POST /api/auth/captcha/verify — 校验滑块位置，通过则签发一次性 pass token。
// 该 token 必须随 /api/auth/send-code、/api/auth/register 提交，否则接口拒绝。
func VerifyCaptcha(c *gin.Context) {
	var req struct {
		ChallengeID string `json:"challenge_id" binding:"required"`
		X           int    `json:"x" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	token, ok := slidecaptcha.Verify(req.ChallengeID, req.X)
	if !ok {
		utils.BadRequest(c, "验证失败，请重试")
		return
	}
	utils.Success(c, gin.H{"captcha_token": token})
}
