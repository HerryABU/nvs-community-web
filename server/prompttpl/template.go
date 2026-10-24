package prompttpl

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"nvs-server/models"

	"gorm.io/gorm"
)

// ==================== 场景化提示词模板引擎（需求 §13.1） ====================
// 定位：开发者提供「模板引擎 + 类型文学默认框架」，站长可在后台自定义模板（含爽文模板，
// 但相应运营与合规责任由站长承担）。
//
// 能力：
//   - 变量注入：{{变量}} 占位，声明变量名/标签/是否必填/默认值/长度上限；
//   - 版本管理：每次保存快照上一版，可查看历史并回滚；
//   - 按场景挂载：novel_chapter（章节编辑器）/ blog（博客编辑器）/ chat（AI 工作台）；
//   - 测试预览：后台可填入变量实时渲染，不落库；
//   - 启用/禁用 + 优先级排序。
//
// 渲染安全：变量值只做文本替换（不解析 HTML/模板指令），并按 max_len 截断，
// 避免用户/上游把模板当成代码执行或塞入超长内容撑爆上下文。

const (
	SceneNovelChapter = "novel_chapter"
	// SceneNovelScript 话本气泡体专用模板（「角色：台词」形态的续写/台词/场景）
	SceneNovelScript = "novel_script"
	SceneBlog        = "blog"
	SceneChat        = "chat"

	KindChat  = "chat"
	KindImage = "image"
	KindVideo = "video"

	maxTitleRunes   = 64
	maxContentRunes = 8000
	maxVarRunes     = 20000
)

// TemplateVar 模板变量声明
type TemplateVar struct {
	Name     string `json:"name"`     // 占位名（与 content 中 {{name}} 对应）
	Label    string `json:"label"`    // 展示名（编辑区表单标签）
	Required bool   `json:"required"` // 是否必填
	Default  string `json:"default"`  // 默认值
	MaxLen   int    `json:"max_len"`  // 单值长度上限（0 = 默认 4000）
}

// Template 提示词模板
type Template struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Key       string    `gorm:"size:64;uniqueIndex;not null" json:"key"`
	Title     string    `gorm:"size:64;not null" json:"title"`
	Scene     string    `gorm:"size:24;not null;default:novel_chapter;index" json:"scene"`
	Category  string    `gorm:"size:32;default:'';index" json:"category"`
	Kind      string    `gorm:"size:16;not null;default:chat" json:"kind"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	Variables string    `gorm:"type:text" json:"-"` // JSON []TemplateVar
	Version   int       `gorm:"not null;default:1" json:"version"`
	Enabled   bool      `gorm:"not null;default:true;index" json:"enabled"`
	Priority  int       `gorm:"not null;default:0" json:"priority"`
	Builtin   bool      `gorm:"not null;default:false" json:"builtin"`
	Note      string    `gorm:"size:256;default:''" json:"note"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Vars []TemplateVar `gorm:"-" json:"variables"`
}

func (Template) TableName() string { return "ai_prompt_templates" }

// TemplateVersion 版本快照（append-only）
type TemplateVersion struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	TemplateID uint      `gorm:"not null;index" json:"template_id"`
	Version    int       `gorm:"not null" json:"version"`
	Content    string    `gorm:"type:text;not null" json:"content"`
	Variables  string    `gorm:"type:text" json:"-"`
	Note       string    `gorm:"size:256;default:''" json:"note"`
	CreatedAt  time.Time `json:"created_at"`

	Vars []TemplateVar `gorm:"-" json:"variables"`
}

func (TemplateVersion) TableName() string { return "ai_prompt_template_versions" }

// Migrate 建表（独立功能包，由 main 在主导航迁移后调用）
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&Template{}, &TemplateVersion{})
}

// ---------- 归一化与校验 ----------

func NormalizeScene(s string) string {
	switch strings.TrimSpace(s) {
	case SceneBlog:
		return SceneBlog
	case SceneChat:
		return SceneChat
	case SceneNovelScript:
		return SceneNovelScript
	default:
		return SceneNovelChapter
	}
}

func normalizeKind(k string) string {
	switch strings.TrimSpace(k) {
	case KindImage:
		return KindImage
	case KindVideo:
		return KindVideo
	default:
		return KindChat
	}
}

// normalizeVars 变量声明清洗：名字只允许字母/数字/下划线/汉字，去重
func normalizeVars(vars []TemplateVar) []TemplateVar {
	out := make([]TemplateVar, 0, len(vars))
	seen := map[string]bool{}
	for _, v := range vars {
		name := strings.TrimSpace(v.Name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		if v.Label == "" {
			v.Label = name
		}
		if v.MaxLen <= 0 || v.MaxLen > maxVarRunes {
			v.MaxLen = 4000
		}
		v.Name = name
		out = append(out, v)
	}
	return out
}

// decodeVars 解析 JSON 变量声明
func decodeVars(raw string) []TemplateVar {
	if strings.TrimSpace(raw) == "" {
		return []TemplateVar{}
	}
	var vars []TemplateVar
	if err := json.Unmarshal([]byte(raw), &vars); err != nil {
		return []TemplateVar{}
	}
	return normalizeVars(vars)
}

func encodeVars(vars []TemplateVar) string {
	b, err := json.Marshal(normalizeVars(vars))
	if err != nil {
		return "[]"
	}
	return string(b)
}

// ---------- 渲染引擎 ----------

var placeholderRe = regexp.MustCompile(`\{\{\s*([\p{Han}A-Za-z0-9_]+)\s*\}\}`)

// RenderResult 渲染结果
type RenderResult struct {
	Text    string   `json:"text"`
	Missing []string `json:"missing"` // 未提供且必填的变量
	Unused  []string `json:"unused"`  // 传了但模板里没用到的变量
	Used    []string `json:"used"`
}

// Render 纯文本替换渲染（不执行任何模板指令，防注入）
func Render(content string, vars []TemplateVar, values map[string]string) RenderResult {
	declared := map[string]TemplateVar{}
	for _, v := range vars {
		declared[v.Name] = v
	}
	used := map[string]bool{}
	missing := []string{}
	missingSeen := map[string]bool{}

	out := placeholderRe.ReplaceAllStringFunc(content, func(m string) string {
		sub := placeholderRe.FindStringSubmatch(m)
		if len(sub) != 2 {
			return m
		}
		name := sub[1]
		used[name] = true
		val, ok := values[name]
		if !ok || strings.TrimSpace(val) == "" {
			// 回落到默认值
			if d, has := declared[name]; has && strings.TrimSpace(d.Default) != "" {
				return clampText(d.Default, d.MaxLen)
			}
			if !missingSeen[name] {
				required := true
				if d, has := declared[name]; has {
					required = d.Required
				}
				if required {
					missing = append(missing, name)
					missingSeen[name] = true
				}
			}
			return m // 保留占位，便于前端高亮待填
		}
		limit := 4000
		if d, has := declared[name]; has && d.MaxLen > 0 {
			limit = d.MaxLen
		}
		return clampText(val, limit)
	})

	unused := []string{}
	for name := range values {
		if !used[name] {
			unused = append(unused, name)
		}
	}
	usedList := make([]string, 0, len(used))
	for name := range used {
		usedList = append(usedList, name)
	}
	return RenderResult{Text: out, Missing: missing, Unused: unused, Used: usedList}
}

// clampText 截断到 maxLen 个字符（rune），并去掉 NUL 等控制字符
func clampText(s string, maxLen int) string {
	s = strings.Map(func(r rune) rune {
		if r == 0 {
			return -1
		}
		return r
	}, s)
	if maxLen <= 0 {
		maxLen = 4000
	}
	r := []rune(s)
	if len(r) > maxLen {
		return string(r[:maxLen])
	}
	return s
}

// ---------- 查询 ----------

// List 列出模板（includeDisabled = 后台视角）
func List(scene, kind string, includeDisabled bool) ([]Template, error) {
	q := models.DB.Model(&Template{})
	if s := strings.TrimSpace(scene); s != "" {
		q = q.Where("scene = ?", NormalizeScene(s))
	}
	if k := strings.TrimSpace(kind); k != "" {
		q = q.Where("kind = ?", normalizeKind(k))
	}
	if !includeDisabled {
		q = q.Where("enabled = ?", true)
	}
	var list []Template
	if err := q.Order("priority DESC, id ASC").Limit(200).Find(&list).Error; err != nil {
		return nil, err
	}
	for i := range list {
		list[i].Vars = decodeVars(list[i].Variables)
	}
	return list, nil
}

// Get 取单个模板
func Get(id uint) (*Template, error) {
	var t Template
	if err := models.DB.Where("id = ?", id).First(&t).Error; err != nil {
		return nil, fmt.Errorf("模板不存在")
	}
	t.Vars = decodeVars(t.Variables)
	return &t, nil
}

// Categories 场景下的分类（去重）
func Categories(scene string) []string {
	var cats []string
	q := models.DB.Model(&Template{}).Where("enabled = ? AND category <> ''", true)
	if s := strings.TrimSpace(scene); s != "" {
		q = q.Where("scene = ?", NormalizeScene(s))
	}
	q.Distinct().Order("category ASC").Pluck("category", &cats)
	return cats
}

// ---------- 增删改（带版本快照） ----------

// Input 保存入参
type Input struct {
	Key      string        `json:"key"`
	Title    string        `json:"title"`
	Scene    string        `json:"scene"`
	Category string        `json:"category"`
	Kind     string        `json:"kind"`
	Content  string        `json:"content"`
	Vars     []TemplateVar `json:"variables"`
	Enabled  *bool         `json:"enabled"`
	Priority int           `json:"priority"`
	Note     string        `json:"note"`
}

func (in *Input) validate() error {
	in.Title = strings.TrimSpace(in.Title)
	in.Content = strings.TrimSpace(in.Content)
	if in.Title == "" {
		return fmt.Errorf("模板标题不能为空")
	}
	if len([]rune(in.Title)) > maxTitleRunes {
		return fmt.Errorf("标题不能超过 %d 个字符", maxTitleRunes)
	}
	if in.Content == "" {
		return fmt.Errorf("模板内容不能为空")
	}
	if len([]rune(in.Content)) > maxContentRunes {
		return fmt.Errorf("模板内容不能超过 %d 个字符", maxContentRunes)
	}
	in.Scene = NormalizeScene(in.Scene)
	in.Kind = normalizeKind(in.Kind)
	in.Vars = normalizeVars(in.Vars)
	return nil
}

// Create 新建模板
func Create(in Input) (*Template, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	key := strings.TrimSpace(in.Key)
	if key == "" {
		key = "custom-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	var n int64
	models.DB.Model(&Template{}).Where("key = ?", key).Count(&n)
	if n > 0 {
		return nil, fmt.Errorf("模板标识（key）已存在")
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	t := &Template{
		Key: key, Title: in.Title, Scene: in.Scene, Category: strings.TrimSpace(in.Category),
		Kind: in.Kind, Content: in.Content, Variables: encodeVars(in.Vars),
		Version: 1, Enabled: enabled, Priority: in.Priority, Note: strings.TrimSpace(in.Note),
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := models.DB.Create(t).Error; err != nil {
		return nil, fmt.Errorf("创建失败")
	}
	// enabled 有默认值 true，false 需精确写回
	if err := models.DB.Model(&Template{}).Where("id = ?", t.ID).
		Update("enabled", enabled).Error; err != nil {
		return nil, err
	}
	t.Vars = in.Vars
	return t, nil
}

// Update 修改模板：先把当前版本快照进历史，再 version+1 落库
func Update(id uint, in Input) (*Template, error) {
	t, err := Get(id)
	if err != nil {
		return nil, err
	}
	if err := in.validate(); err != nil {
		return nil, err
	}
	// 快照（append-only）
	snapshot := &TemplateVersion{
		TemplateID: t.ID, Version: t.Version, Content: t.Content,
		Variables: t.Variables, Note: "保存前自动快照", CreatedAt: time.Now(),
	}
	if err := models.DB.Create(snapshot).Error; err != nil {
		return nil, fmt.Errorf("版本快照失败")
	}
	fields := map[string]interface{}{
		"title": in.Title, "scene": in.Scene, "category": strings.TrimSpace(in.Category),
		"kind": in.Kind, "content": in.Content, "variables": encodeVars(in.Vars),
		"version": t.Version + 1, "priority": in.Priority, "note": strings.TrimSpace(in.Note),
		"updated_at": time.Now(),
	}
	if in.Enabled != nil {
		fields["enabled"] = *in.Enabled
	}
	if strings.TrimSpace(in.Key) != "" && in.Key != t.Key {
		fields["key"] = strings.TrimSpace(in.Key)
	}
	if err := models.DB.Model(&Template{}).Where("id = ?", id).Updates(fields).Error; err != nil {
		return nil, fmt.Errorf("保存失败")
	}
	return Get(id)
}

// Delete 删除（内置模板禁止删除，只能禁用）
func Delete(id uint) error {
	t, err := Get(id)
	if err != nil {
		return err
	}
	if t.Builtin {
		return fmt.Errorf("内置模板不可删除，可改为禁用")
	}
	return models.DB.Where("id = ?", id).Delete(&Template{}).Error
}

// ListVersions 历史版本（新→旧）
func ListVersions(templateID uint) ([]TemplateVersion, error) {
	var list []TemplateVersion
	if err := models.DB.Where("template_id = ?", templateID).
		Order("version DESC, id DESC").Limit(50).Find(&list).Error; err != nil {
		return nil, err
	}
	for i := range list {
		list[i].Vars = decodeVars(list[i].Variables)
	}
	return list, nil
}

// Restore 回滚到指定版本（当前内容先快照，再覆盖并 version+1）
func Restore(templateID uint, version int) (*Template, error) {
	t, err := Get(templateID)
	if err != nil {
		return nil, err
	}
	var v TemplateVersion
	if err := models.DB.Where("template_id = ? AND version = ?", templateID, version).
		First(&v).Error; err != nil {
		return nil, fmt.Errorf("版本不存在")
	}
	// 当前内容入历史
	_ = models.DB.Create(&TemplateVersion{
		TemplateID: t.ID, Version: t.Version, Content: t.Content,
		Variables: t.Variables, Note: "回滚前自动快照", CreatedAt: time.Now(),
	}).Error
	if err := models.DB.Model(&Template{}).Where("id = ?", templateID).Updates(map[string]interface{}{
		"content": v.Content, "variables": v.Variables,
		"version": t.Version + 1, "updated_at": time.Now(),
	}).Error; err != nil {
		return nil, fmt.Errorf("回滚失败")
	}
	return Get(templateID)
}

// ---------- 类型文学默认框架（种子） ----------

// InitSeeds 写入内置模板（幂等：按 key 判重；已存在则不覆盖站长的修改）
func InitSeeds() {
	now := time.Now()
	for i, s := range builtinSeeds {
		var n int64
		models.DB.Model(&Template{}).Where("key = ?", s.Key).Count(&n)
		if n > 0 {
			continue
		}
		s.Variables = encodeVars(s.Vars)
		s.Builtin = true
		s.Version = 1
		s.Priority = 100 - i
		s.CreatedAt = now
		s.UpdatedAt = now
		models.DB.Create(&s)
	}
}

// builtinSeeds 默认模板：按类型文学设计，不含爽文导向模板
var builtinSeeds = []Template{
	{
		Key: "tl-continue", Title: "类型文学续写", Scene: SceneNovelChapter, Category: "续写", Kind: KindChat,
		Note:    "通用续写框架：强调设定自洽、伏笔延续、不总结前文",
		Content: "你是{{类型}}类型小说的资深作者。请续写下面的内容，约{{篇幅}}字。\n\n要求：\n1. 与既有设定严格自洽：世界规则、时间线、人物能力边界不得前后矛盾；\n2. 延续已有伏笔与人物动机，不引入未经铺垫的新设定；\n3. 保持原文人称、时态与语体；\n4. 用场景与动作推进，不要总结前文、不要抒情式拔高；\n5. 只输出续写正文。\n\n【作品】{{作品名}}\n【人物】{{角色信息}}\n【前文】\n{{前文}}\n\n【本次要求】{{要求}}",
		Vars: []TemplateVar{
			{Name: "类型", Label: "类型", Default: "科幻", MaxLen: 20},
			{Name: "篇幅", Label: "字数", Default: "600", MaxLen: 10},
			{Name: "作品名", Label: "作品名", MaxLen: 60},
			{Name: "角色信息", Label: "角色设定", MaxLen: 1200},
			{Name: "前文", Label: "前文", Required: true, MaxLen: 8000},
			{Name: "要求", Label: "本次要求", MaxLen: 500},
		},
	},
	{
		Key: "tl-hard-sf", Title: "硬科幻：设定自洽校验", Scene: SceneNovelChapter, Category: "类型专项", Kind: KindChat,
		Note:    "硬科幻重设定自洽：先校验物理/技术边界，再给修改建议",
		Content: "你是硬科幻编辑，擅长技术设定审查。请按下面结构审阅文本：\n\n一、设定清单：列出文中涉及的物理/技术假设（每条一句话）\n二、自洽性检查：每条假设与已知科学或前文设定是否冲突，冲突点标出原文句子\n三、代价与限制：该技术缺少哪些现实约束（能量、散热、材料、通信延迟、伦理）\n四、最小修改方案：只改必要段落，给出改后文本\n\n【前文设定】\n{{前文}}\n\n【待审文本】\n{{原文}}",
		Vars: []TemplateVar{
			{Name: "前文", Label: "前文设定", MaxLen: 4000},
			{Name: "原文", Label: "待审文本", Required: true, MaxLen: 8000},
		},
	},
	{
		Key: "tl-mystery", Title: "悬疑：信息控制检查", Scene: SceneNovelChapter, Category: "类型专项", Kind: KindChat,
		Note:    "悬疑重信息控制：读者已知/角色已知/作者隐藏三层对齐",
		Content: "你是悬疑小说结构编辑。请对下面章节做「信息控制」检查：\n\n1. 信息分层表：分别列出「读者已知」「主角已知」「其他角色已知」的关键信息；\n2. 泄漏点：哪些句子过早暴露了真相或凶手线索；\n3. 断点：哪些地方信息不足导致读者困惑（而非悬念）；\n4. 节奏建议：本章应在哪个位置抛出下一个问题；\n5. 只输出分析与必要改写段落，不要重写全章。\n\n【本章】\n{{原文}}\n\n【既定真相（作者可见）】\n{{真相}}",
		Vars: []TemplateVar{
			{Name: "原文", Label: "本章内容", Required: true, MaxLen: 8000},
			{Name: "真相", Label: "既定真相（仅作者可见）", MaxLen: 2000},
		},
	},
	{
		Key: "tl-alt-history", Title: "架空历史：时代逻辑审校", Scene: SceneNovelChapter, Category: "类型专项", Kind: KindChat,
		Note:    "架空历史重时代逻辑：制度、技术、语言、物价四个维度",
		Content: "你是架空历史题材的审校编辑。请从四个维度审校下面文本，指出与设定时代不符之处并给出最小修改：\n\n1. 制度与权力结构（官制、军制、宗法、法律）\n2. 技术与物质条件（交通、通讯、医疗、武器、能源）\n3. 语言与称谓（尊卑、避讳、口语习惯）\n4. 经济与物价（货币、购买力、税收）\n\n【时代设定】{{时代}}\n【文本】\n{{原文}}\n\n输出格式：维度 → 问题句 → 为什么不符 → 改法。",
		Vars: []TemplateVar{
			{Name: "时代", Label: "时代设定", Default: "架空王朝中期", MaxLen: 300},
			{Name: "原文", Label: "待审文本", Required: true, MaxLen: 8000},
		},
	},
	{
		Key: "tl-experimental", Title: "实验文学：形式探索建议", Scene: SceneNovelChapter, Category: "类型专项", Kind: KindChat,
		Note:    "实验文学重形式探索：结构/视角/语言三选一给出可执行方案",
		Content: "你是实验文学编辑。针对下面这段文字，给出 3 个可执行的形式实验方案（不要泛泛而谈）：\n\n每个方案包含：形式手法 → 与内容的契合点 → 具体到段落的改法示例（改写 1 段） → 风险（可读性/审稿风险）\n\n三个方案分别落在：结构（时间/章节组织）、视角（人称/叙述者可靠性）、语言（句法/节奏/文体混合）。\n\n【文本】\n{{原文}}",
		Vars: []TemplateVar{
			{Name: "原文", Label: "文本", Required: true, MaxLen: 8000},
		},
	},
	{
		Key: "tl-character", Title: "角色设定卡", Scene: SceneNovelChapter, Category: "设定", Kind: KindChat,
		Note:    "生成可用的人物卡：欲望/缺陷/秘密/说话方式",
		Content: "请为{{作品名}}设计一个{{定位}}角色，输出结构：\n\n姓名与称谓｜年龄与外貌特征（3 个可视觉化细节）｜出身与当前处境｜核心欲望（想要的）｜深层需求（需要的）｜致命缺陷｜秘密（读者后知）｜与主角的关系张力｜说话方式（口头禅/句长/用词层级）｜三个可用于推进情节的行动选项\n\n要求：不要写成简历；每条都要能直接落到场景里。\n\n【已有角色】{{角色信息}}\n【世界观约束】{{设定}}",
		Vars: []TemplateVar{
			{Name: "作品名", Label: "作品名", MaxLen: 60},
			{Name: "定位", Label: "角色定位", Default: "配角", MaxLen: 20},
			{Name: "角色信息", Label: "已有角色", MaxLen: 2000},
			{Name: "设定", Label: "世界观约束", MaxLen: 2000},
		},
	},
	{
		Key: "tl-summary", Title: "章节摘要（记忆库）", Scene: SceneNovelChapter, Category: "摘要", Kind: KindChat,
		Note:    "为长篇记忆库生成结构化摘要：事件/人物变化/伏笔",
		Content: "请为下面章节生成结构化摘要，控制在 200 字以内：\n\n- 事件：本章发生了什么（按因果顺序，3 条以内）\n- 人物：谁发生了变化（动机/关系/处境）\n- 伏笔：新增或推进的伏笔（没有就写「无」）\n- 结尾钩子：下一章承接点\n\n只输出以上四项，不要评价文笔。\n\n【章节】\n{{原文}}",
		Vars: []TemplateVar{
			{Name: "原文", Label: "章节内容", Required: true, MaxLen: 10000},
		},
	},
	{
		Key: "tl-polish", Title: "保风格润色", Scene: SceneNovelChapter, Category: "润色", Kind: KindChat,
		Note:    "不改信息量与语体，只修病句、冗余、标点",
		Content: "请润色下面文字：\n\n1. 不改信息量，不加情节，不删关键细节；\n2. 保持作者原有语体（{{语体}}），不要改成「网文腔」或「翻译腔」；\n3. 修掉病句、重复修饰、指代不明与标点错误；\n4. 输出润色后全文，再用 3 条说明改了什么（不逐句解释）。\n\n【原文】\n{{原文}}",
		Vars: []TemplateVar{
			{Name: "语体", Label: "语体", Default: "半文半白", MaxLen: 30},
			{Name: "原文", Label: "原文", Required: true, MaxLen: 8000},
		},
	},
	{
		Key: "tl-script-continue", Title: "话本续写（气泡对话）", Scene: SceneNovelScript, Category: "话本", Kind: KindChat,
		Note:    "按「角色：台词」续写，只输出可直接粘贴的话本行",
		Content: "你是话本小说作者。请续写下面这段对话，约{{回合数}}个回合。\n\n硬性格式（必须遵守）：\n1. 每行一条，严格写成「角色名：台词」，冒号用中文全角「：」；\n2. 需要环境/动作时，整行写【场景或动作】，不要混进台词；\n3. 需要补充说明时整行写旁白，不要写成某人的台词；\n4. 同一角色连续说话就连续写多行，不要空行断开；\n5. 只输出话本正文，不要标题、不要解释、不要编号。\n\n内容要求：\n- 每人每句推进一个信息或一个意图，禁止「嗯」「哦」「好吧」这类无信息量的水词；\n- 保持角色口吻一致（用词层级、句长、口头禅见【角色信息】）；\n- 冲突或悬念要往前走一步，不要原地打转。\n\n【作品】{{作品名}}\n【角色信息】{{角色信息}}\n【前文】\n{{前文}}\n\n【本次要求】{{要求}}",
		Vars: []TemplateVar{
			{Name: "作品名", Label: "作品名", MaxLen: 60},
			{Name: "角色信息", Label: "角色设定", MaxLen: 2000},
			{Name: "前文", Label: "前文对话", Required: true, MaxLen: 8000},
			{Name: "回合数", Label: "续写回合", Default: "6", MaxLen: 10},
			{Name: "要求", Label: "本次要求", MaxLen: 500},
		},
	},
	{
		Key: "tl-script-voice", Title: "台词口吻校准", Scene: SceneNovelScript, Category: "话本", Kind: KindChat,
		Note:    "让每个角色的台词带上自己的说话方式，不改情节",
		Content: "请校准下面话本里{{角色名}}的台词：\n\n1. 不改情节、不改信息量，只改说话方式；\n2. 用词层级、句长、口头禅、称谓按【口吻】执行；\n3. 同一角色的不同情绪（{{情绪}}）要能从用词看出来，而不是靠「他生气地说」；\n4. 输出格式不变：仍是「角色名：台词」，只输出被改动的角色相关行（含上下文 2 行便于定位）。\n\n【口吻】见【角色信息】中该角色设定；未写明处按{{默认口吻}}处理。\n\n【角色信息】{{角色信息}}\n【话本】\n{{原文}}",
		Vars: []TemplateVar{
			{Name: "角色名", Label: "目标角色", Required: true, MaxLen: 40},
			{Name: "情绪", Label: "目标情绪", Default: "克制但带压迫感", MaxLen: 40},
			{Name: "默认口吻", Label: "默认口吻", Default: "短句、口语、少形容词", MaxLen: 60},
			{Name: "角色信息", Label: "角色设定", MaxLen: 2000},
			{Name: "原文", Label: "话本原文", Required: true, MaxLen: 8000},
		},
	},
	{
		Key: "tl-script-stage", Title: "场景提示生成", Scene: SceneNovelScript, Category: "话本", Kind: KindChat,
		Note:    "生成可直接插入的【场景/动作】行，不写台词",
		Content: "请为下面这段对话补写场景与动作提示，{{条数}}条以内。\n\n格式：每条整行写成【……】，只写看得见的动作/环境/物件变化，不写心理活动，不写评价。\n位置：直接说明该插在哪一句台词之前（用「插在：某角色：某句台词 之前」标注）。\n\n要求：\n- 用具体物件与动作推进（门、茶杯、影子、雨声），不要「气氛凝重」这类抽象词；\n- 与【设定】的世界规则自洽；\n- 只输出提示行与插入位置，不要改写台词。\n\n【作品】{{作品名}}\n【设定】{{设定}}\n【话本】\n{{原文}}",
		Vars: []TemplateVar{
			{Name: "作品名", Label: "作品名", MaxLen: 60},
			{Name: "设定", Label: "世界观约束", MaxLen: 2000},
			{Name: "原文", Label: "话本原文", Required: true, MaxLen: 8000},
			{Name: "条数", Label: "提示条数", Default: "3", MaxLen: 10},
		},
	},
	{
		Key: "tl-script-water-check", Title: "对话去水与信息量检查", Scene: SceneNovelScript, Category: "话本", Kind: KindChat,
		Note:    "挑出无信息量的台词并给出替换建议，不改变剧情走向",
		Content: "请检查下面话本的信息密度：\n\n1. 逐行标出「无信息量」的台词（应酬、重复、纯情绪水词），说明为什么水；\n2. 对每条给出一句替换台词（写成「角色名：台词」），或建议直接删除；\n3. 标出对话里被反复解释、读者已知的信息（说明性对话）；\n4. 输出三部分：去水清单 / 替换后的话本片段 / 一句总体判断。\n不要新增情节，不要改动机。\n\n【话本】\n{{原文}}",
		Vars: []TemplateVar{
			{Name: "原文", Label: "话本原文", Required: true, MaxLen: 10000},
		},
	},
	{
		Key: "tl-blog-review", Title: "博客：观点与结构打磨", Scene: SceneBlog, Category: "博客", Kind: KindChat,
		Note:    "面向博客正文：论点/论据/结构，不改成小说腔",
		Content: "你是专栏编辑。请审阅下面这篇博客，输出：\n\n1. 一句话主旨（作者真正想说的）\n2. 结构问题：哪几段可以合并/前移/删除\n3. 论据缺口：哪些判断缺少事实、数据或例子\n4. 标题建议 5 个（不夸张、不标题党）\n5. 只输出建议清单，不要重写全文\n\n【正文】\n{{原文}}",
		Vars: []TemplateVar{
			{Name: "原文", Label: "博客正文", Required: true, MaxLen: 8000},
		},
	},
}
