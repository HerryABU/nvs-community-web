package aiclient

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// PingAuth 用密钥探测上游是否可用（请求 /models，不消耗生成配额）。
// 上游差异较大：401/403 视为密钥无效；2xx 视为可用；其余返回状态码供站长判断。
func PingAuth(baseURL, apiKey string) error {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return fmt.Errorf("上游地址未配置")
	}
	if strings.TrimSpace(apiKey) == "" {
		return fmt.Errorf("密钥为空")
	}
	url := base + "/models"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", ClaudeVersion)
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("连接失败：%v", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == 401 || resp.StatusCode == 403:
		return fmt.Errorf("密钥无效（HTTP %d）", resp.StatusCode)
	case resp.StatusCode == 404:
		// 部分中转不实现 /models：视为"无法验证"，但不判失败
		return nil
	default:
		return fmt.Errorf("上游返回 HTTP %d", resp.StatusCode)
	}
}
