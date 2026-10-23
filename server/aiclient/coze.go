// 扣子（Coze）智能体适配器：v3/chat 提交 → 轮询 retrieve → 取 message/list
// 文档语义：非流式提交（stream=false）后用 conversation_id + chat_id 轮询状态，
// 完成后再拉消息列表，取 role=assistant 且 type=answer 的内容拼接。
package aiclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// CozeClient 扣子客户端（按智能体配置构造，用后即焚）
type CozeClient struct {
	BaseURL string // 默认 https://api.coze.cn
	Token   string // 个人访问令牌（PAT）
	HTTP    *http.Client
}

// NewCozeClient 构造客户端
func NewCozeClient(baseURL, token string, timeout time.Duration) *CozeClient {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = "https://api.coze.cn"
	}
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return &CozeClient{BaseURL: base, Token: strings.TrimSpace(token), HTTP: &http.Client{Timeout: timeout}}
}

type cozeReq struct {
	BotID             string        `json:"bot_id"`
	UserID            string        `json:"user_id"`
	Stream            bool          `json:"stream"`
	AutoSaveHistory   bool          `json:"auto_save_history"`
	AdditionalMessage []cozeMessage `json:"additional_messages"`
	ConversationID    string        `json:"conversation_id,omitempty"`
}

type cozeMessage struct {
	Role        string `json:"role"`
	Type        string `json:"type"`
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
}

type cozeChatData struct {
	ID             string `json:"id"`
	ConversationID string `json:"conversation_id"`
	Status         string `json:"status"`
	LastError      struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
	} `json:"last_error"`
}

type cozeEnvelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func (c *CozeClient) do(method, path string, body interface{}) (*cozeEnvelope, error) {
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.BaseURL+path, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("扣子上游连接失败：%v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	var env cozeEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("扣子响应解析失败（HTTP %d）", resp.StatusCode)
	}
	if env.Code != 0 {
		msg := env.Msg
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("扣子返回 %d：%s", env.Code, msg)
	}
	return &env, nil
}

// Chat 发起一次对话并等待完成，返回 (回答正文, conversation_id, 错误)
// convID 非空时续接同一会话（保留多轮上下文）。
func (c *CozeClient) Chat(botID, userID, convID, question string, timeout time.Duration) (string, string, error) {
	if strings.TrimSpace(botID) == "" {
		return "", "", fmt.Errorf("未配置 bot_id")
	}
	if c.Token == "" {
		return "", "", fmt.Errorf("未配置扣子访问令牌")
	}
	if timeout <= 0 {
		timeout = 150 * time.Second
	}
	body := cozeReq{
		BotID: botID, UserID: userID, Stream: false, AutoSaveHistory: true,
		ConversationID: strings.TrimSpace(convID),
		AdditionalMessage: []cozeMessage{
			{Role: "user", Type: "question", Content: question, ContentType: "text"},
		},
	}
	env, err := c.do(http.MethodPost, "/v3/chat", body)
	if err != nil {
		return "", "", err
	}
	var cd cozeChatData
	if err := json.Unmarshal(env.Data, &cd); err != nil {
		return "", "", fmt.Errorf("扣子响应结构异常")
	}
	conversationID := cd.ConversationID
	chatID := cd.ID
	if conversationID == "" || chatID == "" {
		return "", "", fmt.Errorf("扣子未返回会话标识")
	}

	deadline := time.Now().Add(timeout)
	interval := 1500 * time.Millisecond
	status := strings.ToLower(cd.Status)
	for time.Now().Before(deadline) {
		switch status {
		case "completed":
			// 完成 → 拉消息
			ans, aerr := c.fetchAnswer(conversationID, chatID)
			if aerr != nil {
				return "", conversationID, aerr
			}
			return ans, conversationID, nil
		case "failed", "requires_action", "canceled", "cancelled":
			msg := cd.LastError.Msg
			if msg == "" {
				msg = "扣子对话未成功完成（状态 " + status + "）"
			}
			return "", conversationID, fmt.Errorf("%s", msg)
		}
		time.Sleep(interval)
		if interval < 3*time.Second {
			interval += 500 * time.Millisecond
		}
		q := fmt.Sprintf("/v3/chat/retrieve?conversation_id=%s&chat_id=%s", conversationID, chatID)
		env, err = c.do(http.MethodGet, q, nil)
		if err != nil {
			continue // 轮询抖动继续
		}
		if err := json.Unmarshal(env.Data, &cd); err != nil {
			continue
		}
		status = strings.ToLower(cd.Status)
	}
	return "", conversationID, fmt.Errorf("扣子响应超时（>%s），请稍后重试", timeout)
}

// fetchAnswer 取 assistant 的 answer 消息
func (c *CozeClient) fetchAnswer(conversationID, chatID string) (string, error) {
	q := fmt.Sprintf("/v3/chat/message/list?conversation_id=%s&chat_id=%s", conversationID, chatID)
	env, err := c.do(http.MethodGet, q, nil)
	if err != nil {
		return "", err
	}
	var msgs []struct {
		Role        string `json:"role"`
		Type        string `json:"type"`
		Content     string `json:"content"`
		ContentType string `json:"content_type"`
	}
	if err := json.Unmarshal(env.Data, &msgs); err != nil {
		return "", fmt.Errorf("扣子消息结构异常")
	}
	var sb strings.Builder
	for _, m := range msgs {
		if m.Role != "assistant" {
			continue
		}
		if m.Type != "" && m.Type != "answer" {
			continue
		}
		sb.WriteString(m.Content)
	}
	out := strings.TrimSpace(sb.String())
	if out == "" {
		return "", fmt.Errorf("扣子未返回回答内容")
	}
	return out, nil
}
