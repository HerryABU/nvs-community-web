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
