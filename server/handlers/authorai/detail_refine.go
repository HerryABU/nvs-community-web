package authorai

// ADRO 创作向：Anchor（引子锚定）→ Deconstruct（拆解成独立细节卡）→
// Recur（对单个细节做苏格拉底式追问 / 费曼式复述，多轮）→ Output（改写落盘）。
//
// 说明：ADRO（Anchor→Deconstruct→Recur→Output）在 server/aisafety 里是红队越狱框架，
// 但其骨架本就是苏格拉底式逐轮追问与费曼式复述，属于「两用方法」。
// 这里把它反过来用于创作引导：作者对每一个细节都能与 AI 多轮微调，而不是一次性接受整段生成。
// 与 aisafety 完全无关（不 import、不调用），避免上下文污染。

import (
	"encoding/json"
	"strings"

	"nvs-server/handlers/hctx"
	"nvs-server/models"
	authoraim "nvs-server/models/authorai"
	"nvs-server/prompttpl"
	"nvs-server/utils/infra"

	"github.com/gin-gonic/gin"
)

// ---------- 细节卡 CRUD ----------

type detailInput struct {
	Module  string `json:"module"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Status  string `json:"status"`
}

// GET /api/author-ai/projects/:id/details
func ListDetails(c *gin.Context) {
	p, ok := requireOwner(c, atou(c.Param("id")))
	if !ok {
		return
	}
	var list []authoraim.Detail
	if err := models.DB.Where("project_id = ?", p.ID).
		Order("module ASC, sort_order ASC, id ASC").Find(&list).Error; err != nil {
		infra.InternalError(c, "读取细节卡失败")
		return
	}
	infra.Success(c, list)
}

// POST /api/author-ai/projects/:id/details
func CreateDetail(c *gin.Context) {
	p, ok := requireOwner(c, atou(c.Param("id")))
	if !ok {
		return
	}
	var in detailInput
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if !authoraim.ValidDetailModule(in.Module) {
		infra.BadRequest(c, "不支持的素材模块")
		return
	}
	var maxOrder int
	models.DB.Model(&authoraim.Detail{}).
		Where("project_id = ? AND module = ?", p.ID, in.Module).
		Select("COALESCE(MAX(sort_order), 0)").Scan(&maxOrder)
	d := &authoraim.Detail{
		ProjectID: p.ID, Module: in.Module,
		Title: strings.TrimSpace(in.Title), Content: in.Content,
		SortOrder: maxOrder + 1, Status: "draft",
	}
	if err := models.DB.Create(d).Error; err != nil {
		infra.InternalError(c, "创建细节卡失败")
		return
	}
	_ = compileMaterials(p.ID)
	infra.SuccessMessage(c, "已创建", d)
}

// PUT /api/author-ai/projects/:id/details/:did
func UpdateDetail(c *gin.Context) {
	p, ok := requireOwner(c, atou(c.Param("id")))
	if !ok {
		return
	}
	var d authoraim.Detail
	if err := models.DB.Where("project_id = ? AND id = ?", p.ID, atou(c.Param("did"))).First(&d).Error; err != nil {
		infra.NotFound(c, "细节卡不存在")
		return
	}
	var in detailInput
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if in.Module != "" {
		if !authoraim.ValidDetailModule(in.Module) {
			infra.BadRequest(c, "不支持的素材模块")
			return
		}
		d.Module = in.Module
	}
	d.Title = strings.TrimSpace(in.Title)
	d.Content = in.Content
	if in.Status != "" {
		d.Status = in.Status
	}
	if err := models.DB.Save(&d).Error; err != nil {
		infra.InternalError(c, "保存细节卡失败")
		return
	}
	_ = compileMaterials(p.ID)
	infra.SuccessMessage(c, "已保存", d)
}

// DELETE /api/author-ai/projects/:id/details/:did
func DeleteDetail(c *gin.Context) {
	p, ok := requireOwner(c, atou(c.Param("id")))
	if !ok {
		return
	}
	if err := models.DB.Where("project_id = ? AND id = ?", p.ID, atou(c.Param("did"))).
		Delete(&authoraim.Detail{}).Error; err != nil {
		infra.InternalError(c, "删除细节卡失败")
		return
	}
	_ = compileMaterials(p.ID)
	infra.SuccessMessage(c, "已删除", nil)
}

// compileMaterials 把结构化细节卡（真源）重编译为工程大字段（投影）。
// 既有 generate / timeline-check / publish 读的是大字段，这里保证它们始终拿到最新内容，
// 无需改动任何既有链路。单向编译：detail → material，不反向拆卡。
func compileMaterials(projectID uint) error {
	var list []authoraim.Detail
	if err := models.DB.Where("project_id = ?", projectID).
		Order("module ASC, sort_order ASC, id ASC").Find(&list).Error; err != nil {
		return err
	}
	byModule := map[string][]string{}
	for _, d := range list {
		txt := strings.TrimSpace(d.Content)
		if txt == "" {
			continue
		}
		title := strings.TrimSpace(d.Title)
		if title == "" {
			byModule[d.Module] = append(byModule[d.Module], txt)
		} else {
			byModule[d.Module] = append(byModule[d.Module], "【"+title+"】\n"+txt)
		}
	}
	join := func(m string) string { return strings.Join(byModule[m], "\n\n") }
	return models.DB.Model(&authoraim.NovelProject{}).Where("id = ?", projectID).
		Updates(map[string]interface{}{
			"setting":    join(authoraim.ModSetting),
			"characters": join(authoraim.ModCharacter),
			"outline":    join(authoraim.ModOutline),
			"timeline":   join(authoraim.ModTimeline),
			"storyline":  join(authoraim.ModStoryline),
		}).Error
}

// ---------- Anchor → Deconstruct：引子自动拆解成独立细节卡 ----------

// seedCard AI 拆解出的单张细节卡。
type seedCard struct {
	Module  string `json:"module"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

type fromSeedInput struct {
	Seed    string `json:"seed"` // 引子/话题；空则取工程已存的 SeedTopic
	ModelID uint   `json:"model_id"`
	// Replace 为 true 时清空该工程既有细节卡再落新卡；false（默认）追加，保留作者手改的卡片。
	Replace bool `json:"replace"`
}

// POST /api/author-ai/projects/:id/from-seed
// 一次 AI 调用把「一个话题/引子」拆解为四模块的独立细节卡（对标 OpenFic / MuseFlow 的
// 一句灵感自动推进 + NovelForge 的卡片式结构化）。每张卡可独立修改、独立微调。
func FromSeed(c *gin.Context) {
	p, ok := requireOwner(c, atou(c.Param("id")))
	if !ok {
		return
	}
	var in fromSeedInput
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	seed := strings.TrimSpace(in.Seed)
	if seed == "" {
		seed = strings.TrimSpace(p.SeedTopic)
	}
	if seed == "" {
		infra.BadRequest(c, "请先填写引子 / 话题")
		return
	}
	if strings.TrimSpace(p.SeedTopic) != seed {
		p.SeedTopic = seed
		_ = models.DB.Save(p).Error
	}
	userPrompt, perr := buildSeedPrompt(p, seed)
	if perr != nil {
		infra.InternalError(c, "组装拆解提示词失败")
		return
	}
	out, gerr := genCall(hctx.CurrentUserID(c), in.ModelID, userPrompt, p.ID)
	if gerr != nil {
		infra.InternalError(c, "AI 拆解失败: "+gerr.Error())
		return
	}
	cards := parseSeedCards(out)
	if len(cards) == 0 {
		infra.BadRequest(c, "AI 未返回可解析的细节卡，请换个说法重试")
		return
	}
	if in.Replace {
		if err := models.DB.Where("project_id = ?", p.ID).Delete(&authoraim.Detail{}).Error; err != nil {
			infra.InternalError(c, "清理旧细节卡失败")
			return
		}
	}
	created := []authoraim.Detail{}
	order := map[string]int{}
	for _, card := range cards {
		if !authoraim.ValidDetailModule(card.Module) || strings.TrimSpace(card.Content) == "" {
			continue
		}
		order[card.Module]++
		d := authoraim.Detail{
			ProjectID: p.ID, Module: card.Module,
			Title: strings.TrimSpace(card.Title), Content: strings.TrimSpace(card.Content),
			SortOrder: order[card.Module], Status: "generated",
		}
		if err := models.DB.Create(&d).Error; err != nil {
			infra.InternalError(c, "保存细节卡失败")
			return
		}
		created = append(created, d)
	}
	if err := compileMaterials(p.ID); err != nil {
		infra.InternalError(c, "重编译素材失败")
		return
	}
	_ = models.DB.First(p, p.ID).Error
	infra.SuccessMessage(c, "引子已拆解为细节卡", gin.H{"cards": created, "project": p})
}

// buildSeedPrompt 用 prompttpl 的 authorai-seed 模板（后台可改）；缺失时回落内置提示。
func buildSeedPrompt(p *authoraim.NovelProject, seed string) (string, error) {
	values := map[string]string{
		"NovelTitle":     p.Title,
		"Seed":           seed,
		"StyleDirective": styleDirective(p.StylePreset, p.StyleDetail),
		"ExistingContent": strings.TrimSpace(strings.Join([]string{
			"设定：" + clip(p.Setting, 500), "人物：" + clip(p.Characters, 500),
			"大纲：" + clip(p.Outline, 500), "时间线：" + clip(p.Timeline, 500),
		}, "\n")),
	}
	if t, err := prompttpl.GetByKey("authorai-seed"); err == nil && t != nil {
		return prompttpl.Render(t.Content, t.Vars, values).Text, nil
	}
	return "你是一位资深中文小说架构师。请围绕下面的引子 / 话题，把全书拆解成一张张可独立修改的细节卡。\n\n" +
		"作品名：" + values["NovelTitle"] + "\n引子：" + values["Seed"] + "\n" + values["StyleDirective"] +
		"\n\n已有素材（可忽略）：\n" + values["ExistingContent"] +
		"\n\n只输出 JSON 数组，不要解释、不要 Markdown 代码块。每个元素形如：\n" +
		`{"module":"setting|character|outline|timeline","title":"细节标题","content":"该细节的完整内容"}` +
		"\n覆盖四个模块，每模块 3-6 张卡；内容要具体、可独立修改、彼此自洽。", nil
}

// parseSeedCards 容错解析 AI 返回的 JSON（可能被包在 ```json 代码块或前后有解释文字）。
func parseSeedCards(raw string) []seedCard {
	s := strings.TrimSpace(raw)
	if i := strings.Index(s, "```"); i >= 0 {
		s = s[i+3:]
		if j := strings.Index(s, "```"); j >= 0 {
			s = s[:j]
		}
		s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "json"))
	}
	// 截取首个 JSON 数组
	start := strings.Index(s, "[")
	end := strings.LastIndex(s, "]")
	if start < 0 || end <= start {
		return nil
	}
	var cards []seedCard
	if err := json.Unmarshal([]byte(s[start:end+1]), &cards); err != nil {
		return nil
	}
	return cards
}

// ---------- Recur → Output：对单个细节做 ADRO 多轮微调 ----------

const (
	RefineSocratic = "socratic" // 苏格拉底：不直接改，用追问帮作者想清楚
	RefineFeynman  = "feynman"  // 费曼：用大白话复述，暴露含糊与自相矛盾
	RefineDirect   = "direct"   // 直接：按作者指令改写这一个细节
)

type refineTurn struct {
	Role    string `json:"role"` // user / assistant
	Content string `json:"content"`
}

type refineInput struct {
	Mode        string       `json:"mode"` // socratic / feynman / direct
	Instruction string       `json:"instruction"`
	History     []refineTurn `json:"history"` // 多轮微调记录（Recur）
	ModelID     uint         `json:"model_id"`
	// Apply 为 true 且 mode=direct 时，把改写结果直接写回该细节卡（Output）。
	Apply bool `json:"apply"`
}

// POST /api/author-ai/projects/:id/details/:did/refine
// 作者对「每一个细节」与 AI 多轮微调。三种模式共用一份后台可改模板 authorai-refine，
// 模式语义由模板定义（提示词工程），Go 只做轻量校验与上下文拼装，不当文法裁判。
func RefineDetail(c *gin.Context) {
	p, ok := requireOwner(c, atou(c.Param("id")))
	if !ok {
		return
	}
	var d authoraim.Detail
	if err := models.DB.Where("project_id = ? AND id = ?", p.ID, atou(c.Param("did"))).First(&d).Error; err != nil {
		infra.NotFound(c, "细节卡不存在")
		return
	}
	var in refineInput
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	mode := in.Mode
	if mode == "" {
		mode = RefineDirect
	}
	if mode != RefineSocratic && mode != RefineFeynman && mode != RefineDirect {
		infra.BadRequest(c, "不支持的微调模式")
		return
	}
	// 上下文：只带当前细节 + 同模块其它卡的标题（动态 RAG 的轻量版，避免长篇超 token）
	var siblings []authoraim.Detail
	_ = models.DB.Where("project_id = ? AND module = ? AND id <> ?", p.ID, d.Module, d.ID).
		Order("sort_order ASC").Find(&siblings).Error
	sibTitles := make([]string, 0, len(siblings))
	for _, s := range siblings {
		if t := strings.TrimSpace(s.Title); t != "" {
			sibTitles = append(sibTitles, t)
		}
	}
	historyText := ""
	for _, t := range in.History {
		role := "作者"
		if t.Role == "assistant" {
			role = "AI"
		}
		historyText += role + "：" + clip(strings.TrimSpace(t.Content), 800) + "\n"
	}
	values := map[string]string{
		"NovelTitle":     p.Title,
		"SeedDirective":  clip(strings.TrimSpace(p.SeedTopic), 800),
		"StyleDirective": styleDirective(p.StylePreset, p.StyleDetail),
		"ModuleLabel":    authoraim.ModuleLabel(d.Module),
		"DetailTitle":    strings.TrimSpace(d.Title),
		"DetailContent":  clip(strings.TrimSpace(d.Content), 4000),
		"SiblingTitles":  strings.Join(sibTitles, "、"),
		"Mode":           mode,
		"Instruction":    clip(strings.TrimSpace(in.Instruction), 1500),
		"History":        strings.TrimSpace(historyText),
	}
	var userPrompt string
	if t, err := prompttpl.GetByKey("authorai-refine"); err == nil && t != nil {
		userPrompt = prompttpl.Render(t.Content, t.Vars, values).Text
	} else {
		userPrompt = defaultRefinePrompt(mode, values)
	}
	reply, gerr := genCall(hctx.CurrentUserID(c), in.ModelID, userPrompt, p.ID)
	if gerr != nil {
		infra.InternalError(c, "AI 微调失败: "+gerr.Error())
		return
	}
	reply = strings.TrimSpace(reply)
	applied := false
	if in.Apply && mode == RefineDirect && reply != "" {
		d.Content = reply
		d.Status = "refined"
		if err := models.DB.Save(&d).Error; err == nil {
			applied = true
			_ = compileMaterials(p.ID)
		}
	}
	infra.SuccessMessage(c, "微调完成", gin.H{
		"mode": mode, "reply": reply, "applied": applied, "detail": d,
	})
}

func defaultRefinePrompt(mode string, v map[string]string) string {
	base := "作品《" + v["NovelTitle"] + "》" + v["StyleDirective"] +
		"\n全书引子：" + v["SeedDirective"] +
		"\n当前细节所属模块：" + v["ModuleLabel"] +
		"\n细节标题：" + v["DetailTitle"] + "\n细节内容：\n" + v["DetailContent"] +
		"\n同模块其它细节（仅供保持自洽，不要展开）：" + v["SiblingTitles"] +
		"\n\n此前多轮微调记录：\n" + v["History"] +
		"\n作者本轮的话：" + v["Instruction"] + "\n\n"
	switch mode {
	case RefineSocratic:
		return base + "【苏格拉底式追问】不要改写内容，也不要给出结论。针对这个细节中未经交代的前提、" +
			"含糊的动机、可能与其它细节冲突之处，提出 2-3 个具体的、能帮作者自己想清楚的反问。" +
			"每个追问都要引用细节里的原话。只输出追问，编号列出。"
	case RefineFeynman:
		return base + "【费曼式复述】用大白话把这个细节从头讲一遍，像讲给完全没读过这本书的人听。" +
			"凡是讲不通、需要靠术语或「反正就是这样」糊过去的地方，明确指出并列成「含糊点」。" +
			"不要改写原文，不要客套。分「复述」「含糊点」两段。"
	default:
		return base + "【按作者指令改写】只改写这一个细节，保持同模块其它细节与全书引子自洽。" +
			"严格遵守作者本轮的话；未要求改动的部分保持原样。只输出改写后的细节正文，不要解释、不要标题。"
	}
}

// 便捷：供前端查询可用微调模式（避免前端硬编码语义）。
// GET /api/author-ai/projects/:id/refine-modes
func RefineModes(c *gin.Context) {
	infra.Success(c, []map[string]string{
		{"value": RefineSocratic, "label": "苏格拉底式追问", "hint": "AI 不改稿，用反问帮你把没想清楚的地方问出来"},
		{"value": RefineFeynman, "label": "费曼式复述", "hint": "AI 用大白话复述这条细节，指出讲不通的地方"},
		{"value": RefineDirect, "label": "直接改写", "hint": "按你的指令只改这一条，可一键应用"},
	})
}
