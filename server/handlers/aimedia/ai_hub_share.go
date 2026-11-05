package aimedia

import (
	"nvs-server/handlers/hctx"
	"nvs-server/utils/infra"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	aim "nvs-server/models/ai"
)

// ==================== AI 会话：重命名 / 分享 / 公开只读 ====================

// AiHubRenameSession PUT /api/ai/sessions/:id — 重命名会话
func AiHubRenameSession(c *gin.Context) {
	userID := hctx.CurrentUserID(c)
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Title string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	if err := aim.RenameAiChatSession(uint(id), userID, req.Title); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.Success(c, gin.H{"message": "已重命名"})
}

// AiHubShareSession POST /api/ai/sessions/:id/share — 生成/读取分享链接
func AiHubShareSession(c *gin.Context) {
	userID := hctx.CurrentUserID(c)
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	token, err := aim.ShareAiChatSession(uint(id), userID)
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.Success(c, gin.H{"share_token": token, "path": "/ai/share/" + token})
}

// AiHubUnshareSession DELETE /api/ai/sessions/:id/share — 取消分享
func AiHubUnshareSession(c *gin.Context) {
	userID := hctx.CurrentUserID(c)
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := aim.UnshareAiChatSession(uint(id), userID); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.Success(c, gin.H{"message": "已取消分享"})
}

// AiSharedSession GET /api/ai/shared/:token — 公开只读快照（无需登录）
// 只暴露：会话标题、智能体名、消息角色与正文；不外泄用户、模型、费用信息。
func AiSharedSession(c *gin.Context) {
	token := strings.TrimSpace(c.Param("token"))
	s, err := aim.GetSharedSessionByToken(token)
	if err != nil {
		infra.NotFound(c, err.Error())
		return
	}
	msgs, err := aim.ListAiChatMessages(s.ID, 500)
	if err != nil {
		infra.InternalError(c, "读取失败")
		return
	}
	out := make([]gin.H, 0, len(msgs))
	for i := range msgs {
		m := &msgs[i]
		out = append(out, gin.H{
			"role":       m.Role,
			"content":    m.Content,
			"created_at": m.CreatedAt,
		})
	}
	infra.Success(c, gin.H{
		"title":      s.Title,
		"agent_name": s.AgentName,
		"shared_at":  s.SharedAt,
		"messages":   out,
	})
}
