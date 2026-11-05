package badword

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"nvs-server/handlers/admincommon"
	"strings"
	"time"

	"nvs-server/aiclient"
	"nvs-server/sentiment/store"
	"nvs-server/sentiment/updater"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	ai "nvs-server/models/ai"
	aihub "nvs-server/models/aihub"
	"nvs-server/models/security"
)

// ════════════════ 检索模式 ════════════════
//
// 三种联网方式在同一个接口里共存，按可用性自动降级：
//
//	endpoint —— 上游额外提供 /search 检索接口（DeepSeek/Kimi 的 /v1/tools/search、
//	            Perplexity 的 /search 等）。平台先检索拿到公开资料摘要，再让对话模型
//	            据此归纳成规则（RAG）。结果可控、可截断，且不依赖模型自己会不会联网。
//	native   —— 对话模型本身就带联网检索（OpenAI 的 *-search-preview 等）。
//	            平台在请求里带上网检索参数，模型自己联网。
//	prompt   —— 不联网：只用模型自身知识（离线降级，保证任何时候都有可用路径）。
const (
	badwordSearchModeAuto     = "auto"
	badwordSearchModeEndpoint = "endpoint"
	badwordSearchModeNative   = "native"
	badwordSearchModePrompt   = "prompt"
)

// SearchAugmentBadWords POST /api/admin/bad-words/search-augment
//
// 用 AI（带联网检索能力的上游模型）检索当下「高风险、新出现、或常见规避写法」的违禁词与变体，
// 解析为结构化规则后批量导入。安全设计：
//   - 仅取模型返回的「字面关键词」（不采纳 AI 给的正则），避免 AI 注入危险正则；
//   - 默认动作一律 mark（仅标记待审，不自动删改内容），由管理员在待审列表复核确认；
//   - 按 name+match_type+pattern 去重，已存在的规则自动跳过，不会重复创建。
func SearchAugmentBadWords(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	var req struct {
		ModelID    uint   `json:"model_id"`    // 可选：指定对话模型；不填则用平台默认启用的对话模型
		Category   string `json:"category"`    // 可选：统一归类
		Scope      string `json:"scope"`       // 可选：all 或 chapter,comment,forum,blog；不填=all
		Focus      string `json:"focus"`       // 可选：侧重方向（政治/色情/毒品/诈骗/赌博/广告/辱骂/全部）
		SearchMode string `json:"search_mode"` // 可选：auto/endpoint/native/prompt；不填=auto
	}
	_ = c.ShouldBindJSON(&req)

	// ── 1) 选对话模型（负责把检索结果归纳成结构化规则）──
	chatModel, merr := pickBadwordChatModel(req.ModelID)
	if merr != nil {
		utils.BadRequest(c, merr.Error())
		return
	}

	// ── 2) 解析检索源与检索模式 ──
	searchModel, searchPath := resolveBadwordSearchModel(chatModel)
	mode := resolveBadwordSearchMode(req.SearchMode, chatModel, searchModel != nil)

	// 复用平台密钥租赁（不向管理员暴露密钥，也不做用户级计费）
	apiKey, kerr := acquireBadwordModelKey(chatModel)
	if kerr != nil {
		utils.InternalError(c, "获取模型密钥失败，无法发起检索："+kerr.Error())
		return
	}
	// 上游超时 180 秒：这是「检索 + 归纳出几十条规则」的一次重活，
	// 实测单次就跑了 100 秒以上（原 120s 已贴着上限，稍微慢一点就整段失败）。
	// 前端该请求的超时是 200s，与此对齐。
	client := aiclient.NewClient(chatModel.BaseURL, apiKey, 180*time.Second)

	ctx := c.Request.Context()
	systemPrompt := buildSearchAugmentSystemPrompt(req.Focus, mode == badwordSearchModeEndpoint)
	userPrompt := buildSearchAugmentUserPrompt()
	searchNote := ""

	// ── 3) Path 3：/search 检索 → 摘要注入提示词（RAG）──
	if mode == badwordSearchModeEndpoint {
		searchKey, skerr := acquireBadwordModelKey(searchModel)
		var hits []updater.Snippet
		var serr error
		if skerr != nil {
			serr = skerr
		} else {
			hits, serr = badwordSearchSnippets(ctx, searchModel, searchKey, searchPath, badwordSearchQuery(req.Focus))
		}
		if serr != nil {
			// 检索失败不阻断整个功能：退回原生/提示词模式，管理员照样能拿到结果
			log.Printf("[badword-search] /search 检索失败，自动降级：err=%v", serr)
			mode = resolveBadwordSearchMode(badwordSearchModeAuto, chatModel, false)
			systemPrompt = buildSearchAugmentSystemPrompt(req.Focus, false)
		} else {
			userPrompt = buildBadwordRAGPrompt(req.Focus, hits)
			searchNote = fmt.Sprintf("已通过 %s%s 联网检索 %d 条公开资料", searchModel.Name, searchPath, len(hits))
		}
	}

	chatReq := aiclient.ChatRequest{
		Ctx:         ctx,
		Model:       chatModel.ModelID,
		Messages:    []aiclient.Message{{Role: "system", Content: systemPrompt}, {Role: "user", Content: userPrompt}},
		MaxTokens:   2048,
		Temperature: 0.2,
	}
	// ── 4) Path 2：模型原生联网（请求里带联网检索参数）──
	if mode == badwordSearchModeNative {
		attachNativeSearch(&chatReq, chatModel)
	}
	content, _, _, cerr := client.Complete(chatReq)
	if cerr != nil && chatReq.WebSearchOptions != nil && isUnsupportedSearchParamError(cerr) {
		// 上游不认这个联网参数（部分网关直接 400）→ 去掉参数重试一次，降级为提示词模式
		log.Printf("[badword-search] 上游不接受联网检索参数，降级为提示词模式：%v", cerr)
		chatReq.Tools = nil
		chatReq.WebSearchOptions = nil
		mode = badwordSearchModePrompt
		content, _, _, cerr = client.Complete(chatReq)
	}
	if cerr != nil {
		utils.InternalError(c, "AI 检索失败："+cerr.Error())
		return
	}

	items, perr := parseBadWordJSON(content)
	if perr != nil {
		// 模型偶尔不按 JSON 数组返回（加前言、换成对象、只吐了半截）。
		// 把原始返回记进服务端日志，便于定位；响应里只给可操作提示，不回显模型原文。
		log.Printf("[badword-search] 模型返回无法解析为清单：err=%v raw=%s",
			perr, truncateForLog(content, 800))
		utils.BadRequest(c, "AI 返回无法解析为违禁词清单："+perr.Error()+
			"（模型原始返回已记入服务端日志，可换个侧重方向后重试）")
		return
	}
	if len(items) == 0 {
		utils.Success(c, gin.H{
			"total": 0, "created": 0, "skipped": 0,
			"items":       []any{},
			"search_mode": mode,
			"search_note": searchNote,
			"message":     "未解析到可导入的违禁词",
		})
		return
	}

	// 安全归一化：只收字面关键词（丢弃 AI 可能给的正则），动作强制 mark，分类/适用范围归一
	cat := normalizeBadWordCategory(req.Category)
	scope := normalizeBadWordScope(req.Scope)
	for i := range items {
		items[i].MatchType = "literal" // 不信任 AI 给的正则，统一字面匹配
		items[i].Action = "mark"       // 仅标记待审，不自动删改
		if items[i].Category == "" {
			items[i].Category = cat
		} else {
			items[i].Category = normalizeBadWordCategory(items[i].Category)
		}
		if items[i].Scope == "" {
			items[i].Scope = scope
		} else {
			items[i].Scope = normalizeBadWordScope(items[i].Scope)
		}
		items[i].Tags = normalizeBadWordTags(items[i].Tags)
		if items[i].Enabled == nil {
			t := true
			items[i].Enabled = &t
		}
	}

	created, skipped, ierr := security.ImportBadWords(items)
	if ierr != nil {
		utils.InternalError(c, "导入失败："+ierr.Error())
		return
	}
	ReloadBadWords()
	utils.Success(c, gin.H{
		"total":       len(items),
		"created":     created,
		"skipped":     skipped,
		"items":       items,
		"search_mode": mode,
		"search_note": searchNote,
		"message":     fmt.Sprintf("联网检索补充完成：新建 %d 条，跳过（已存在/无效）%d 条，均进入待审由管理员复核", created, skipped),
	})
}

// buildSearchAugmentSystemPrompt 构造内容安全分析师的系统提示词。
//
// searched=true 表示用户消息里已经附带联网检索到的公开资料摘要，
// 这时提示词要改成「只依据这些摘要」，避免模型再去脑补。
func buildSearchAugmentSystemPrompt(focus string, searched bool) string {
	var b strings.Builder
	b.WriteString("你是一名中文互联网内容安全分析师，熟悉各平台最新的违禁词、黑话与规避变体。")
	if searched {
		b.WriteString("用户消息里会附带联网检索到的公开资料摘要。")
		b.WriteString("请**只依据这些检索摘要**提炼违禁词，证据不足的不要编造，也不要输出资料原文。\n")
	} else {
		b.WriteString("请基于联网检索（若你的 API 支持联网搜索，请务必检索最新信息；否则基于你掌握的最新知识），")
		b.WriteString("整理一份「当前高风险、新出现、或常见规避写法」的违禁词清单。\n")
	}
	b.WriteString("严格要求：\n")
	b.WriteString("1. 每条只给「字面关键词/短语」（pattern），不要给正则表达式，不要给需要转义的元字符；\n")
	b.WriteString("2. 动作固定为 mark（命中后仅标记待审，不自动删改内容）；\n")
	b.WriteString("3. 字段：name（简短中文名）、pattern（要匹配的字面词/短语）、category（政治/色情/毒品/诈骗/赌博/广告/辱骂/其他）、tags（逗号分隔的标签）、action（固定 \"mark\"）；\n")
	b.WriteString("4. 返回严格 JSON 数组，元素形如 {\"name\":\"...\",\"pattern\":\"...\",\"category\":\"...\",\"tags\":\"...\",\"action\":\"mark\"}；\n")
	b.WriteString("5. 不要任何额外说明、不要 Markdown 代码围栏、不要数组外的任何文字。")
	if f := strings.TrimSpace(focus); f != "" && f != "全部" {
		b.WriteString(fmt.Sprintf("\n本次侧重方向：%s。", f))
	}
	return b.String()
}

// buildSearchAugmentUserPrompt 常规（联网/提示词）模式的用户提示词。
func buildSearchAugmentUserPrompt() string {
	return "请联网检索（若你的 API 支持联网搜索，请务必检索最新信息；否则基于你掌握的最新知识），" +
		"整理一份「当前高风险、新出现、或常见规避写法」的违禁词清单，" +
		"覆盖网络黑话、新型诈骗/赌博术语、新型毒品代称、违规广告与辱骂变体等。" +
		"只返回 JSON 数组，不要任何解释、前言或 Markdown 代码围栏。"
}

// buildBadwordRAGPrompt 把联网检索到的公开资料摘要拼成用户提示词。
//
// 合规边界与舆情词表更新模块一致：**检索结果原文不入库**，
// 落库的只是模型归纳后的「字面词 + 分类 + 标签」。
func buildBadwordRAGPrompt(focus string, hits []updater.Snippet) string {
	var b strings.Builder
	b.WriteString("以下是联网检索到的公开资料摘要（只作判断依据：不要照抄原文，也不要输出原文）：\n")
	for i, h := range hits {
		if i >= 8 {
			break
		}
		title := strings.TrimSpace(h.Title)
		if title == "" {
			title = "（无标题）"
		}
		sum := strings.TrimSpace(h.Summary)
		if len([]rune(sum)) > 240 {
			sum = string([]rune(sum)[:240]) + "…"
		}
		fmt.Fprintf(&b, "%d. %s —— %s\n", i+1, title, sum)
	}
	b.WriteString("\n请基于以上检索摘要，整理一份「当前高风险、新出现、或常见规避写法」的违禁词清单，")
	b.WriteString("覆盖网络黑话、新型诈骗/赌博术语、新型毒品代称、违规广告与辱骂变体等。")
	if f := strings.TrimSpace(focus); f != "" && f != "全部" {
		b.WriteString(fmt.Sprintf("本次侧重方向：%s。", f))
	}
	b.WriteString("证据不足的词不要写。只返回 JSON 数组，不要任何解释、前言或 Markdown 代码围栏。")
	return b.String()
}

// badwordSearchQuery 构造联网检索用的查询词。
func badwordSearchQuery(focus string) string {
	f := strings.TrimSpace(focus)
	if f == "" || f == "全部" {
		return "最新 网络 违禁词 黑话 规避写法 变体 赌博 诈骗 广告 引流"
	}
	return "最新 网络 " + f + " 违禁词 黑话 规避写法 变体"
}

// pickBadwordChatModel 选用于「归纳成结构化规则」的对话模型。
// 显式指定优先；否则用平台默认启用的对话模型。
func pickBadwordChatModel(modelID uint) (*ai.AiModel, error) {
	if modelID > 0 {
		m, err := ai.GetAiModel(modelID)
		if err != nil || m == nil || !m.Enabled {
			return nil, fmt.Errorf("指定的模型不可用")
		}
		return m, nil
	}
	m, err := ai.GetEnabledAiModelOfKind(0, ai.AiKindChat)
	if err != nil {
		return nil, fmt.Errorf("未配置可用的对话模型，无法发起联网检索")
	}
	if m == nil {
		return nil, fmt.Errorf("未配置可用的对话模型，无法发起联网检索")
	}
	return m, nil
}

// resolveBadwordSearchModel 解析用于 /search 检索的模型与路径。
//
// 两条来源（任一可用即可）：
//  1. 对话模型自己被标注了「提供 /search 接口」能力位 → 直接用它；
//  2. 平台共用的检索源（舆情词表更新里配置的搜索模型 + /search 路径）——
//     全站只配一次，违禁词补充与舆情词表共用同一个联网通道。
//
// 都没有 → 返回 nil，表示这条路径不可用（会退到原生联网或提示词模式）。
func resolveBadwordSearchModel(chatModel *ai.AiModel) (*ai.AiModel, string) {
	if chatModel != nil && chatModel.SearchEndpoint {
		return chatModel, "/search"
	}
	cfg, err := store.GetConfig()
	if err != nil || cfg == nil || !cfg.SearchEnabled {
		return nil, ""
	}
	path := strings.TrimSpace(cfg.SearchPath)
	if path == "" {
		path = "/search"
	}
	if cfg.SearchModelID > 0 {
		m, err := ai.GetAiModel(cfg.SearchModelID)
		if err != nil || m == nil || !m.Enabled {
			return nil, ""
		}
		return m, path
	}
	// 没指定检索模型就沿用对话模型（它可能本身就带 /search）
	return chatModel, path
}

// resolveBadwordSearchMode 决定实际使用的检索模式。
//
// requested=auto     → 有检索源用 endpoint，其次模型原生 native，最后 prompt
// requested=endpoint → 有检索源才 endpoint，否则退回 auto 的判定
// requested=native   → 模型确实带原生联网能力才 native，否则退回 prompt
// requested=prompt   → 不联网
func resolveBadwordSearchMode(requested string, chatModel *ai.AiModel, hasSearchSource bool) string {
	switch strings.ToLower(strings.TrimSpace(requested)) {
	case badwordSearchModeEndpoint:
		if hasSearchSource {
			return badwordSearchModeEndpoint
		}
		// 没有检索源：落到 auto 的判定
	case badwordSearchModeNative:
		if chatModel != nil && chatModel.SearchNative {
			return badwordSearchModeNative
		}
		return badwordSearchModePrompt
	case badwordSearchModePrompt:
		return badwordSearchModePrompt
	}
	if hasSearchSource {
		return badwordSearchModeEndpoint
	}
	if chatModel != nil && chatModel.SearchNative {
		return badwordSearchModeNative
	}
	return badwordSearchModePrompt
}

// acquireBadwordModelKey 取模型密钥（走平台密钥租赁，不暴露给前端、不做用户级计费）。
func acquireBadwordModelKey(m *ai.AiModel) (string, error) {
	if m == nil {
		return "", fmt.Errorf("未配置模型")
	}
	lease, err := aihub.AcquireModelKey(m)
	if err != nil {
		return "", err
	}
	if lease == nil || lease.Key == "" {
		return "", fmt.Errorf("模型无可用密钥")
	}
	return lease.Key, nil
}

// badwordSearchSnippets 调用上游 /search 拿公开资料摘要（Path 3）。
//
// 复用舆情词表更新模块的检索实现（同一套厂商 /search 协议与多返回结构兼容），
// 保证两个功能用的是同一个联网通道、同一套返回结构解析。
func badwordSearchSnippets(ctx context.Context, m *ai.AiModel, apiKey, path, query string) ([]updater.Snippet, error) {
	if m == nil {
		return nil, fmt.Errorf("未配置检索模型")
	}
	if path == "" {
		path = "/search"
	}
	// 时间窗：黑话变化快，搜半年前的语境等于判断过时用法
	return updater.DoSearch(ctx, m, apiKey, path, query, 30, 8)
}

// attachNativeSearch 给请求挂上「模型原生联网」参数（Path 2）。
//
// 不同厂商的联网检索开关不一样，这里按兼容性组合两个都带上：
//   - web_search_options：OpenAI 的 *-search-preview 系列
//   - tools: [{"type":"web_search"}]：一部分中转/自建网关
//
// 两者都是可选字段；上游不认时会在 Complete 报错，由调用方降级重试。
func attachNativeSearch(req *aiclient.ChatRequest, m *ai.AiModel) {
	if req == nil || m == nil || !m.SearchNative {
		return
	}
	req.WebSearchOptions = map[string]interface{}{"search_context_size": "medium"}
	req.Tools = []map[string]interface{}{
		{"type": "web_search"},
		{"type": "web_search_preview"},
	}
}

// isUnsupportedSearchParamError 判断上游错误是不是「不认联网检索参数」。
// 这类错误要降级重试（去掉参数再问一次），而不是直接报错。
func isUnsupportedSearchParamError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	for _, k := range []string{
		"web_search_options",
		"unrecognized request argument",
		"unknown parameter",
		"unsupported parameter",
		"unsupported value",
		"invalid parameter",
	} {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

// parseBadWordJSON 从模型返回中稳健提取 JSON 数组。
//
// 模型不总是听话，常见跑偏方式都覆盖：
//  1. 前后带说明文字（「以下是清单：」）；
//  2. 带 Markdown 代码围栏；
//  3. 说明文字里本身带 [ ]（如「见下表[1]」）⇒ 不能用「首个 [ 到末个 ]」切；
//  4. 包成对象 {"words": [...]};
//  5. 一行一个对象的 JSONL。
func parseBadWordJSON(content string) ([]security.BadWordImportItem, error) {
	s := stripCodeFence(strings.TrimSpace(content))
	if s == "" {
		return nil, fmt.Errorf("模型返回为空")
	}
	// 1) 括号配对取出最外层数组
	if arr, ok := matchBracket(s, '[', ']'); ok {
		if items, ok2 := unmarshalItems(arr); ok2 {
			return items, nil
		}
	}
	// 2) 对象里包着数组
	if obj, ok := matchBracket(s, '{', '}'); ok {
		if arr, ok2 := findArrayField(obj); ok2 {
			if items, ok3 := unmarshalItems(arr); ok3 {
				return items, nil
			}
		}
	}
	// 3) 逐个花括号片段兜底（模型只吐了部分、或前后混了散文）
	if items, ok := salvageBraces(s); ok {
		return items, nil
	}
	return nil, fmt.Errorf("未找到 JSON 数组")
}

// stripCodeFence 去掉 Markdown 代码围栏（```json ... ```）。
func stripCodeFence(s string) string {
	if !strings.HasPrefix(s, "```") {
		return strings.TrimSpace(s)
	}
	if i := strings.Index(s, "\n"); i >= 0 {
		s = s[i+1:]
	} else {
		s = strings.TrimPrefix(s, "```")
	}
	if j := strings.LastIndex(s, "```"); j >= 0 {
		s = s[:j]
	}
	return strings.TrimSpace(s)
}

// matchBracket 从第一个 open 位置起做括号配对（正确跳过字符串字面量与转义），
// 返回配对区间内的原文。比「首个[到末个]」稳，不会被正文里的方括号带偏。
func matchBracket(s string, open, close byte) (string, bool) {
	start := strings.IndexByte(s, open)
	if start < 0 {
		return "", false
	}
	depth := 0
	inStr := false
	escaped := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inStr {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return s[start : i+1], true
			}
		}
	}
	return "", false
}

// findArrayField 在 JSON 对象里找第一个数组值。
func findArrayField(obj string) (string, bool) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(obj), &m); err != nil {
		return "", false
	}
	for _, v := range m {
		trimmed := strings.TrimSpace(string(v))
		if strings.HasPrefix(trimmed, "[") {
			return trimmed, true
		}
	}
	return "", false
}

// unmarshalItems 尝试把一段 JSON 解析成违禁词清单。
func unmarshalItems(arr string) ([]security.BadWordImportItem, bool) {
	var items []security.BadWordImportItem
	if err := json.Unmarshal([]byte(arr), &items); err == nil {
		return items, true
	}
	// 单个对象（不是数组）
	var one security.BadWordImportItem
	if err := json.Unmarshal([]byte(arr), &one); err == nil && one.Pattern != "" {
		return []security.BadWordImportItem{one}, true
	}
	return nil, false
}

// salvageBraces 逐个抓 {…} 片段尝试解析（覆盖 JSONL 与「只吐了前几条」）。
func salvageBraces(s string) ([]security.BadWordImportItem, bool) {
	var out []security.BadWordImportItem
	rest := s
	for {
		seg, ok := matchBracket(rest, '{', '}')
		if !ok {
			break
		}
		var one security.BadWordImportItem
		if err := json.Unmarshal([]byte(seg), &one); err == nil && one.Pattern != "" {
			out = append(out, one)
		}
		idx := strings.Index(rest, seg)
		if idx < 0 {
			break
		}
		rest = rest[idx+len(seg):]
	}
	return out, len(out) > 0
}

// truncateForLog 截断日志内容。
func truncateForLog(s string, n int) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), "\n", "\\n")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…(截断)"
}
