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

// ==================== 流式对话（SSE） ====================

// StreamChunk 流式增量；最后一包 Done=true 且带 usage（上游未返回则为 0）。
// Err 非空表示上游失败（内容增量不可用）。
type StreamChunk struct {
	Content string `json:"content"`
	// Reasoning 思考型模型（DeepSeek-R1 / GLM / Qwen thinking 等）的推理过程。
	// 这类模型经常 content 为空、正文全在 reasoning_content 里，只取 content 会表现为
	// 「模型没回复」——接入第三方网关时踩过（glm-5.3-flash / Qwen3.8-Flash-Next）。
	Reasoning string `json:"reasoning,omitempty"`
	Done      bool   `json:"done"`
	TokensIn  int    `json:"tokens_in,omitempty"`
	TokensOut int    `json:"tokens_out,omitempty"`
	Err       string `json:"err,omitempty"`
}

// sseLine 流式 data 行（OpenAI 兼容）
type sseLine struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
			// 各家命名不一致：DeepSeek / vLLM 用 reasoning_content，OpenRouter / 部分网关用 reasoning
			ReasoningContent string `json:"reasoning_content"`
			Reasoning        string `json:"reasoning"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// CompleteStream 发起流式补全，返回增量通道。
// 通道关闭即结束；最后一包 Done=true。连接失败时首包即为错误增量。
func (c *Client) CompleteStream(req ChatRequest) <-chan StreamChunk {
	ch := make(chan StreamChunk, 32)
	go func() {
		defer close(ch)
		if c.BaseURL == "" {
			ch <- StreamChunk{Done: true}
			return
		}
		req.Stream = true
		req.StreamOptions = map[string]interface{}{"include_usage": true}
		body, _ := json.Marshal(req)
		httpReq, err := http.NewRequest(http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			ch <- StreamChunk{Done: true}
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "text/event-stream")
		if c.APIKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
		}
		resp, err := c.HTTP.Do(httpReq)
		if err != nil {
			ch <- StreamChunk{Done: true, Err: fmt.Sprintf("上游连接失败：%v", err)}
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
			var er ErrorResponse
			msg := fmt.Sprintf("上游返回 %d", resp.StatusCode)
			if json.Unmarshal(raw, &er) == nil && er.Error.Message != "" {
				msg = fmt.Sprintf("上游返回 %d：%s", resp.StatusCode, er.Error.Message)
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
			if payload == "[DONE]" {
				break
			}
			var sl sseLine
			if json.Unmarshal([]byte(payload), &sl) != nil {
				continue
			}
			if sl.Usage != nil {
				tokensIn = sl.Usage.PromptTokens
				tokensOut = sl.Usage.CompletionTokens
			}
			for _, choice := range sl.Choices {
				if r := choice.Delta.ReasoningContent; r != "" {
					ch <- StreamChunk{Reasoning: r}
				} else if r := choice.Delta.Reasoning; r != "" {
					ch <- StreamChunk{Reasoning: r}
				}
				if choice.Delta.Content != "" {
					ch <- StreamChunk{Content: choice.Delta.Content}
				}
			}
		}
		ch <- StreamChunk{Done: true, TokensIn: tokensIn, TokensOut: tokensOut}
	}()
	return ch
}

// ==================== 绘画（/images/generations） ====================

// ImagesRequest 绘画请求（OpenAI 兼容）
type ImagesRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	N              int    `json:"n,omitempty"`
	Size           string `json:"size,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"` // url / b64_json
}

// ImagesResponse 绘画响应
type ImagesResponse struct {
	Data []struct {
		URL     string `json:"url"`
		B64JSON string `json:"b64_json"`
	} `json:"data"`
}

// GenerateImages 生成图片，返回 URL 或 data URI 列表
func (c *Client) GenerateImages(req ImagesRequest) ([]string, error) {
	if c.BaseURL == "" {
		return nil, fmt.Errorf("上游地址未配置")
	}
	if req.ResponseFormat == "" {
		req.ResponseFormat = "url"
	}
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequest(http.MethodPost, c.BaseURL+"/images/generations", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("上游连接失败：%v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var er ErrorResponse
		if json.Unmarshal(raw, &er) == nil && er.Error.Message != "" {
			return nil, fmt.Errorf("上游返回 %d：%s", resp.StatusCode, er.Error.Message)
		}
		return nil, fmt.Errorf("上游返回 %d", resp.StatusCode)
	}
	var ir ImagesResponse
	if json.Unmarshal(raw, &ir) != nil {
		return nil, fmt.Errorf("上游响应解析失败")
	}
	urls := make([]string, 0, len(ir.Data))
	for _, d := range ir.Data {
		if d.URL != "" {
			urls = append(urls, d.URL)
		} else if d.B64JSON != "" {
			urls = append(urls, "data:image/png;base64,"+d.B64JSON)
		}
	}
	if len(urls) == 0 {
		return nil, fmt.Errorf("上游未返回图片")
	}
	return urls, nil
}

// ==================== 视频（异步任务：/videos 提交 + /videos/{id} 查询） ====================

// VideosCreateRequest 视频提交请求（通用字段，兼容主流上游）
type VideosCreateRequest struct {
	Model    string `json:"model"`
	Prompt   string `json:"prompt"`
	Duration int    `json:"duration,omitempty"`
	Size     string `json:"size,omitempty"`
}

// VideosCreateResponse 视频提交响应（id 或 task_id 任一）
type VideosCreateResponse struct {
	ID     string `json:"id"`
	TaskID string `json:"task_id"`
	Status string `json:"status"`
}

// VideosQueryResponse 视频查询响应
type VideosQueryResponse struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	VideoURL string `json:"video_url"`
	URL      string `json:"url"`
	Error    string `json:"error"`
}

// CreateVideo 提交视频生成任务，返回上游任务 ID 与初始状态
func (c *Client) CreateVideo(req VideosCreateRequest) (string, string, error) {
	if c.BaseURL == "" {
		return "", "", fmt.Errorf("上游地址未配置")
	}
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequest(http.MethodPost, c.BaseURL+"/videos", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return "", "", fmt.Errorf("上游连接失败：%v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var er ErrorResponse
		if json.Unmarshal(raw, &er) == nil && er.Error.Message != "" {
			return "", "", fmt.Errorf("上游返回 %d：%s", resp.StatusCode, er.Error.Message)
		}
		return "", "", fmt.Errorf("上游返回 %d", resp.StatusCode)
	}
	var vr VideosCreateResponse
	if json.Unmarshal(raw, &vr) != nil {
		return "", "", fmt.Errorf("上游响应解析失败")
	}
	remoteID := vr.ID
	if remoteID == "" {
		remoteID = vr.TaskID
	}
	if remoteID == "" {
		return "", "", fmt.Errorf("上游未返回任务 ID")
	}
	status := vr.Status
	if status == "" {
		status = "running"
	}
	return remoteID, status, nil
}

// QueryVideo 查询视频任务状态与成品地址
// CancelVideo 尝试撤销上游视频任务（DELETE /videos/{id}）。
//
// 各家上游支持度不一：OpenAI 风格的 /videos 支持 DELETE，部分聚合网关只支持查询。
// 因此调用方必须把这里的失败当成「可忽略」——本地取消与退款不依赖它。
func (c *Client) CancelVideo(remoteID string) error {
	if c.BaseURL == "" || remoteID == "" {
		return fmt.Errorf("缺少上游任务标识")
	}
	httpReq, err := http.NewRequest(http.MethodDelete, c.BaseURL+"/videos/"+remoteID, nil)
	if err != nil {
		return err
	}
	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, derr := c.HTTP.Do(httpReq)
	if derr != nil {
		return derr
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	return fmt.Errorf("上游返回 %d：%s", resp.StatusCode, strings.TrimSpace(string(raw)))
}

func (c *Client) QueryVideo(remoteID string) (status, videoURL, errMsg string, err error) {
	if c.BaseURL == "" || remoteID == "" {
		return "", "", "", fmt.Errorf("参数不完整")
	}
	httpReq, rerr := http.NewRequest(http.MethodGet, c.BaseURL+"/videos/"+remoteID, nil)
	if rerr != nil {
		return "", "", "", rerr
	}
	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, derr := c.HTTP.Do(httpReq)
	if derr != nil {
		return "", "", "", fmt.Errorf("上游连接失败：%v", derr)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", "", fmt.Errorf("上游返回 %d", resp.StatusCode)
	}
	var qr VideosQueryResponse
	if json.Unmarshal(raw, &qr) != nil {
		return "", "", "", fmt.Errorf("上游响应解析失败")
	}
	videoURL = qr.VideoURL
	if videoURL == "" {
		videoURL = qr.URL
	}
	status = qr.Status
	if status == "" {
		if videoURL != "" {
			status = "succeeded"
		} else {
			status = "running"
		}
	}
	return status, videoURL, qr.Error, nil
}
