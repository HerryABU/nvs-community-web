package oc

// ==================== OC 角色卡 AI 生成 / 补全 ====================
// 借鉴作者工作台 AI 写作流（prompttpl 模板引擎 + aicall 模型路由/计费/日志），
// 但数据走 oc_characters，与作者工程完全隔离。兼容 furry：furry 时把物种/特征注入模板，
// AI 必须在 persona/appearance/speech 中展开兽人设定。
//
// 输出采用结构化 JSON（角色卡各字段），Go 端解析后落库；解析失败回落为整段写入 persona。

import (
	"encoding/json"
	"fmt"
	"strings"

	"nvs-server/aicall"
	"nvs-server/prompttpl"
)

// OcGenSeed OC 角色卡生成入参
type OcGenSeed struct {
	Concept           string // 核心概念/灵感
	StyleDirective    string // 风格基调
	OcKind            string // native / furry（furry 时要求展开兽人设定）
	Species           string
	SpeciesTraits     string
	AnthroType        string
	ExtraRequirements string
}

// OcFillSeed OC 角色卡补全入参
type OcFillSeed struct {
	Focus             string // 重点补全字段
	ExtraRequirements string
	// furry 深化用（可选）
	OcKind        string
	Species       string
	SpeciesTraits string
	AnthroType    string
}

// buildFurryHint 在用户选择 furry 时构造模板提示；原生 OC 返回空串
func buildFurryHint(kind, species, traits, anthro string) string {
	if NormalizeOcKind(kind) != "furry" {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("Furry 设定要求：")
	if s := strings.TrimSpace(species); s != "" {
		sb.WriteString("物种 " + s + "；")
	}
	if t := strings.TrimSpace(traits); t != "" {
		sb.WriteString("特征 " + t + "；")
	}
	if a := strings.TrimSpace(anthro); a != "" {
		sb.WriteString("拟人类型 " + a + "；")
	}
	sb.WriteString("请据此展开兽人设定（oc_kind 必须为 furry）。\n")
	return sb.String()
}

// GenerateOCCharacter 从灵感生成一张完整的 OC 角色卡（落库，归 userID 所有）
func GenerateOCCharacter(userID uint, seed OcGenSeed) (*Character, error) {
	t, terr := prompttpl.GetByKey("oc-character")
	if terr != nil || t == nil {
		return nil, fmt.Errorf("OC 生成模板未就绪")
	}
	values := map[string]string{
		"Concept":           strings.TrimSpace(seed.Concept),
		"StyleDirective":    strings.TrimSpace(seed.StyleDirective),
		"FurryHint":         buildFurryHint(seed.OcKind, seed.Species, seed.SpeciesTraits, seed.AnthroType),
		"ExtraRequirements": strings.TrimSpace(seed.ExtraRequirements),
	}
	rendered := prompttpl.Render(t.Content, t.Vars, values)
	userText := rendered.Text
	if strings.TrimSpace(userText) == "" {
		userText = "请基于概念设计一张 OC 角色卡：" + strings.TrimSpace(seed.Concept)
	}

	res, gerr := aicall.Complete(aicall.Request{
		PayerID: userID, Scene: "oc_ai", Action: "oc_character",
		RefType: "oc_character", RefID: 0,
		System:    "你是专业的 OC 角色卡设计助手，请严格按用户给出的结构与要求输出 JSON，不解释、不评价。",
		User:      userText,
		MaxTokens: 2000, Temperature: 0.85,
	})
	if gerr != nil {
		return nil, fmt.Errorf("AI 生成失败：%w", gerr)
	}

	in := characterInputFromJSON(res.Content)
	// 用户显式选 furry 时兜底保证分类正确
	if NormalizeOcKind(seed.OcKind) == "furry" && in.OcKind != "furry" {
		in.OcKind = "furry"
		if in.Species == "" {
			in.Species = strings.TrimSpace(seed.Species)
		}
		if in.SpeciesTraits == "" {
			in.SpeciesTraits = strings.TrimSpace(seed.SpeciesTraits)
		}
	}
	if strings.TrimSpace(in.Name) == "" {
		in.Name = "未命名OC"
	}
	return CreateCharacter(userID, in)
}

// FillOCCharacter 在已有角色卡基础上补全/润色（落库更新）
func FillOCCharacter(userID, charID uint, seed OcFillSeed) (*Character, error) {
	ch, err := GetCharacter(charID, userID, false)
	if err != nil {
		return nil, err
	}
	existing := characterToJSON(ch)

	t, terr := prompttpl.GetByKey("oc-character-fill")
	if terr != nil || t == nil {
		return nil, fmt.Errorf("OC 补全模板未就绪")
	}
	values := map[string]string{
		"ExistingContent":   existing,
		"FurryHint":         buildFurryHint(seed.OcKind, seed.Species, seed.SpeciesTraits, seed.AnthroType),
		"Focus":             strings.TrimSpace(seed.Focus),
		"ExtraRequirements": strings.TrimSpace(seed.ExtraRequirements),
	}
	rendered := prompttpl.Render(t.Content, t.Vars, values)
	userText := rendered.Text
	if strings.TrimSpace(userText) == "" {
		userText = "请补全这张 OC 角色卡：" + existing
	}

	res, gerr := aicall.Complete(aicall.Request{
		PayerID: userID, Scene: "oc_ai", Action: "oc_fill",
		RefType: "oc_character", RefID: charID,
		System:    "你是专业的 OC 角色卡精修助手，请严格按用户给出的结构与要求输出完整 JSON，不解释、不评价。",
		User:      userText,
		MaxTokens: 2000, Temperature: 0.8,
	})
	if gerr != nil {
		return nil, fmt.Errorf("AI 补全失败：%w", gerr)
	}

	in := characterInputFromJSON(res.Content)
	// 保留不可被 AI 改写的归属/可见性/养成状态字段
	in.Name = pickNonEmpty(in.Name, ch.Name)
	in.Visibility = ch.Visibility
	in.OcKind = NormalizeOcKind(in.OcKind)
	if in.OcKind != "furry" {
		// 若原卡是 furry，AI 没返回 species 时保留原值
		if ch.OcKind == "furry" {
			in.OcKind = "furry"
			if in.Species == "" {
				in.Species = ch.Species
			}
			if in.SpeciesTraits == "" {
				in.SpeciesTraits = ch.SpeciesTraits
			}
			if in.AnthroType == "" {
				in.AnthroType = ch.AnthroType
			}
		}
	}
	if in.BookID == 0 {
		in.BookID = ch.BookID
	}
	// 状态字段若 AI 没给有效值则沿用原卡
	if in.Mood == 0 {
		in.Mood = ch.Mood
	}
	if in.Affinity == 0 {
		in.Affinity = ch.Affinity
	}
	if in.Energy == 0 {
		in.Energy = ch.Energy
	}
	return UpdateCharacter(userID, charID, in)
}

// characterToJSON 把角色卡现有字段序列化为模板需要的 JSON 文本
func characterToJSON(ch *Character) string {
	m := map[string]interface{}{
		"name":           ch.Name,
		"persona":        ch.Persona,
		"appearance":     ch.Appearance,
		"speech":         ch.Speech,
		"scenario":       ch.Scenario,
		"first_message":  ch.FirstMessage,
		"mes_example":    ch.MesExample,
		"system_prompt":  ch.SystemPrompt,
		"oc_kind":        ch.OcKind,
		"species":        ch.Species,
		"species_traits": ch.SpeciesTraits,
		"anthro_type":    ch.AnthroType,
		"tags":           ch.Tags,
		"mood":           ch.Mood,
		"affinity":       ch.Affinity,
		"energy":         ch.Energy,
	}
	b, _ := json.MarshalIndent(m, "", "  ")
	return string(b)
}

// characterInputFromJSON 解析 AI 返回的 JSON 为 CharacterInput；解析失败回落为整段写入 persona
func characterInputFromJSON(content string) CharacterInput {
	raw := strings.TrimSpace(content)
	// 去掉可能的 ```json ... ``` 围栏
	if i := strings.Index(raw, "{"); i >= 0 {
		if j := strings.LastIndex(raw, "}"); j > i {
			raw = raw[i : j+1]
		}
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		// 回落：整段当人设
		return CharacterInput{Persona: strings.TrimSpace(content), OcKind: "native"}
	}
	str := func(k string) string {
		if v, ok := m[k]; ok {
			if s, ok2 := v.(string); ok2 {
				return strings.TrimSpace(s)
			}
		}
		return ""
	}
	num := func(k string) int {
		if v, ok := m[k]; ok {
			switch n := v.(type) {
			case float64:
				return int(n)
			case int:
				return n
			}
		}
		return 0
	}
	return CharacterInput{
		Name:          str("name"),
		Persona:       str("persona"),
		Appearance:    str("appearance"),
		Speech:        str("speech"),
		Scenario:      str("scenario"),
		FirstMessage:  str("first_message"),
		MesExample:    str("mes_example"),
		SystemPrompt:  str("system_prompt"),
		OcKind:        str("oc_kind"),
		Species:       str("species"),
		SpeciesTraits: str("species_traits"),
		AnthroType:    str("anthro_type"),
		Tags:          str("tags"),
		Mood:          num("mood"),
		Affinity:      num("affinity"),
		Energy:        num("energy"),
	}
}

func pickNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
