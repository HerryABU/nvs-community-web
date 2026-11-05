package crisis

import (
	"fmt"
	"strings"
	"time"

	"nvs-server/aisafety"
	"nvs-server/models"
	securitym "nvs-server/models/security"
	"nvs-server/security"
	"nvs-server/sentiment"
	"nvs-server/sentiment/store"
)

// HandleMinorCrisis 评论落库后的危机处置入口。
//
// 同步部分只做「黑话命中」——必须立刻处置，用户马上就能拿到热线。
// 语义绝望（「今天天气真好，可惜我再也看不到了」这类没有黑话的求救）
// 交给 AI 异步分析：一次模型调用要几秒，挂在发评论的同步链路上会拖垮交互。
func HandleMinorCrisis(comment *models.Comment, raw string) *models.CommentCrisis {
	if comment == nil {
		return nil
	}
	cfg, _ := aisafety.GetConfig()
	if cfg == nil || !cfg.MinorCrisisEnabled {
		return nil
	}

	t := Target{
		CommentID:     comment.ID,
		UserID:        comment.UserID,
		NovelID:       comment.NovelID,
		BlogID:        comment.BlogID,
		ChapterNumber: comment.ChapterNumber,
		Raw:           raw,
	}

	// 一、黑话命中：立即处置（用户马上拿到热线与抚慰私信）
	if hits := DetectSelfHarmSlang(raw, "comment"); len(hits) > 0 {
		reason := "命中内置自残/抑郁倾向黑话：" + strings.Join(MatchedWordList(hits), "、")
		status := ApplyCrisisHandling(t, "keyword", "intercept", reason, cfg)
		// 同步回内存对象：广播门禁要看得到状态，否则被折叠的内容还会被推到公共频道
		comment.ModerationStatus = status
		comment.FlaggedMinor = true
		return BuildCrisisCard("intercept", reason, cfg)
	}

	// 二、语义绝望信号：AI 语义分析（异步，不阻塞用户发评论）
	//
	// 为什么要「粗筛」再送 AI：一次模型调用几秒、几厘钱，
	// 全站每条评论都送审既慢又贵。这里先用内置的低落语义信号粗筛一遍，
	// 只有命中（「今天天气真好，可惜我再也看不到了」这类）才真的花模型调用。
	if len(ScanDistressSignals(raw)) > 0 {
		go AsyncAICrisisCheck(t, cfg)
	}
	return nil
}

// AsyncAICrisisCheck 异步 AI 判危：模型识别出的绝望语义（无黑话）同样要接住。
func AsyncAICrisisCheck(t Target, cfg *aisafety.SafetyConfig) {
	// 极短文本不送审：一两个字的评论不含可判定的绝望语义，也省一次模型调用
	if len([]rune(strings.TrimSpace(t.Raw))) < 12 {
		return
	}
	v := aisafety.JudgeMinorCrisis(t.Raw)
	if v == nil || v.Failed {
		return
	}
	action := strings.TrimSpace(v.Action)
	if !v.Crisis && (action == "" || action == "ignore") {
		return
	}
	if action == "" || action == "ignore" {
		// 模型判了危机却没给动作 → 保守按 review（先审后发），不漏判
		action = "review"
	}
	ApplyCrisisHandling(t, "ai", action, v.Reason, cfg)
}

// ApplyCrisisHandling 统一的危机处置：标记 → 观察期 → 留痕 → 报站长 → 抚慰本人。
//
// intercept = 自动折叠（内容不公开，站长复核后再定）；review = 先审后发。
// 返回落库后的 moderation_status（供调用方同步内存态）。
func ApplyCrisisHandling(t Target, source, action, reason string, cfg *aisafety.SafetyConfig) string {
	if t.CommentID == 0 {
		return ""
	}
	status := "pending"
	if action == "intercept" {
		status = "folded"
	}
	_ = models.DB.Model(&models.Comment{}).Where("id = ?", t.CommentID).Updates(map[string]any{
		"flagged_minor":     true,
		"moderation_status": status,
	})
	// 进入高危观察期 → 绝对保护模式对其生效
	TagUserCrisisObservation(t.UserID, cfg)
	// 留痕（供《高危用户心理干预报告》）
	RecordCrisisEvent(t, source, action, reason)
	// 报给站长：AI 做到极致，最后的生死按钮由活人按
	NotifyAdminsCrisis(t, action, reason)
	// 向本人发送抚慰私信（心理干预智能体），而不是冷冰冰的「内容违规」
	SendComfortMessage(t.UserID, cfg)
	return status
}

// TagUserCrisisObservation 把用户置入高危观察期。
// 已有更长的观察期时只顺延、不缩短（反复触发不能把保护期越缩越短）。
func TagUserCrisisObservation(userID uint, cfg *aisafety.SafetyConfig) {
	if userID == 0 {
		return
	}
	hours := 72
	if cfg != nil && cfg.MinorObservationHours > 0 {
		hours = cfg.MinorObservationHours
	}
	until := time.Now().Add(time.Duration(hours) * time.Hour)
	var u models.User
	if err := models.DB.First(&u, userID).Error; err != nil {
		return
	}
	if u.RiskYouthMentalUntil != nil && u.RiskYouthMentalUntil.After(until) {
		until = *u.RiskYouthMentalUntil
	}
	_ = models.DB.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]any{
		"risk_youth_mental":       true,
		"risk_youth_mental_until": until,
	})
}

// RecordCrisisEvent 写危机事件留痕。
func RecordCrisisEvent(t Target, source, action, reason string) {
	username := ""
	if u, e := models.GetUserByID(t.UserID); e == nil && u != nil {
		username = u.Username
	}
	_ = securitym.CreateCrisisEvent(&securitym.CrisisEvent{
		UserID:      t.UserID,
		Username:    username,
		Source:      source,
		ContentType: "comment",
		ContentID:   t.CommentID,
		Snippet:     snippetOf(t.Raw),
		Action:      action,
		Reason:      reason,
	})
}

// NotifyAdminsCrisis 把高危信号报给所有管理员（人在回路）。
func NotifyAdminsCrisis(t Target, action, reason string) {
	var admins []models.User
	models.DB.Where("role = ?", "admin").Find(&admins)
	if len(admins) == 0 {
		return
	}
	excerpt := snippetOf(t.Raw)
	if runes := []rune(excerpt); len(runes) > 80 {
		excerpt = string(runes[:80]) + "…"
	}
	content := fmt.Sprintf("用户 #%d 的评论触发心理危机信号（处置：%s）。\n片段：%s",
		t.UserID, CrisisActionLabel(action), excerpt)
	if strings.TrimSpace(reason) != "" {
		content += "\n判据：" + reason
	}
	link := models.LinkForComment(&models.Comment{
		ID: t.CommentID, NovelID: t.NovelID, ChapterNumber: t.ChapterNumber,
	})
	for _, a := range admins {
		_ = models.PushNotification(t.UserID, a.ID, "youth_crisis",
			"⚠️ 未成年人心理危机信号", content, link)
	}
}

// SendComfortMessage 向本人发送心理干预智能体私信。
// actorID=0 表示系统发出（不是别人在盯着他看，这很重要）。
func SendComfortMessage(userID uint, cfg *aisafety.SafetyConfig) {
	if userID == 0 {
		return
	}
	comfort := aisafety.DefaultMinorCrisisComfort()
	hotline := aisafety.DefaultMinorCrisisHotline()
	if cfg != nil {
		if strings.TrimSpace(cfg.MinorCrisisComfort) != "" {
			comfort = cfg.MinorCrisisComfort
		}
		if strings.TrimSpace(cfg.MinorCrisisHotline) != "" {
			hotline = cfg.MinorCrisisHotline
		}
	}
	content := comfort + "\n\n心理危机干预热线：\n" + hotline
	_ = models.PushNotification(0, userID, "crisis_comfort", "你不是一个人", content, "")
}

// BuildCrisisCard 组装回传给前端的柔和卡片信息。
func BuildCrisisCard(action, reason string, cfg *aisafety.SafetyConfig) *models.CommentCrisis {
	hotline := aisafety.DefaultMinorCrisisHotline()
	comfort := aisafety.DefaultMinorCrisisComfort()
	if cfg != nil {
		if strings.TrimSpace(cfg.MinorCrisisHotline) != "" {
			hotline = cfg.MinorCrisisHotline
		}
		if strings.TrimSpace(cfg.MinorCrisisComfort) != "" {
			comfort = cfg.MinorCrisisComfort
		}
	}
	return &models.CommentCrisis{
		Triggered: true,
		Action:    action,
		Hotline:   hotline,
		Comfort:   comfort,
		Reason:    reason,
	}
}

// ShieldHighRiskTargets 绝对保护模式（落库前判定）。
//
// 抑郁倾向者几乎总是伴随着网暴。所以对处于高危观察期的用户，
// 指向他的攻击性言论（哪怕只是「你太矫情了」这种轻度攻击）一律先折叠，
// 不让它出现在受害者屏幕上。返回被保护的用户 id 列表（用于事后柔和提示）。
func ShieldHighRiskTargets(comment *models.Comment, parentAuthorID uint, mentions []models.MentionRef) []uint {
	if comment == nil {
		return nil
	}
	cfg, _ := aisafety.GetConfig()
	if cfg == nil || !cfg.MinorCrisisEnabled {
		return nil
	}

	targets := map[uint]bool{}
	if parentAuthorID > 0 {
		targets[parentAuthorID] = true
	}
	for _, m := range mentions {
		if m.UserID > 0 {
			targets[m.UserID] = true
		}
	}
	if len(targets) == 0 {
		return nil
	}

	var protected []uint
	for uid := range targets {
		var u models.User
		if models.DB.First(&u, uid).Error != nil {
			continue
		}
		if IsUserInObservation(&u) {
			protected = append(protected, uid)
		}
	}
	if len(protected) == 0 {
		return nil
	}

	// 对指向高危用户的评论，裁决标准比平时更严：flag（标注）也折叠。
	ct, cid := contentRef(comment.NovelID, comment.BlogID)
	var state store.TopicState
	if ct != "" && cid > 0 {
		state = store.StateOf(ct, cid)
	}
	d := sentiment.Evaluate(state, comment.Content)
	if d.Action != sentiment.ActionBlock && d.Action != sentiment.ActionFlag {
		return nil
	}
	comment.ModerationStatus = "folded"
	return protected
}

// NotifyShieldedUser 绝对保护模式命中后，柔和地告诉被保护的人。
// 措辞刻意不指责攻击者，也不复述攻击内容 —— 受害者不需要再读一遍那些话。
func NotifyShieldedUser(protected []uint, actorID uint) {
	for _, uid := range protected {
		if uid == 0 || uid == actorID {
			continue
		}
		_ = models.PushNotification(actorID, uid, "shield_notice",
			"我们为你拦下了一条评论",
			"有人在你附近发表了可能让你不适的言论，我们已经把它折叠处理了。你很重要，不用一个人扛。",
			"")
	}
}

// MatchedWordList 取出命中词（去重，逗号分隔）。
func MatchedWordList(hits []security.BadWordHit) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		w := strings.TrimSpace(h.Matched)
		if w == "" || seen[w] {
			continue
		}
		seen[w] = true
		out = append(out, w)
	}
	return out
}

// CrisisActionLabel 处置动作的中文标签（给站长看）。
func CrisisActionLabel(action string) string {
	switch action {
	case "intercept":
		return "自动折叠 + 标记高危"
	case "review":
		return "先审后发 + 标记待复核"
	default:
		return action
	}
}
