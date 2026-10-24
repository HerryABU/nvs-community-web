package models

// ==================== 三维交叉限流（gap-audit #15）====================
// 已有的限流维度：用户级速率（api_key_quota）+ 每日调用/代币限额 + 单模型密钥轮询。
// 缺的是「人 × 模型 × 时间窗」这一层：一个人狂刷同一个贵模型，把全站预算和这个模型的密钥
// 同时打爆，而单纯的人级速率又拦不住「换着模型刷同一家」的变体。
//
// 三维分别管什么：
//   - 用户 × 模型 × 分钟：挡住「一个人死磕一个模型」（最常见：把贵模型当免费额度刷）
//   - 用户 × 分钟：挡住「一个人换模型刷」
//   - 模型 × 分钟：挡住「一群人同时打一个模型」（这个模型会被上游限流，连坐其他人）
//
// 实现：进程内滑动窗口计数（分桶到秒），单实例足够；多实例部署时每实例各限一份，
// 属「近似限流」——这点必须在后台说明里写清楚，不能让人以为是集群级精确限流。

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// AiThrottleConfig 三维限流配置（platform_configs.ai_throttle）
type AiThrottleConfig struct {
	Enabled bool `json:"enabled"`
	// 三个维度，0 = 不限
	PerUserPerModelPerMin int `json:"per_user_per_model_per_min"`
	PerUserPerMin         int `json:"per_user_per_min"`
	PerModelPerMin        int `json:"per_model_per_min"`
	// 超限行为：reject（直接拒绝） / queue（不排队，只返回提示让前端退避）
	OnExceed  string    `json:"on_exceed"`
	UpdatedAt time.Time `json:"updated_at"`
}

const aiThrottleKey = "ai_throttle"

// DefaultAiThrottleConfig 默认：单模型 20/分、单用户 40/分、单模型全站 200/分
func DefaultAiThrottleConfig() AiThrottleConfig {
	return AiThrottleConfig{
		Enabled:               true,
		PerUserPerModelPerMin: 20,
		PerUserPerMin:         40,
		PerModelPerMin:        200,
		OnExceed:              "reject",
	}
}

// LoadAiThrottleConfig 读取配置
func LoadAiThrottleConfig() AiThrottleConfig {
	raw := GetPlatformConfig(aiThrottleKey)
	c := DefaultAiThrottleConfig()
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			return DefaultAiThrottleConfig()
		}
	}
	if c.OnExceed != "queue" {
		c.OnExceed = "reject"
	}
	if c.PerUserPerModelPerMin < 0 {
		c.PerUserPerModelPerMin = 0
	}
	if c.PerUserPerMin < 0 {
		c.PerUserPerMin = 0
	}
	if c.PerModelPerMin < 0 {
		c.PerModelPerMin = 0
	}
	return c
}

// SaveAiThrottleConfig 保存配置
func SaveAiThrottleConfig(c AiThrottleConfig) error {
	if c.OnExceed != "queue" {
		c.OnExceed = "reject"
	}
	c.UpdatedAt = time.Now()
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return SetPlatformConfig(aiThrottleKey, string(b))
}

// ─────────────── 滑动窗口 ───────────────

type windowCounter struct {
	mu     sync.Mutex
	bucket map[string]int // "key|epochSecond" → 次数
	lastGC time.Time
}

var throttleCounter = &windowCounter{bucket: map[string]int{}, lastGC: time.Now()}

func (w *windowCounter) incr(key string, now time.Time) int {
	sec := now.Unix()
	w.mu.Lock()
	defer w.mu.Unlock()
	k := fmt.Sprintf("%s|%d", key, sec)
	w.bucket[k]++
	// 每 10 秒清一次过期桶（窗口 60 秒，留 2 分钟余量）
	if now.Sub(w.lastGC) > 10*time.Second {
		cut := now.Add(-2 * time.Minute).Unix()
		for kk := range w.bucket {
			var s int64
			if i := strings.LastIndex(kk, "|"); i >= 0 {
				fmt.Sscanf(kk[i+1:], "%d", &s)
				if s < cut {
					delete(w.bucket, kk)
				}
			}
		}
		w.lastGC = now
	}
	total := 0
	for kk, v := range w.bucket {
		if strings.HasPrefix(kk, key+"|") {
			total += v
		}
	}
	return total
}

// ThrottleError 超限错误（带维度与建议退避秒数）
type ThrottleError struct {
	Dimension string
	Limit     int
	RetrySec  int
}

func (e *ThrottleError) Error() string {
	return fmt.Sprintf("调用过于频繁（%s 维度每分钟上限 %d 次），请 %d 秒后再试",
		e.Dimension, e.Limit, e.RetrySec)
}

// CheckAiThrottle 三维检查（命中即自增，返回错误表示应当拒绝）。
// 注意：命中判定用「当前窗口已有次数 + 1 > 上限」，即第 limit+1 次被拒。
func CheckAiThrottle(userID, modelID uint) error {
	cfg := LoadAiThrottleConfig()
	if !cfg.Enabled {
		return nil
	}
	now := time.Now()
	// 1) 用户 × 模型
	if cfg.PerUserPerModelPerMin > 0 {
		key := fmt.Sprintf("um:%d:%d", userID, modelID)
		if throttleCounter.incr(key, now) > cfg.PerUserPerModelPerMin {
			return &ThrottleError{Dimension: "用户×模型", Limit: cfg.PerUserPerModelPerMin, RetrySec: 60}
		}
	} else {
		throttleCounter.incr(fmt.Sprintf("um:%d:%d", userID, modelID), now)
	}
	// 2) 用户
	if cfg.PerUserPerMin > 0 {
		if throttleCounter.incr(fmt.Sprintf("u:%d", userID), now) > cfg.PerUserPerMin {
			return &ThrottleError{Dimension: "用户", Limit: cfg.PerUserPerMin, RetrySec: 60}
		}
	}
	// 3) 模型（全站）
	if cfg.PerModelPerMin > 0 {
		if throttleCounter.incr(fmt.Sprintf("m:%d", modelID), now) > cfg.PerModelPerMin {
			return &ThrottleError{Dimension: "模型", Limit: cfg.PerModelPerMin, RetrySec: 30}
		}
	}
	return nil
}

// AiThrottleSnapshot 当前窗口计数（后台展示「现在谁在刷」）
func AiThrottleSnapshot(topN int) []map[string]interface{} {
	if topN <= 0 {
		topN = 10
	}
	throttleCounter.mu.Lock()
	defer throttleCounter.mu.Unlock()
	agg := map[string]int{}
	now := time.Now().Unix()
	for k, v := range throttleCounter.bucket {
		i := strings.LastIndex(k, "|")
		if i < 0 {
			continue
		}
		var sec int64
		fmt.Sscanf(k[i+1:], "%d", &sec)
		if now-sec > 60 {
			continue
		}
		agg[k[:i]] += v
	}
	out := make([]map[string]interface{}, 0, len(agg))
	for k, v := range agg {
		dim := "unknown"
		switch {
		case strings.HasPrefix(k, "um:"):
			dim = "user_model"
		case strings.HasPrefix(k, "u:"):
			dim = "user"
		case strings.HasPrefix(k, "m:"):
			dim = "model"
		}
		out = append(out, map[string]interface{}{"key": k, "dimension": dim, "count_1min": v})
	}
	// 简单排序（次数降序）
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j]["count_1min"].(int) > out[i]["count_1min"].(int) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	if len(out) > topN {
		out = out[:topN]
	}
	return out
}

// ResetAiThrottle 测试用
func ResetAiThrottle() {
	throttleCounter.mu.Lock()
	throttleCounter.bucket = map[string]int{}
	throttleCounter.lastGC = time.Now()
	throttleCounter.mu.Unlock()
}
