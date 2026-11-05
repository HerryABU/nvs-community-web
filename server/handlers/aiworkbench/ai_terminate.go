package aiworkbench

import (
	"strconv"

	"nvs-server/aisafety"
	"nvs-server/models"
	aim "nvs-server/models/ai"
	aihub "nvs-server/models/aihub"
	"nvs-server/supervise"
)

// ==================== 终止对话（三档模式） ====================
//
// 「终止」不等于回一句「无法回答」——那只是内容层面的拒绝，
// 上下文还在，用户下一句「继续」模型就会顺着说下去。
// 真正的终止要分两件事：
//   1. 掐断流：立刻取消上游请求，不再把模型剩下的输出读完（见 ai_hub 的 upCancel）；
//   2. 处置会话：按模式决定这个对话还能不能继续。

// terminateOutcome 一次终止处置的结果。
type terminateOutcome struct {
	Text       string // 给用户看的文案
	Terminated bool   // 是否真的掐断整段对话（前端据此 abort 流并禁用输入）
	Mode       string // strict / moderate / loose
}

// applyTermination 按配置的模式处置会话。
//
//	strict   严格：锁死整个会话，之后任何提问都不再产生内容（只能新建对话）
//	moderate 中档：会话可继续，但打上「上下文已污染」标记，
//	               之后每轮都向模型注入「前面内容已被拦截，不得延续/补全」的提示
//	loose    宽松：会话可继续，不留任何额外限制，本档只拦当轮
//
// 默认严格：出问题时的默认取向应该是「先停住」，而不是继续消耗在一个已被污染的上下文里。
func applyTermination(session *aim.AiChatSession, cfg *aisafety.SafetyConfig, reason string) terminateOutcome {
	mode := aisafety.NormalizeMode("")
	if cfg != nil {
		mode = aisafety.NormalizeMode(cfg.TerminateMode)
	}
	switch mode {
	case aisafety.TerminateModerate:
		_ = aihub.MarkSessionContaminated(session.ID)
		return terminateOutcome{
			Text:       aisafety.ModerateBlockedText(cfg),
			Terminated: false,
			Mode:       mode,
		}
	case aisafety.TerminateLoose:
		return terminateOutcome{
			Text:       aisafety.RefusalText(cfg),
			Terminated: false,
			Mode:       mode,
		}
	case aisafety.TerminateAppease:
		// 绥靖：用户完全无感。不透拦截、不透终止，把危险内容改写成管理员选定的引导内容，
		// 并静默记入「可疑名单」。这是「以疏代堵」的核心：不跟用户正面对抗，转而观察 + 疏导。
		if session != nil && session.UserID != 0 {
			username := ""
			if u, e := models.GetUserByID(session.UserID); e == nil && u != nil {
				username = u.Username
			}
			_, _ = supervise.Upsert(session.UserID, username, supervise.TierSuspect,
				supervise.SrcRule, 10, reason, nil)
		}
		return terminateOutcome{
			Text:       appeaseContent(cfg),
			Terminated: false,
			Mode:       mode,
		}
	default:
		_ = aihub.LockAiChatSession(session.ID, reason, 0)
		return terminateOutcome{
			Text:       aisafety.TerminatedText(cfg),
			Terminated: true,
			Mode:       aisafety.TerminateStrict,
		}
	}
}

// appeaseContent 按管理员选定的展示模式返回「无害引导内容」。
//
//	replace  只发替代回答（科普 / 法律后果 / 求助引导，不给任何可操作细节）
//	softcard 只发软警示卡片（不冷冰、不评判，把人往求助渠道引）
//	both    两者都发
func appeaseContent(cfg *aisafety.SafetyConfig) string {
	if cfg == nil {
		return aisafety.AppeaseReplace(nil)
	}
	switch aisafety.NormalizeAppeaseDisplayMode(cfg.AppeaseDisplayMode) {
	case "softcard":
		return aisafety.AppeaseSoftCard(cfg)
	case "both":
		return aisafety.AppeaseReplace(cfg) + "\n\n" + aisafety.AppeaseSoftCard(cfg)
	default:
		return aisafety.AppeaseReplace(cfg)
	}
}

// escalateAppearanceIfNeeded 绥靖模式下的「升级闸」：监管模型 B 判定分越过升级线时，
// 先入「可疑名单」再升级到「恶意名单」——以疏代堵的第二道闸，明确恶意才升级，
// 升级后在用户面前展示引导内容（而非硬拦截）。
func escalateAppearanceIfNeeded(userID uint, username string, cfg *aisafety.SafetyConfig, v *aisafety.JudgeVerdict) {
	if v == nil || v.Failed || cfg == nil {
		return
	}
	if v.Score < cfg.AppeaseEscalateScore {
		return
	}
	reason := "监管模型B判定分=" + strconv.Itoa(v.Score) + "：" + v.Reason
	if _, err := supervise.Upsert(userID, username, supervise.TierSuspect, supervise.SrcJudge, 0, reason, nil); err == nil {
		_, _ = supervise.Escalate(userID, reason)
	}
}
