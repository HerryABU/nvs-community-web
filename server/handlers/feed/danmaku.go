package feed

// ==================== 视频弹幕接口 ====================
// 公开读（未登录也能看弹幕），登录可发。

import (
	"strconv"
	"strings"

	"nvs-server/handlers/hctx"
	"nvs-server/models/feed"
	"nvs-server/utils/infra"

	"github.com/gin-gonic/gin"
)

// GET /api/feed/:id/danmaku?from=0&to=60000&limit=3000
func ListDanmaku(c *gin.Context) {
	postID := parseUint(c.Param("id"))
	if postID == 0 {
		infra.BadRequest(c, "参数错误")
		return
	}
	from, _ := strconv.Atoi(c.DefaultQuery("from", "0"))
	to, _ := strconv.Atoi(c.DefaultQuery("to", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "3000"))
	list, err := feed.ListDanmaku(postID, from, to, limit)
	if err != nil {
		infra.InternalError(c, "弹幕加载失败")
		return
	}
	infra.Success(c, gin.H{"list": list, "total": feed.CountDanmaku(postID)})
}

// POST /api/feed/:id/danmaku
func CreateDanmaku(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	if uid == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	var in struct {
		Content string `json:"content"`
		TimeMs  int    `json:"time_ms"`
		Mode    string `json:"mode"`
		Color   string `json:"color"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	// 与评论同一套清洗：去 HTML + 敏感词替换，弹幕只能纯文本
	in.Content = sanitize(strings.TrimSpace(in.Content))
	d, err := feed.CreateDanmaku(parseUint(c.Param("id")), uid, in.Content, in.TimeMs, in.Mode, in.Color)
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "弹幕已发送", gin.H{
		"id": d.ID, "time_ms": d.TimeMs, "content": d.Content, "mode": d.Mode, "color": d.Color,
	})
}

// DELETE /api/feed/danmaku/:did —— 删自己的弹幕
func DeleteDanmaku(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	if uid == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	if err := feed.DeleteDanmaku(uid, parseUint(c.Param("did"))); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已删除弹幕", nil)
}
