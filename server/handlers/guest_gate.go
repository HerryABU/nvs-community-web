package handlers

import (
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// 访客门禁（站级开关）
//
// 背景：后台「访客设置」里的 allow_guest_browse / allow_guest_search 之前
// **只有管理端读写、没有任何读取链路校验**，属于空转开关 ——
// 站长关掉「允许未登录浏览」后，前台行为毫无变化。
// 这里提供统一的判定入口，让开关真正生效。

// guestReadBlocked 匿名且站长关闭了「允许未登录浏览」时返回 true。
func guestReadBlocked(c *gin.Context) bool {
	if currentUserID(c) != 0 {
		return false
	}
	return !models.LoadVisitSetting().AllowGuestBrowse
}

// guestSearchBlocked 匿名且站长关闭了「允许未登录搜索」时返回 true。
func guestSearchBlocked(c *gin.Context) bool {
	if currentUserID(c) != 0 {
		return false
	}
	return !models.LoadVisitSetting().AllowGuestSearch
}

// requireGuestRead 便捷守卫：被拦时已写好响应，调用方直接 return。
func requireGuestRead(c *gin.Context) bool {
	if guestReadBlocked(c) {
		utils.Unauthorized(c, "本站已关闭未登录浏览，请先登录")
		return false
	}
	return true
}

// requireGuestSearch 便捷守卫：被拦时已写好响应，调用方直接 return。
func requireGuestSearch(c *gin.Context) bool {
	if guestSearchBlocked(c) {
		utils.Unauthorized(c, "本站已关闭未登录搜索，请先登录")
		return false
	}
	return true
}
