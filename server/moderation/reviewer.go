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
}

// reviewSystemPrompt 中立审核系统提示词（强制客观，不受「被举报」干扰）。
const reviewSystemPrompt = `你是一名客观中立的内容安全审核员。请仅依据内容本身判断是否存在违规，不要被"被举报"等外部因素左右。
必须严格以如下 JSON 输出，不要输出任何多余文字：
{"verdict":"pass"|"fail"|"review","categories":["类别1","类别2"],"reason":"不超过60字的中文判定依据"}
其中：pass=内容合规；fail=明显违规（如色情、暴力、违法、仇恨言论等）；review=无法判定、需人工复核。categories 仅列出命中的违规类别（没有则为空数组）。`

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
