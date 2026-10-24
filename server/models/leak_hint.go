package models

import (
	"encoding/json"
	"strings"
	"time"
)

// ==================== 个人信息泄漏提示统计 ====================
// 隐私红线：只累计「命中类型的次数」，不记录任何用户输入内容、不关联用户 ID。
// 数据落 platform_config KV（leak_hint_stats），便于站长观察误报/风险趋势。

const leakHintStatsKey = "leak_hint_stats"

// LeakHintStats 统计结构
type LeakHintStats struct {
	Total map[string]int64 `json:"total"` // 各类型累计
	Daily map[string]int64 `json:"daily"` // 按天累计（保留最近 30 天）
}

// 允许的类型白名单（与前端 LeakType 对齐）
var allowedLeakTypes = map[string]bool{
	"phone": true, "email": true, "idcard": true,
	"apikey": true, "bankcard": true, "semantic": true,
}

// RecordLeakHint 累计命中类型（幂等累加，不落内容）
func RecordLeakHint(types []string) error {
	if len(types) == 0 {
		return nil
	}
	stats := GetLeakHintStats()
	if stats.Total == nil {
		stats.Total = map[string]int64{}
	}
	if stats.Daily == nil {
		stats.Daily = map[string]int64{}
	}
	day := time.Now().Format("2006-01-02")
	n := int64(0)
	for _, t := range types {
		t = strings.ToLower(strings.TrimSpace(t))
		if !allowedLeakTypes[t] {
			continue
		}
		stats.Total[t]++
		n++
	}
	if n == 0 {
		return nil
	}
	stats.Daily[day] += n
	// 只保留最近 30 天
	if len(stats.Daily) > 30 {
		cut := time.Now().AddDate(0, 0, -30).Format("2006-01-02")
		for d := range stats.Daily {
			if d < cut {
				delete(stats.Daily, d)
			}
		}
	}
	b, err := json.Marshal(stats)
	if err != nil {
		return err
	}
	return SetPlatformConfig(leakHintStatsKey, string(b))
}

// GetLeakHintStats 读取统计（未初始化返回空结构）
func GetLeakHintStats() LeakHintStats {
	out := LeakHintStats{Total: map[string]int64{}, Daily: map[string]int64{}}
	raw := GetPlatformConfig(leakHintStatsKey)
	if strings.TrimSpace(raw) == "" {
		return out
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return LeakHintStats{Total: map[string]int64{}, Daily: map[string]int64{}}
	}
	if out.Total == nil {
		out.Total = map[string]int64{}
	}
	if out.Daily == nil {
		out.Daily = map[string]int64{}
	}
	return out
}
