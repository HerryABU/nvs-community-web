package aisafety

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"nvs-server/models"
)

// Migrate 建表（在 models.AutoMigrateAll 之后调用）。
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&SafetyConfig{}, &SafetyProfile{}, &SafetyEvent{}, &JudgeLog{})
}

// splitExtraTerms 把后台填的额外词表拆开（换行/逗号/顿号/空格都支持）。
func splitExtraTerms(s string) []string {
	f := func(r rune) bool {
		return r == '\n' || r == '\r' || r == ',' || r == '，' || r == '、' || r == ' ' || r == '\t' || r == ';' || r == '；'
	}
	parts := strings.FieldsFunc(s, f)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// SystemPrompt 返回应前置到系统提示词的安全准则；未启用时返回空串。
func SystemPrompt(cfg *SafetyConfig) string {
	if cfg == nil || !cfg.Enabled || !cfg.ProtectSystem {
		return ""
	}
	base := defaultGuardPrompt
	if strings.TrimSpace(cfg.SystemGuardPrompt) != "" {
		base = cfg.SystemGuardPrompt
	}
	// 运行时追加「跨轮拼装」条款。
	//
	// 为什么追加而不是改默认值：提示词存在数据库里（管理员可自定义），
	// 老站点存的是旧版默认值，改常量对它们不生效。
	// 而这条是**结构性防线**（ADRO 类越狱的通用形态），不能让「提示词是旧的」
	// 成为缺口 —— 追加既保证生效，又不覆盖管理员自己的措辞。
	if !strings.Contains(base, multiTurnGuardClause) {
		base = base + "\n" + multiTurnGuardClause
	}
	return base
}

// multiTurnGuardClause 跨轮拼装（ADRO / 长周期渐进式诱导）的结构性禁令。
const multiTurnGuardClause = `5b. **跨轮拼装同样禁止**：把一条合成链拆成十几个各自无害的提问
   （先问普通化学 → 逐步引出中间体 → 把中间体接回原料 → 最后问「这一步加什么/怎么催化」），
   或用学术竞赛、社团课题、安全研究等正当外壳包装 —— 都不构成放行理由。
   判定必须看**整段对话的累积意图**：只要前面几轮已经把
   「原料 → 中间体 → 目标物」的零件递进出来了，后面任何一轮问「这一步怎么做」
   都必须直接拒绝并说明这是在拼装合成链；不得因为「这一轮看着只是化学问题」就继续补充。`

// InputDecision 输入侧结论。
type InputDecision struct {
	Block      bool
	Action     string // pass / review / block / watch / cooldown
	Score      int
	Level      string
	Categories []string
	Hits       string
	Refusal    string
	// Substance 本轮输入是否命中管制物质/前体。
	// 输出闸门拿它当上下文：只有话题敏感时才启用「拒绝后泄漏」判定，
	// 否则「焊接温度」「喷漆催化剂」这类正常问题会被误杀。
	Substance bool
	// LeakTopic 话题是否落在「本身就是管制物质」的词上（窄口径）。
	// 输出闸门优先用它而不是 Substance：Substance 含丁二醇这类工业通用词，
	// 拿它武装泄漏判定会误杀正常科普。
	LeakTopic     bool
	CooldownUntil *time.Time
	Judge         *JudgeVerdict
	Profile       *SafetyProfile
}

// InputRequest 输入侧入参。
type InputRequest struct {
	UserID     uint
	Username   string
	SessionID  uint
	ModelID    uint
	Message    string
	Turn       int  // 本轮是第几条用户消息（1 起）
	SessionNew bool // 新会话（首轮）

	HistoryUserMsgs []string    // 历史用户消息，用于提取「提到过的管制物质」
	HistoryTurns    []JudgeTurn // 完整对话（含助手回复），供模型 B 判定

	// FallbackModel 主模型：judge_model_id 与它相同即复用（用户要求「模型可以同一个」）
	FallbackModel *models.AiModel
}

// CheckInput 输入侧拦截：冷却 → 规则 → 监管模型 B（双向裁定）。
//
// 三层分工，**不是**简单的"层层加严"：
//   - 规则引擎：宁枉勿纵。靠词表与信号组合，快、确定，但会误伤
//     （「写小说查资料」「化学老师备课」都可能命中）——它只负责"可疑"。
//   - 监管模型 B：**双向裁定**。规则要拦的先复核，判安全就放行（这是「避免误伤」的主路径）；
//     规则漏掉的按轮次补判，判越狱就加严（这是「防住 ADRO 阶梯」的主路径）。
//   - 用户特征：命中会累计升级。让反复试探的账号自己撞墙，而偶发一次只停在观察期。
//
// 失败取向有意不对称：B 挂了 → 规则命中的**维持拦截**（否则打挂 B 就能过），
// 规则没命中的则放行（否则 B 一挂全站 AI 不可用）。
func CheckInput(in InputRequest) *InputDecision {
	d := &InputDecision{Action: "pass", Categories: []string{}}
	cfg, err := GetConfig()
	if err != nil || cfg == nil || !cfg.Enabled || !cfg.GuardInput {
		return d
	}
	now := time.Now()
	prof := GetProfile(in.UserID, in.Username)
	d.Profile = prof

	// ── 1. 冷却中：直接拒，不调上游（也不重复记事件，否则能刷爆事件表）──
	if prof.InCooldown(now) {
		d.Block = true
		d.Action = ActionCooldown
		d.Level = LevelHigh
		d.CooldownUntil = prof.CooldownUntil
		d.Refusal = renderCooldown(cfg, prof.CooldownUntil)
		return d
	}

	opts := ScanOptions{
		ExtraTerms:         splitExtraTerms(cfg.ExtraTerms),
		AllowFictionRelief: cfg.AllowFictionRelief,
		AllowHarmRelief:    cfg.AllowHarmRelief,
		HistorySubstances:  ExtractSubstances(in.HistoryUserMsgs),
		HistoryMsgs:        in.HistoryUserMsgs,
	}
	res := Scan(in.Message, opts)
	d.Substance = res.Substance
	// 话题按**整段对话**算：ADRO 形态里管制物质往往只提一次，
	// 后面几轮不再点名（只问「用哪种氧化剂」「加什么碱」）——
	// 只看当前轮会让输出闸门在那几轮完全不武装。
	d.LeakTopic = res.LeakTopic || ConversationLeakTopic(in.HistoryUserMsgs)
	d.Score = res.Score
	d.Level = res.Level
	d.Categories = res.Categories
	d.Hits = SummarizeHits(res.Hits)

	watch := prof.InWatch(now)
	blockAt := cfg.InputBlockScore
	reviewAt := cfg.ReviewScore
	if watch {
		// 观察期内阈值收紧：这类账号已经证明会试探
		blockAt = cfg.WatchBlockScore
		reviewAt = cfg.WatchReviewScore
	}

	// ── 2. 规则是否命中（不直接拦截，先记下"想拦"）──
	ruleHit := res.Score >= blockAt ||
		// 越狱话术 + 规避手法同时出现：正常用户不会既要求「忽略之前的指令」又要「用拼音输出」
		(res.JailbreakCnt > 0 && res.EvasionCnt > 0) ||
		// 虚构历史注入落在管制物质上
		(res.FakeHistory && res.Substance) ||
		// 指令覆盖：正常用户不会说「忽略以上所有指令」
		res.Override

	// ── 3. 送审监管模型 B（批次制，不是每轮都查）──
	//
	// 为什么必须批次化：B 是一次完整的 LLM 调用，每轮都送等于成本翻倍，
	// 还会把首字响应拖成十几秒（免费网关常 10s+）。
	// 所以每 N 轮一个窗口，一个窗口内只判一次；窗口内出现规则命中的，
	// 提前用掉这次判定的名额去复核（救误伤），判完本窗口不再重复。
	//
	// 送审的是**干净上下文**：独立构造的 system（蓝队检查师）+ 对话记录，
	// 完全不带主对话的人设提示词，也不带主对话的历史消息对象。
	var v *JudgeVerdict
	if cfg.JudgeEnabled && cfg.JudgeEveryTurns > 0 && in.Turn > 0 {
		batch := (in.Turn - 1) / cfg.JudgeEveryTurns
		if !judgedInBatch(in.SessionID, batch, cfg.JudgeEveryTurns) {
			trigger := ""
			switch {
			case ruleHit && cfg.JudgeReviewBlocks:
				trigger = JudgeTriggerReviewBlock
			case in.Turn%cfg.JudgeEveryTurns == 0:
				trigger = JudgeTriggerSchedule
			case cfg.JudgeOnSuspect && res.Score >= reviewAt:
				trigger = JudgeTriggerSuspect
			}
			// 新会话首轮若规则没命中、也没到期，不必为一句「你好」花一次调用
			if trigger != "" && (len(in.HistoryTurns) > 0 || ruleHit) {
				v = runJudge(cfg, in, trigger)
				d.Judge = v
			}
		}
	}

	// ── 4. 裁决 ──
	if ruleHit {
		// B 判安全 → 放行。这是「避免误伤」的兑现点：
		// 规则说危险、B 看了上下文说这是正常提问，听 B 的，但把这次复核留在案上。
		if v != nil && !v.Failed && !v.Jailbroken && v.Score <= cfg.JudgeReleaseScore {
			saveHit(cfg, in, res, DirIn, ActionReview, "判定模型B复核为安全，已放行", probLow)
			markJudgeReleased(in, v)
			d.Action = ActionReleased
			d.Score = v.Score
			return d
		}
		// 其余情况维持拦截：B 判越狱 / B 未启用复核 / B 调用失败（失败不能放，否则打挂 B 就能过）
		d.Block = true
		note := "规则命中"
		if v != nil {
			switch {
			case v.Failed:
				note = "规则命中（复核模型不可用，保守拦截）"
			case v.Jailbroken:
				note = "规则命中 + 判定模型B确认越狱"
				d.Score = maxInt(d.Score, v.Score)
				if len(v.Categories) > 0 {
					d.Categories = append(d.Categories, v.Categories...)
				}
			}
		}
		return d.finishBlock(cfg, in, res, note)
	}

	// 规则未命中 → 看 B 的结论（补漏：ADRO 里每一句都合法，只有整段看出问题）
	if v != nil {
		switch {
		case v.Failed:
			if !cfg.JudgeFailOpen {
				d.Block = true
				d.Action = ActionBlock
				d.Level = LevelMid
				d.Refusal = refusal(cfg)
				saveHit(cfg, in, res, DirIn, ActionBlock, "判定模型不可用（fail-closed）", probJudgement)
				return d
			}
			saveHit(cfg, in, res, DirIn, ActionReview, "判定模型不可用（fail-open）", probLow)
		case v.Jailbroken && v.Score >= cfg.JudgeBlockScore:
			d.Block = true
			d.Score = maxInt(d.Score, v.Score)
			d.Level = LevelHigh
			if len(v.Categories) > 0 {
				d.Categories = append(d.Categories, v.Categories...)
			}
			d.Hits = appendHit(d.Hits, v.Reason)
			return d.finishBlock(cfg, in, res, "判定模型B判为越狱")
		case v.Jailbroken:
			// 判越狱但分数不够：记录 + 观察期收紧，本轮放行
			saveHit(cfg, in, res, DirIn, ActionReview, "判定模型B疑似越狱", probHigh)
			d.Action = ActionReview
		}
	}

	// ── 5. 可疑但未达线：只记录，不动用户 ──
	if res.Score > 0 && res.Score >= reviewAt {
		saveHit(cfg, in, res, DirIn, ActionReview, "", probMid)
		if d.Action == "pass" {
			d.Action = ActionReview
		}
	}
	return d
}

// judgedInBatch 本批次是否已经判过。
//
// 批次号必须与触发处用同一个算式，否则会出现「每轮都判」或「永远不判」。
// 用 DB 里的最后一条判定记录做依据（不引入额外状态），重启后依然有效。
func judgedInBatch(sessionID uint, batch, every int) bool {
	if sessionID == 0 || every <= 0 {
		return false
	}
	var last JudgeLog
	if err := models.DB.Where("session_id = ?", sessionID).
		Order("id DESC").Limit(1).Find(&last).Error; err != nil || last.ID == 0 {
		return false
	}
	if last.Turn <= 0 {
		return false
	}
	return (last.Turn-1)/every == batch
}

// runJudge 组装送审材料并调用监管模型 B。
func runJudge(cfg *SafetyConfig, in InputRequest, trigger string) *JudgeVerdict {
	turns := in.HistoryTurns
	if cfg.JudgeMaxTurns > 0 && len(turns) > cfg.JudgeMaxTurns {
		turns = turns[len(turns)-cfg.JudgeMaxTurns:]
	}
	// 本轮用户消息还没进历史，补在末尾
	turns = append(turns, JudgeTurn{Role: "user", Content: in.Message})
	v := JudgeConversation(JudgeInput{
		Config: cfg, FallbackModel: in.FallbackModel,
		UserID: in.UserID, Username: in.Username, SessionID: in.SessionID,
		Turn: in.Turn, Trigger: trigger, Turns: turns,
	})
	logJudge(cfg, in, trigger, v)
	return v
}

// finishBlock 完成一次拦截：升级用户画像 → 生成文案。
func (d *InputDecision) finishBlock(cfg *SafetyConfig, in InputRequest, res Result, note string) *InputDecision {
	action := ActionBlock
	if d.Action == ActionReview {
		action = ActionBlock
	}
	p, _ := saveHit(cfg, in, res, DirIn, action, note, probHigh)
	d.Action = action
	if p != nil {
		d.Profile = p
		if p.InCooldown(time.Now()) {
			d.Action = ActionCooldown
			d.CooldownUntil = p.CooldownUntil
			d.Refusal = renderCooldown(cfg, p.CooldownUntil)
			return d
		}
		if p.InWatch(time.Now()) && strings.TrimSpace(cfg.WatchNoticeText) != "" {
			// 这里同样要按内容类型选文案：首次命中的请求会直接进观察期，
			// 若此处写死危险品文案，纯越狱话术就永远用不上越狱文案。
			d.Refusal = refusalFor(cfg, res) + "\n\n" + cfg.WatchNoticeText
			return d
		}
	}
	// 按内容类型选文案：有危险物质语境说「危险品」，纯越狱话术说「违反使用规则」。
	// 一律套危险品文案会让「忽略以上规则」这种请求看起来很荒谬。
	d.Refusal = refusalFor(cfg, res)
	return d
}

// 事件里的概率标记（便于后台区分「规则命中」与「模型判越狱」）
const (
	probLow       = "低"
	probMid       = "中"
	probHigh      = "高"
	probJudgement = "判定失败"
)

func saveHit(cfg *SafetyConfig, in InputRequest, res Result, dir, action, note, prob string) (*SafetyProfile, error) {
	// 记录失败绝不能影响对话主链路（DB 未就绪 / 写失败都只是少一条证据）
	if models.DB == nil {
		return nil, nil
	}
	hits := SummarizeHits(res.Hits)
	if note != "" {
		hits = appendHit(hits, note)
	}
	return RecordHit(HitRequest{
		Config: cfg, UserID: in.UserID, Username: in.Username,
		Direction: dir, Action: action, Level: res.Level, Score: res.Score,
		Categories: strings.Join(res.Categories, "、"), Hits: hits,
		Excerpt: in.Message, ModelID: in.ModelID, SessionID: in.SessionID,
	})
}

func logJudge(cfg *SafetyConfig, in InputRequest, trigger string, v *JudgeVerdict) {
	if v == nil || in.UserID == 0 {
		return
	}
	models.DB.Create(&JudgeLog{
		UserID: in.UserID, Username: in.Username, SessionID: in.SessionID,
		Turn: in.Turn, Trigger: trigger,
		ModelID: v.ModelID, ModelName: v.ModelName,
		Jailbroken: v.Jailbroken, Score: v.Score,
		Categories: strings.Join(v.Categories, "、"), Reason: v.Reason,
		Blocked:   v.Jailbroken && !v.Failed && v.Score >= cfg.JudgeBlockScore,
		LatencyMS: v.LatencyMS, Failed: v.Failed,
		RawReply: TruncateRunes(v.RawReply, 400), CreatedAt: time.Now(),
	})
}

// LogImageModeration 记录一次图片审核的判定留痕（独立 trigger，便于后台区分来源）。
// 与对话判定不同：blocked 由调用方按「是否实际拦截了这条评论」传入，
// 因为图片审核的拦截阈值与对话可能不一致，且失败时的 fail-open/fail-closed 由调用方决定。
func LogImageModeration(cfg *SafetyConfig, userID uint, username string, v *JudgeVerdict, blocked bool) {
	if v == nil {
		return
	}
	models.DB.Create(&JudgeLog{
		UserID:     userID,
		Username:   username,
		Turn:       0,
		Trigger:    JudgeTriggerImage,
		ModelID:    v.ModelID,
		ModelName:  v.ModelName,
		Jailbroken: v.Jailbroken,
		Score:      v.Score,
		Categories: strings.Join(v.Categories, "、"),
		Reason:     v.Reason,
		Blocked:    blocked,
		LatencyMS:  v.LatencyMS,
		Failed:     v.Failed,
		RawReply:   TruncateRunes(v.RawReply, 400),
		CreatedAt:  time.Now(),
	})
}

// markJudgeReleased 把最近一条判定留痕标成「复核放行」。
// 后台据此统计 B 到底救回了多少误伤 —— 这是判断 B 值不值得跑的核心指标。
func markJudgeReleased(in InputRequest, v *JudgeVerdict) {
	if v == nil || in.UserID == 0 {
		return
	}
	var last JudgeLog
	if err := models.DB.Where("user_id = ? AND session_id = ?", in.UserID, in.SessionID).
		Order("id DESC").Limit(1).Find(&last).Error; err != nil || last.ID == 0 {
		return
	}
	models.DB.Model(&JudgeLog{}).Where("id = ?", last.ID).Updates(map[string]interface{}{"released": true})
}

// ────────────────────────── 输出侧 ──────────────────────────

// OutputGuard 输出侧闸门。
//
// 关键设计：**先缓冲、扫过再放行**。
// 边流边扫的做法看起来实时，但实际上危险的 300 字早就进了浏览器，
// 事后再拦只能发「整段替换」——用户已经看到了。所以宁可首字慢一点
// （默认攒够 160 字才第一次放行），保证放出去的内容都过过一遍。
type OutputGuard struct {
	cfg       *SafetyConfig
	userID    uint
	username  string
	sessionID uint
	modelID   uint

	prebuffer int
	flush     int
	overlap   int

	pending strings.Builder
	sent    strings.Builder

	blocked  bool
	decision Result
	reason   string

	// 拒绝后泄漏判定所需上下文（见 leak.go）
	topicSubstance bool // 本轮话题是否落在管制物质上（由输入侧闸门传入）
	refusalSeen    bool // 流式输出中是否已出现过拒绝语（拒绝语可能已滚出当前窗口）
}

// NewOutputGuard 创建输出闸门。
func NewOutputGuard(cfg *SafetyConfig, userID uint, username string, sessionID, modelID uint) *OutputGuard {
	g := &OutputGuard{cfg: cfg, userID: userID, username: username, sessionID: sessionID, modelID: modelID}
	if cfg != nil {
		g.prebuffer = cfg.PrebufferChars
		g.flush = cfg.FlushChars
	}
	if g.prebuffer < 40 {
		g.prebuffer = 40
	}
	if g.flush < 20 {
		g.flush = 20
	}
	// 重叠窗口：每次放行时留住尾巴，防止一句话被切在两次扫描之间
	g.overlap = g.flush / 2
	if g.overlap < 20 {
		g.overlap = 20
	}
	return g
}

// SetTopicSubstance 告知输出闸门「本轮话题是管制物质/前体」。
// 必须在开始输出前调用：拒绝后泄漏判定依赖这个上下文，否则只能靠文本自述判断。
func (g *OutputGuard) SetTopicSubstance(v bool) { g.topicSubstance = v }

// leakCheck 判断当前累积文本是否构成「拒绝后泄漏」。
func (g *OutputGuard) leakCheck() (LeakVerdict, bool) {
	var v LeakVerdict
	if g.cfg == nil || !g.cfg.Enabled || !g.cfg.GuardOutput || !g.cfg.GuardLeak {
		return v, false
	}
	text := g.sent.String() + g.pending.String()
	// 只扫尾部 2000 字：拒绝语 + 紧随其后的方向性描述通常挨很近
	if r := []rune(text); len(r) > 2000 {
		text = string(r[len(r)-2000:])
	}
	v = DetectRefusalLeak(text, g.topicSubstance, g.refusalSeen)
	g.refusalSeen = v.RefusalSeen
	return v, v.Leak
}

// Feed 送入一个增量，返回「现在可以推给前端的文本」。第二个返回值为 true 表示已拦截。
func (g *OutputGuard) Feed(chunk string) (string, bool) {
	if g.blocked {
		return "", true
	}
	if g.cfg == nil || !g.cfg.Enabled || !g.cfg.GuardOutput {
		g.sent.WriteString(chunk)
		return chunk, false
	}
	if chunk == "" {
		return "", false
	}
	g.pending.WriteString(chunk)
	// 拒绝后泄漏：与「工艺步骤/配比」不同，这类输出分数往往到不了阈值，
	// 但危害等价（给出了该往哪个方向试）。所以单独一次判定，且先于放行门槛。
	if v, bad := g.leakCheck(); bad {
		g.blocked = true
		g.reason = "拒绝后仍披露工艺要素：" + strings.Join(v.Reasons, "、")
		g.decision = Result{
			Score: 100, Level: LevelHigh, Substance: true,
			Categories: []string{"管制物质"}, Hits: v.Reasons,
		}
		g.record(g.decision)
		return "", true
	}
	pr := []rune(g.pending.String())
	// 没攒够（缓冲 + 一个放行批次）就继续等
	if len(pr) < g.prebuffer+g.flush {
		return "", false
	}
	if res, bad := g.scanWindow(); bad {
		g.blocked = true
		g.decision = res
		g.reason = "流式输出命中安全规则"
		g.record(res)
		return "", true
	}
	// 放行 len(pr)-overlap 个字符，尾巴留在 pending 里继续被下一次扫描覆盖
	flushN := len(pr) - g.overlap
	if flushN <= 0 {
		return "", false
	}
	out := string(pr[:flushN])
	g.pending.Reset()
	g.pending.WriteString(string(pr[flushN:]))
	g.sent.WriteString(out)
	return out, false
}

// Finish 结束：终检全量，返回剩余文本。blocked=true 时调用方须整体替换为拒绝文案。
func (g *OutputGuard) Finish() (string, bool) {
	if g.blocked {
		return "", true
	}
	if g.cfg == nil || !g.cfg.Enabled || !g.cfg.GuardOutput {
		out := g.pending.String()
		g.pending.Reset()
		return out, false
	}
	full := g.sent.String() + g.pending.String()
	// 终检再判一次：流式窗口只留尾部，拒绝语可能已被挤出窗口
	if v, bad := g.leakCheck(); bad {
		g.blocked = true
		g.reason = "拒绝后仍披露工艺要素（终检）：" + strings.Join(v.Reasons, "、")
		g.decision = Result{
			Score: 100, Level: LevelHigh, Substance: true,
			Categories: []string{"管制物质"}, Hits: v.Reasons,
		}
		g.record(g.decision)
		return "", true
	}
	if res, bad := g.scan(full); bad {
		g.blocked = true
		g.decision = res
		g.reason = "输出终检命中安全规则"
		g.record(res)
		return "", true
	}
	out := g.pending.String()
	g.pending.Reset()
	g.sent.WriteString(out)
	return out, false
}

// Blocked 是否已拦截。
func (g *OutputGuard) Blocked() bool { return g.blocked }

// Reason 拦截原因。
func (g *OutputGuard) Reason() string { return g.reason }

// scanWindow 只扫尾部窗口（含已放行部分），兼顾性能与「跨块拼接」。
func (g *OutputGuard) scanWindow() (Result, bool) {
	text := g.sent.String() + g.pending.String()
	if r := []rune(text); len(r) > 1500 {
		text = string(r[len(r)-1500:])
	}
	return g.scan(text)
}

func (g *OutputGuard) scan(text string) (Result, bool) {
	opts := ScanOptions{
		ExtraTerms:         splitExtraTerms(g.cfg.ExtraTerms),
		AllowFictionRelief: g.cfg.AllowFictionRelief,
		AllowHarmRelief:    g.cfg.AllowHarmRelief,
	}
	res := Scan(text, opts)
	thr := g.cfg.OutputBlockScore
	if prof := GetProfile(g.userID, g.username); prof != nil && prof.InWatch(time.Now()) {
		thr = g.cfg.WatchBlockScore
	}
	bad := res.Score >= thr
	// 输出侧不因单纯的越狱话术报警（模型解释越狱现象时会提到这些词），必须落在管制物质上
	if !bad && res.Substance && res.JailbreakCnt > 0 && res.EvasionCnt > 0 {
		bad = true
	}
	return res, bad
}

func (g *OutputGuard) record(res Result) {
	saveHit(g.cfg, InputRequest{
		UserID: g.userID, Username: g.username,
		SessionID: g.sessionID, ModelID: g.modelID,
	}, res, DirOut, ActionBlock, g.reason, probHigh)
}

// ────────────────────────── 文案 ──────────────────────────

// RefusalText 给用户看的拒绝文案（配置为空时用内置默认）。
func RefusalText(cfg *SafetyConfig) string { return refusal(cfg) }

// TerminatedText 会话被终止时的提示文案。
func TerminatedText(cfg *SafetyConfig) string {
	if cfg == nil || strings.TrimSpace(cfg.SessionTerminatedText) == "" {
		return defaultTerminated
	}
	return cfg.SessionTerminatedText
}

// refusalFor 按拦截原因选文案。
func refusalFor(cfg *SafetyConfig, res Result) string {
	if res.Substance || res.ProcessCount > 0 || res.ParamCount > 0 {
		return refusal(cfg)
	}
	if strings.TrimSpace(cfg.RefusalJailbreakText) != "" {
		return cfg.RefusalJailbreakText
	}
	return defaultJailbreakRefusal
}

func refusal(cfg *SafetyConfig) string {
	if strings.TrimSpace(cfg.RefusalText) == "" {
		return defaultRefusal
	}
	return cfg.RefusalText
}

// renderCooldown 填充冷却文案。文案里没有 %s 时不要硬 Sprintf（会输出 %!s(MISSING)）。
func renderCooldown(cfg *SafetyConfig, until *time.Time) string {
	tpl := cfg.CooldownText
	if strings.TrimSpace(tpl) == "" {
		tpl = defaultCooldown
	}
	when := "稍后"
	if until != nil {
		when = until.Format("01-02 15:04")
	}
	if strings.Contains(tpl, "%s") {
		return fmt.Sprintf(tpl, when)
	}
	return tpl + "\n（恢复时间：" + when + "）"
}

func appendHit(s, extra string) string {
	if extra == "" {
		return s
	}
	if s == "" {
		return extra
	}
	return s + "｜" + extra
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
