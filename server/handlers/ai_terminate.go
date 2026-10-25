package handlers

import (
	"nvs-server/aisafety"
	"nvs-server/models"
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
func applyTermination(session *models.AiChatSession, cfg *aisafety.SafetyConfig, reason string) terminateOutcome {
	mode := aisafety.NormalizeMode("")
	if cfg != nil {
		mode = aisafety.NormalizeMode(cfg.TerminateMode)
	}
	switch mode {
	case aisafety.TerminateModerate:
		_ = models.MarkSessionContaminated(session.ID)
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
	default:
		_ = models.LockAiChatSession(session.ID, reason, 0)
		return terminateOutcome{
			Text:       aisafety.TerminatedText(cfg),
			Terminated: true,
			Mode:       aisafety.TerminateStrict,
		}
	}
}
