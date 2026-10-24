package models

// ==================== AI 路由策略（gap-audit #10） ====================
// 解决「永远只用列表第一个模型」的问题：
//   manual   —— 用户/角色指定（现在的行为，默认值，保证不改变既有语义）
//   cheapest —— 按 (输入价 + 输出价) 升序，免费模型优先
//   fastest  —— 按近 7 天实测平均延迟升序（无数据退化为价格升序）
//   scene    —— 按场景覆盖：如 oc_turn 走 cheapest、ai_hub 走 fastest
//
// 合规开关：PreferDomestic（境内厂商优先）。它只影响排序，不硬性排除境外模型。

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
)

const aiRouteConfigKey = "ai_route_config"

// 路由策略
const (
	RouteManual   = "manual"
	RouteCheapest = "cheapest"
	RouteFastest  = "fastest"
	RouteScene    = "scene"
)

// AiRouteConfig 路由配置
type AiRouteConfig struct {
	Strategy       string            `json:"strategy"`        // manual / cheapest / fastest / scene
	SceneStrategy  map[string]string `json:"scene_strategy"`  // 场景 → 策略（仅 Strategy=scene 时生效）
	PreferDomestic bool              `json:"prefer_domestic"` // 境内厂商优先
	Fallback       bool              `json:"fallback"`        // 失败是否自动切备用模型（必须提示，不静默）
	MaxFallback    int               `json:"max_fallback"`    // 最多切换几次（1~3）
}

// DefaultAiRouteConfig 默认：保持既有行为（不自动改用户的模型选择）
func DefaultAiRouteConfig() AiRouteConfig {
	return AiRouteConfig{
		Strategy: RouteManual,
		SceneStrategy: map[string]string{
			"oc_turn":    RouteCheapest,
			"group_bot":  RouteCheapest,
			"ai_hub":     RouteFastest,
			"oc_publish": RouteCheapest,
		},
		PreferDomestic: false,
		Fallback:       true,
		MaxFallback:    1,
	}
}

// NormalizeAiRouteConfig 归一化（非法值回落默认，防止配置写坏后全线异常）
func NormalizeAiRouteConfig(in AiRouteConfig) AiRouteConfig {
	def := DefaultAiRouteConfig()
	switch in.Strategy {
	case RouteManual, RouteCheapest, RouteFastest, RouteScene:
	default:
		in.Strategy = def.Strategy
	}
	if in.SceneStrategy == nil {
		in.SceneStrategy = def.SceneStrategy
	} else {
		// 场景策略只接受合法值，其余丢弃
		clean := map[string]string{}
		for k, v := range in.SceneStrategy {
			switch v {
			case RouteManual, RouteCheapest, RouteFastest:
				if strings.TrimSpace(k) != "" {
					clean[k] = v
				}
			}
		}
		in.SceneStrategy = clean
	}
	if in.MaxFallback <= 0 {
		in.MaxFallback = 1
	}
	if in.MaxFallback > 3 {
		in.MaxFallback = 3
	}
	return in
}

// LoadAiRouteConfig 读取配置
func LoadAiRouteConfig() AiRouteConfig {
	raw := GetPlatformConfig(aiRouteConfigKey)
	cfg := DefaultAiRouteConfig()
	if strings.TrimSpace(raw) == "" {
		return cfg
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return cfg
	}
	return NormalizeAiRouteConfig(cfg)
}

// SaveAiRouteConfig 保存配置
func SaveAiRouteConfig(cfg AiRouteConfig) error {
	cfg = NormalizeAiRouteConfig(cfg)
	b, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return SetPlatformConfig(aiRouteConfigKey, string(b))
}

// EffectiveStrategy 场景实际生效的策略
func EffectiveStrategy(cfg AiRouteConfig, scene string) string {
	if cfg.Strategy != RouteScene {
		return cfg.Strategy
	}
	if s, ok := cfg.SceneStrategy[scene]; ok && s != "" {
		return s
	}
	return RouteCheapest // scene 模式下未配置的场景按最便宜
}

// avgLatency7d 近 7 天平均延迟（毫秒；无样本返回 -1）
func avgLatency7d(modelID uint) int {
	type row struct {
		Avg float64
		N   int
	}
	var r row
	DB.Model(&AiCallLog{}).
		Select("COALESCE(AVG(latency_ms),0) as avg, COUNT(*) as n").
		Where("model_id = ? AND latency_ms > 0 AND created_at >= ?", modelID, time.Now().AddDate(0, 0, -7)).
		Scan(&r)
	if r.N == 0 {
		return -1
	}
	return int(r.Avg)
}

// rankChatModels 按策略排序候选（已启用、chat 类型）
func rankChatModels(strategy string, preferDomestic bool) []AiModel {
	list, err := ListAiModelsByKind(AiKindChat, true)
	if err != nil || len(list) == 0 {
		return nil
	}
	out := make([]AiModel, 0, len(list))
	for _, m := range list {
		if m.Enabled && m.Kind == AiKindChat {
			out = append(out, m)
		}
	}
	latency := map[uint]int{}
	if strategy == RouteFastest {
		for _, m := range out {
			latency[m.ID] = avgLatency7d(m.ID)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if preferDomestic {
			ad, bd := a.ProviderHint == "domestic", b.ProviderHint == "domestic"
			if ad != bd {
				return ad // 境内优先
			}
		}
		switch strategy {
		case RouteCheapest:
			pa, pb := a.PriceIn+a.PriceOut, b.PriceIn+b.PriceOut
			if pa != pb {
				return pa < pb
			}
		case RouteFastest:
			la, lb := latency[a.ID], latency[b.ID]
			// 无样本(-1)排在最后；都有样本时快的优先
			if (la < 0) != (lb < 0) {
				return lb < 0
			}
			if la >= 0 && lb >= 0 && la != lb {
				return la < lb
			}
			pa, pb := a.PriceIn+a.PriceOut, b.PriceIn+b.PriceOut
			if pa != pb {
				return pa < pb
			}
		default: // manual
			if a.Priority != b.Priority {
				return a.Priority > b.Priority
			}
			pa, pb := a.PriceIn+a.PriceOut, b.PriceIn+b.PriceOut
			if pa != pb {
				return pa < pb
			}
		}
		return a.ID < b.ID
	})
	return out
}

// ErrNoChatModel 没有可用对话模型
var ErrNoChatModel = errors.New("没有可用的对话模型")

// PickChatModel 选模型：用户/角色指定优先，否则按策略排序取第一个
func PickChatModel(userChoice uint, scene string) (*AiModel, error) {
	if userChoice > 0 {
		if m, err := GetAiModel(userChoice); err == nil && m != nil && m.Enabled && m.Kind == AiKindChat {
			return m, nil
		}
	}
	cfg := LoadAiRouteConfig()
	strategy := EffectiveStrategy(cfg, scene)
	list := rankChatModels(strategy, cfg.PreferDomestic)
	if len(list) == 0 {
		return nil, ErrNoChatModel
	}
	return &list[0], nil
}

// FallbackChatModels 降级候选（排除已试过的模型），上限由配置决定
func FallbackChatModels(tried []uint, scene string, limit int) []AiModel {
	if limit <= 0 {
		limit = 1
	}
	cfg := LoadAiRouteConfig()
	if !cfg.Fallback {
		return nil
	}
	if limit > cfg.MaxFallback {
		limit = cfg.MaxFallback
	}
	strategy := EffectiveStrategy(cfg, scene)
	all := rankChatModels(strategy, cfg.PreferDomestic)
	skip := map[uint]bool{}
	for _, id := range tried {
		skip[id] = true
	}
	out := make([]AiModel, 0, limit)
	for _, m := range all {
		if skip[m.ID] {
			continue
		}
		out = append(out, m)
		if len(out) >= limit {
			break
		}
	}
	return out
}
