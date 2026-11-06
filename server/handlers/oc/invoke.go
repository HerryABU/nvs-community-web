package ochandler

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"nvs-server/aicall"
	"nvs-server/groups"
	"nvs-server/middleware"
	"nvs-server/models"
	ai "nvs-server/models/ai"
	aihub "nvs-server/models/aihub"
	wallet "nvs-server/models/wallet"
	"nvs-server/oc"
	"nvs-server/security"
	"nvs-server/utils/infra"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

// InvokeOcInGroup POST /api/groups/:id/oc/invoke
// 群成员召唤一个已绑定本群的 OC 角色，让它以角色人设在群里发一条帖。
// 这是「群组调用 OC」的一等公民动作：与对话流（FlowPanel）互补，产物直接进群内容流。
func InvokeOcInGroup(c *gin.Context) {
	uid := middleware.UserID(c)
	if uid == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	gid64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if gid64 == 0 {
		infra.BadRequest(c, "参数错误")
		return
	}
	gid := uint(gid64)
	g, err := groups.GetGroup(gid)
	if err != nil || g == nil {
		infra.NotFound(c, "群组不存在")
		return
	}
	// 仅群成员可召唤 OC
	m, merr := groups.GetMembership(gid, uid)
	if merr != nil || m == nil || m.Status != groups.GroupMemberActive {
		infra.Forbidden(c, "只有群成员可召唤 OC 角色")
		return
	}
	// OC 模式需开启
	setting := oc.GetGroupSetting(gid)
	if !setting.Enabled {
		infra.BadRequest(c, "该群未启用 OC 模式（去群组 OC 模式开启）")
		return
	}
	if !ai.IsAiEnabled() || !ai.IsAiBotEnabled() {
		infra.BadRequest(c, "平台 AI 未开启")
		return
	}

	var req struct {
		AgentID uint   `json:"agent_id"`
		Prompt  string `json:"prompt"`
	}
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}

	// 解析要召唤的 OC agent
	agents, aerr := oc.ListAgents(gid)
	if aerr != nil || len(agents) == 0 {
		infra.BadRequest(c, "该群尚未绑定 OC 角色（去群组 OC 模式 → 角色卡加入本群 AI）")
		return
	}
	var agent *oc.Agent
	if req.AgentID > 0 {
		for i := range agents {
			if agents[i].ID == req.AgentID && agents[i].Enabled {
				agent = &agents[i]
				break
			}
		}
	}
	if agent == nil {
		for i := range agents {
			if agents[i].Enabled {
				agent = &agents[i]
				break
			}
		}
	}
	if agent == nil {
		infra.BadRequest(c, "该群没有可用的 OC 角色（AI 角色需启用）")
		return
	}

	ch, cerr := oc.GetCharacterByID(agent.CharacterID)
	if cerr != nil {
		infra.BadRequest(c, "OC 角色卡不可用")
		return
	}

	// 组装角色人设（含 furry 物种特征）
	sysParts := []string{}
	if ch.SystemPrompt != "" {
		sysParts = append(sysParts, ch.SystemPrompt)
	}
	persona := strings.TrimSpace(ch.Persona)
	if persona != "" {
		sysParts = append(sysParts, "人设："+persona)
	}
	if ch.OcKind == "furry" {
		species := strings.TrimSpace(ch.Species)
		traits := strings.TrimSpace(ch.SpeciesTraits)
		if species != "" || traits != "" {
			sysParts = append(sysParts, fmt.Sprintf("Furry 设定：物种 %s；特征 %s", species, traits))
		}
	}
	if ch.Appearance != "" {
		sysParts = append(sysParts, "外貌："+ch.Appearance)
	}
	if ch.Speech != "" {
		sysParts = append(sysParts, "说话风格："+ch.Speech)
	}
	if ch.Scenario != "" {
		sysParts = append(sysParts, "场景："+ch.Scenario)
	}
	sysParts = append(sysParts, fmt.Sprintf("请以《%s》的口吻发言，直接输出内容，不要破第四面墙。", ch.Name))
	system := strings.Join(sysParts, "\n")

	userPrompt := strings.TrimSpace(req.Prompt)
	if userPrompt == "" {
		userPrompt = "在群里和大家打个招呼，说点符合你人设的话。"
	} else if len([]rune(userPrompt)) > 800 {
		userPrompt = string([]rune(userPrompt)[:800]) + "…"
	}

	res, gerr := aicall.Complete(aicall.Request{
		PayerID: uid, Scene: "group_oc", Action: "group_oc",
		RefType: "group", RefID: gid,
		System: system, User: userPrompt,
		MaxTokens: 600, Temperature: 0.8,
	})
	if gerr != nil {
		infra.InternalError(c, "OC 生成失败："+gerr.Error())
		return
	}

	botID, berr := ai.EnsureAiBotUser()
	if berr != nil {
		infra.InternalError(c, "AI 账号初始化失败")
		return
	}

	cleaned, _ := security.ApplyBadWords(res.Content, "group")
	body := fmt.Sprintf("【%s】%s\n\n—— AI 生成（OC 角色）· %s", ch.Name, cleaned, res.ModelName)
	if res.Cost > 0 {
		body += fmt.Sprintf(" · 消耗 %d %s", res.Cost, wallet.GetTokenName())
	}

	post := &groups.GroupPost{
		GroupID:    g.ID,
		UserID:     botID,
		Title:      ch.Name + " 在群里发言",
		Content:    body,
		Status:     groups.GroupPostPublished,
		AiAssisted: true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if cerr := models.DB.Create(post).Error; cerr != nil {
		infra.InternalError(c, "发帖失败")
		return
	}
	// AI 内容标记（与章节/博客同一套，便于审核识别）
	if mmerr := aihub.MarkAiContent(aihub.AiMarkGroupPost, post.ID, ""); mmerr != nil {
		// 标记失败不阻断主流程
		_ = mmerr
	}
	infra.SuccessMessage(c, "OC 已发言", gin.H{"post": post, "character": ch.Name})
}
