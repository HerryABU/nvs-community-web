package moderation

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

func itoa(n int) string { return strconv.Itoa(n) }

func joinJSON(s []string) string {
	if len(s) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(s)
	return string(b)
}

// extractJSON 从 AI 可能夹带的文本中抠出第一个 JSON 对象并解析。
func extractJSON(s string) (verdict string, categories []string, reason string, err error) {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return "", nil, "", errors.New("no json found")
	}
	var out struct {
		Verdict    string   `json:"verdict"`
		Categories []string `json:"categories"`
		Reason     string   `json:"reason"`
	}
	if e := json.Unmarshal([]byte(s[start:end+1]), &out); e != nil {
		return "", nil, "", e
	}
	return out.Verdict, out.Categories, out.Reason, nil
}

// extractTopics 从同一段 JSON 里取话题标签与置信度。
//
// 独立于 extractJSON：话题是「附加信息」，缺失时不该让整条审核失败
// （老模型/自定义提示词可能不返回这两个字段）。
func extractTopics(s string) ([]string, int) {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return nil, 0
	}
	var out struct {
		Topics          []string `json:"topics"`
		TopicConfidence int      `json:"topic_confidence"`
	}
	if e := json.Unmarshal([]byte(s[start:end+1]), &out); e != nil {
		return nil, 0
	}
	// 过滤空白标签，避免把 AI 的半截输出写进库
	clean := make([]string, 0, len(out.Topics))
	for _, t := range out.Topics {
		if t = strings.TrimSpace(t); t != "" {
			clean = append(clean, t)
		}
	}
	c := out.TopicConfidence
	if c < 0 {
		c = 0
	}
	if c > 100 {
		c = 100
	}
	return clean, c
}
