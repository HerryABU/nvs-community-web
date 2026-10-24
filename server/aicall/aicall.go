// Package aicall 统一的「一次对话调用」入口：选模型 → 计费 → 调用 → 失败降级。
//
// 为什么要这层（gap-audit #10/#11）：
//   - 以前每个调用点自己写「取列表第一个模型 + 计费 + 调用」，路由策略无处落地；
//   - 模型故障时直接报错，用户只看到「调用失败」，没有备用链路；
//   - 降级必须**明确提示**（Result.Fallbacks），不允许静默换模型（会让用户以为换了答案来源）。
package aicall

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"nvs-server/aiclient"
	"nvs-server/models"
)

// FallbackNote 一次降级的事实（给前端/正文提示用）
type FallbackNote struct {
	From   string `json:"from"`   // 原模型展示名
	To     string `json:"to"`     // 实际使用的模型展示名
	Reason string `json:"reason"` // 失败原因（截断）
}

// Result 调用结果
type Result struct {
	Content   string         `json:"content"`
	ModelID   uint           `json:"model_id"`
	ModelName string         `json:"model_name"`
	TokensIn  int            `json:"tokens_in"`
	TokensOut int            `json:"tokens_out"`
	Cost      int64          `json:"cost"`
	Free      bool           `json:"free"`
	LatencyMs int            `json:"latency_ms"`
	Fallbacks []FallbackNote `json:"fallbacks,omitempty"`
	Strategy  string         `json:"strategy"`
}

// ErrNoModel 没有可用模型
var ErrNoModel = models.ErrNoChatModel

// Request 调用入参
type Request struct {
	PayerID     uint   // 计费归属（免费额度按此人判定）
	Scene       string // 场景：ai_hub / oc_turn / group_bot / oc_publish …
	Action      string // 动作（写进流水，便于按动作统计）
	RefType     string
	RefID       uint
	System      string
	User        string
	MaxTokens   int
	PreferModel uint // 用户/角色显式指定；0=按策略
	Temperature float64
	Timeout     time.Duration
}

// Complete 执行一次对话调用（含降级链）
func Complete(req Request) (*Result, error) {
	if req.PayerID == 0 {
		return nil, errors.New("缺少调用者")
	}
	if strings.TrimSpace(req.User) == "" {
		return nil, errors.New("内容为空")
	}
	first, err := models.PickChatModel(req.PreferModel, req.Scene)
	if err != nil {
		return nil, err
	}
	cfg := models.LoadAiRouteConfig()
	strategy := models.EffectiveStrategy(cfg, req.Scene)

	tried := make([]uint, 0, 2)
	candidates := make([]*models.AiModel, 0, 2)
	candidates = append(candidates, first)
	if cfg.Fallback {
		for _, m := range models.FallbackChatModels([]uint{first.ID}, req.Scene, cfg.MaxFallback) {
			mm := m
			candidates = append(candidates, &mm)
		}
	}

	res := &Result{Strategy: strategy}
	var lastErr error
	for i, m := range candidates {
		if m == nil {
			continue
		}
		tried = append(tried, m.ID)
		out, callErr := attempt(req, m, res)
		if callErr == nil {
			return out, nil
		}
		lastErr = callErr
		// 每次失败都记一条事实：谁失败、为什么、切到了谁（没有下一个就留空=无备用）
		note := FallbackNote{From: m.Name, Reason: truncate(callErr.Error(), 80)}
		if i+1 < len(candidates) && candidates[i+1] != nil {
			note.To = candidates[i+1].Name
		} else {
			note.To = "无可用备用模型"
		}
		res.Fallbacks = append(res.Fallbacks, note)
	}
	if lastErr == nil {
		lastErr = ErrNoModel
	}
	return nil, lastErr
}

// attempt 用指定模型尝试一次
func attempt(req Request, m *models.AiModel, res *Result) (*Result, error) {
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = m.MaxTokens
	}
	if maxTokens <= 0 {
		maxTokens = 1024
	}
	temp := req.Temperature
	if temp <= 0 {
		temp = m.Temperature
	}
	if temp <= 0 {
		temp = 0.7
	}
	est := models.EstimateTokens(req.System+"\n"+req.User) + maxTokens

	// 免费额度 / 余额预检（与既有一致：免费优先，其次代币）
	freeOK, _, _ := models.CheckAiFreeQuota(req.PayerID, est)
	if !freeOK {
		if !models.IsTokenEnabled() {
			return nil, errors.New("免费额度已用完且代币未启用")
		}
		if _, ok := models.PrecheckAiBalance(req.PayerID, m, est, maxTokens); !ok {
			return nil, errors.New("余额不足")
		}
	}

	lease, kerr := models.AcquireModelKey(m)
	if kerr != nil {
		return nil, fmt.Errorf("模型密钥不可用：%v", kerr)
	}
	apiKey := ""
	if lease != nil {
		apiKey = lease.Key
	}
	callLog, lerr := models.CreateAiCallLog(req.PayerID, m, req.Action, req.RefType, req.RefID, est, maxTokens, freeOK)
	if lerr != nil {
		return nil, errors.New("创建调用记录失败")
	}

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	client := aiclient.NewClient(m.BaseURL, apiKey, timeout)
	start := time.Now()
	content, realIn, realOut, cerr := client.Complete(aiclient.ChatRequest{
		Model: m.ModelID,
		Messages: []aiclient.Message{
			{Role: "system", Content: req.System},
			{Role: "user", Content: req.User},
		},
		MaxTokens: maxTokens, Temperature: temp,
	})
	latency := int(time.Since(start).Milliseconds())
	models.SetAiCallLatency(callLog.ID, latency)

	if cerr != nil {
		if lease != nil {
			lease.Fail()
		}
		models.FailAiCallLog(callLog.ID, cerr.Error())
		return nil, fmt.Errorf("%s：%v", m.Name, cerr)
	}
	if strings.TrimSpace(content) == "" {
		if lease != nil {
			lease.Fail()
		}
		models.FailAiCallLog(callLog.ID, "上游未返回内容")
		return nil, fmt.Errorf("%s：上游未返回内容", m.Name)
	}
	if realIn <= 0 {
		realIn = est
	}
	if realOut <= 0 {
		realOut = models.EstimateTokens(content)
	}
	cost := models.EstimateAiCost(m, realIn, realOut)
	if freeOK {
		_ = models.RecordAiFreeUsage(req.PayerID, 1, realIn+realOut)
		cost = 0
	} else if _, serr := models.SettleAiCallLog(callLog.ID, realIn, realOut); serr != nil {
		return nil, errors.New("结算失败")
	}
	lease.OK()

	res.Content = content
	res.ModelID = m.ID
	res.ModelName = m.Name
	res.TokensIn = realIn
	res.TokensOut = realOut
	res.Cost = cost
	res.Free = freeOK
	res.LatencyMs = latency
	return res, nil
}

// Notice 降级提示文案（调用方**必须**贴到输出里，不能静默换模型）
func (r *Result) Notice() string {
	if r == nil || len(r.Fallbacks) == 0 {
		return ""
	}
	parts := make([]string, 0, len(r.Fallbacks))
	for _, f := range r.Fallbacks {
		parts = append(parts, fmt.Sprintf("%s → %s（%s）", f.From, f.To, f.Reason))
	}
	return "模型已降级：" + strings.Join(parts, "；")
}

// Suffix 追加到正文尾部的提示（行内可见）
func (r *Result) Suffix() string {
	if r == nil || len(r.Fallbacks) == 0 {
		return ""
	}
	return "\n\n—— " + r.Notice()
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
