package grouphandler

import (
	"nvs-server/groups"
	"nvs-server/handlers/hctx"
	"nvs-server/utils/infra"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ==================== 群聊（类微信/QQ 的群内即时聊天） ====================
// 与「帖子」互补：帖子偏主题讨论，群聊偏即时交流。仅群成员（active）可读写。

// canGroupChat 校验群存在且当前用户是已加入成员
func canGroupChat(c *gin.Context, groupID uint) (*groups.Group, bool) {
	g, err := groups.GetGroup(groupID)
	if err != nil {
		infra.NotFound(c, "群组不存在")
		return nil, false
	}
	userID := hctx.CurrentUserID(c)
	if userID == 0 {
		infra.Unauthorized(c, "请先登录")
		return nil, false
	}
	m, _ := groups.GetMembership(groupID, userID)
	if m == nil || m.Status != "active" {
		infra.Forbidden(c, "仅群成员可参与群聊")
		return nil, false
	}
	return g, true
}

// ListChat GET /api/groups/:id/chat — 最近 50 条群聊消息
func ListChat(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if _, ok := canGroupChat(c, uint(id)); !ok {
		return
	}
	list, err := groups.ListChatMessages(uint(id), 50)
	if err != nil {
		infra.InternalError(c, "获取聊天记录失败")
		return
	}
	infra.Success(c, gin.H{"list": list})
}

// SendChat POST /api/groups/:id/chat — 发送群聊消息
func SendChat(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if _, ok := canGroupChat(c, uint(id)); !ok {
		return
	}
	var req struct {
		Content   string `json:"content"`
		ReplyToID uint   `json:"reply_to_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		infra.BadRequest(c, "消息不能为空")
		return
	}
	if len([]rune(content)) > 2000 {
		infra.BadRequest(c, "单条消息过长（≤2000 字）")
		return
	}
	m := &groups.GroupChatMessage{
		GroupID: uint(id), UserID: hctx.CurrentUserID(c),
		Content: content, ReplyToID: req.ReplyToID, CreatedAt: time.Now(),
	}
	if err := groups.CreateChatMessage(m); err != nil {
		infra.InternalError(c, "发送失败")
		return
	}
	infra.SuccessMessage(c, "已发送", m)
}
