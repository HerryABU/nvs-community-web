package handlers

import (
	"strconv"
	"strings"
	"time"

	"nvs-server/models"
	"nvs-server/security"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// maxMessageContent 私信内容最大字符数（与前端 maxlength 一致；
// MySQL TEXT 可容纳，此处按字符数截断防多字节超长）
const maxMessageContent = 2000

// auditMsgSend / auditMsgSettings 用户行为审计动作标识（user_audit_logs.action，长度<=30）
const (
	auditMsgSend     = "send_message"
	auditMsgSettings = "message_settings"
	auditMsgRecall   = "recall_message"
	auditMsgDelConv  = "delete_conversation"
)

// SendMessage POST /api/messages
// body: {receiver_id, content}
// 规则：未登录 401；不能给自己发；内容截断 2000 字符；
// 第 3 条消息起，若接收方 RequireFollowToMessage=true 则要求双向关注，否则 403；
// 接收方拉黑（F13 block 模式）发送方时拒绝发送。
func SendMessage(c *gin.Context) {
	userID := currentUserID(c)
	if userID == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}

	// 全局禁言用户禁止发私信（防骚扰，与评论/帖子一致）
	if checkUserMuted(c) {
		return
	}

	var req struct {
		ReceiverID uint   `json:"receiver_id"`
		Content    string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if req.ReceiverID == 0 {
		utils.BadRequest(c, "请指定接收方")
		return
	}
	if req.ReceiverID == userID {
		utils.BadRequest(c, "不能给自己发送消息")
		return
	}

	// 净化 HTML + 按字符数截断（前端同步 maxlength=2000）
	content := utils.ClampString(security.SanitizePlainText(strings.TrimSpace(req.Content)), maxMessageContent)
	if content == "" {
		utils.BadRequest(c, "请填写消息内容")
		return
	}

	receiver, err := models.GetUserByID(req.ReceiverID)
	if err != nil {
		utils.NotFound(c, "对方不存在")
		return
	}

	// 黑名单门禁（F13）：被接收方拉黑（block 模式）者无法发送私信
	if models.IsBlacklisted(req.ReceiverID, userID) {
		utils.Forbidden(c, "你已被对方拉黑，无法发送私信")
		return
	}

	// 第 3 条消息起（已有 >=2 条），若对方要求互相关注，则校验双向关注
	count, err := models.CountBetween(userID, req.ReceiverID)
	if err != nil {
		utils.InternalError(c, "发送失败")
		return
	}
	if count >= 2 && receiver.RequireFollowToMessage {
		if !models.IsFollowing(userID, req.ReceiverID) || !models.IsFollowing(req.ReceiverID, userID) {
			utils.Forbidden(c, "对方设置了互相关注才能继续发送")
			return
		}
	}

	msg := &models.Message{
		SenderID:   userID,
		ReceiverID: req.ReceiverID,
		Content:    content,
	}
	if err := models.CreateMessage(msg); err != nil {
		utils.InternalError(c, "发送失败")
		return
	}

	// 合规审计：记录私信发送（接收方ID，不留内容全文）
	recordUserAudit(c, userID, auditMsgSend, "发送私信给用户 "+strconv.FormatUint(uint64(req.ReceiverID), 10), "success")

	// 站内通知（best-effort，失败不影响发送结果）
	senderName := ""
	if sender, err := models.GetUserByID(userID); err == nil {
		senderName = sender.Nickname
		if senderName == "" {
			senderName = sender.Username
		}
	}
	_ = models.PushNotification(userID, req.ReceiverID, "message",
		senderName+" 给你发来一条私信", utils.ClampString(content, 50), "/user/messages")

	utils.Success(c, msg)
}

// RecallMessage POST /api/messages/recall
// body: {id} —— 仅发送者本人、2 分钟内、未撤回的消息可撤回
func RecallMessage(c *gin.Context) {
	userID := currentUserID(c)
	if userID == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}
	var req struct {
		ID uint `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == 0 {
		utils.BadRequest(c, "参数错误")
		return
	}
	msg, err := models.GetMessageByID(req.ID)
	if err != nil {
		utils.NotFound(c, "消息不存在")
		return
	}
	if msg.SenderID != userID {
		utils.Forbidden(c, "只能撤回自己发送的消息")
		return
	}
	if msg.Recalled {
		utils.BadRequest(c, "消息已撤回")
		return
	}
	if time.Since(msg.CreatedAt) > 2*time.Minute {
		utils.BadRequest(c, "超过 2 分钟的消息无法撤回")
		return
	}
	if _, err := models.RecallMessage(msg.ID, userID); err != nil {
		utils.InternalError(c, "撤回失败")
		return
	}
	recordUserAudit(c, userID, auditMsgRecall, "撤回私信 "+strconv.FormatUint(uint64(msg.ID), 10), "success")
	utils.SuccessMessage(c, "已撤回", nil)
}

// DeleteConversation POST /api/messages/conversation/delete
// body: {peer_id} —— 单方删除会话（对方视图不受影响）
func DeleteConversation(c *gin.Context) {
	userID := currentUserID(c)
	if userID == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}
	var req struct {
		PeerID uint `json:"peer_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.PeerID == 0 {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := models.DeleteConversationBetween(userID, req.PeerID); err != nil {
		utils.InternalError(c, "删除会话失败")
		return
	}
	recordUserAudit(c, userID, auditMsgDelConv, "删除与用户 "+strconv.FormatUint(uint64(req.PeerID), 10)+" 的会话", "success")
	utils.SuccessMessage(c, "会话已删除", nil)
}

// ListConversations GET /api/messages/conversations
func ListConversations(c *gin.Context) {
	userID := currentUserID(c)
	if userID == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}
	convs, err := models.ListConversations(userID)
	if err != nil {
		utils.InternalError(c, "获取会话列表失败")
		return
	}
	utils.Success(c, gin.H{"list": convs, "total": len(convs)})
}

// ListMessages GET /api/messages/with/:userId?page=&page_size=
// 返回按 id 倒序（新消息在前），前端展示时反转 + 上拉加载更早消息
func ListMessages(c *gin.Context) {
	userID := currentUserID(c)
	if userID == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}
	peerID, err := strconv.ParseUint(c.Param("userId"), 10, 64)
	if err != nil || peerID == 0 {
		utils.BadRequest(c, "参数错误")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "30"))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 30
	}

	list, total, err := models.ListMessagesBetween(userID, uint(peerID), page, pageSize)
	if err != nil {
		utils.InternalError(c, "获取聊天记录失败")
		return
	}
	utils.Success(c, gin.H{
		"list":     list,
		"total":    total,
		"page":     page,
		"has_more": int64((page-1)*pageSize+len(list)) < total,
	})
}

// MarkMessageRead POST /api/messages/read
// body: {peer_id} —— 把对方发给我的未读消息标记已读
func MarkMessageRead(c *gin.Context) {
	userID := currentUserID(c)
	if userID == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}
	var req struct {
		PeerID uint `json:"peer_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.PeerID == 0 {
		utils.BadRequest(c, "参数错误")
		return
	}
	if _, err := models.MarkReadBetween(userID, req.PeerID); err != nil {
		utils.InternalError(c, "操作失败")
		return
	}
	utils.SuccessMessage(c, "已标记为已读", nil)
}

// UnreadMessageCount GET /api/messages/unread-count
func UnreadMessageCount(c *gin.Context) {
	userID := currentUserID(c)
	if userID == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}
	count, err := models.UnreadCount(userID)
	if err != nil {
		utils.InternalError(c, "获取未读数失败")
		return
	}
	utils.Success(c, gin.H{"unread": count})
}

// GetMessageSettings GET /api/messages/settings
func GetMessageSettings(c *gin.Context) {
	userID := currentUserID(c)
	if userID == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}
	user, err := models.GetUserByID(userID)
	if err != nil {
		utils.NotFound(c, "用户不存在")
		return
	}
	utils.Success(c, gin.H{"require_follow_to_message": user.RequireFollowToMessage})
}

// UpdateMessageSettings PUT /api/messages/settings
// body: {require_follow_to_message}
func UpdateMessageSettings(c *gin.Context) {
	userID := currentUserID(c)
	if userID == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}
	var req struct {
		RequireFollowToMessage *bool `json:"require_follow_to_message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.RequireFollowToMessage == nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := models.DB.Model(&models.User{}).Where("id = ?", userID).
		Update("require_follow_to_message", *req.RequireFollowToMessage).Error; err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	recordUserAudit(c, userID, auditMsgSettings, "修改私信设置：互相关注才能发消息", "success")
	utils.Success(c, gin.H{"require_follow_to_message": *req.RequireFollowToMessage})
}
