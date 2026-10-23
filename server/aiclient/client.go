// Package aiclient — OpenAI 兼容上游 chat/completions 客户端（§5.3）。
// 只用标准库 net/http + encoding/json，避免引入三方依赖；
// 超时、错误体解析、usage 提取集中在此，handler 不关心协议细节。
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

// DefaultTimeout 上游默认超时（长文生成可放宽，由调用方覆盖）
const DefaultTimeout = 90 * time.Second

// Message 对话消息
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest /chat/completions 请求体
type ChatRequest struct {
	Model         string                 `json:"model"`
	Messages      []Message              `json:"messages"`
	MaxTokens     int                    `json:"max_tokens,omitempty"`
	Temperature   float64                `json:"temperature,omitempty"`
	Stream        bool                   `json:"stream,omitempty"`
	StreamOptions map[string]interface{} `json:"stream_options,omitempty"`
}

// ChatResponse /chat/completions 响应体（只取需要的字段）
type ChatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// ErrorResponse 上游错误体（OpenAI 风格）
type ErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// Client 上游客户端（按模型配置构造，用后即焚）
type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

// NewClient 构造客户端；baseURL 尾部斜杠会归一化剔除
func NewClient(baseURL, apiKey string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Client{
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		APIKey:  strings.TrimSpace(apiKey),
		HTTP:    &http.Client{Timeout: timeout},
	}
}

// Complete 发起一次对话补全，返回内容与真实 token 用量
func (c *Client) Complete(req ChatRequest) (content string, tokensIn, tokensOut int, err error) {
	if c.BaseURL == "" {
		return "", 0, 0, fmt.Errorf("上游地址未配置")
	}
	body, merr := json.Marshal(req)
	if merr != nil {
		return "", 0, 0, merr
	}
	httpReq, rerr := http.NewRequest(http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if rerr != nil {
		return "", 0, 0, rerr
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, derr := c.HTTP.Do(httpReq)
	if derr != nil {
		return "", 0, 0, fmt.Errorf("上游连接失败：%v", derr)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20)) // 4MB 上限防滥用
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var er ErrorResponse
		if json.Unmarshal(raw, &er) == nil && er.Error.Message != "" {
			return "", 0, 0, fmt.Errorf("上游返回 %d：%s", resp.StatusCode, er.Error.Message)
		}
		return "", 0, 0, fmt.Errorf("上游返回 %d", resp.StatusCode)
	}
	var cr ChatResponse
	if uerr := json.Unmarshal(raw, &cr); uerr != nil {
		return "", 0, 0, fmt.Errorf("上游响应解析失败")
	}
	if len(cr.Choices) == 0 {
		return "", 0, 0, fmt.Errorf("上游未返回内容")
	}
	return cr.Choices[0].Message.Content, cr.Usage.PromptTokens, cr.Usage.CompletionTokens, nil
}
