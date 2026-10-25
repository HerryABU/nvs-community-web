// Anthropic Claude Messages API 适配器（/v1/messages）
// 与 OpenAI 兼容协议的差异：
//   - 鉴权头：x-api-key（而非 Authorization: Bearer）+ 必填 anthropic-version
//   - system 是顶层字段，不放在 messages 里
//   - 响应 content 是 block 数组（type=text 的 text 拼接）
//   - 流式事件为 message_start / content_block_delta / message_delta / message_stop
package aiclient

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ClaudeVersion 当前 Messages API 版本（Anthropic 要求显式声明）
const ClaudeVersion = "2023-06-01"

// claudeMessage 消息体：content 可以是字符串（普通对话）或 block 数组（图片/文件）
type claudeMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type claudeRequest struct {
	Model       string          `json:"model"`
	MaxTokens   int             `json:"max_tokens"`
	System      string          `json:"system,omitempty"`
	Messages    []claudeMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type claudeBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type claudeResponse struct {
	Content []claudeBlock `json:"content"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// buildClaudeRequest 把 OpenAI 风格请求转成 Claude 请求（抽出 system 消息）
func buildClaudeRequest(req ChatRequest) claudeRequest {
	cr := claudeRequest{Model: req.Model, MaxTokens: req.MaxTokens, Temperature: req.Temperature, Stream: req.Stream}
	if cr.MaxTokens <= 0 {
		cr.MaxTokens = 2048
	}
	sys := []string{}
	for _, m := range req.Messages {
		// content 可能是纯字符串，也可能是多模态 parts；统一转成 Claude 需要的样子
		content := claudeContentOf(m)
		if m.Role == "system" {
			if s, ok := content.(string); ok && strings.TrimSpace(s) != "" {
				sys = append(sys, s)
			}
			continue
		}
		role := m.Role
		if role != "assistant" {
			role = "user"
		}
		cr.Messages = append(cr.Messages, claudeMessage{Role: role, Content: content})
	}
	cr.System = strings.Join(sys, "\n\n")
	// Claude 要求至少一条 user 消息
	if len(cr.Messages) == 0 {
		cr.Messages = []claudeMessage{{Role: "user", Content: "hi"}}
	}
	return cr
}

// claudeContentOf 把 Message.Content（string 或 parts）转成 Claude 的 content：
// 字符串原样返回；parts 转成 text/image/document block 数组。
func claudeContentOf(m Message) any {
	switch v := m.Content.(type) {
	case string:
		return v
	case []ContentPart:
		blocks := ClaudeBlocksFromParts(v)
		if len(blocks) == 0 {
			return ""
		}
		return blocks
	default:
		return m.Content
	}
}

func (c *Client) claudeURL() string {
	base := strings.TrimRight(c.BaseURL, "/")
	if strings.HasSuffix(base, "/v1") {
		return base + "/messages"
	}
	return base + "/v1/messages"
}

func (c *Client) claudeHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", ClaudeVersion)
	if c.APIKey != "" {
		req.Header.Set("x-api-key", c.APIKey)
		// 部分中转同时认 Bearer，一并带上（官方忽略该头）
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
}

// CompleteClaude 非流式补全
func (c *Client) CompleteClaude(req ChatRequest) (content string, tokensIn, tokensOut int, err error) {
	if c.BaseURL == "" {
		return "", 0, 0, fmt.Errorf("上游地址未配置")
	}
	body, _ := json.Marshal(buildClaudeRequest(req))
	httpReq, rerr := http.NewRequest(http.MethodPost, c.claudeURL(), bytes.NewReader(body))
	if rerr != nil {
		return "", 0, 0, rerr
	}
	c.claudeHeaders(httpReq)
	resp, derr := c.HTTP.Do(httpReq)
	if derr != nil {
		return "", 0, 0, fmt.Errorf("上游连接失败：%v", derr)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	var cr claudeResponse
	if uerr := json.Unmarshal(raw, &cr); uerr != nil {
		return "", 0, 0, fmt.Errorf("上游响应解析失败（HTTP %d）", resp.StatusCode)
	}
	if cr.Error.Message != "" {
		return "", 0, 0, fmt.Errorf("上游返回 %d：%s", resp.StatusCode, cr.Error.Message)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", 0, 0, fmt.Errorf("上游返回 %d", resp.StatusCode)
	}
	var sb strings.Builder
	for _, b := range cr.Content {
		if b.Type == "text" || b.Type == "" {
			sb.WriteString(b.Text)
		}
	}
	if sb.Len() == 0 {
		return "", 0, 0, fmt.Errorf("上游未返回内容")
	}
	return sb.String(), cr.Usage.InputTokens, cr.Usage.OutputTokens, nil
}

// claudeStreamEvent 流式事件（字段平铺，按 type 取用）
type claudeStreamEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Delta struct {
		Type       string `json:"type"`
		Text       string `json:"text"`
		StopReason string `json:"stop_reason"`
	} `json:"delta"`
	Message struct {
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// CompleteStreamClaude 流式补全（SSE 事件：content_block_delta 取增量文本）
func (c *Client) CompleteStreamClaude(req ChatRequest) <-chan StreamChunk {
	ch := make(chan StreamChunk, 32)
	go func() {
		defer close(ch)
		if c.BaseURL == "" {
			ch <- StreamChunk{Done: true, Err: "上游地址未配置"}
			return
		}
		req.Stream = true
		body, _ := json.Marshal(buildClaudeRequest(req))
		httpReq, err := http.NewRequest(http.MethodPost, c.claudeURL(), bytes.NewReader(body))
		if err != nil {
			ch <- StreamChunk{Done: true, Err: "请求构造失败"}
			return
		}
		c.claudeHeaders(httpReq)
		httpReq.Header.Set("Accept", "text/event-stream")
		resp, err := c.HTTP.Do(httpReq)
		if err != nil {
			ch <- StreamChunk{Done: true, Err: fmt.Sprintf("上游连接失败：%v", err)}
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
			var cr claudeResponse
			msg := fmt.Sprintf("上游返回 %d", resp.StatusCode)
			if json.Unmarshal(raw, &cr) == nil && cr.Error.Message != "" {
				msg = fmt.Sprintf("上游返回 %d：%s", resp.StatusCode, cr.Error.Message)
			}
			ch <- StreamChunk{Done: true, Err: msg}
			return
		}
		var tokensIn, tokensOut int
		scanner := bufio.NewScanner(io.LimitReader(resp.Body, 64<<20))
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "" {
				continue
			}
			var ev claudeStreamEvent
			if json.Unmarshal([]byte(payload), &ev) != nil {
				continue
			}
			switch ev.Type {
			case "message_start":
				tokensIn = ev.Message.Usage.InputTokens
			case "content_block_delta":
				if ev.Delta.Text != "" {
					ch <- StreamChunk{Content: ev.Delta.Text}
				}
			case "message_delta":
				if ev.Usage.OutputTokens > 0 {
					tokensOut = ev.Usage.OutputTokens
				}
				if ev.Usage.InputTokens > 0 {
					tokensIn = ev.Usage.InputTokens
				}
			case "message_stop":
				ch <- StreamChunk{Done: true, TokensIn: tokensIn, TokensOut: tokensOut}
				return
			case "error":
				ch <- StreamChunk{Done: true, Err: "上游错误：" + ev.Error.Message}
				return
			}
		}
		ch <- StreamChunk{Done: true, TokensIn: tokensIn, TokensOut: tokensOut}
	}()
	return ch
}
