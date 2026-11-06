package vipcard

import (
	"time"

	"nvs-server/handlers/hctx"
	"nvs-server/utils/infra"

	vc "nvs-server/vipcard"

	"github.com/gin-gonic/gin"
)

// Redeem POST /api/vip-cards/redeem — 用户兑换（路由上挂了限流防爆破）
func Redeem(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "请输入卡密")
		return
	}
	r, err := vc.Redeem(hctx.CurrentUserID(c), req.Code)
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.Success(c, gin.H{
		"message": "兑换成功",
		"level":   r.Level,
		"until":   r.Until,
		"days":    r.Days,
	})
}

// MyMembership GET /api/vip-cards/membership — 当前会员等级与到期时间
func MyMembership(c *gin.Context) {
	level, until, err := vc.Membership(hctx.CurrentUserID(c))
	if err != nil {
		infra.InternalError(c, "查询失败")
		return
	}
	active := until != nil && until.After(time.Now())
	infra.Success(c, gin.H{
		"level":        level,
		"member_until": until,
		"active":       active,
	})
}
