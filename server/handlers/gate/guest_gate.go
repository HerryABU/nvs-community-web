package gate

import (
	"nvs-server/handlers/hctx"
	"nvs-server/utils/infra"

	"github.com/gin-gonic/gin"
	siteops "nvs-server/models/siteops"
)

// 访客门禁（站级开关）
//
// 背景：后台「访客设置」里的 allow_guest_browse / allow_guest_search 之前
// **只有管理端读写、没有任何读取链路校验**，属于空转开关 ——
// 站长关掉「允许未登录浏览」后，前台行为毫无变化。
// 这里提供统一的判定入口，让开关真正生效。

// guestReadBlocked 匿名且站长关闭了「允许未登录浏览」时返回 true。
func guestReadBlocked(c *gin.Context) bool {
	if hctx.CurrentUserID(c) != 0 {
		return false
	}
	return !siteops.LoadVisitSetting().AllowGuestBrowse
}

// guestSearchBlocked 匿名且站长关闭了「允许未登录搜索」时返回 true。
func guestSearchBlocked(c *gin.Context) bool {
	if hctx.CurrentUserID(c) != 0 {
		return false
	}
	return !siteops.LoadVisitSetting().AllowGuestSearch
}

// RequireGuestRead 便捷守卫：被拦时已写好响应，调用方直接 return。
func RequireGuestRead(c *gin.Context) bool {
	if guestReadBlocked(c) {
		infra.Unauthorized(c, "本站已关闭未登录浏览，请先登录")
		return false
	}
	return true
}

// RequireGuestSearch 便捷守卫：被拦时已写好响应，调用方直接 return。
func RequireGuestSearch(c *gin.Context) bool {
	if guestSearchBlocked(c) {
		infra.Unauthorized(c, "本站已关闭未登录搜索，请先登录")
		return false
	}
	return true
}
