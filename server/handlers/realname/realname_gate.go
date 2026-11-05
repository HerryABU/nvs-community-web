// realname_gate.go — 实名强制门禁（F10）。
// 站长开启 realname_required 后，发布作品/章节/评论需已完成实名；
// 站长（admin）与警方（police）不受限。关闭时为零开销（一次配置读取）。
package realname

import (
	"nvs-server/handlers/hctx"
	"nvs-server/models"
	"nvs-server/utils/infra"

	"github.com/gin-gonic/gin"
)

// CheckRealNameRequired 未实名且站长要求实名时拦截并返回 true（已代发错误响应）。
func CheckRealNameRequired(c *gin.Context) bool {
	if !models.IsRealNameRequired() {
		return false
	}
	userID := hctx.CurrentUserID(c)
	if userID == 0 {
		return false // 未登录由后续鉴权处理
	}
	// 管理员/警方豁免（取证与治理需要）
	user, err := models.GetUserByID(userID)
	if err != nil || user == nil {
		return false
	}
	if user.Role == "admin" || user.Role == "police" {
		return false
	}
	if user.RealNameVerified {
		return false
	}
	infra.Forbidden(c, "本站要求实名后即可发布内容，请先完成实名认证")
	return true
}
