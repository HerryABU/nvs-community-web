package handlers

import (
	"strconv"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== 对话分支（同 DeepSeek / Qwen 官网） ====================

// chatMessageView 把消息列表转成前端视图，并附上兄弟分支信息。
//
// 前端要靠 sibling_index / sibling_count 渲染「‹ 2/3 ›」，
// sibling_ids 则用于点击左右切换（切到兄弟所在分支的最新叶子）。
func chatMessageView(list []models.AiChatMessage) []gin.H {
	out := make([]gin.H, 0, len(list))
	for i := range list {
		m := &list[i]
		idx, cnt := models.SiblingIndex(m)
		var ids []uint
		if cnt > 1 {
			ids = models.SiblingIDs(m)
		}
		out = append(out, gin.H{
			"id": m.ID, "session_id": m.SessionID, "role": m.Role, "content": m.Content,
			"tokens_in": m.TokensIn, "tokens_out": m.TokensOut, "cost": m.Cost,
			"call_log_id": m.CallLogID, "parent_id": m.ParentID, "created_at": m.CreatedAt,
			"safety_blocked": m.SafetyBlocked,
			"sibling_index":  idx, "sibling_count": cnt, "sibling_ids": ids,
		})
	}
	return out
}

// AiHubSwitchBranch POST /api/ai/sessions/:id/switch
// 切到某条消息所在的分支。
//
// 只改「显示位置」，不删任何消息：切到 A 分支再切回 B，两边内容都还在。
func AiHubSwitchBranch(c *gin.Context) {
	userID := currentUserID(c)
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	s, err := models.GetAiChatSession(uint(id), userID)
	if err != nil || s == nil {
		utils.NotFound(c, "会话不存在")
		return
	}
	var req struct {
		MessageID uint `json:"message_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.MessageID == 0 {
		utils.BadRequest(c, "参数错误")
		return
	}
	m, err := models.GetAiChatMessage(req.MessageID, s.ID)
	if err != nil || m == nil {
		utils.NotFound(c, "消息不存在")
		return
	}
	// 兄弟分支可能各自又发展到不同深度，必须跟到该分支的末端，
	// 否则切过去只显示到一半，后面的追问全丢了。
	leaf := models.DeepestLeafFrom(s.ID, m.ID)
	if err := models.SetActiveLeaf(s.ID, leaf); err != nil {
		utils.InternalError(c, "切换失败")
		return
	}
	list, err := models.ListAiChatThread(s.ID, leaf)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	s.ActiveLeafID = leaf
	utils.Success(c, gin.H{"session": s, "messages": chatMessageView(list)})
}
