package handlers

import (
	"regexp"
	"strconv"

	"nvs-server/models"
	"nvs-server/security"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ReloadXssRules 从数据库重新加载 XSS 规则到安全层（启动时 + 每次 CRUD 后调用）
func ReloadXssRules() {
	rules, err := models.ListEnabledXssRules()
	if err != nil {
		return
	}
	dyn := make([]security.DynamicXssRule, 0, len(rules))
	for _, r := range rules {
		dyn = append(dyn, security.DynamicXssRule{
			ID:          r.ID,
			Pattern:     r.Pattern,
			Action:      r.Action,
			Replacement: r.Replacement,
		})
	}
	security.SetDynamicXssRules(dyn)
}

// normalizeXssAction 归一化动作：空或非法值回退为 escape
func normalizeXssAction(action string) string {
	switch action {
	case "replace", "delete":
		return action
	default:
		return "escape"
	}
}

// GET /api/admin/xss-rules — 列出所有 XSS 规则
func ListXssRules(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	rules, err := models.ListXssRules()
	if err != nil {
		utils.InternalError(c, "获取规则列表失败")
		return
	}
	utils.Success(c, rules)
}

// POST /api/admin/xss-rules — 新增 XSS 规则
func CreateXssRule(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		Name        string `json:"name"`
		Pattern     string `json:"pattern"`
		Action      string `json:"action"`
		Replacement string `json:"replacement"`
		Enabled     *bool  `json:"enabled"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误")
		return
	}
	if req.Name == "" || req.Pattern == "" {
		utils.BadRequest(c, "规则名称和正则表达式不能为空")
		return
	}
	if err := security.ValidateUserPattern(req.Pattern); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if hint := security.UserPatternHint(req.Pattern); hint != "" {
		utils.BadRequest(c, hint)
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	rule := &models.XssRule{
		Name:        req.Name,
		Pattern:     req.Pattern,
		Action:      normalizeXssAction(req.Action),
		Replacement: req.Replacement,
		Enabled:     enabled,
		Description: req.Description,
	}
	if err := models.CreateXssRule(rule); err != nil {
		utils.InternalError(c, "创建规则失败")
		return
	}
	ReloadXssRules()
	utils.Success(c, rule)
}

// PUT /api/admin/xss-rules/:id — 更新 XSS 规则
func UpdateXssRule(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	rule, err := models.GetXssRule(uint(id))
	if err != nil {
		utils.NotFound(c, "规则不存在")
		return
	}
	var req struct {
		Name        string `json:"name"`
		Pattern     string `json:"pattern"`
		Action      string `json:"action"`
		Replacement string `json:"replacement"`
		Enabled     *bool  `json:"enabled"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误")
		return
	}
	if req.Name != "" {
		rule.Name = req.Name
	}
	if req.Pattern != "" {
		if err := security.ValidateUserPattern(req.Pattern); err != nil {
			utils.BadRequest(c, err.Error())
			return
		}
		if hint := security.UserPatternHint(req.Pattern); hint != "" {
			utils.BadRequest(c, hint)
			return
		}
		rule.Pattern = req.Pattern
	}
	if req.Action != "" {
		rule.Action = normalizeXssAction(req.Action)
	}
	if req.Replacement != "" {
		rule.Replacement = req.Replacement
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}
	if req.Description != "" {
		rule.Description = req.Description
	}
	if err := models.UpdateXssRule(rule); err != nil {
		utils.InternalError(c, "更新规则失败")
		return
	}
	ReloadXssRules()
	utils.Success(c, rule)
}

// DELETE /api/admin/xss-rules/:id — 删除 XSS 规则
func DeleteXssRule(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := models.DeleteXssRule(uint(id)); err != nil {
		utils.InternalError(c, "删除规则失败")
		return
	}
	ReloadXssRules()
	utils.Success(c, gin.H{"message": "规则已删除"})
}

// POST /api/admin/xss-rules/test — 测试正则规则（不落库，返回匹配结果）
func TestXssRule(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		Pattern     string `json:"pattern"`
		Action      string `json:"action"`
		Replacement string `json:"replacement"`
		Text        string `json:"text"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误")
		return
	}
	if req.Pattern == "" {
		utils.BadRequest(c, "正则表达式不能为空")
		return
	}
	if err := security.ValidateUserPattern(req.Pattern); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	re, err := regexp.Compile(req.Pattern)
	if err != nil {
		utils.BadRequest(c, "正则表达式无效: "+err.Error())
		return
	}
	matches := re.FindAllString(req.Text, -1)
	action := normalizeXssAction(req.Action)
	var result string
	switch action {
	case "replace":
		result = re.ReplaceAllString(req.Text, req.Replacement)
	case "delete":
		result = re.ReplaceAllString(req.Text, "")
	default: // escape：转义匹配到的 HTML 标签，保留完整文本
		result = re.ReplaceAllStringFunc(req.Text, func(match string) string {
			return security.EscapeHTMLTags(match)
		})
	}
	utils.Success(c, gin.H{
		"action":      action,
		"result":      result,
		"match_count": len(matches),
		"matches":     matches,
	})
}
