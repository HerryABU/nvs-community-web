package handlers

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"nvs-server/aiclient"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// SearchAugmentBadWords POST /api/admin/bad-words/search-augment
//
// 用 AI（带联网检索能力的上游模型）检索当下「高风险、新出现、或常见规避写法」的违禁词与变体，
// 解析为结构化规则后批量导入。安全设计：
//   - 仅取模型返回的「字面关键词」（不采纳 AI 给的正则），避免 AI 注入危险正则；
//   - 默认动作一律 mark（仅标记待审，不自动删改内容），由管理员在待审列表复核确认；
//   - 按 name+match_type+pattern 去重，已存在的规则自动跳过，不会重复创建。
func SearchAugmentBadWords(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		ModelID  uint   `json:"model_id"` // 可选：指定对话模型；不填则用平台默认启用的对话模型
		Category string `json:"category"` // 可选：统一归类
		Scope    string `json:"scope"`    // 可选：all 或 chapter,comment,forum,blog；不填=all
		Focus    string `json:"focus"`    // 可选：侧重方向（政治/色情/毒品/诈骗/赌博/广告/辱骂/全部）
	}
	_ = c.ShouldBindJSON(&req)

	// 解析目标模型
	var m *models.AiModel
	var err error
	if req.ModelID > 0 {
		m, err = models.GetAiModel(req.ModelID)
		if err != nil || m == nil || !m.Enabled {
			utils.BadRequest(c, "指定的模型不可用")
			return
		}
	} else {
		m, err = models.GetEnabledAiModelOfKind(0, models.AiKindChat)
		if err != nil || m == nil {
			utils.BadRequest(c, "未配置可用的对话模型，无法发起联网检索")
			return
		}
	}

	// 复用平台密钥租赁（不向管理员暴露密钥，也不做用户级计费）
	lease, kerr := models.AcquireModelKey(m)
	apiKey := ""
	if lease != nil {
		apiKey = lease.Key
	}
	if kerr != nil || apiKey == "" {
		utils.InternalError(c, "获取模型密钥失败，无法发起检索")
		return
	}
	client := aiclient.NewClient(m.BaseURL, apiKey, 120*time.Second)

	systemPrompt := buildSearchAugmentSystemPrompt(req.Focus)
	userPrompt := "请联网检索（若你的 API 支持联网搜索，请务必检索最新信息；否则基于你掌握的最新知识），" +
		"整理一份「当前高风险、新出现、或常见规避写法」的违禁词清单，" +
		"覆盖网络黑话、新型诈骗/赌博术语、新型毒品代称、违规广告与辱骂变体等。" +
		"只返回 JSON 数组，不要任何解释、前言或 Markdown 代码围栏。"
	chatReq := aiclient.ChatRequest{
		Model:       m.ModelID,
		Messages:    []aiclient.Message{{Role: "system", Content: systemPrompt}, {Role: "user", Content: userPrompt}},
		MaxTokens:   2048,
		Temperature: 0.2,
	}
	content, _, _, cerr := client.Complete(chatReq)
	if cerr != nil {
		utils.InternalError(c, "AI 检索失败："+cerr.Error())
		return
	}

	items, perr := parseBadWordJSON(content)
	if perr != nil {
		utils.BadRequest(c, "AI 返回无法解析为违禁词清单："+perr.Error())
		return
	}
	if len(items) == 0 {
		utils.Success(c, gin.H{
			"total": 0, "created": 0, "skipped": 0,
			"items":   []any{},
			"message": "未解析到可导入的违禁词",
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

	created, skipped, ierr := models.ImportBadWords(items)
	if ierr != nil {
		utils.InternalError(c, "导入失败："+ierr.Error())
		return
	}
	ReloadBadWords()
	utils.Success(c, gin.H{
		"total":   len(items),
		"created": created,
		"skipped": skipped,
		"items":   items,
		"message": fmt.Sprintf("联网检索补充完成：新建 %d 条，跳过（已存在/无效）%d 条，均进入待审由管理员复核", created, skipped),
	})
}

// buildSearchAugmentSystemPrompt 构造内容安全分析师的系统提示词
func buildSearchAugmentSystemPrompt(focus string) string {
	var b strings.Builder
	b.WriteString("你是一名中文互联网内容安全分析师，熟悉各平台最新的违禁词、黑话与规避变体。")
	b.WriteString("请基于联网检索（若你的 API 支持联网搜索，请务必检索最新信息；否则基于你掌握的最新知识），")
	b.WriteString("整理一份「当前高风险、新出现、或常见规避写法」的违禁词清单。\n")
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

// parseBadWordJSON 从模型返回中稳健提取 JSON 数组（容忍代码围栏与前后多余的文字）
func parseBadWordJSON(content string) ([]models.BadWordImportItem, error) {
	s := strings.TrimSpace(content)
	// 去掉 ```json ... ``` 围栏
	if strings.HasPrefix(s, "```") {
		if i := strings.Index(s, "\n"); i >= 0 {
			s = s[i+1:]
		}
		if j := strings.LastIndex(s, "```"); j >= 0 {
			s = s[:j]
		}
	}
	s = strings.TrimSpace(s)
	start := strings.Index(s, "[")
	end := strings.LastIndex(s, "]")
	if start < 0 || end < 0 || end < start {
		return nil, fmt.Errorf("未找到 JSON 数组")
	}
	s = s[start : end+1]
	var items []models.BadWordImportItem
	if err := json.Unmarshal([]byte(s), &items); err != nil {
		return nil, err
	}
	return items, nil
}
