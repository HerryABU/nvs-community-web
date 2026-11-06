package readperm

import (
	"net/http"

	"nvs-server/utils/infra"

	rp "nvs-server/readperm"

	"github.com/gin-gonic/gin"
)

// WriteDeny 写出统一的权限拒绝响应。
//
// 除 infra 的标准 message 外，data 里附带 reason（机器码）与生效等级，
// 供前端决定引导动作：去登录 / 关注作者 / 开通会员 / 购买内容。
// 响应结构与 infra.Error 完全一致，只是补了 data，不影响既有调用方。
func WriteDeny(c *gin.Context, d rp.Decision) {
	status, code := http.StatusForbidden, infra.CodeForbidden
	if d.Reason == rp.ReasonLoginRequired {
		status, code = http.StatusUnauthorized, infra.CodeUnauthorized
	}
	c.JSON(status, infra.Response{
		Code:    code,
		Message: d.Message,
		Data: gin.H{
			"reason":          d.Reason,
			"read_permission": d.Level,
		},
	})
	c.Abort()
}
