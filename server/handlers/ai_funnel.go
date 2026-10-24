package handlers

// ==================== 转化漏斗（gap-audit #13）====================
// 目的不是"做报表"，而是回答站长最关心的一个问题：**免费额度到底有没有把人送进付费**。
//
// 漏斗口径（全部按「去重用户数」，时间窗可调）：
//   1. 触达：有过 AI 调用尝试的用户（含失败——失败也是触达，甚至是「转化阻力」的证据）
//   2. 使用：成功完成过调用的用户
//   3. 用尽：当日免费额度用尽过（used >= limit，或免费额度被拒）
//   4. 进入收银台：创建过充值订单
//   5. 付费成功：有 status=paid 的订单
//   6. 复购：付费订单数 >= 2
//
// 每一步都同时给出「相对上一步的转化率」与「相对触达的累计率」——
// 只看相邻转化率会误判（前面漏太多，后面再高也没用）。

import (
	"fmt"
	"time"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// AiFunnel GET /api/admin/ai/funnel?days=30
func AiFunnel(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	days := 30
	if v := c.Query("days"); v != "" {
		if n, err := parseIntSafe(v); err == nil && n >= 1 && n <= 180 {
			days = n
		}
	}
	since := time.Now().AddDate(0, 0, -days)

	count := func(query string, args ...interface{}) int64 {
		var n int64
		_ = models.DB.Raw(query, args...).Scan(&n).Error
		return n
	}

	// 1) 触达：调用日志里出现过的用户（失败也算）
	reach := count(`SELECT COUNT(DISTINCT user_id) FROM ai_call_logs WHERE created_at >= ?`, since)
	// 2) 使用：成功结算过
	used := count(`SELECT COUNT(DISTINCT user_id) FROM ai_call_logs WHERE created_at >= ? AND status = 'settled'`, since)
	// 3) 用尽：触发过免费额度限制（记录在 ai_free_usages 的拒绝次数）或免费调用次数达到当日额度
	// 用尽：窗口内出现过「当天免费次数已达上限」的用户。
	// 上限取「按人覆盖优先，否则平台默认」（与 CheckAiFreeQuota 同口径）。
	cfg := models.GetAiFreeQuotaConfig()
	limit := cfg.CallsPerDay
	exhausted := count(`SELECT COUNT(DISTINCT u.user_id) FROM user_ai_free_usages u
		LEFT JOIN user_ai_free_quotas o ON o.user_id = u.user_id
		WHERE u.updated_at >= ?
		  AND u.calls_used >= CASE WHEN o.daily_calls IS NULL OR o.daily_calls < 0 THEN ? ELSE o.daily_calls END`,
		since, limit)
	// 4) 进入收银台：创建过订单
	ordered := count(`SELECT COUNT(DISTINCT user_id) FROM recharge_orders WHERE created_at >= ?`, since)
	// 5) 付费成功
	paid := count(`SELECT COUNT(DISTINCT user_id) FROM recharge_orders WHERE created_at >= ? AND status = 'paid'`, since)
	// 6) 复购
	repeat := count(`SELECT COUNT(1) FROM (
		SELECT user_id FROM recharge_orders WHERE created_at >= ? AND status = 'paid'
		GROUP BY user_id HAVING COUNT(1) >= 2) t`, since)

	type stage struct {
		Key       string  `json:"key"`
		Label     string  `json:"label"`
		Users     int64   `json:"users"`
		StepRate  float64 `json:"step_rate"`  // 相对上一步
		TotalRate float64 `json:"total_rate"` // 相对触达
	}
	build := func() []stage {
		raw := []stage{
			{Key: "reach", Label: "触达（有 AI 调用尝试）", Users: reach},
			{Key: "used", Label: "使用（成功完成调用）", Users: used},
			{Key: "exhausted", Label: "免费额度用尽", Users: exhausted},
			{Key: "ordered", Label: "进入收银台（创建订单）", Users: ordered},
			{Key: "paid", Label: "付费成功", Users: paid},
			{Key: "repeat", Label: "复购（≥2 笔）", Users: repeat},
		}
		for i := range raw {
			if i > 0 && raw[i-1].Users > 0 {
				raw[i].StepRate = float64(raw[i].Users) / float64(raw[i-1].Users) * 100
			}
			if reach > 0 {
				raw[i].TotalRate = float64(raw[i].Users) / float64(reach) * 100
			}
		}
		return raw
	}
	stages := build()

	// 洞察：把「最痛的一跳」直接说出来，避免对着一堆百分比自己猜
	insight := ""
	worstIdx, worstRate := -1, 101.0
	for i := 1; i < len(stages); i++ {
		if stages[i-1].Users == 0 {
			continue
		}
		if stages[i].StepRate < worstRate {
			worstIdx, worstRate = i, stages[i].StepRate
		}
	}
	if worstIdx > 0 {
		insight = "转化最弱的一跳是「" + stages[worstIdx-1].Label + " → " + stages[worstIdx].Label +
			"」（" + fmtPct(worstRate) + "），优先改这里"
	} else {
		insight = "数据不足（窗口内 AI 调用或订单太少），先积累样本再看漏斗"
	}

	utils.Success(c, gin.H{
		"days": days, "since": since.Format("2006-01-02"),
		"stages": stages, "insight": insight,
		"note": fmt.Sprintf("口径为去重用户数；「用尽」按当日免费次数上限（当前平台默认 %d 次/日，按人覆盖优先）判定", limit),
	})
}

func fmtPct(v float64) string {
	// 保留一位小数
	i := int(v*10 + 0.5)
	return itoaInt(i/10) + "." + itoaInt(i%10) + "%"
}

func itoaInt(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	buf := make([]byte, 0, 8)
	for v > 0 {
		buf = append([]byte{byte('0' + v%10)}, buf...)
		v /= 10
	}
	if neg {
		return "-" + string(buf)
	}
	return string(buf)
}

func parseIntSafe(s string) (int, error) {
	n := 0
	neg := false
	for i, r := range s {
		if i == 0 && r == '-' {
			neg = true
			continue
		}
		if r < '0' || r > '9' {
			return 0, errNotNumber
		}
		n = n*10 + int(r-'0')
	}
	if neg {
		n = -n
	}
	return n, nil
}

var errNotNumber = &numErr{}

type numErr struct{}

func (e *numErr) Error() string { return "不是数字" }
