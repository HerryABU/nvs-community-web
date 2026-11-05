package notification

import (
	"nvs-server/utils/infra"
	"strconv"

	"nvs-server/handlers/hctx"
	"nvs-server/models"

	"github.com/gin-gonic/gin"
)

// ============ 站内消息通知 ============

// GET /api/notifications — 分页列出我的通知（未读优先）
func ListNotifications(c *gin.Context) {
	userID := hctx.CurrentUserID(c)
	if userID == 0 {
		infra.Unauthorized(c, "未登录")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	list, total, err := models.ListNotifications(userID, page, pageSize)
	if err != nil {
		infra.InternalError(c, "获取通知失败")
		return
	}
	infra.Success(c, gin.H{"list": list, "total": total, "page": page, "page_size": pageSize})
}

// GET /api/notifications/unread-count — 未读通知数（导航栏徽标轮询用）
func UnreadNotificationCount(c *gin.Context) {
	userID := hctx.CurrentUserID(c)
	if userID == 0 {
		infra.Unauthorized(c, "未登录")
		return
	}
	count, err := models.CountUnreadNotifications(userID)
	if err != nil {
		count = 0
	}
	infra.Success(c, gin.H{"count": count})
}

// POST /api/notifications/read — 标记已读。body: {"id": 123}；id 缺省/0 = 全部已读。
func MarkNotificationRead(c *gin.Context) {
	userID := hctx.CurrentUserID(c)
	if userID == 0 {
		infra.Unauthorized(c, "未登录")
		return
	}
	var req struct {
		ID uint `json:"id"`
	}
	_ = c.ShouldBindJSON(&req)
	if _, err := models.MarkNotificationRead(userID, req.ID); err != nil {
		infra.InternalError(c, "标记已读失败")
		return
	}
	infra.SuccessMessage(c, "已更新", nil)
}
