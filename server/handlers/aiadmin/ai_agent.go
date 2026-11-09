package aiadmin

import (
	"nvs-server/handlers/admincommon"
	"nvs-server/utils/infra"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	aim "nvs-server/models/ai"
)

// ==================== AI 智能体（§5.4） ====================

// AiAgentList GET /api/ai/agents — 用户端：启用智能体
func AiAgentList(c *gin.Context) {
	list, err := aim.ListAiAgents(true)
	if err != nil {
		infra.InternalError(c, "查询失败")
		return
	}
	infra.Success(c, gin.H{"list": list, "categories": aim.ListAiAgentCategories()})
}

// ---------- 管理端 ----------

// aiAgentVO 管理端视图
func aiAgentVO(a *aim.AiAgent) gin.H {
	return gin.H{
		"id": a.ID, "name": a.Name, "emoji": a.Emoji, "description": a.Description,
		"system_prompt": a.SystemPrompt, "greeting": a.Greeting, "builtin": a.Builtin,
		"persona": a.Persona, "appearance": a.Appearance, "speech": a.Speech,
		"enabled": a.Enabled, "sort_order": a.SortOrder,
		"created_at": a.CreatedAt, "updated_at": a.UpdatedAt,
	}
}

// AdminListAiAgents GET /api/admin/ai/agents
func AdminListAiAgents(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	list, err := aim.ListAiAgents(false)
	if err != nil {
		infra.InternalError(c, "查询失败")
		return
	}
	items := make([]gin.H, 0, len(list))
	for i := range list {
		items = append(items, aiAgentVO(&list[i]))
	}
	infra.Success(c, gin.H{"list": items, "total": len(items)})
}

// AdminCreateAiAgent POST /api/admin/ai/agents
func AdminCreateAiAgent(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	var req struct {
		Name         string `json:"name"`
		Emoji        string `json:"emoji"`
		Description  string `json:"description"`
		SystemPrompt string `json:"system_prompt"`
		Greeting     string `json:"greeting"`
		Persona      string `json:"persona"`
		Appearance   string `json:"appearance"`
		Speech       string `json:"speech"`
		Enabled      *bool  `json:"enabled"`
		SortOrder    *int   `json:"sort_order"`
		Category     string `json:"category"`
		Upstream     string `json:"upstream"`
		BotID        string `json:"bot_id"`
		CozeBase     string `json:"coze_base"`
		CozeToken    string `json:"coze_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		infra.BadRequest(c, "名称不能为空")
		return
	}
	if strings.TrimSpace(req.SystemPrompt) == "" {
		infra.BadRequest(c, "系统提示词不能为空")
		return
	}
	a := &aim.AiAgent{
		Name: strings.TrimSpace(req.Name), Emoji: strings.TrimSpace(req.Emoji),
		Description:  strings.TrimSpace(req.Description),
		SystemPrompt: req.SystemPrompt, Greeting: strings.TrimSpace(req.Greeting),
		Persona: req.Persona, Appearance: req.Appearance, Speech: req.Speech,
		Category: strings.TrimSpace(req.Category),
		Upstream: strings.TrimSpace(req.Upstream), BotID: strings.TrimSpace(req.BotID),
		CozeBase: strings.TrimSpace(req.CozeBase),
		Enabled:  true,
	}
	if req.Enabled != nil {
		a.Enabled = *req.Enabled
	}
	if req.SortOrder != nil {
		a.SortOrder = *req.SortOrder
	}
	if err := aim.CreateAiAgent(a); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	if strings.TrimSpace(req.CozeToken) != "" {
		if err := a.SetCozeToken(req.CozeToken); err != nil {
			infra.InternalError(c, "扣子令牌加密保存失败")
			return
		}
	}
	infra.Success(c, gin.H{"message": "已创建", "id": a.ID})
}

// AdminUpdateAiAgent PUT /api/admin/ai/agents/:id
func AdminUpdateAiAgent(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if _, err := aim.GetAiAgent(uint(id)); err != nil {
		infra.NotFound(c, "智能体不存在")
		return
	}
	var req struct {
		Name         string `json:"name"`
		Emoji        string `json:"emoji"`
		Description  string `json:"description"`
		SystemPrompt string `json:"system_prompt"`
		Greeting     string `json:"greeting"`
		Persona      string `json:"persona"`
		Appearance   string `json:"appearance"`
		Speech       string `json:"speech"`
		Enabled      *bool  `json:"enabled"`
		SortOrder    *int   `json:"sort_order"`
		Category     string `json:"category"`
		Upstream     string `json:"upstream"`
		BotID        string `json:"bot_id"`
		CozeBase     string `json:"coze_base"`
		CozeToken    string `json:"coze_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	fields := map[string]interface{}{}
	if s := strings.TrimSpace(req.Name); s != "" {
		fields["name"] = s
	}
	fields["emoji"] = strings.TrimSpace(req.Emoji)
	fields["description"] = strings.TrimSpace(req.Description)
	if s := strings.TrimSpace(req.SystemPrompt); s != "" {
		fields["system_prompt"] = s
	}
	fields["greeting"] = strings.TrimSpace(req.Greeting)
	fields["persona"] = strings.TrimSpace(req.Persona)
	fields["appearance"] = strings.TrimSpace(req.Appearance)
	fields["speech"] = strings.TrimSpace(req.Speech)
	if req.Enabled != nil {
		fields["enabled"] = *req.Enabled
	}
	if req.SortOrder != nil {
		fields["sort_order"] = *req.SortOrder
	}
	fields["category"] = strings.TrimSpace(req.Category)
	upstream := strings.ToLower(strings.TrimSpace(req.Upstream))
	if upstream != "" && upstream != "llm" && upstream != "coze" {
		infra.BadRequest(c, "upstream 仅支持 llm 或 coze")
		return
	}
	if upstream == "llm" {
		upstream = ""
	}
	fields["upstream"] = upstream
	fields["bot_id"] = strings.TrimSpace(req.BotID)
	fields["coze_base"] = strings.TrimSpace(req.CozeBase)
	if err := aim.UpdateAiAgentFields(uint(id), fields); err != nil {
		infra.InternalError(c, "保存失败")
		return
	}
	if strings.TrimSpace(req.CozeToken) != "" {
		if a, e := aim.GetAiAgent(uint(id)); e == nil {
			if err := a.SetCozeToken(req.CozeToken); err != nil {
				infra.InternalError(c, "扣子令牌加密保存失败")
				return
			}
		}
	}
	infra.Success(c, gin.H{"message": "已保存"})
}

// AdminDeleteAiAgent DELETE /api/admin/ai/agents/:id
func AdminDeleteAiAgent(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := aim.DeleteAiAgent(uint(id)); err != nil {
		infra.InternalError(c, "删除失败")
		return
	}
	infra.Success(c, gin.H{"message": "已删除"})
}
