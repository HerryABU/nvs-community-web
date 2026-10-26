// Package updater 敏感词表的每日自动更新。
//
// 两条线（对应产品要求）：
//   - 对内：从近期被标注的评论里找可疑新词，AI 归纳成候选；
//   - 对外：调用 AI 厂商自带的 /search 检索「这个词在公开网络上怎么用」，
//     再由模型总结成「词 + 风险标签 + 依据摘要 + 置信度」。
//
// 两条线共用一条流水线：候选 → （检索）→ 总结 → 待审/入库 → 审计留痕。
//
// ⚠️ 合规边界（评审明确要求）：
//   - **检索结果原文不入库**。搜索结果只作为判断依据，落库的是「我判断后的词 + 标签 + 一句摘要」；
//   - 摘要由模型生成，不是网页原文搬运，避免第三方内容版权/隐私问题；
//   - 提示词必须中立（不预设「这词一定是黑话」），证据不足要允许输出 unknown。
package updater

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"nvs-server/aiclient"
	"nvs-server/models"
)

// Snippet 一次检索结果的最小投影。
//
// 只在内存里存在，**不落库**：URL/正文都属于第三方内容，存下来就是另一回事了。
type Snippet struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Summary string `json:"summary"`
	Date    string `json:"date"`
}

// Candidate AI 总结出的候选词。
type Candidate struct {
	Word string   `json:"word"`
	Cat  string   `json:"cat"`
	Tags []string `json:"tags"`
	Risk string   `json:"risk"`
	// Confidence 0-100：低于配置门槛时只入库为 pending，永不自动进 active
	Confidence int    `json:"confidence"`
	Basis      string `json:"basis"`
}

// searchShape 兼容几种常见返回结构（不同厂商字段名不一致，硬编一种会直接不可用）。
type searchShape struct {
	Results []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Snippet string `json:"snippet"`
		Content string `json:"content"`
		Desc    string `json:"description"`
		Date    string `json:"date"`
	} `json:"results"`
	Data []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Snippet string `json:"snippet"`
		Desc    string `json:"description"`
		Date    string `json:"date"`
	} `json:"data"`
}

// DoSearch 调用厂商自带的检索接口。
//
// path 可配（默认 /search），因为各家的实际路径不同（DeepSeek/Kimi 走 /v1/tools/search，
// Perplexity 走 /search）。base_url 已含 /v1 的情况下这里只拼相对路径。
func DoSearch(ctx context.Context, m *models.AiModel, apiKey, path, query string, recencyDays, maxResults int) ([]Snippet, error) {
	if m == nil {
		return nil, fmt.Errorf("未配置检索模型")
	}
	if path == "" {
		path = "/search"
	}
	if maxResults <= 0 {
		maxResults = 8
	}
	body := map[string]interface{}{
		"query":       query,
		"max_results": maxResults,
	}
	// 时间窗：黑话变化快，搜半年期的语境等于判断过时用法
	if recencyDays > 0 {
		body["recency_days"] = recencyDays
	}
	raw, _ := json.Marshal(body)

	url := strings.TrimRight(m.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("检索接口返回 %d", resp.StatusCode)
	}

	var sh searchShape
	if err := json.Unmarshal(b, &sh); err != nil {
		return nil, fmt.Errorf("检索结果无法解析")
	}
	out := make([]Snippet, 0, maxResults)
	for _, r := range sh.Results {
		s := r.Snippet
		if s == "" {
			s = r.Content
		}
		if s == "" {
			s = r.Desc
		}
		out = append(out, Snippet{Title: r.Title, URL: r.URL, Summary: s, Date: r.Date})
	}
	for _, r := range sh.Data {
		s := r.Snippet
		if s == "" {
			s = r.Desc
		}
		out = append(out, Snippet{Title: r.Title, URL: r.URL, Summary: s, Date: r.Date})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("检索无结果")
	}
	if len(out) > maxResults {
		out = out[:maxResults]
	}
	return out, nil
}

// summarizePrompt 总结用的系统提示（**中立**，不预设结论）。
const summarizePrompt = `你是内容安全词表的分析助手。下面给出某个候选词在公开网络上的检索摘要（只有摘要，没有原文）。

请**中立地**判断：这个词在当前网络语境中，是否被用作辱骂、刷屏动员、群体蔑称或圈层黑话。

要求：
1. 只依据给出的摘要，不要凭记忆补充；证据不足就降低 confidence，不要为了给出结论而脑补。
2. 如果这个词在正常语境与黑话语境都出现（多义词），在 basis 里写明，并把 confidence 压到 60 以下。
3. **不要输出检索摘要的原文**，basis 用你自己的话概括，一句话即可。
4. 只输出一行 JSON 数组，不要 markdown、不要解释：
[{"word":"候选词","cat":"abuse|raid|slang|bigotry","tags":["亚文化"],"risk":"low|mid|midhigh|high","confidence":0,"basis":"一句话依据"}]
5. 如果判断不出是黑话，输出 []。`

// Summarize 让模型把检索摘要归纳成候选词。
//
// 输出的 basis 是**模型自己的概括**，不是网页原文——这是合规边界的关键一步。
func Summarize(ctx context.Context, client *aiclient.Client, modelID, query string, snippets []Snippet) ([]Candidate, error) {
	var b strings.Builder
	b.WriteString("候选词：")
	b.WriteString(query)
	b.WriteString("\n\n检索摘要：\n")
	for i, s := range snippets {
		if i >= 8 {
			break
		}
		text := strings.TrimSpace(s.Summary)
		if len([]rune(text)) > 240 {
			text = string([]rune(text)[:240]) + "…"
		}
		fmt.Fprintf(&b, "%d. %s —— %s\n", i+1, s.Title, text)
	}

	content, _, _, err := client.Complete(aiclient.ChatRequest{
		Ctx:   ctx,
		Model: modelID,
		Messages: []aiclient.Message{
			{Role: "system", Content: summarizePrompt},
			{Role: "user", Content: b.String()},
		},
		MaxTokens:   600,
		Temperature: 0,
	})
	if err != nil {
		return nil, err
	}
	return parseCandidates(content), nil
}

// parseCandidates 从模型回复里抠 JSON 数组；失败返回空（宁可当天不更新，也不乱入库）。
func parseCandidates(s string) []Candidate {
	raw := strings.TrimSpace(s)
	i := strings.Index(raw, "[")
	j := strings.LastIndex(raw, "]")
	if i < 0 || j <= i {
		return nil
	}
	var out []Candidate
	if err := json.Unmarshal([]byte(raw[i:j+1]), &out); err != nil {
		return nil
	}
	clean := make([]Candidate, 0, len(out))
	for _, c := range out {
		c.Word = strings.TrimSpace(c.Word)
		if c.Word == "" || len([]rune(c.Word)) > 12 {
			continue
		}
		if c.Cat == "" {
			c.Cat = "slang"
		}
		if c.Confidence < 0 {
			c.Confidence = 0
		}
		if c.Confidence > 100 {
			c.Confidence = 100
		}
		clean = append(clean, c)
	}
	return clean
}
