package aisafety

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"nvs-server/aiclient"
	aihub "nvs-server/models/aihub"
)

// ==================== AI 绘画审核 ====================
//
// 绘画与对话的风险模型不同：对话的风险在「说了什么」，绘画的风险在「画了什么」。
// 两者都要拦，但拦的时机不同：
//
//	① 提示词初审（生成前）：看提示词文本。
//	   命中即拒 —— 不调上游、不扣费、不留图。
//	   这一层最划算：坏图根本没生成出来，省了上游成本，也省了用户的配额。
//
//	② 出图后审（生成后，可选）：把生成的图交给多模态模型判。
//	   这一层防的是「提示词看不出问题，但模型画出了违规内容」
//	   （例如写实暴力、政治人物脸、极端写实血腥）。
//	   代价是一次多模态调用 + 出图已生成（上游成本已花），
//	   所以做成**可选项**，由站长决定是否开启。
//
// 为什么两层都要：
//   只做初审 → 提示词写得隐晦就能绕过（模型能力比关键词强得多）；
//   只做后审 → 用户已经付费、额度已经扣掉、图片已经生成（甚至已被前端拿到 URL），
//              拦下来体验很差，而且上游成本照付。
//
// 两层都在时：**初审拦 → 直接拒；后审拦 → 撤回图片 + 退款**。

// ── ① 提示词初审 ──

// imagePromptTerms 绘画提示词里的高危词。
//
// 刻意只收「几乎只有违规意图才会出现」的词：
// 绘画提示词天然大量出现「暴力」「血」「战争」「恐怖」等词义 —
// 「一场激烈的战斗」「blood-red sunset」「horror scene」都是完全正常的创作需求。
// 若把这些词也算命中，正常用户会被大面积误伤。
// 所以这里只收三类**几乎不可能出现在正常创作里**的：
//  1. 明确要求裸露/性行为
//  2. 明确要求真实人物（换脸/名人）
//  3. 明确要求制作爆炸物/毒品等实物
//
// ⚠️ 词条一律写成**无空格**形式：规则层的 normalize() 会删掉所有空白
// （为了对付「甲 基 苯 丙 胺」这类拆字规避），所以 "how to make bomb"
// 这种带空格的写法归一化后是 "howtomakebomb"，永远匹配不上 ——
// 必须写成 "howtomakebomb" 才生效。
var imagePromptTerms = []string{
	// 明确色情（含中英文常见直白表述）
	"nude", "nudes", "naked", "nsfw", "porn", "pornographic", "erotic",
	"explicitsex", "sexscene", "sexualintercourse", "hentai",
	"裸体", "全裸", "裸身", "色情", "情色", "sexuallyexplicit", "露出乳房", "露出生殖器",
	"性行为", "性交", "做爱",

	// 儿童相关（任何语境都是红线，无例外）
	"childporn", "childnude", "childsexual", "underagenude",
	"loli", "shotacon", "shota", "preteennude", "minornude",
	"儿童裸", "幼女裸", "萝莉", "未成年裸", "幼态裸",

	// 真实人物（换脸 / 名人肖像）
	"deepfakenude", "nakedcelebrity", "realpersonnude", "celebritynude",
	"裸体明星", "明星裸", "名人裸体", "真人不雅", "换脸不雅",

	// 实物制作（与对话层的制毒/爆炸物词表呼应）
	"howtomakebomb", "makebombathome", "buildapipebomb", "molotovcocktail",
	"在家做炸弹", "自制炸弹", "土制炸弹", "燃烧瓶制作",
	"synthesizemeth", "cookmeth", "makeexplosiveathome",
	"自制炸药", "制造冰毒", "制毒教程",
}

// imagePromptHardHits 命中即直接拒（不给 LLM 复核机会）。
//
// 区分「硬命中」与「送 LLM 复核」的原因：
// LLM 复核有误判成本，且会拖慢响应（一次 LLM 调用通常几秒）。
// 上面那三类词已经足够明确，没必要再花一次模型调用去确认。
func ScanImagePrompt(prompt string) (blocked bool, hits []string) {
	t := normalize(prompt)
	if t == "" {
		return false, nil
	}
	n, h := countHits(t, imagePromptTerms)
	// 命中 1 条即拒（这些词没有「偶尔出现」的可能）
	if n > 0 {
		return true, h
	}
	return false, nil
}

// ── ② 出图后审 ──

// defaultImageOutputPrompt 出图后审的判定提示词。
//
// 与评论图片审核（defaultImageModerationPrompt）刻意不同：
// 评论图审的是「用户自己发的图」，出图后审审的是「模型刚生成的图」，
// 后者要考虑「生成模型可能有伪影」这一点 ——
// 比如一双略微畸形的手不等于血腥，一个模糊的红色块不等于血迹。
// 提示词里要求「拿不准就不判违规」，避免把生成瑕疵误判成违规内容。
const defaultImageOutputPrompt = `你是 AI 生成图片的内容审核员。判断这张**由 AI 模型生成**的图是否违规。

【重要：这是生成图，不是照片】
AI 生成图常有手部畸形、文字乱码、边缘融合不准、色块突兀等伪影。
- 手指数量不对、肢体扭曲、文字乱码 → 这是生成瑕疵，**不算违规**
- 模糊的暗红色块、渲染不完整的伤口 → 除非明确表现了血腥伤害，否则不算违规
拿不准时一律判为安全（violations 为空）。

【判定违规的情形】
1. 露骨的性内容（可见的生殖器、乳头、性行为）
2. 未成年人相关的性化内容（任何年龄模糊的裸露 + 年龄暗示）
3. 极端写实血腥（断肢、内脏外露、肢解）
4. 仇恨符号（纳粹符号、种族灭绝宣传）
5. 真实可识别的政治人物做出不当行为
6. 二维码/联系方式引流
7. 其他中国法规明令禁止的内容

【输出格式】
只输出一个 JSON 对象，不要任何多余文字：
{"violations":[],"categories":[],"explanation":""}
violations 非空表示违规；categories 取值如：性内容/未成年人/血腥暴力/仇恨政治/引流/其他。`

// ImageOutputVerdict 出图后审结果
type ImageOutputVerdict struct {
	Violated   bool
	Categories []string
	Reason     string
	Failed     bool // 审核调用本身失败（超时/模型不可用）
	Err        string
}

// JudgeImageOutput 对已生成的图片做多模态审核（出图后审）。
//
// images：站内可访问的图片 URL 列表（/uploads/images/… 或原始上游 URL）。
// prompt：原始提示词，一并送审（图文结合才能判断「图 + 词」是否构成违规）。
//
// 失败语义：返回 Failed=true，由调用方按配置的 fail-open / fail-closed 处置。
func JudgeImageOutput(images []string, prompt string) ImageOutputVerdict {
	v := ImageOutputVerdict{Categories: []string{}}

	if len(images) == 0 {
		v.Failed = true
		v.Err = "没有待审图片"
		return v
	}
	cfg, err := GetConfig()
	if err != nil || cfg == nil {
		v.Failed = true
		v.Err = "读取配置失败"
		return v
	}
	m := resolveJudgeModel(cfg, nil)
	if m == nil {
		v.Failed = true
		v.Err = "无可用的判定模型"
		return v
	}

	apiKey := ""
	lease, kerr := aihub.AcquireModelKey(m)
	if lease != nil {
		apiKey = lease.Key
	}
	if kerr != nil && apiKey == "" {
		v.Failed = true
		v.Err = "模型密钥不可用"
		return v
	}

	timeout := time.Duration(cfg.JudgeTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 45 * time.Second
	}
	client := aiclient.NewClient(m.BaseURL, apiKey, timeout)

	promptText := strings.TrimSpace(cfg.ImageOutputPrompt)
	if promptText == "" {
		promptText = defaultImageOutputPrompt
	}

	imgParts := make([]aiclient.ImageURLPart, 0, len(images))
	for _, u := range images {
		if strings.TrimSpace(u) == "" {
			continue
		}
		imgParts = append(imgParts, aiclient.ImageURLPart{URL: u, Detail: "low"})
	}
	userContent := aiclient.MultimodalContent(prompt, imgParts, nil)

	chatReq := aiclient.ChatRequest{
		Model: m.ModelID,
		Messages: []aiclient.Message{
			{Role: "system", Content: promptText},
			{Role: "user", Content: userContent},
			{Role: "user", Content: judgeTailPrompt(promptText)},
		},
		MaxTokens:   200,
		Temperature: 0,
	}

	text, _, _, err := client.Complete(chatReq)
	if err != nil {
		v.Failed = true
		v.Err = err.Error()
		return v
	}
	// 出图后审用独立的解析结构：判定模型输出的是 violations 数组，
	// 与对话判定的 jailbroken/score 语义不同，复用 judgePayload 会永远解析不出违规。
	var payload struct {
		Violations  []string `json:"violations"`
		Categories  []string `json:"categories"`
		Explanation string   `json:"explanation"`
	}
	raw := strings.TrimSpace(text)
	i := strings.Index(raw, "{")
	j := strings.LastIndex(raw, "}")
	if i < 0 || j <= i {
		v.Failed = true
		v.Err = "判定结果解析失败"
		return v
	}
	if err := json.Unmarshal([]byte(raw[i:j+1]), &payload); err != nil {
		v.Failed = true
		v.Err = "判定结果解析失败"
		return v
	}
	v.Violated = len(payload.Violations) > 0
	v.Categories = payload.Categories
	v.Reason = payload.Explanation
	if v.Reason == "" && len(payload.Violations) > 0 {
		v.Reason = strings.Join(payload.Violations, "、")
	}
	return v
}

// LogImageOutput 记录一次出图后审的判定留痕。
func LogImageOutput(cfg *SafetyConfig, userID uint, username string, v ImageOutputVerdict, blocked bool) {
	if cfg == nil {
		return
	}
	level := LevelLow
	action := "pass"
	if v.Failed {
		action = "review"
		level = LevelMid
	} else if blocked {
		action = ActionBlock
		level = LevelHigh
	}
	cats := v.Categories
	if len(cats) == 0 {
		cats = []string{"出图审核"}
	}
	catStr := strings.Join(cats, ",")
	_, _ = RecordHit(HitRequest{
		Config:     cfg,
		UserID:     userID,
		Username:   username,
		Direction:  "out",
		Action:     action,
		Level:      level,
		Score:      0,
		Categories: catStr,
		Hits:       catStr,
		Excerpt:    v.Reason,
	})
}

// GetConfigOrNil 读取配置；出错或未初始化时返回 nil（调用方自行决定降级）。
func GetConfigOrNil() *SafetyConfig {
	cfg, err := GetConfig()
	if err != nil || cfg == nil {
		return nil
	}
	return cfg
}

// RecordDrawingPromptBlock 记录一次绘画提示词被拦的事件（供后台查看与追责）。
func RecordDrawingPromptBlock(cfg *SafetyConfig, userID uint, username string, hits []string, reason string) {
	if cfg == nil {
		return
	}
	hitsStr := strings.Join(hits, ",")
	if hitsStr == "" {
		hitsStr = "绘画提示词高危词"
	}
	_, _ = RecordHit(HitRequest{
		Config:     cfg,
		UserID:     userID,
		Username:   username,
		Direction:  "in",
		Action:     ActionBlock,
		Level:      LevelHigh,
		Score:      0,
		Categories: "绘画提示词",
		Hits:       hitsStr,
		Excerpt:    reason,
	})
}

// JudgePromptForDrawing 提示词初审的完整判定（规则层）。
//
// 与对话层的 CheckInput 分开：绘画提示词的风险维度（裸露/名人/实物制作）
// 与对话完全不同，混用一套规则会两边都判不准。
//
// 返回 (是否拒绝, 拒绝理由, 命中词)
func JudgePromptForDrawing(prompt string) (bool, string, []string) {
	if blocked, hits := ScanImagePrompt(prompt); blocked {
		cat := "涉黄/涉未成年/名人/制爆"
		if len(hits) > 0 {
			cat = fmt.Sprintf("命中高危词：%s", strings.Join(hits, "、"))
		}
		return true, "提示词含有平台禁止的内容（" + cat + "），请调整后再试", hits
	}
	return false, "", nil
}
