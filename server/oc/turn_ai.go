package oc

// ==================== AI 发言生成（单轮） ====================
// 提示词 = 角色卡（人设/外貌/说话风格）+ 世界书相关条目 + 近期上下文 + 发言策略
// 输出侧独立过审核：角色设定不能成为绕过审核的理由。
// 计费走现有账本（scene=oc_turn），预算三道闸门前置校验。

import (
	"errors"
	"strings"
	"time"

	"nvs-server/aicall"
	"nvs-server/models"
	"nvs-server/security"
)

// StepResult 单轮推进结果
type StepResult struct {
	Turn    *Turn  `json:"turn"`
	Stopped bool   `json:"stopped"`
	Reason  string `json:"reason,omitempty"`
}

// Step 推进一轮（AI 角色发言）。manualOnly=true 时只用于「单步」按钮。
func Step(flowID, operatorID uint) (*StepResult, error) {
	f, err := GetFlow(flowID)
	if err != nil {
		return nil, err
	}
	if f.Status != FlowRunning {
		return nil, errors.New("对话流未在运行")
	}
	if f.TurnCount >= f.MaxTurns {
		_ = SetFlowStatus(f.ID, FlowStopped, "达到最大轮次")
		return &StepResult{Stopped: true, Reason: "达到最大轮次"}, nil
	}
	set := GetGroupSetting(f.GroupID)
	agents, err := ListAgents(f.GroupID)
	if err != nil || len(agents) == 0 {
		return nil, errors.New("群内没有可用的 AI 角色")
	}
	// 纯 AI 群：忽略真人角色；混合群：AI 角色照常轮转
	ag, err := nextSpeaker(f, agents)
	if err != nil {
		return nil, err
	}

	// 模型：角色绑定优先，其次平台第一个可用 chat 模型
	var m *models.AiModel
	if ag.ModelID > 0 {
		if mm, merr := models.GetAiModel(ag.ModelID); merr == nil && mm != nil && mm.Enabled && mm.Kind == models.AiKindChat {
			m = mm
		}
	}
	prompt, ctx, perr := buildPrompt(f, ag)
	if perr != nil {
		return nil, perr
	}

	// 预算闸门（用首选模型估算；降级后实际成本会略差，记账按真实值累加）
	estForBudget := int(models.EstimateAiCost(m, models.EstimateTokens(prompt)+512, m.MaxTokens))
	if berr := CheckBudget(f.GroupID, f.ID, ag.ID, estForBudget); berr != nil {
		_ = SetFlowStatus(f.ID, FlowPaused, "预算上限")
		return &StepResult{Stopped: true, Reason: berr.Error()}, nil
	}

	// 计费归属：流发起人（导演）承担
	payer := f.StarterID
	if payer == 0 {
		payer = operatorID
	}

	// 统一调用入口：路由策略（最便宜/最快/场景）+ 失败自动降级（降级事实必须提示）
	res, aerr := aicall.Complete(aicall.Request{
		PayerID: payer, Scene: "oc_turn", Action: "oc_turn",
		RefType: "oc_flow", RefID: f.ID,
		System: prompt, User: ctx,
		MaxTokens: m.MaxTokens, Temperature: 0.8,
		PreferModel: ag.ModelID,
	})
	if aerr != nil {
		return nil, errors.New("模型调用失败：" + aerr.Error())
	}
	content := res.Content
	realIn, realOut, cost := res.TokensIn, res.TokensOut, res.Cost
	m = &models.AiModel{ID: res.ModelID, Name: res.ModelName}
	// 降级提示：贴进正文尾部（不静默换模型），并单独存字段供前端标黄
	degradeNote := res.Notice()

	// 输出侧审核（角色设定不能成为绕过审核的理由）
	cleaned, hits := security.ApplyBadWords(content, "oc")
	if len([]rune(cleaned)) > MaxTurnOutput {
		cleaned = string([]rune(cleaned)[:MaxTurnOutput])
	}
	// 降级提示：写进正文尾部（用户必须能看见换了模型）
	if degradeNote != "" {
		cleaned += "\n\n—— " + degradeNote
	}
	// 情绪标记：剥离出来单独存，用于立绘/气泡表情与状态养成
	emotion, cleaned := ParseEmotion(cleaned)
	_ = ApplyTurnToState(ag.CharacterID, emotion, f.TurnCount > 0)
	set2 := set
	review := ReviewApproved
	if len(hits) > 0 {
		review = ReviewPending // 命中待复核（先发后审模式下由窗口统一处理）
	}

	seq := f.TurnCount + 1
	t := &Turn{
		FlowID: f.ID, GroupID: f.GroupID, Seq: seq,
		CharacterID: ag.CharacterID, AgentID: ag.ID,
		Content: cleaned, ModelID: m.ID, ModelName: m.Name,
		Emotion:  emotion,
		TokensIn: realIn, TokensOut: realOut, Cost: int(cost),
		Review: review, CreatedAt: time.Now(),
	}
	if err := models.DB.Create(t).Error; err != nil {
		return nil, err
	}
	_ = models.DB.Model(&Flow{}).Where("id = ?", f.ID).
		Updates(map[string]interface{}{"turn_count": seq, "updated_at": time.Now()}).Error
	AddUsage(f.GroupID, int(cost))

	// 严格模式 + 先审后发：命中即停流等待复核
	if set2.AuditLevel == AuditStrict && set2.AuditTiming == AuditPre && len(hits) > 0 {
		_ = SetFlowStatus(f.ID, FlowPaused, "命中审核，等待复核")
	}

	// 多条一起审：窗口累积到阈值触发
	maybeAuditWindow(f.ID)

	t.CharacterName = ag.CharacterName
	t.IsAI = true
	return &StepResult{Turn: t, Stopped: false}, nil
}

// buildPrompt 拼装提示词：角色卡 + 世界书相关条目 + 上下文 + 策略
func buildPrompt(f *Flow, ag *Agent) (string, string, error) {
	var c Character
	if err := models.DB.Where("id = ?", ag.CharacterID).First(&c).Error; err != nil {
		return "", "", errors.New("角色卡不存在")
	}
	// 上下文：最近 12 轮（同时作为世界书的扫描文本）
	recent, _ := ListTurns(f.ID, 0, 12)
	var ctx strings.Builder
	ctx.WriteString("【对话上下文】\n")
	for _, t := range recent {
		name := t.CharacterName
		if name == "" {
			if t.UserID > 0 {
				name = "真人"
			} else {
				name = "角色"
			}
		}
		ctx.WriteString(name + "：" + t.Content + "\n")
	}

	// 世界书：按键触发注入（酒馆 Lorebook 玩法，避免整本塞上下文）
	bookID := f.BookID
	if bookID == 0 {
		bookID = c.BookID // 角色自带世界书（character_book）
	}
	before, after, inChat := "", "", ""
	if bookID > 0 {
		var book WorldBook
		if err := models.DB.Where("id = ?", bookID).First(&book).Error; err == nil {
			entries, _ := ListEntries(bookID)
			act := Activate(&book, entries, recent)
			before, after, inChat = RenderLore(act)
		}
	}

	otherName := "对方"
	for i := len(recent) - 1; i >= 0; i-- {
		if recent[i].CharacterID != ag.CharacterID && recent[i].CharacterName != "" {
			otherName = recent[i].CharacterName
			break
		}
	}
	macro := func(s string) string { return ExpandMacros(s, c.Name, otherName) }

	var b strings.Builder
	b.WriteString("你在参与一场角色扮演对话。严格以角色身份发言，只输出该角色这一轮的台词与动作，不要解释、不要加旁白标签。\n\n")
	if c.SystemPrompt != "" {
		b.WriteString(macro(c.SystemPrompt) + "\n\n")
	}
	// 世界书：角色定义之前的条目
	if before != "" {
		b.WriteString("【世界设定·前置】\n" + macro(before) + "\n")
	}
	b.WriteString("【你的角色】" + c.Name + "\n")
	if c.Persona != "" {
		b.WriteString("人设与性格：" + macro(c.Persona) + "\n")
	}
	if c.Appearance != "" {
		b.WriteString("外貌：" + macro(c.Appearance) + "\n")
	}
	if c.Speech != "" {
		b.WriteString("说话风格（必须遵守）：" + macro(c.Speech) + "\n")
	}
	if c.Scenario != "" {
		b.WriteString("当前场景：" + macro(c.Scenario) + "\n")
	}
	// 世界书：角色定义之后的条目
	if after != "" {
		b.WriteString("\n【世界设定】\n" + macro(after))
	}
	// 对话示例（mes_example）：最强的风格锚点
	if c.MesExample != "" {
		b.WriteString("\n【对话示例（照此风格，不要照抄内容）】\n" + macro(c.MesExample) + "\n")
	}
	if ag.Strategy != "" {
		b.WriteString("\n【发言策略】" + macro(ag.Strategy) + "\n")
	}
	if c.PostHistory != "" {
		b.WriteString("\n【补充约束】" + macro(c.PostHistory) + "\n")
	}
	// 小团子式状态：让状态影响语气
	stateDesc := describeState(c)
	if stateDesc != "" {
		b.WriteString("\n【当前状态】" + stateDesc + "\n")
	}
	b.WriteString("\n【硬性约束】不得输出违规、暴力、色情、政治敏感内容；不替其他角色发言；长度控制在 200 字内。")
	b.WriteString("\n【输出格式】第一行用 [情绪:词] 标注你这一句的情绪（如 [情绪:笑]），换行后接台词。")

	ctx.WriteString("\n【当前世界提示】\n" + macro(inChat))
	ctx.WriteString("\n请以上下文为基础，以「" + c.Name + "」的身份接着说这一句。")
	return b.String(), ctx.String(), nil
}

// describeState 把角色状态数值转成自然语言（小团子式养成：互动改变状态）
func describeState(c Character) string {
	parts := make([]string, 0, 3)
	switch {
	case c.Mood >= 80:
		parts = append(parts, "心情很好")
	case c.Mood >= 60:
		parts = append(parts, "心情不错")
	case c.Mood >= 40:
		parts = append(parts, "心情平静")
	case c.Mood >= 20:
		parts = append(parts, "心情低落")
	default:
		parts = append(parts, "心情很差")
	}
	switch {
	case c.Energy >= 80:
		parts = append(parts, "精力充沛")
	case c.Energy >= 40:
		parts = append(parts, "状态尚可")
	default:
		parts = append(parts, "疲惫")
	}
	if c.Affinity >= 70 {
		parts = append(parts, "对对方很亲近")
	} else if c.Affinity <= 20 {
		parts = append(parts, "对对方保持距离")
	}
	return strings.Join(parts, "，")
}
