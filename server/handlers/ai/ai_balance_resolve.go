package ai

import (
	"strings"
	"time"

	"nvs-server/aiclient"
	aim "nvs-server/models/ai"
)

// resolveModelBalance 按模型的「余额查询方式」取余额。
//
// 三类渠道逻辑完全不同，必须分开而不是靠域名猜：
//   - deepseek：官方 GET /user/balance，用模型 Key 本身即可
//   - newapi  ：中转站，需要 nap_ 系统访问令牌 + 用户 ID 调 /api/user/self（quota÷500000）
//   - custom  ：站长自己填 URL + 字段路径 + 除数/乘数（陌生渠道兜底）
//   - auto    ：按域名推断；配了账户令牌优先走 newapi
//   - none    ：明确不查（返回原因，前端不显示假数据）
func resolveModelBalance(m *aim.AiModel, key string) aiclient.ChannelBalance {
	provider := strings.ToLower(strings.TrimSpace(m.BalanceProvider))
	acct, _ := m.DecryptAccountToken()

	switch provider {
	case "none":
		return aiclient.ChannelBalance{
			Provider: "none", Supported: false,
			Error: "该模型已设置为「不查询余额」", FetchedAt: time.Now().Format(time.RFC3339),
		}
	case "custom":
		return aiclient.QueryCustomBalance(aiclient.CustomBalanceSpec{
			URL: m.BalanceURL, Field: m.BalanceField,
			Divisor: m.BalanceDivisor, Multiplier: m.BalanceMultiplier,
			Currency: m.BalanceCurrency,
		}, key, m.TokenUserID, 15*time.Second)
	case "deepseek":
		// 官方分支在 QueryChannelBalanceWithUser 内按域名识别；这里强制走官方路径
		return aiclient.QueryChannelBalanceWithUser("https://api.deepseek.com", key, m.ProviderHint, m.TokenUserID)
	case "newapi":
		return queryNewAPIAccountBalance(m, acct)
	}

	// auto / 未配置
	if strings.TrimSpace(acct) != "" {
		return queryNewAPIAccountBalance(m, acct)
	}
	return aiclient.QueryChannelBalanceWithUser(m.BaseURL, key, m.ProviderHint, m.TokenUserID)
}

// queryNewAPIAccountBalance 用系统访问令牌查账户级余额（真·账户剩余）
func queryNewAPIAccountBalance(m *aim.AiModel, acct string) aiclient.ChannelBalance {
	na := aiclient.QueryNewAPIAccount(m.BaseURL, acct, m.TokenUserID, 15*time.Second)
	return aiclient.ChannelBalance{
		Provider: "newapi", TotalRemaining: na.Quota, TotalUsed: na.UsedQuota,
		Currency: na.Currency, Supported: na.OK,
		FetchedAt: time.Now().Format(time.RFC3339), Error: na.Error,
	}
}
