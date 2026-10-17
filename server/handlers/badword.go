package handlers

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"nvs-server/models"
	"nvs-server/security"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ════════════════ 违禁词规则管理 ════════════════

// ReloadBadWords 从数据库重新加载违禁词规则到安全层（启动时 + 每次 CRUD 后调用）
func ReloadBadWords() {
	words, err := models.ListEnabledBadWords()
	if err != nil {
		return
	}
	dyn := make([]security.DynamicBadWord, 0, len(words))
	for _, w := range words {
		dyn = append(dyn, security.DynamicBadWord{
			ID:          w.ID,
			Name:        w.Name,
			MatchType:   w.MatchType,
			Pattern:     w.Pattern,
			Action:      w.Action,
			Replacement: w.Replacement,
			Scope:       w.Scope,
		})
	}
	security.SetDynamicBadWords(dyn)
}

// normalizeBadWordMatchType 归一化匹配方式：空或非法回退为 literal（字面关键词）
func normalizeBadWordMatchType(mt string) string {
	if mt == "regex" {
		return "regex"
	}
	return "literal"
}

// normalizeBadWordAction 归一化动作：空或非法回退为 mark（标记待审）
func normalizeBadWordAction(action string) string {
	switch action {
	case "mark", "delete", "replace":
		return action
	default:
		return "mark"
	}
}

// normalizeBadWordScope 归一化适用范围：all / 或 chapter,comment,forum,blog 组合
func normalizeBadWordScope(scope string) string {
	if scope == "" || scope == "all" {
		return "all"
	}
	seen := map[string]bool{}
	var parts []string
	for _, s := range strings.Split(scope, ",") {
		s = strings.TrimSpace(s)
		switch s {
		case "chapter", "comment", "forum", "blog":
			if !seen[s] {
				seen[s] = true
				parts = append(parts, s)
			}
		}
	}
	if len(parts) == 0 {
		return "all"
	}
	return strings.Join(parts, ",")
}

// validateBadWordPattern 校验正则合法性（仅 regex 模式）：长度上限 + RE2 编译。
// 第二个返回值为误用提示（如嵌套量词），可保存但需提醒管理员。
func validateBadWordPattern(matchType, pattern string) (error, string) {
	if matchType == "regex" {
		if err := security.ValidateUserPattern(pattern); err != nil {
			return err, ""
		}
		return nil, security.UserPatternHint(pattern)
	}
	return nil, ""
}

// GET /api/admin/bad-words — 列出所有违禁词规则
func ListBadWords(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	words, err := models.ListBadWords()
	if err != nil {
		utils.InternalError(c, "获取违禁词列表失败")
		return
	}
	utils.Success(c, words)
}

// POST /api/admin/bad-words — 新增违禁词规则
func CreateBadWord(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		Name        string `json:"name"`
		MatchType   string `json:"match_type"`
		Pattern     string `json:"pattern"`
		Action      string `json:"action"`
		Replacement string `json:"replacement"`
		Scope       string `json:"scope"`
		Enabled     *bool  `json:"enabled"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误")
		return
	}
	if req.Name == "" || req.Pattern == "" {
		utils.BadRequest(c, "规则名称和违禁词不能为空")
		return
	}
	matchType := normalizeBadWordMatchType(req.MatchType)
	if err, hint := validateBadWordPattern(matchType, req.Pattern); err != nil {
		utils.BadRequest(c, err.Error())
		return
	} else if hint != "" {
		utils.BadRequest(c, hint)
		return
	}
	action := normalizeBadWordAction(req.Action)
	if action == "replace" && req.Replacement == "" {
		utils.BadRequest(c, "替换动作必须填写替换文本")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	word := &models.BadWord{
		Name:        req.Name,
		MatchType:   matchType,
		Pattern:     req.Pattern,
		Action:      action,
		Replacement: req.Replacement,
		Scope:       normalizeBadWordScope(req.Scope),
		Enabled:     enabled,
		Description: req.Description,
	}
	if err := models.CreateBadWord(word); err != nil {
		utils.InternalError(c, "创建规则失败")
		return
	}
	ReloadBadWords()
	utils.Success(c, word)
}

// PUT /api/admin/bad-words/:id — 更新违禁词规则
func UpdateBadWord(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	word, err := models.GetBadWord(uint(id))
	if err != nil {
		utils.NotFound(c, "规则不存在")
		return
	}
	var req struct {
		Name        string `json:"name"`
		MatchType   string `json:"match_type"`
		Pattern     string `json:"pattern"`
		Action      string `json:"action"`
		Replacement string `json:"replacement"`
		Scope       string `json:"scope"`
		Enabled     *bool  `json:"enabled"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误")
		return
	}
	if req.Name != "" {
		word.Name = req.Name
	}
	if req.MatchType != "" {
		word.MatchType = normalizeBadWordMatchType(req.MatchType)
	}
	if req.Pattern != "" {
		if err, hint := validateBadWordPattern(word.MatchType, req.Pattern); err != nil {
			utils.BadRequest(c, err.Error())
			return
		} else if hint != "" {
			utils.BadRequest(c, hint)
			return
		}
		word.Pattern = req.Pattern
	}
	if req.Action != "" {
		word.Action = normalizeBadWordAction(req.Action)
	}
	if req.Replacement != "" {
		word.Replacement = req.Replacement
	}
	if req.Scope != "" {
		word.Scope = normalizeBadWordScope(req.Scope)
	}
	if req.Enabled != nil {
		word.Enabled = *req.Enabled
	}
	if req.Description != "" {
		word.Description = req.Description
	}
	if err := models.UpdateBadWord(word); err != nil {
		utils.InternalError(c, "更新规则失败")
		return
	}
	ReloadBadWords()
	utils.Success(c, word)
}

// DELETE /api/admin/bad-words/:id — 删除违禁词规则
func DeleteBadWord(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := models.DeleteBadWord(uint(id)); err != nil {
		utils.InternalError(c, "删除规则失败")
		return
	}
	ReloadBadWords()
	utils.Success(c, gin.H{"message": "规则已删除"})
}

// POST /api/admin/bad-words/test — 测试违禁词规则（不落库，返回匹配与处理结果）
func TestBadWord(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		MatchType   string `json:"match_type"`
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
		utils.BadRequest(c, "违禁词不能为空")
		return
	}
	matchType := normalizeBadWordMatchType(req.MatchType)
	pattern := req.Pattern
	if len(pattern) > security.MaxUserPatternLen {
		utils.BadRequest(c, "正则长度超过 256 字节上限，请拆分多条规则")
		return
	}
	if matchType == "literal" {
		pattern = regexp.QuoteMeta(pattern)
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		utils.BadRequest(c, "正则表达式无效: "+err.Error())
		return
	}
	matches := re.FindAllString(req.Text, -1)
	action := normalizeBadWordAction(req.Action)
	var result string
	switch action {
	case "replace":
		result = re.ReplaceAllString(req.Text, req.Replacement)
	case "delete":
		result = re.ReplaceAllString(req.Text, "")
	default: // mark：不改文本
		result = req.Text
	}
	utils.Success(c, gin.H{
		"action":      action,
		"result":      result,
		"match_count": len(matches),
		"matches":     matches,
	})
}

// ════════════════ 内容写入点统一接入 ════════════════

// processBadWords 内容保存前调用：在 XSS 净化基础上再应用违禁词 delete/replace 净化。
// 返回净化后的文本与全部命中（含 mark，供保存成功后补写待审标记）。
func processBadWords(content, scope string) (string, []security.BadWordHit) {
	return security.ApplyBadWords(content, scope)
}

// flagBadWords 内容保存成功后调用：将 mark 动作的命中写入 BadWordFlag 待审记录（去重）。
func flagBadWords(hits []security.BadWordHit, contentType string, contentID, userID uint, raw, title string) {
	var matched []string
	for _, h := range hits {
		if h.Action == "mark" {
			matched = append(matched, h.Matched)
		}
	}
	if len(matched) == 0 {
		return
	}
	if exists, _ := models.HasPendingFlag(contentType, contentID); exists {
		return // 已有待审标记，不重复标记
	}
	b, _ := json.Marshal(matched)
	flag := &models.BadWordFlag{
		ContentType:  contentType,
		ContentID:    contentID,
		UserID:       userID,
		ContentTitle: title,
		Snippet:      snippetOf(raw),
		MatchedWords: string(b),
		Status:       "pending",
	}
	_ = models.CreateBadWordFlag(flag)
}

// snippetOf 截取内容快照（前 200 字）供管理员快速判断
func snippetOf(s string) string {
	runes := []rune(s)
	if len(runes) > 200 {
		return string(runes[:200])
	}
	return s
}

// ════════════════ 违禁词标记（待审）处理 ════════════════

// GET /api/admin/bad-word-flags — 标记待审列表（status 可选过滤，分页）
func ListBadWordFlags(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	status := c.Query("status")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	flags, err := models.ListBadWordFlags(status, pageSize, (page-1)*pageSize)
	if err != nil {
		utils.InternalError(c, "获取标记列表失败")
		return
	}
	total, err := models.CountBadWordFlags(status)
	if err != nil {
		total = 0
	}
	// 补充命中词列表（JSON 数组解析）
	type flagView struct {
		models.BadWordFlag
		Matched []string `json:"matched_list"`
	}
	views := make([]flagView, 0, len(flags))
	for _, f := range flags {
		v := flagView{BadWordFlag: f}
		_ = json.Unmarshal([]byte(f.MatchedWords), &v.Matched)
		views = append(views, v)
	}
	pendingTotal, _ := models.CountBadWordFlags("pending")
	utils.Success(c, gin.H{
		"items":         views,
		"total":         total,
		"page":          page,
		"page_size":     pageSize,
		"pending_count": pendingTotal,
	})
}

// PUT /api/admin/bad-word-flags/:id — 处理标记
// body: { status: "confirmed" | "dismissed", note?, delete_content? }
//   - confirmed：确认违规（可选 delete_content=true 同时软删除内容并留证）
//   - dismissed：误报放行
func HandleBadWordFlag(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	flag, err := models.GetBadWordFlag(uint(id))
	if err != nil {
		utils.NotFound(c, "标记记录不存在")
		return
	}
	var req struct {
		Status        string `json:"status"`
		Note          string `json:"note"`
		DeleteContent bool   `json:"delete_content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误")
		return
	}
	switch req.Status {
	case "confirmed":
		// 可选：同时软删除对应内容并留证
		if req.DeleteContent {
			if err := softDeleteContent(c, flag.ContentType, flag.ContentID, req.Note, "badword", "delete"); err != nil {
				if err == errContentNotFound {
					utils.NotFound(c, "对应内容不存在或已被删除")
					return
				}
				utils.InternalError(c, "删除内容失败: "+err.Error())
				return
			}
		}
		flag.Status = "confirmed"
	case "dismissed":
		flag.Status = "dismissed"
	default:
		utils.BadRequest(c, "status 仅支持 confirmed / dismissed")
		return
	}
	flag.Note = req.Note
	if err := models.UpdateBadWordFlag(flag); err != nil {
		utils.InternalError(c, "更新标记失败")
		return
	}
	utils.Success(c, flag)
}
