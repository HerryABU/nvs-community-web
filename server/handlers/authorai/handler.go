// Package authorai 作者工作台内置 AI 写作流的处理器。
//
// 与 AI 中台（handlers/aiworkbench）完全独立：数据落在 authorai_projects 表，
// 仅借用 aiclient 调上游生成，不读写任何中台会话 / 智能体表。
package authorai

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"nvs-server/aiclient"
	"nvs-server/config"
	"nvs-server/handlers/hctx"
	"nvs-server/models"
	aim "nvs-server/models/ai"
	aihub "nvs-server/models/aihub"
	authoraim "nvs-server/models/authorai"
	"nvs-server/prompttpl"
	"nvs-server/utils/infra"

	"github.com/gin-gonic/gin"
)

// 风格库预设
const (
	StyleRealism    = "realism"     // 现实主义
	StyleHardSciFi  = "hard_sci_fi" // 硬科幻
	StyleLightNovel = "light_novel" // 轻小说
	StyleCustom     = "custom"      // 自定义（以 StyleDetail 为准）
)

// 可生成的组件
const (
	CompSetting   = "setting"   // 世界观设定
	CompCharacter = "character" // 人物设定
	CompOutline   = "outline"   // 大纲
	CompTimeline  = "timeline"  // 大事年表（含嵌套循环时间线）
	CompBody      = "body"      // 章节正文
)

// ---------- 输入结构 ----------

type createProjectInput struct {
	Title       string `json:"title"`
	StylePreset string `json:"style_preset"`
	StyleDetail string `json:"style_detail"`
	SeedTopic   string `json:"seed_topic"` // 引子/话题（ADRO Anchor）
}

type updateProjectInput struct {
	Title          *string `json:"title"`
	Setting        *string `json:"setting"`
	Characters     *string `json:"characters"`
	Timeline       *string `json:"timeline"`
	Outline        *string `json:"outline"`
	StylePreset    *string `json:"style_preset"`
	StyleDetail    *string `json:"style_detail"`
	SeedTopic      *string `json:"seed_topic"` // 引子/话题（ADRO Anchor）
	BodyDraft      *string `json:"body_draft"`
	SettingInput   *string `json:"setting_input"`
	CharacterInput *string `json:"character_input"`
	OutlineInput   *string `json:"outline_input"`
	TimelineInput  *string `json:"timeline_input"`
	Status         *string `json:"status"`
}

type generateInput struct {
	Component string `json:"component"` // setting/character/outline/timeline/body
	ModelID   uint   `json:"model_id"`  // 0=自动挑默认对话模型
	Prompt    string `json:"prompt"`    // 用户补充要求（可选）
}

// ---------- 权限辅助 ----------

func requireOwner(c *gin.Context, id uint) (*authoraim.NovelProject, bool) {
	userID := hctx.CurrentUserID(c)
	var p authoraim.NovelProject
	if err := models.DB.First(&p, id).Error; err != nil {
		infra.NotFound(c, "写作工程不存在")
		return nil, false
	}
	if p.UserID != userID {
		infra.Forbidden(c, "无权访问此写作工程")
		return nil, false
	}
	return &p, true
}

// ---------- 工程 CRUD ----------

// GET /api/author-ai/projects
func ListProjects(c *gin.Context) {
	userID := hctx.CurrentUserID(c)
	var list []authoraim.NovelProject
	if err := models.DB.Where("user_id = ?", userID).Order("updated_at DESC").Find(&list).Error; err != nil {
		infra.InternalError(c, "获取工程列表失败")
		return
	}
	infra.Success(c, list)
}

// POST /api/author-ai/projects
func CreateProject(c *gin.Context) {
	userID := hctx.CurrentUserID(c)
	var in createProjectInput
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = "未命名写作工程"
	}
	p := &authoraim.NovelProject{
		UserID:      userID,
		Title:       title,
		StylePreset: in.StylePreset,
		StyleDetail: in.StyleDetail,
		SeedTopic:   strings.TrimSpace(in.SeedTopic),
		Status:      "draft",
	}
	if err := models.DB.Create(p).Error; err != nil {
		infra.InternalError(c, "创建工程失败")
		return
	}
	infra.SuccessMessage(c, "创建成功", p)
}

// GET /api/author-ai/projects/:id
func GetProject(c *gin.Context) {
	id := atou(c.Param("id"))
	p, ok := requireOwner(c, id)
	if !ok {
		return
	}
	infra.Success(c, p)
}

// PUT /api/author-ai/projects/:id  （保存进度与设置）
func UpdateProject(c *gin.Context) {
	id := atou(c.Param("id"))
	p, ok := requireOwner(c, id)
	if !ok {
		return
	}
	var in updateProjectInput
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if in.Title != nil {
		p.Title = strings.TrimSpace(*in.Title)
	}
	if in.Setting != nil {
		p.Setting = *in.Setting
	}
	if in.Characters != nil {
		p.Characters = *in.Characters
	}
	if in.Timeline != nil {
		p.Timeline = *in.Timeline
	}
	if in.Outline != nil {
		p.Outline = *in.Outline
	}
	if in.StylePreset != nil {
		p.StylePreset = *in.StylePreset
	}
	if in.StyleDetail != nil {
		p.StyleDetail = *in.StyleDetail
	}
	if in.SeedTopic != nil {
		p.SeedTopic = strings.TrimSpace(*in.SeedTopic)
	}
	if in.BodyDraft != nil {
		p.BodyDraft = *in.BodyDraft
	}
	if in.SettingInput != nil {
		p.SettingInput = *in.SettingInput
	}
	if in.CharacterInput != nil {
		p.CharacterInput = *in.CharacterInput
	}
	if in.OutlineInput != nil {
		p.OutlineInput = *in.OutlineInput
	}
	if in.TimelineInput != nil {
		p.TimelineInput = *in.TimelineInput
	}
	if in.Status != nil {
		p.Status = *in.Status
	}
	if err := models.DB.Save(p).Error; err != nil {
		infra.InternalError(c, "保存失败")
		return
	}
	infra.SuccessMessage(c, "已保存", p)
}

// DELETE /api/author-ai/projects/:id
func DeleteProject(c *gin.Context) {
	id := atou(c.Param("id"))
	p, ok := requireOwner(c, id)
	if !ok {
		return
	}
	if err := models.DB.Delete(p).Error; err != nil {
		infra.InternalError(c, "删除失败")
		return
	}
	infra.SuccessMessage(c, "已删除", nil)
}

// ---------- AI 生成 ----------

// POST /api/author-ai/projects/:id/generate
func GenerateComponent(c *gin.Context) {
	id := atou(c.Param("id"))
	p, ok := requireOwner(c, id)
	if !ok {
		return
	}
	var in generateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	content, err := generateViaTemplate(p, in.Component, in.Prompt)
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	genContent, gerr := genCall(hctx.CurrentUserID(c), in.ModelID, content, p.ID)
	if gerr != nil {
		infra.InternalError(c, "AI 生成失败: "+gerr.Error())
		return
	}
	// 写回对应字段（body 累加到草稿末尾，其余覆盖）
	switch in.Component {
	case CompSetting:
		p.Setting = genContent
	case CompCharacter:
		p.Characters = genContent
	case CompOutline:
		p.Outline = genContent
	case CompTimeline:
		p.Timeline = genContent
	case CompBody:
		if strings.TrimSpace(p.BodyDraft) == "" {
			p.BodyDraft = genContent
		} else {
			p.BodyDraft = p.BodyDraft + "\n\n" + genContent
		}
	}
	if err := models.DB.Save(p).Error; err != nil {
		infra.InternalError(c, "保存生成结果失败")
		return
	}
	infra.SuccessMessage(c, "生成成功", gin.H{"component": in.Component, "content": genContent})
}

// genCall 以通用系统提示包装一次 AI 调用（模板渲染结果作为用户提示，统一风格基调与输出约束）。
func genCall(userID, modelID uint, userText string, refID uint) (string, error) {
	return callAI(userID, modelID,
		"你是专业的中文小说创作助手，请严格按用户给出的要求输出，不解释、不评价，不重复指令本身。",
		userText, refID)
}

// materialTplKey 素材/正文组件 → 对应 prompttpl 模板 key（与 ai_prompt_templates 表联动，可后台自定义）。
var materialTplKey = map[string]string{
	CompSetting:   "authorai-setting",
	CompCharacter: "authorai-character",
	CompOutline:   "authorai-outline",
	CompTimeline:  "authorai-timeline",
	CompBody:      "authorai-body",
}

// materialExistingAndInput 返回组件的「已有内容」与「用户补充要求」。
func materialExistingAndInput(p *authoraim.NovelProject, comp string) (existing, input string) {
	switch comp {
	case CompSetting:
		return p.Setting, p.SettingInput
	case CompCharacter:
		return p.Characters, p.CharacterInput
	case CompOutline:
		return p.Outline, p.OutlineInput
	case CompTimeline:
		return p.Timeline, p.TimelineInput
	case CompBody:
		return p.BodyDraft, ""
	}
	return "", ""
}

// generateViaTemplate 用 prompttpl 渲染指定组件的生成提示词。
// 关键：是否「从零创作」还是「在已有基础上补充」由 Go 在组装时决定（写入 TaskMode 变量），
// 模板内不写条件分支——AI 每次只面对一个明确任务。
// 模板缺失时回落到内置 buildGeneratePrompt，保证健壮性。
func generateViaTemplate(p *authoraim.NovelProject, comp, extraOverride string) (string, error) {
	key, ok := materialTplKey[comp]
	if !ok {
		return "", fmt.Errorf("不支持的生成组件: %s", comp)
	}
	existing, input := materialExistingAndInput(p, comp)
	extra := strings.TrimSpace(input)
	if strings.TrimSpace(extraOverride) != "" {
		extra = strings.TrimSpace(extraOverride)
	}
	// 引子（ADRO Anchor）作为全书锚点注入每个组件：即使模板是旧版、后台改过的版本，
	// 也通过 ExtraRequirements 前缀生效；新模板可用 {{SeedDirective}} 单独放置。
	seedDirective := ""
	if s := strings.TrimSpace(p.SeedTopic); s != "" {
		seedDirective = "【全书引子 / 话题锚点】" + clip(s, 1200) +
			"（所有设定、人物、大纲、时间线必须围绕这一引子展开并保持自洽）"
		extra = seedDirective + "\n" + extra
	}
	taskMode := "从零创作完整内容"
	if strings.TrimSpace(existing) != "" {
		taskMode = "在已有内容基础上补充与完善，保持与已有内容严格自洽"
	}
	t, terr := prompttpl.GetByKey(key)
	if terr != nil || t == nil {
		// 模板未播种：回落到内置默认提示（仅取 user 部分作为渲染结果）
		_, usr := buildGeneratePrompt(p, comp, extra)
		return usr, nil
	}
	values := map[string]string{
		"NovelTitle":        p.Title,
		"StyleDirective":    styleDirective(p.StylePreset, p.StyleDetail),
		"ExtraRequirements": extra,
		"ExistingContent":   strings.TrimSpace(existing),
		"TaskMode":          taskMode,
		"SeedDirective":     seedDirective,
	}
	res := prompttpl.Render(t.Content, t.Vars, values)
	return res.Text, nil
}

// 构建各组件的 system / user 提示词（注入风格库 + 详细风格要求）
func buildGeneratePrompt(p *authoraim.NovelProject, comp, extra string) (system, user string) {
	style := styleDirective(p.StylePreset, p.StyleDetail)
	switch comp {
	case CompSetting:
		return "你是一位资深的世界观架构师。请为长篇中文小说设计完整、自洽的世界观设定，覆盖时代背景、地理/社会结构、力量体系或核心矛盾、关键势力。" + style,
			"作品标题：" + p.Title + "。" + extra
	case CompCharacter:
		return "你是一位资深的中文小说角色设定师。请设计 3-6 个有血有肉的主要人物卡，每卡含：姓名与称谓、外貌（3 个可视觉化细节）、性格与致命缺陷、出身与当前处境、核心欲望/深层需求、秘密（读者后知）、与其他角色的关系张力、说话方式、三个可推进情节的行动选项。" + style,
			"作品标题：" + p.Title + "；已有设定：" + clip(p.Setting, 800) + "。" + extra
	case CompOutline:
		return "你是一位擅长结构化的中文小说编剧。请输出分卷/分章大纲，标注每章核心事件与人物弧光，保持节奏与伏笔回收。" + style,
			"作品标题：" + p.Title + "；设定：" + clip(p.Setting, 600) + "；人物：" + clip(p.Characters, 600) + "。" + extra
	case CompTimeline:
		return "你是一位严谨的年代学家。请输出作品大事年表，并显式标注任何「嵌套循环 / 时间循环」结构（用 [LOOP] 标记循环起点与终点、循环次数与变量）。使用清晰的年表条目。" + style,
			"作品标题：" + p.Title + "；设定：" + clip(p.Setting, 600) + "。" + extra
	case CompBody:
		return "你是一位专业的中文小说写作助手。请基于给定设定、人物与大纲续写一章正文，保持人物、语气、设定一致，只输出正文。" + style,
			"标题：" + p.Title + "；设定：" + clip(p.Setting, 400) + "；人物：" + clip(p.Characters, 400) + "；大纲：" + clip(p.Outline, 400) + "；时间线：" + clip(p.Timeline, 400) + "。" + extra
	default:
		return "", ""
	}
}

func styleDirective(preset, detail string) string {
	base := ""
	switch preset {
	case StyleRealism:
		base = "风格基调：现实主义，克制抒情、重细节与逻辑、贴近生活真实。"
	case StyleHardSciFi:
		base = "风格基调：硬科幻，设定需有科学内洽性，术语准确，想象有边界。"
	case StyleLightNovel:
		base = "风格基调：轻小说，节奏轻快、对话鲜明、适当吐槽与萌点，兼具热血与日常。"
	case StyleCustom:
		base = "风格基调：自定义。"
	default:
		base = "风格基调：自然流畅的中文小说叙事。"
	}
	if strings.TrimSpace(detail) != "" {
		base += " 作者补充的风格要求：" + detail + "。"
	}
	return base
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// callAI 解析模型 + 获取密钥 + 调上游 + 记调用日志（复用 aiclient，但与中台数据隔离）。
func callAI(userID, modelID uint, system, user string, refID uint) (string, error) {
	m, err := aim.GetEnabledAiModelOfKind(modelID, aim.AiKindChat)
	if err != nil {
		return "", fmt.Errorf("解析模型失败: %w", err)
	}
	if m == nil {
		return "", fmt.Errorf("未配置可用的对话模型")
	}
	lease, kerr := aihub.AcquireModelKey(m)
	if kerr != nil || lease == nil {
		return "", fmt.Errorf("获取模型密钥失败")
	}
	apiKey := lease.Key
	client := aiclient.NewClient(m.BaseURL, apiKey, 180*time.Second)
	req := aiclient.ChatRequest{
		Model:       m.ModelID,
		Messages:    []aiclient.Message{{Role: "system", Content: system}, {Role: "user", Content: user}},
		MaxTokens:   4000,
		Temperature: 0.8,
	}
	var content string
	var tin, tout int
	if aim.NormalizeAiApiKind(m.ApiKind) == aim.AiApiClaude {
		content, tin, tout, err = client.CompleteClaude(req)
	} else {
		content, tin, tout, err = client.Complete(req)
	}
	if err != nil {
		return "", err
	}
	_, _ = aim.CreateAiCallLog(userID, m, "author_ai", "project", refID, tin, tout, false)
	return content, nil
}

// ---------- 生成小说 ----------

// POST /api/author-ai/projects/:id/publish
// 把写作工程落地成一个 Novel（草稿），并回写 project.novel_id 便于后续在普通编辑器继续。
func PublishNovel(c *gin.Context) {
	id := atou(c.Param("id"))
	p, ok := requireOwner(c, id)
	if !ok {
		return
	}
	userID := hctx.CurrentUserID(c)
	if strings.TrimSpace(p.Title) == "" {
		infra.BadRequest(c, "请先填写作品标题")
		return
	}
	summary := clip(p.Setting, 500)
	nv := &models.Novel{
		AuthorID:       userID,
		Title:          p.Title,
		Category:       "其他",
		Tags:           "[]",
		Summary:        summary,
		Mode:           "prose",
		Status:         "draft",
		SourceType:     "original",
		CreationMethod: "human_ai_assisted",
		ReadPermission: "public",
	}
	if err := models.CreateNovel(nv); err != nil {
		infra.InternalError(c, "生成小说失败")
		return
	}
	// 维持与 CreateNovel 一致的文件落地（作者目录 + index.json），避免普通编辑器读取缺数据。
	authorDir := filepath.Join(config.NovelDataDir, "authors", fmt.Sprintf("%d", userID))
	novelDir := filepath.Join(authorDir, fmt.Sprintf("%d", nv.ID))
	_ = os.MkdirAll(novelDir, 0755)
	indexPath := filepath.Join(novelDir, "index.json")
	indexData := map[string]interface{}{"novel_id": nv.ID, "title": nv.Title, "chapters": []interface{}{}}
	if b, e := json.MarshalIndent(indexData, "", "  "); e == nil {
		_ = os.WriteFile(indexPath, b, 0644)
	}
	p.NovelID = nv.ID
	models.DB.Save(p)

	infra.SuccessMessage(c, "已生成小说（草稿），可在普通编辑器继续创作", nv)
}

// atou 解析路径参数 ID（非法返回 0）。
func atou(s string) uint {
	var v uint
	fmt.Sscanf(s, "%d", &v)
	return v
}
