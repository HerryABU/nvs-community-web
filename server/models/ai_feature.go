package models

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// ==================== AI 功能开关（平台级） ====================
// 单独一套 KV：与「免费额度」「评论机器人」解耦，站长可逐项放开/收紧。

// AiFeatureFlags AI 功能开关
type AiFeatureFlags struct {
	AllowByok      bool `json:"allow_byok"`       // 允许用户自备密钥
	AllowI2I       bool `json:"allow_i2i"`        // 允许混图（参考图再创作）
	AllowFaceSwap  bool `json:"allow_faceswap"`   // 允许换脸
	ContextMsgs    int  `json:"context_msgs"`     // 对话带多少条历史（默认 20，上限 60）
	DailyChatLimit int  `json:"daily_chat_limit"` // 每人每日对话次数（-1 不限 / 0 禁止 / N 次）
	PublicGallery  bool `json:"public_gallery"`   // 灵感广场是否公开可浏览（关闭则仅登录可见）
	// MaxCostPerCall 单次调用代币上限（0 = 不限制）；超限直接拒绝，防一次点击烧穿余额
	MaxCostPerCall int64 `json:"max_cost_per_call"`
	// ConfirmThreshold 单次预估达到该代币数时要求二次确认（0 = 不要求）
	ConfirmThreshold int64 `json:"confirm_threshold"`
}

const aiFeatureFlagsKey = "ai_feature_flags"

// DefaultAiFeatureFlags 默认：全部放开、每日不限
func DefaultAiFeatureFlags() AiFeatureFlags {
	return AiFeatureFlags{
		AllowByok: true, AllowI2I: true, AllowFaceSwap: false,
		ContextMsgs: 20, DailyChatLimit: -1, PublicGallery: true,
		MaxCostPerCall: 0, ConfirmThreshold: 0,
	}
}

// GetAiFeatureFlags 读取开关（未配置返回默认值；脏值逐项兜底）
func GetAiFeatureFlags() AiFeatureFlags {
	out := DefaultAiFeatureFlags()
	raw := GetPlatformConfig(aiFeatureFlagsKey)
	if strings.TrimSpace(raw) == "" {
		return out
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return DefaultAiFeatureFlags()
	}
	if out.ContextMsgs <= 0 {
		out.ContextMsgs = 20
	}
	if out.ContextMsgs > 60 {
		out.ContextMsgs = 60
	}
	if out.DailyChatLimit < -1 {
		out.DailyChatLimit = -1
	}
	if out.MaxCostPerCall < 0 {
		out.MaxCostPerCall = 0
	}
	if out.ConfirmThreshold < 0 {
		out.ConfirmThreshold = 0
	}
	return out
}

// SetAiFeatureFlags 保存开关（范围校正后落库）
func SetAiFeatureFlags(f AiFeatureFlags) error {
	if f.ContextMsgs <= 0 {
		f.ContextMsgs = 20
	}
	if f.ContextMsgs > 60 {
		f.ContextMsgs = 60
	}
	if f.DailyChatLimit < -1 {
		f.DailyChatLimit = -1
	}
	if f.MaxCostPerCall < 0 {
		f.MaxCostPerCall = 0
	}
	if f.ConfirmThreshold < 0 {
		f.ConfirmThreshold = 0
	}
	b, err := json.Marshal(f)
	if err != nil {
		return err
	}
	return SetPlatformConfig(aiFeatureFlagsKey, string(b))
}

// CountTodayAiChats 今日对话次数（含免费与付费；按 AiCallLog 统计）
func CountTodayAiChats(userID uint) int64 {
	if userID == 0 {
		return 0
	}
	// 当日 0 点（本地时区）
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var n int64
	DB.Model(&AiCallLog{}).
		Where("user_id = ? AND action = ? AND created_at >= ?", userID, "chat", start).
		Count(&n)
	return n
}

// CheckAiChatDailyLimit 校验每日对话上限，返回 (是否允许, 中文说明)
func CheckAiChatDailyLimit(userID uint) (bool, string) {
	f := GetAiFeatureFlags()
	if f.DailyChatLimit < 0 {
		return true, ""
	}
	if f.DailyChatLimit == 0 {
		return false, "站长已关闭 AI 对话功能"
	}
	if CountTodayAiChats(userID) >= int64(f.DailyChatLimit) {
		return false, "今日对话次数已用完（上限 " + strconv.Itoa(f.DailyChatLimit) + " 次），明日自动恢复"
	}
	return true, ""
}

// CallGuard 单次调用资金闸门（对话与按次媒体共用）
// 返回：allowed=false 时带拒绝原因；needConfirm=true 表示需前端二次确认后带 confirm=true 重试。
func CallGuard(cost int64, confirmed bool) (allowed bool, needConfirm bool, reason string) {
	f := GetAiFeatureFlags()
	if f.MaxCostPerCall > 0 && cost > f.MaxCostPerCall {
		return false, false, "单次调用需 " + strconv.FormatInt(cost, 10) +
			" " + GetTokenName() + "，超出站长设定的单次上限 " + strconv.FormatInt(f.MaxCostPerCall, 10)
	}
	if f.ConfirmThreshold > 0 && cost >= f.ConfirmThreshold && !confirmed {
		return true, true, "本次预计消耗 " + strconv.FormatInt(cost, 10) + " " + GetTokenName()
	}
	return true, false, ""
}
