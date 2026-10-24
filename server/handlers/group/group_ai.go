package grouphandler

import (
	"fmt"
	"nvs-server/groups"
	"nvs-server/security"
	"strconv"
	"strings"
	"sync"
	"time"

	"nvs-server/aicall"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== 群组 @ai 智能体（群内机器人） ====================
// 触发条件：Group.AiEnabled = true 且正文包含 @ai / @智能体名。
// 身份：复用平台机器人账号（EnsureAiBotUser），权限等同普通群成员（只能发帖/回复，不能管理群）。
// 计费：消耗发起人（免费额度优先 → 代币），与评论区机器人、AI 工作台共用同一账本。
// 安全：输入/输出都过违禁词；输出带「AI 生成 + 模型」标识；按群 × 人限流。

var (
	groupAiMu     sync.Mutex
	groupAiRecent = map[string]time.Time{} // key: group:post → 最近触发
)

const groupAiDedupWindow = 60 * time.Second

// maybeTriggerGroupAi 新帖触发（机器人在该帖下回复）
func maybeTriggerGroupAi(g *groups.Group, postID, triggerUser uint, title, content string) {
	triggerGroupAi(g, postID, triggerUser, title+"\n"+content)
}

// maybeTriggerGroupAiReply 回帖触发（机器人跟帖回复）
func maybeTriggerGroupAiReply(g *groups.Group, postID, triggerUser uint, content string, replyID uint) {
	triggerGroupAi(g, postID, triggerUser, content)
}

func triggerGroupAi(g *groups.Group, postID, triggerUser uint, content string) {
	if g == nil || !g.AiEnabled || !models.IsAiEnabled() || !models.IsAiBotEnabled() {
		return
	}
	_, prompt, ok := models.ParseAiBotInvocation(content)
	if !ok || strings.TrimSpace(prompt) == "" {
		return
	}
	cfg := models.GetAiBotConfig()
	if !models.CheckAiBotQuota(triggerUser, cfg.DailyLimit) {
		return // 超限静默（前端在 AI 设置里可见剩余次数）
	}
	key := fmt.Sprintf("%d:%d", g.ID, postID)
	groupAiMu.Lock()
	if last, seen := groupAiRecent[key]; seen && time.Since(last) < groupAiDedupWindow {
		groupAiMu.Unlock()
		return
	}
	groupAiRecent[key] = time.Now()
	groupAiMu.Unlock()

	groupID, agentID, post := g.ID, g.AiAgentID, postID
	question := prompt
	if len([]rune(question)) > 800 {
		question = string([]rune(question)[:800]) + "…"
	}
	go func() {
		defer func() { _ = recover() }()
		generateGroupAiReply(groupID, agentID, post, triggerUser, question, cfg)
	}()
}

// generateGroupAiReply 异步生成并以机器人身份回帖
func generateGroupAiReply(groupID, agentID, postID, triggerUser uint, question string, cfg models.AiBotConfig) {
	// 人设：群绑定的智能体优先，其次平台默认
	systemPrompt := "你是群组里的 AI 助手，回答简洁、贴合讨论，直接输出内容，不要解释。"
	pickedID := agentID
	if pickedID == 0 && cfg.AgentID > 0 {
		pickedID = cfg.AgentID
	}
	if pickedID > 0 {
		if a, aerr := models.GetAiAgent(pickedID); aerr == nil && a.Enabled {
			systemPrompt = a.SystemPrompt + "\n（请以该人设简洁回复，直接输出内容）"
		}
	}

	// 统一调用入口：按路由策略选模型，失败自动降级并在正文里明确提示（不静默换模型）
	res, aerr := aicall.Complete(aicall.Request{
		PayerID: triggerUser, Scene: "group_bot", Action: "group_bot",
		RefType: "group_post", RefID: postID,
		System: systemPrompt, User: question,
		MaxTokens: 512, Temperature: 0.7,
	})
	if aerr != nil {
		return
	}
	models.RecordAiBotUsage(triggerUser)

	botID, berr := models.EnsureAiBotUser()
	if berr != nil {
		return
	}
	// 输出侧过违禁词（与用户投稿同一套规则）
	cleaned, _ := security.ApplyBadWords(res.Content, "group")
	body := cleaned + "\n\n—— AI 生成 · " + res.ModelName
	if res.Cost > 0 {
		body += fmt.Sprintf(" · 消耗 %d %s", res.Cost, models.GetTokenName())
	}
	if note := res.Notice(); note != "" {
		body += "\n—— " + note
	}
	reply := &groups.GroupPostReply{
		GroupID: groupID, PostID: postID, UserID: botID,
		Content: body, Status: groups.GroupPostPublished, CreatedAt: time.Now(),
	}
	if err := models.DB.Create(reply).Error; err != nil {
		return
	}
	models.DB.Model(&groups.GroupPostReply{}).Where("id = ?", reply.ID).Update("status", groups.GroupPostPublished)
	models.DB.Model(&groups.GroupPost{}).Where("id = ?", postID).
		UpdateColumn("reply_cnt", models.DB.Raw("reply_cnt + 1"))
}

// AiStatus GET /api/groups/:id/ai —— 群 AI 可用性（前端展示 @ai 提示）
func AiStatus(c *gin.Context) {
	id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	id := uint(id64)
	if id == 0 {
		utils.BadRequest(c, "参数错误")
		return
	}
	g, err := groups.GetGroup(id)
	if err != nil {
		utils.NotFound(c, "群组不存在")
		return
	}
	agentName := ""
	if g.AiAgentID > 0 {
		if a, aerr := models.GetAiAgent(g.AiAgentID); aerr == nil {
			agentName = a.Name
		}
	}
	utils.Success(c, gin.H{
		"enabled":    g.AiEnabled && models.IsAiEnabled() && models.IsAiBotEnabled(),
		"agent_id":   g.AiAgentID,
		"agent_name": agentName,
	})
}
