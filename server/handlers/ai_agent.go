package handlers

import (
	"strconv"
	"strings"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== AI 智能体（§5.4） ====================

// AiAgentList GET /api/ai/agents — 用户端：启用智能体
func AiAgentList(c *gin.Context) {
	list, err := models.ListAiAgents(true)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{"list": list, "categories": models.ListAiAgentCategories()})
}

// ---------- 管理端 ----------

// aiAgentVO 管理端视图
func aiAgentVO(a *models.AiAgent) gin.H {
	return gin.H{
		"id": a.ID, "name": a.Name, "emoji": a.Emoji, "description": a.Description,
		"system_prompt": a.SystemPrompt, "greeting": a.Greeting, "builtin": a.Builtin,
		"enabled": a.Enabled, "sort_order": a.SortOrder,
		"created_at": a.CreatedAt, "updated_at": a.UpdatedAt,
	}
}

// AdminListAiAgents GET /api/admin/ai/agents
func AdminListAiAgents(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	list, err := models.ListAiAgents(false)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	items := make([]gin.H, 0, len(list))
	for i := range list {
		items = append(items, aiAgentVO(&list[i]))
	}
	utils.Success(c, gin.H{"list": items, "total": len(items)})
}

// AdminCreateAiAgent POST /api/admin/ai/agents
func AdminCreateAiAgent(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		Name         string `json:"name"`
		Emoji        string `json:"emoji"`
		Description  string `json:"description"`
		SystemPrompt string `json:"system_prompt"`
		Greeting     string `json:"greeting"`
		Enabled      *bool  `json:"enabled"`
		SortOrder    *int   `json:"sort_order"`
		Category     string `json:"category"`
		Upstream     string `json:"upstream"`
		BotID        string `json:"bot_id"`
		CozeBase     string `json:"coze_base"`
		CozeToken    string `json:"coze_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		utils.BadRequest(c, "名称不能为空")
		return
	}
	if strings.TrimSpace(req.SystemPrompt) == "" {
		utils.BadRequest(c, "系统提示词不能为空")
		return
	}
	a := &models.AiAgent{
		Name: strings.TrimSpace(req.Name), Emoji: strings.TrimSpace(req.Emoji),
		Description: strings.TrimSpace(req.Description),
		SystemPrompt: req.SystemPrompt, Greeting: strings.TrimSpace(req.Greeting),
		Category: strings.TrimSpace(req.Category),
		Upstream: strings.TrimSpace(req.Upstream), BotID: strings.TrimSpace(req.BotID),
		CozeBase: strings.TrimSpace(req.CozeBase),
		Enabled: true,
	}
	if req.Enabled != nil {
		a.Enabled = *req.Enabled
	}
	if req.SortOrder != nil {
		a.SortOrder = *req.SortOrder
	}
	if err := models.CreateAiAgent(a); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if strings.TrimSpace(req.CozeToken) != "" {
		if err := a.SetCozeToken(req.CozeToken); err != nil {
			utils.InternalError(c, "扣子令牌加密保存失败")
			return
		}
	}
	utils.Success(c, gin.H{"message": "已创建", "id": a.ID})
}

// AdminUpdateAiAgent PUT /api/admin/ai/agents/:id
func AdminUpdateAiAgent(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if _, err := models.GetAiAgent(uint(id)); err != nil {
		utils.NotFound(c, "智能体不存在")
		return
	}
	var req struct {
		Name         string `json:"name"`
		Emoji        string `json:"emoji"`
		Description  string `json:"description"`
		SystemPrompt string `json:"system_prompt"`
		Greeting     string `json:"greeting"`
		Enabled      *bool  `json:"enabled"`
		SortOrder    *int   `json:"sort_order"`
		Category     string `json:"category"`
		Upstream     string `json:"upstream"`
		BotID        string `json:"bot_id"`
		CozeBase     string `json:"coze_base"`
		CozeToken    string `json:"coze_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
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
	if req.Enabled != nil {
		fields["enabled"] = *req.Enabled
	}
	if req.SortOrder != nil {
		fields["sort_order"] = *req.SortOrder
	}
	fields["category"] = strings.TrimSpace(req.Category)
	upstream := strings.ToLower(strings.TrimSpace(req.Upstream))
	if upstream != "" && upstream != "llm" && upstream != "coze" {
		utils.BadRequest(c, "upstream 仅支持 llm 或 coze")
		return
	}
	if upstream == "llm" {
		upstream = ""
	}
	fields["upstream"] = upstream
	fields["bot_id"] = strings.TrimSpace(req.BotID)
	fields["coze_base"] = strings.TrimSpace(req.CozeBase)
	if err := models.UpdateAiAgentFields(uint(id), fields); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	if strings.TrimSpace(req.CozeToken) != "" {
		if a, e := models.GetAiAgent(uint(id)); e == nil {
			if err := a.SetCozeToken(req.CozeToken); err != nil {
				utils.InternalError(c, "扣子令牌加密保存失败")
				return
			}
		}
	}
	utils.Success(c, gin.H{"message": "已保存"})
}

// AdminDeleteAiAgent DELETE /api/admin/ai/agents/:id
func AdminDeleteAiAgent(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := models.DeleteAiAgent(uint(id)); err != nil {
		utils.InternalError(c, "删除失败")
		return
	}
	utils.Success(c, gin.H{"message": "已删除"})
}
