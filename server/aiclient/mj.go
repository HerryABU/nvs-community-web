// Midjourney 代理协议适配器（midjourney-proxy 兼容：/mj/submit/imagine + /mj/task/{id}/fetch）
// 覆盖 Midjourney 与 Niji·Journey —— 两者用同一套接口，靠 botType 区分。
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

// MJBotType 机器人类型
type MJBotType string

// 机器人类型常量
const (
	MidJourney  MJBotType = "MID_JOURNEY"
	NijiJourney MJBotType = "NIJI_JOURNEY"
)

// MJImagineParams 提交绘画参数
type MJImagineParams struct {
	Prompt     string   `json:"prompt"`
	BotType    MJBotType `json:"botType,omitempty"`
	Base64Arr  []string `json:"base64Array,omitempty"`
	NotifyHook string   `json:"notifyHook,omitempty"`
}

// mjSubmitResp /mj/submit/imagine 响应
type mjSubmitResp struct {
	Code        int    `json:"code"`
	Description string `json:"description"`
	Result      string `json:"result"`
}

// mjTaskResp /mj/task/{id}/fetch 响应
type mjTaskResp struct {
	Status     string `json:"status"`
	ImageURL   string `json:"imageUrl"`
	Progress   string `json:"progress"`
	FailReason string `json:"failReason"`
	Description string `json:"description"`
	Code       int    `json:"code"`
}

func (c *Client) mjHeaders(req *http.Request) {
	if c.APIKey != "" {
		// midjourney-proxy 认 mj-api-secret；部分中转用 Bearer，一并带上
		req.Header.Set("mj-api-secret", c.APIKey)
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	req.Header.Set("Content-Type", "application/json")
}

func (c *Client) mjURL(path string) string {
	base := strings.TrimRight(c.BaseURL, "/")
	// 兼容 base 已带 /mj 的情况
	if strings.HasSuffix(base, "/mj") {
		return base + strings.TrimPrefix(path, "/mj")
	}
	return base + path
}

// MJImagine 提交 imagine 任务，返回任务 ID
func (c *Client) MJImagine(p MJImagineParams) (string, error) {
	if c.BaseURL == "" {
		return "", fmt.Errorf("上游地址未配置")
	}
	if strings.TrimSpace(p.Prompt) == "" {
		return "", fmt.Errorf("提示词为空")
	}
	if p.BotType == "" {
		p.BotType = MidJourney
	}
	body, _ := json.Marshal(p)
	req, err := http.NewRequest(http.MethodPost, c.mjURL("/mj/submit/imagine"), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	c.mjHeaders(req)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("MJ 上游连接失败：%v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var sr mjSubmitResp
	if err := json.Unmarshal(raw, &sr); err != nil {
		return "", fmt.Errorf("MJ 响应解析失败（HTTP %d）", resp.StatusCode)
	}
	if sr.Code != 1 && sr.Code != 0 && sr.Code != 22 {
		msg := sr.Description
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return "", fmt.Errorf("MJ 提交失败：%s", msg)
	}
	if sr.Result == "" {
		return "", fmt.Errorf("MJ 未返回任务 ID")
	}
	return sr.Result, nil
}

// MJFetch 查询任务
func (c *Client) MJFetch(taskID string) (status, imageURL, failReason string, err error) {
	req, e := http.NewRequest(http.MethodGet, c.mjURL("/mj/task/"+taskID+"/fetch"), nil)
	if e != nil {
		return "", "", "", e
	}
	c.mjHeaders(req)
	resp, e := c.HTTP.Do(req)
	if e != nil {
		return "", "", "", fmt.Errorf("MJ 查询失败：%v", e)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var tr mjTaskResp
	if e := json.Unmarshal(raw, &tr); e != nil {
		return "", "", "", fmt.Errorf("MJ 查询响应解析失败")
	}
	return strings.ToUpper(tr.Status), tr.ImageURL, firstNonEmpty(tr.FailReason, tr.Description), nil
}

// MJImagineAndWait 提交并轮询直到出图（或超时/失败）。同步语义，供「同步出图」接口使用。
func (c *Client) MJImagineAndWait(p MJImagineParams, timeout time.Duration) ([]string, error) {
	if timeout <= 0 {
		timeout = 180 * time.Second
	}
	taskID, err := c.MJImagine(p)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(timeout)
	interval := 3 * time.Second
	for time.Now().Before(deadline) {
		time.Sleep(interval)
		if interval < 6*time.Second {
			interval += time.Second
		}
		status, url, fail, ferr := c.MJFetch(taskID)
		if ferr != nil {
			continue // 查询抖动：继续轮询
		}
		switch status {
		case "SUCCESS":
			if url == "" {
				return nil, fmt.Errorf("MJ 任务成功但未返回图片")
			}
			return []string{url}, nil
		case "FAILURE", "FAILED", "CANCEL", "CANCELED":
			if fail == "" {
				fail = "上游任务失败"
			}
			return nil, fmt.Errorf("MJ 任务失败：%s", fail)
		}
	}
	return nil, fmt.Errorf("MJ 出图超时（>%s），请稍后到「历史作品」查看", timeout)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
