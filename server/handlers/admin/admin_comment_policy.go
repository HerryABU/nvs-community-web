package admin

import (
	"github.com/gin-gonic/gin"

	"nvs-server/models"
	"nvs-server/utils"
)

// 评论区策略的管理端接口。
//
// 背景：「楼加密」有两种语义 —— 只锁楼正文（评论区照常公开），
// 或连楼内回复一起藏（整块对话私密）。两种都保留，由站长按站点调性选。
// 详见 models/comment_policy.go 的说明。

// AdminGetCommentPolicy GET /api/admin/comment-policy
func AdminGetCommentPolicy(c *gin.Context) {
	if !EnsureAdmin(c) {
		return
	}
	utils.Success(c, gin.H{"config": models.LoadCommentPolicy()})
}

// AdminSaveCommentPolicy PUT /api/admin/comment-policy
func AdminSaveCommentPolicy(c *gin.Context) {
	if !EnsureAdmin(c) {
		return
	}
	var in models.CommentPolicy
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数不合法")
		return
	}
	if err := models.SaveCommentPolicy(in); err != nil {
		utils.InternalError(c, "保存失败："+err.Error())
		return
	}
	utils.Success(c, gin.H{"config": models.LoadCommentPolicy()})
}
