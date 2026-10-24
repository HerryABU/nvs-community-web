package grouphandler

import (
	"strconv"

	"nvs-server/groups"
	"nvs-server/middleware"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// MarkRead POST /api/groups/:id/read { post_id? } —— 标记群消息已读
// 口径：把「最后阅读时间」推到现在；未读数随之归零。非成员不能标记（也就没有未读）。
func MarkRead(c *gin.Context) {
	uid := middleware.UserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	gid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if gid == 0 {
		utils.BadRequest(c, "群组 ID 无效")
		return
	}
	ctx, ok := loadGroupCtx(c, uint(gid))
	if !ok {
		return
	}
	if ctx.Status != groups.GroupMemberActive {
		utils.Success(c, gin.H{"unread": 0})
		return
	}
	var req struct {
		PostID uint `json:"post_id"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := groups.MarkGroupRead(uint(gid), uid, req.PostID); err != nil {
		utils.InternalError(c, "标记已读失败")
		return
	}
	utils.Success(c, gin.H{"unread": 0})
}
