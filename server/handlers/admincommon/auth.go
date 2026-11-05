package admincommon

import (
	"nvs-server/handlers/hctx"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// EnsureAdmin 管理端统一鉴权。原本定义在 handlers/adminsite/admin.go，因
// adminsite 与 admingov 拆分后双方都需要，故下沉到共享基础包，避免两包互相
// import 形成循环。
func EnsureAdmin(c *gin.Context) bool {
	userID := hctx.CurrentUserID(c)
	user, _ := models.GetUserByID(userID)
	if user == nil || user.Role != "admin" {
		utils.Forbidden(c, "需要管理员权限")
		c.Abort()
		return false
	}
	return true
}
