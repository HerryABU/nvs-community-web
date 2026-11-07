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

	system := oc.BuildOcPersona(ch)

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

// InvokeOcGroupInGroup POST /api/groups/:id/oc/invoke-group
// 「群组调用某个 OC 组」：把用户选定的 OC 组（阵容）整队召唤进群，
// 组内每个角色都以各自人设（含 furry 设定）在群里发一条帖，共同回应同一个场景/提示。
// 与单角色 InvokeOcInGroup 互补，是「群组 ↔ OC 组」双向联动的反向入口。
func InvokeOcGroupInGroup(c *gin.Context) {
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
	m, merr := groups.GetMembership(gid, uid)
	if merr != nil || m == nil || m.Status != groups.GroupMemberActive {
		infra.Forbidden(c, "只有群成员可召唤 OC 组")
		return
	}
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
		OcGroupID uint   `json:"oc_group_id"`
		Prompt    string `json:"prompt"`
	}
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil || req.OcGroupID == 0 {
		infra.BadRequest(c, "请选择要召唤的 OC 组")
		return
	}

	// OC 组读取权限：本人 / 公开过审 / 管理员
	og, gerr := oc.GetOcGroup(req.OcGroupID, uid, isAdmin(c))
	if gerr != nil {
		infra.Forbidden(c, gerr.Error())
		return
	}

	chars, cerr := oc.GetOcGroupCharacters(og.ID)
	if cerr != nil {
		infra.InternalError(c, "读取 OC 组成员失败")
		return
	}
	if len(chars) == 0 {
		infra.BadRequest(c, "该 OC 组还没有成员角色卡")
		return
	}
	const maxMembers = 12 // 防成本失控：单次最多召唤 12 个角色
	if len(chars) > maxMembers {
		chars = chars[:maxMembers]
	}

	userPrompt := strings.TrimSpace(req.Prompt)
	if userPrompt == "" {
		userPrompt = "你们这群角色此刻同处一个场景，请各自以符合人设的方式互动/发言。"
	} else if len([]rune(userPrompt)) > 800 {
		userPrompt = string([]rune(userPrompt)[:800]) + "…"
	}

	botID, berr := ai.EnsureAiBotUser()
	if berr != nil {
		infra.InternalError(c, "AI 账号初始化失败")
		return
	}

	posts := make([]gin.H, 0, len(chars))
	for i := range chars {
		ch := &chars[i]
		system := oc.BuildOcPersona(ch)
		res, gerr := aicall.Complete(aicall.Request{
			PayerID: uid, Scene: "group_oc_group", Action: "group_oc_group",
			RefType: "group", RefID: gid,
			System: system, User: userPrompt,
			MaxTokens: 500, Temperature: 0.85,
		})
		if gerr != nil {
			// 单个角色失败不阻断整队，记录跳过
			continue
		}
		cleaned, _ := security.ApplyBadWords(res.Content, "group")
		body := fmt.Sprintf("【%s】%s\n\n—— AI 生成（OC 组·%s）· %s", ch.Name, cleaned, og.Name, res.ModelName)
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
			continue
		}
		aihub.MarkAiContent(aihub.AiMarkGroupPost, post.ID, "")
		posts = append(posts, gin.H{"post": post, "character": ch.Name})
	}

	if len(posts) == 0 {
		infra.InternalError(c, "OC 组召唤失败（生成或发帖出错）")
		return
	}
	infra.SuccessMessage(c, "OC 组已整队发言", gin.H{"count": len(posts), "posts": posts})
}
