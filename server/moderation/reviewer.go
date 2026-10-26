package moderation

import (
	"errors"
	"strings"
	"time"

	"nvs-server/aiclient"
	"nvs-server/models"
)

// ReviewResult 一次 AI 中立判定的结果。
type ReviewResult struct {
	Verdict    string
	Categories []string
	Reason     string
	ModelID    uint
	ModelName  string
	// Topics AI 敲定的**舆论话题标签**（亚文化/性别议题/政治批判…），
	// 与 Categories（违规类别）不是一回事：Categories 判「违不违规」，
	// Topics 判「这段话处在什么舆论语境」，后者决定评论区该拦还是只标注。
	Topics []string
	// TopicConfidence 话题标注置信度 0-100；低于门槛时守卫只标注、不拦截
	TopicConfidence int
}

// reviewSystemPrompt 中立审核系统提示词（强制客观，不受「被举报」干扰）。
const reviewSystemPrompt = `你是一名客观中立的内容安全审核员。请仅依据内容本身判断是否存在违规，不要被"被举报"等外部因素左右。
必须严格以如下 JSON 输出，不要输出任何多余文字：
{"verdict":"pass"|"fail"|"review","categories":["类别1"],"reason":"不超过60字","topics":["话题标签"],"topic_confidence":0}
其中：
- verdict：pass=合规；fail=明显违规（色情/暴力/违法/仇恨言论等）；review=无法判定需人工复核。
- categories：命中的违规类别，没有则空数组。
- topics：这段话所处的**舆论话题标签**，从下列词里选（可多选，无关则空数组）：
  亚文化、二次元、性别议题、政治批判、资本批判、地域民族、医疗教育、饭圈追星。
  注意：话题标签不是违规判定——「政治批判」是合法表达，它只表示这段内容容易引发舆论冲突。
- topic_confidence：你对 topics 判断的把握，0-100。拿不准就给低分（<60），不要硬猜。
`

// ReviewText 对单段文本做中立判定。
// reTrigger=true 表示因重复举报达阈值而复检，提示词会强制注入「客观中立、不预设立场」要求。
func ReviewText(scope string, content string, reTrigger bool, reportCount int) (*ReviewResult, error) {
	if strings.TrimSpace(content) == "" {
		return &ReviewResult{Verdict: VerdictPass, Reason: "内容为空，视为合规"}, nil
	}
	m, err := pickModel()
	if err != nil {
		return nil, err
	}
	key, kerr := m.DecryptApiKey()
	if kerr != nil {
		return nil, kerr
	}

	user := "请审核以下内容（类型：" + scope + "）：\n---\n" + content + "\n---"
	if reTrigger {
		user = "该内容已被多人举报（累计 " + itoa(reportCount) + " 次），但此前 AI 审核判定为合规。" +
			"请注意：现在无法判断是恶意举报还是内容本身存在问题，请以客观中立态度重新审查，" +
			"既不因为举报次数多就偏向判定违规，也不预设内容一定合规。\n" + user
	}

	client := aiclient.NewClient(m.BaseURL, key, 60*time.Second)
	out, _, _, cerr := client.Complete(aiclient.ChatRequest{
		Model: m.ModelID,
		Messages: []aiclient.Message{
			{Role: "system", Content: reviewSystemPrompt},
			{Role: "user", Content: user},
		},
		MaxTokens:   600,
		Temperature: 0,
	})
	if cerr != nil {
		return nil, cerr
	}

	res := &ReviewResult{ModelID: m.ID, ModelName: m.Name}
	verdict, cats, reason, perr := extractJSON(out)
	if perr != nil {
		// 解析失败 → 转人工复核，绝不误判
		res.Verdict = VerdictReview
		res.Reason = "AI 响应解析失败，转人工复核"
		return res, nil
	}
	res.Verdict = verdict
	res.Categories = cats
	res.Reason = reason
	res.Topics, res.TopicConfidence = extractTopics(out)
	switch res.Verdict {
	case VerdictPass, VerdictFail, VerdictReview:
	default:
		res.Verdict = VerdictReview
	}
	return res, nil
}

// pickModel 选取审核用 chat 模型：优先配置指定，否则取第一个启用 chat 模型。
func pickModel() (*models.AiModel, error) {
	if cfg, err := GetConfig(); err == nil && cfg != nil && cfg.ModelID != 0 {
		if m, e := models.GetAiModel(cfg.ModelID); e == nil && m != nil && m.Enabled {
			return m, nil
		}
	}
	list, err := models.ListAiModelsByKind("chat", true)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, errors.New("无可用审核模型")
	}
	return &list[0], nil
}
