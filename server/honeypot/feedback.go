package honeypot

// ==================== 规则反哺（M4） ====================
// 蜜罐最有价值的产出不是「抓到谁」，而是「把扫我们的人的特征变成规则」：
//   命中事件的 UA / path 特征 → 写进限流配置的爬虫阻断关键词 → 立刻对全网生效。
//
// 约束：
//   - 只反哺**明确恶意**的特征（诱饵命中 / 提权尝试），普通 404 不反哺；
//   - 关键词最小长度 4，避免误伤（如 "go" 会误杀 Go 客户端）；
//   - 反哺动作留痕（写审计事件），可回溯是哪次命中带来的规则。

import (
	"sort"
	"strings"
	"time"

	"nvs-server/models"
	"nvs-server/security"
)

// Suggestion 一条建议规则
type Suggestion struct {
	Keyword string `json:"keyword"`
	Reason  string `json:"reason"`
	Hits    int    `json:"hits"`
	Kind    string `json:"kind"` // ua / path
}

// SuggestRules 从事件里提炼候选规则（不改配置，供管理员确认）
func SuggestRules(limit int) []Suggestion {
	if limit <= 0 || limit > 50 {
		limit = 15
	}
	var events []Event
	_ = models.DB.Where("risk >= ?", 60).Order("created_at DESC").Limit(500).Find(&events).Error

	uaCount := map[string]int{}
	pathCount := map[string]int{}
	for _, e := range events {
		if kw := uaKeyword(e.UA); kw != "" {
			uaCount[kw]++
		}
		if kw := pathKeyword(e.Path); kw != "" {
			pathCount[kw]++
		}
	}
	out := make([]Suggestion, 0, limit)
	for kw, n := range uaCount {
		out = append(out, Suggestion{Keyword: kw, Reason: "蜜罐命中 UA 特征", Hits: n, Kind: "ua"})
	}
	for kw, n := range pathCount {
		out = append(out, Suggestion{Keyword: kw, Reason: "蜜罐诱饵路径探测", Hits: n, Kind: "path"})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Hits > out[j].Hits })
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// uaKeyword 从 UA 里抽取可做阻断的标识（工具名/版本段）
func uaKeyword(ua string) string {
	if ua == "" || len(ua) < 4 {
		return ""
	}
	lower := strings.ToLower(ua)
	known := []string{"sqlmap", "nikto", "nuclei", "gobuster", "dirbuster", "masscan",
		"zgrab", "nmap", "acunetix", "netsparker", "wpscan", "ffuf", "feroxbuster",
		"xray", "burpsuite", "python-requests", "go-http-client", "curl/", "wget/"}
	for _, k := range known {
		if strings.Contains(lower, k) {
			return k
		}
	}
	return ""
}

// pathKeyword 从诱饵路径抽特征（只取明确属于探测的扩展/目录）
func pathKeyword(p string) string {
	lp := strings.ToLower(p)
	for _, k := range []string{"/.env", "/.git/", "/wp-login", "/phpinfo", "/actuator", "/phpmyadmin", "/swagger", "/adminer"} {
		if strings.Contains(lp, k) {
			return k
		}
	}
	return ""
}

// ApplyRules 把选中的关键词写进爬虫阻断名单（追加 + 去重），并立即重载
func ApplyRules(keywords []string) ([]string, error) {
	cfg := models.LoadAPIRateLimitConfig()
	have := map[string]bool{}
	for _, k := range cfg.BotBlockKeywords {
		have[strings.ToLower(strings.TrimSpace(k))] = true
	}
	added := make([]string, 0, len(keywords))
	for _, k := range keywords {
		k = strings.TrimSpace(k)
		lk := strings.ToLower(k)
		if len(k) < 4 || have[lk] {
			continue
		}
		cfg.BotBlockKeywords = append(cfg.BotBlockKeywords, k)
		have[lk] = true
		added = append(added, k)
	}
	if len(added) == 0 {
		return nil, nil
	}
	if err := models.SaveAPIRateLimitConfig(cfg); err != nil {
		return nil, err
	}
	security.ReloadBotRules()

	// 留痕：记一条导出类事件（kind=feedback）
	ev := &Event{
		SessionID: 0, Kind: "feedback", Decoy: "rule_feedback",
		Method: "APPLY", Path: "/admin/honeypot/feedback",
		BodySnippet: truncate("反哺关键词："+strings.Join(added, ","), 600),
		Risk:        0, Action: "feedback", CreatedAt: time.Now(),
	}
	_ = models.DB.Create(ev).Error
	return added, nil
}
