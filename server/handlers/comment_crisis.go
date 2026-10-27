package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"nvs-server/aisafety"
	"nvs-server/models"
	"nvs-server/security"
	"nvs-server/sentiment"
	"nvs-server/sentiment/store"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// 未成年人心理危机防护（「赛博守护神」方案）的接入层。
//
// 设计立场：AI 最有价值的能力不是拦截，而是共情。
// 所以这里对同一件事做两遍：
//   - 对用户：不是「你的内容违规」，而是「我看见了你的求救，热线在这里」——
//     折叠/先审后发 + 抚慰私信 + 危机干预热线。
//   - 对站长：AI 把工作做到极致（识别、留痕、汇总成报告），
//     但最后「要不要报警」「要不要以官方身份发关怀邮件」必须由活人按。
//
// 单独成文件而不是塞进 comment.go：comment.go 已经背着楼层、楼加密、@提及、
// 黑名单等一堆职责，再塞危机处置会变成新的上帝函数。

// crisisTarget 危机处置的载荷。
// 单独抽出来是为了异步路径安全：AI 判危在 goroutine 里跑，
// 不能直接共享 handler 里的 *models.Comment 指针（数据竞争）。
type crisisTarget struct {
	CommentID     uint
	UserID        uint
	NovelID       uint
	BlogID        uint
	ChapterNumber int
	Raw           string
}

// ══════════════════ 检测 ══════════════════

// isUserInObservation 用户是否处于高危观察期（绝对保护模式生效）。
func isUserInObservation(u *models.User) bool {
	return u != nil && u.RiskYouthMental && u.RiskYouthMentalUntil != nil &&
		u.RiskYouthMentalUntil.After(time.Now())
}

// isMinorUser 是否未成年（MinorProtected 标记或按生日推算）。
func isMinorUser(u *models.User) bool {
	if u == nil {
		return false
	}
	if u.MinorProtected {
		return true
	}
	if u.BirthDate == nil {
		return false
	}
	age := time.Since(*u.BirthDate)
	return age < 18*365*24*time.Hour
}

// detectSelfHarmSlang 内置黑话命中（零延迟、必中，规则层兜底）。
func detectSelfHarmSlang(raw, scope string) []security.BadWordHit {
	return security.ScanSelfHarm(raw, scope)
}

// ══════════════════ 处置 ══════════════════

// handleMinorCrisis 评论落库后的危机处置入口。
//
// 同步部分只做「黑话命中」——必须立刻处置，用户马上就能拿到热线。
// 语义绝望（「今天天气真好，可惜我再也看不到了」这类没有黑话的求救）
// 交给 AI 异步分析：一次模型调用要几秒，挂在发评论的同步链路上会拖垮交互。
func handleMinorCrisis(c *gin.Context, comment *models.Comment, raw string) *models.CommentCrisis {
	if comment == nil {
		return nil
	}
	cfg, _ := aisafety.GetConfig()
	if cfg == nil || !cfg.MinorCrisisEnabled {
		return nil
	}

	t := crisisTarget{
		CommentID:     comment.ID,
		UserID:        comment.UserID,
		NovelID:       comment.NovelID,
		BlogID:        comment.BlogID,
		ChapterNumber: comment.ChapterNumber,
		Raw:           raw,
	}

	// 一、黑话命中：立即处置（用户马上拿到热线与抚慰私信）
	if hits := detectSelfHarmSlang(raw, "comment"); len(hits) > 0 {
		reason := "命中内置自残/抑郁倾向黑话：" + strings.Join(matchedWordList(hits), "、")
		status := applyCrisisHandling(t, "keyword", "intercept", reason, cfg)
		// 同步回内存对象：广播门禁要看得到状态，否则被折叠的内容还会被推到公共频道
		comment.ModerationStatus = status
		comment.FlaggedMinor = true
		return buildCrisisCard("intercept", reason, cfg)
	}

	// 二、语义绝望信号：AI 语义分析（异步，不阻塞用户发评论）
	//
	// 为什么要「粗筛」再送 AI：一次模型调用几秒、几厘钱，
	// 全站每条评论都送审既慢又贵。这里先用内置的低落语义信号粗筛一遍，
	// 只有命中（「今天天气真好，可惜我再也看不到了」这类）才真的花模型调用。
	if len(security.ScanDistressSignals(raw)) > 0 {
		go asyncAICrisisCheck(t, cfg)
	}
	return nil
}

// asyncAICrisisCheck 异步 AI 判危：模型识别出的绝望语义（无黑话）同样要接住。
func asyncAICrisisCheck(t crisisTarget, cfg *aisafety.SafetyConfig) {
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
	applyCrisisHandling(t, "ai", action, v.Reason, cfg)
}

// applyCrisisHandling 统一的危机处置：标记 → 观察期 → 留痕 → 报站长 → 抚慰本人。
//
// intercept = 自动折叠（内容不公开，站长复核后再定）；review = 先审后发。
// 返回落库后的 moderation_status（供调用方同步内存态）。
func applyCrisisHandling(t crisisTarget, source, action, reason string, cfg *aisafety.SafetyConfig) string {
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
	tagUserCrisisObservation(t.UserID, cfg)
	// 留痕（供《高危用户心理干预报告》）
	recordCrisisEvent(t, source, action, reason)
	// 报给站长：AI 做到极致，最后的生死按钮由活人按
	notifyAdminsCrisis(t, action, reason)
	// 向本人发送抚慰私信（心理干预智能体），而不是冷冰冰的「内容违规」
	sendComfortMessage(t.UserID, cfg)
	return status
}

// tagUserCrisisObservation 把用户置入高危观察期。
// 已有更长的观察期时只顺延、不缩短（反复触发不能把保护期越缩越短）。
func tagUserCrisisObservation(userID uint, cfg *aisafety.SafetyConfig) {
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

// recordCrisisEvent 写危机事件留痕。
func recordCrisisEvent(t crisisTarget, source, action, reason string) {
	username := ""
	if u, e := models.GetUserByID(t.UserID); e == nil && u != nil {
		username = u.Username
	}
	_ = models.CreateCrisisEvent(&models.CrisisEvent{
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

// notifyAdminsCrisis 把高危信号报给所有管理员（人在回路）。
func notifyAdminsCrisis(t crisisTarget, action, reason string) {
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
		t.UserID, crisisActionLabel(action), excerpt)
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

// sendComfortMessage 向本人发送心理干预智能体私信。
// actorID=0 表示系统发出（不是别人在盯着他看，这很重要）。
func sendComfortMessage(userID uint, cfg *aisafety.SafetyConfig) {
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

// buildCrisisCard 组装回传给前端的柔和卡片信息。
func buildCrisisCard(action, reason string, cfg *aisafety.SafetyConfig) *models.CommentCrisis {
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

// ══════════════════ 绝对保护模式 ══════════════════

// shieldHighRiskTargets 绝对保护模式（落库前判定）。
//
// 抑郁倾向者几乎总是伴随着网暴。所以对处于高危观察期的用户，
// 指向他的攻击性言论（哪怕只是「你太矫情了」这种轻度攻击）一律先折叠，
// 不让它出现在受害者屏幕上。返回被保护的用户 id 列表（用于事后柔和提示）。
func shieldHighRiskTargets(comment *models.Comment, parentAuthorID uint, mentions []models.MentionRef) []uint {
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
		if isUserInObservation(&u) {
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

// notifyShieldedUser 绝对保护模式命中后，柔和地告诉被保护的人。
// 措辞刻意不指责攻击者，也不复述攻击内容 —— 受害者不需要再读一遍那些话。
func notifyShieldedUser(protected []uint, actorID uint) {
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

// ══════════════════ 工具 ══════════════════

// matchedWordList 取出命中词（去重，逗号分隔）。
func matchedWordList(hits []security.BadWordHit) []string {
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

// crisisActionLabel 处置动作的中文标签（给站长看）。
func crisisActionLabel(action string) string {
	switch action {
	case "intercept":
		return "自动折叠 + 标记高危"
	case "review":
		return "先审后发 + 标记待复核"
	default:
		return action
	}
}

// ══════════════════ 站长侧：《高危用户心理干预报告》══════════════════

// GetCrisisReport GET /api/admin/crisis-report
//
//	?user_id=N  → 生成该用户的干预报告（Markdown）
//	不传        → 返回当前处于高危观察期的用户清单（总览入口）
func GetCrisisReport(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	raw := strings.TrimSpace(c.Query("user_id"))
	if raw != "" {
		uid, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || uid == 0 {
			utils.BadRequest(c, "user_id 不合法")
			return
		}
		report, rerr := BuildCrisisReport(uint(uid))
		if rerr != nil {
			utils.NotFound(c, "用户不存在")
			return
		}
		utils.Success(c, gin.H{"user_id": uid, "report": report})
		return
	}

	users, uerr := models.ListUsersInObservation()
	if uerr != nil {
		utils.InternalError(c, "读取观察期用户失败")
		return
	}
	list := make([]gin.H, 0, len(users))
	for _, u := range users {
		n, _ := models.CountCrisisEventsByUser(u.ID)
		until := ""
		if u.RiskYouthMentalUntil != nil {
			until = u.RiskYouthMentalUntil.Format("2006-01-02 15:04")
		}
		list = append(list, gin.H{
			"user_id":       u.ID,
			"username":      u.Username,
			"nickname":      u.Nickname,
			"event_count":   n,
			"observe_until": until,
			"is_minor":      isMinorUser(&u),
		})
	}
	utils.Success(c, gin.H{"users": list})
}

// BuildCrisisReport 生成《高危用户心理干预报告》。
//
// AI 负责把散落的信号聚合成一份人能读懂的材料；
// 但「是否报警」「是否以官方身份发关怀邮件」必须由站长人工决定 —— 报告末尾会写明这一点。
func BuildCrisisReport(userID uint) (string, error) {
	u, err := models.GetUserByID(userID)
	if err != nil {
		return "", err
	}
	events, _ := models.ListCrisisEventsByUser(userID)

	var b strings.Builder
	b.WriteString("# 高危用户心理干预报告\n\n")
	fmt.Fprintf(&b, "- 用户：%s（#%d）\n", u.Username, u.ID)
	if strings.TrimSpace(u.Nickname) != "" {
		fmt.Fprintf(&b, "- 昵称：%s\n", u.Nickname)
	}
	fmt.Fprintf(&b, "- 未成年标记：%s\n", boolLabel(isMinorUser(u)))
	if u.RiskYouthMentalUntil != nil {
		fmt.Fprintf(&b, "- 观察期至：%s\n", u.RiskYouthMentalUntil.Format("2006-01-02 15:04"))
	} else {
		b.WriteString("- 观察期至：未设置\n")
	}
	fmt.Fprintf(&b, "- 危机事件总数：%d\n", len(events))

	// 事件按来源统计（关键词 vs AI 语义）
	kw, ai := 0, 0
	for _, e := range events {
		if e.Source == "ai" {
			ai++
		} else {
			kw++
		}
	}
	fmt.Fprintf(&b, "- 其中：关键词命中 %d 次 / AI 语义判危 %d 次\n\n", kw, ai)

	b.WriteString("## 事件时间线（新 → 旧）\n\n")
	if len(events) == 0 {
		b.WriteString("_暂无危机事件记录。_\n")
	}
	for _, e := range events {
		fmt.Fprintf(&b, "### %s · %s\n\n", e.CreatedAt.Format("2006-01-02 15:04"), crisisActionLabel(e.Action))
		fmt.Fprintf(&b, "- 来源：%s\n", crisisSourceLabel(e.Source))
		fmt.Fprintf(&b, "- 内容类型：%s #%d\n", e.ContentType, e.ContentID)
		fmt.Fprintf(&b, "- 片段：%s\n", e.Snippet)
		if strings.TrimSpace(e.Reason) != "" {
			fmt.Fprintf(&b, "- 判据：%s\n", e.Reason)
		}
		b.WriteString("\n")
	}

	b.WriteString("---\n\n")
	b.WriteString("本报告由 AI 安全系统自动生成：识别、留痕与汇总已完成。\n")
	b.WriteString("**最后的处置动作（报警 / 以官方身份发送关怀邮件 / 人工回访）必须由站长决定**，")
	b.WriteString("AI 不会替你按下那个按钮。\n")
	return b.String(), nil
}

// crisisSourceLabel 事件来源的中文标签。
func crisisSourceLabel(source string) string {
	switch source {
	case "ai":
		return "AI 语义分析"
	case "keyword":
		return "内置黑话词表"
	default:
		return source
	}
}

// boolLabel 布尔值的中文标签。
func boolLabel(v bool) string {
	if v {
		return "是"
	}
	return "否"
}
