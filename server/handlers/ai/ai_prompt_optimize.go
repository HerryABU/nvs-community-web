package ai

// ==================== AI 绘画：提示词优化 ====================
//
// 为什么需要这个：
// 绘画模型对提示词的「理解度」高度依赖描述质量。同一个画面，
// 写得越具体（主体 / 动作 / 环境 / 光线 / 镜头 / 风格 / 画质）出图越稳。
// 但普通用户往往只会写「一个女孩」—— 生成出来的东西随机性极大。
//
// 优化做的事：把用户的原意**展开**成一条信息密度更高的提示词，
// 而不是替用户改创作意图（不擅自改主体、不加不该加的东西）。
//
// 三条边界（很重要，否则这个功能会变成「AI 擅自改你的需求」）：
//  1. 只做「补全与具体化」，不改主体、不改风格方向；
//  2. 不删用户已有的限定词（用户写了「赛博朋克」就不能被优化掉）；
//  3. 不生成任何违规内容 —— 优化后仍要过提示词初审。

import (
	"strings"
	"time"

	"nvs-server/aiclient"
	"nvs-server/aisafety"
	"nvs-server/handlers/hctx"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	aim "nvs-server/models/ai"
	aihub "nvs-server/models/aihub"
)

// promptOptimizeSystem 提示词优化的系统提示词。
//
// 刻意强调「不要改用户意图」：用户最反感的是按下优化键之后，
// 自己的创意被换成另一个东西。
const promptOptimizeSystem = `你是 AI 绘画提示词优化助手。用户给出一段简短的画面描述，你要把它扩写成一条信息密度更高、更容易出好图的提示词。

【铁律】
1. **绝不改变用户的创作意图**：主体、场景、风格方向、情绪基调必须与原文一致。
2. **绝不删除用户已有的限定词**：用户写了「赛博朋克」「水墨」「雨天」，优化后必须保留。
3. **只做补全与具体化**：补上用户没写但属于同一画面的细节（光线、镜头、材质、氛围、构图）。
4. **不添加冲突元素**：原文是「安静的森林」就不要加「热闹的集市」。
5. **不写负面词**：负面提示词由界面上的独立输入框承担，你只输出正向描述。

【输出】
只输出优化后的提示词正文，一行写完，用英文或中文跟随原文语言。
不要任何解释、前言、引号或 Markdown 标记。`

// AiOptimizePrompt POST /api/ai/image/optimize-prompt
//
// 入参：{ prompt: string, model_id?: number }
// 出参：{ prompt: string }
func AiOptimizePrompt(c *gin.Context) {
	if !aim.IsAiEnabled() {
		utils.BadRequest(c, "AI 功能未开启")
		return
	}
	userID := hctx.CurrentUserID(c)

	var req struct {
		Prompt  string `json:"prompt"`
		ModelID uint   `json:"model_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	original := strings.TrimSpace(req.Prompt)
	if original == "" {
		utils.BadRequest(c, "请先输入提示词")
		return
	}
	if len([]rune(original)) > 2000 {
		utils.BadRequest(c, "提示词过长（上限 2000 字）")
		return
	}

	// 优化前先过一遍初审：不让「优化」成为绕过提示词审核的旁路
	// （用户可以先写违规提示词，再让 AI 把它「优化得更详细」）。
	if safetyCfg := aisafety.GetConfigOrNil(); safetyCfg != nil && safetyCfg.Enabled && safetyCfg.ImageGenGuard {
		if blocked, reason, hits := aisafety.JudgePromptForDrawing(original); blocked {
			username := ""
			if u, e := models.GetUserByID(userID); e == nil && u != nil {
				username = u.Username
			}
			aisafety.RecordDrawingPromptBlock(safetyCfg, userID, username, hits, reason)
			utils.Forbidden(c, reason)
			return
		}
	}

	// 选模型：优先用用户指定的；没指定就自动挑一个启用的 chat 模型
	m, err := aim.GetEnabledAiModelOfKind(req.ModelID, aim.AiKindChat)
	if err != nil || m == nil {
		utils.BadRequest(c, "没有可用的对话模型，无法优化提示词")
		return
	}

	lease, kerr := aihub.AcquireModelKey(m)
	apiKey := ""
	if lease != nil {
		apiKey = lease.Key
	}
	if kerr != nil && apiKey == "" {
		utils.InternalError(c, "模型密钥不可用，请联系管理员")
		return
	}
	client := aiclient.NewClient(m.BaseURL, apiKey, 60*time.Second)

	out, _, _, cerr := client.Complete(aiclient.ChatRequest{
		Model: m.ModelID,
		Messages: []aiclient.Message{
			{Role: "system", Content: promptOptimizeSystem},
			{Role: "user", Content: "请优化这条绘画提示词：\n" + original},
		},
		MaxTokens:   800,
		Temperature: 0.7,
	})
	if cerr != nil {
		utils.BadRequest(c, "优化失败："+cerr.Error())
		return
	}

	optimized := cleanOptimizedPrompt(out)
	if optimized == "" {
		utils.BadRequest(c, "优化结果为空，请重试")
		return
	}
	// 优化结果同样过初审：模型可能「贴心地」把违规内容补得更详细
	if safetyCfg := aisafety.GetConfigOrNil(); safetyCfg != nil && safetyCfg.Enabled && safetyCfg.ImageGenGuard {
		if blocked, reason, hits := aisafety.JudgePromptForDrawing(optimized); blocked {
			username := ""
			if u, e := models.GetUserByID(userID); e == nil && u != nil {
				username = u.Username
			}
			aisafety.RecordDrawingPromptBlock(safetyCfg, userID, username, hits, reason)
			utils.Forbidden(c, reason)
			return
		}
	}

	utils.Success(c, gin.H{
		"prompt":   optimized,
		"original": original,
	})
}

// cleanOptimizedPrompt 清理模型输出：去掉模型爱加的前言/引号/Markdown。
//
// 不清理的话，用户会拿到「优化后的提示词：xxx」这种带壳文本，
// 直接粘回输入框会污染下一次生成。
func cleanOptimizedPrompt(s string) string {
	t := strings.TrimSpace(s)
	if t == "" {
		return ""
	}
	// 去掉代码围栏
	if strings.HasPrefix(t, "```") {
		if i := strings.Index(t, "\n"); i >= 0 {
			t = strings.TrimSpace(t[i+1:])
		}
		t = strings.TrimSuffix(strings.TrimSpace(t), "```")
		t = strings.TrimSpace(t)
	}
	// 去掉成对引号（中文引号是「左」「右」两个不同字符，成对处理）
	// ⚠️ 必须按 rune 切片：中文引号是 3 字节 UTF-8，按字节切会把半个字符留下。
	pairs := [][2]string{{`"`, `"`}, {"「", "」"}, {"『", "』"}}
	for _, p := range pairs {
		rs := []rune(t)
		if len(rs) >= 2 && strings.HasPrefix(t, p[0]) && strings.HasSuffix(t, p[1]) {
			t = strings.TrimSpace(string(rs[1 : len(rs)-1]))
			break
		}
	}
	// 去掉常见前缀（模型爱加的说明性开头）
	for _, p := range []string{"优化后的提示词：", "优化后的提示词:", "优化结果：", "优化结果:", "Optimized prompt:", "Optimized Prompt:", "Prompt:"} {
		if strings.HasPrefix(t, p) {
			t = strings.TrimSpace(strings.TrimPrefix(t, p))
			break
		}
	}
	// 去掉首尾的星号（模型有时用 **加粗** 包裹）
	t = strings.TrimSpace(t)
	if len(t) >= 4 && strings.HasPrefix(t, "**") && strings.HasSuffix(t, "**") {
		t = strings.TrimSpace(t[2 : len(t)-2])
	}
	return t
}
