package oc

// ==================== 角色卡 / 世界书 导入导出（gap-audit #24）====================
// 互通目标：SillyTavern（酒馆）V2 角色卡（PNG/JSON 里的 `chara_card_v2`）与 Lorebook。
// 为什么值得做：用户已经在酒馆里积累了大量角色卡，能导入就是「零成本迁移」；
// 能导出就是「不锁定」——这两点比任何新玩法都更能拉 OC 作者。
//
// 兼容策略（按现实数据写，不按理想 spec 写）：
//   - V2：`{"spec":"chara_card_v2","data":{...}}`
//   - V1：扁平 `{"name","description","personality","scenario","first_mes","mes_example"}`
//   - 酒馆把很多字段塞进 `data.extensions`；我们把自己的扩展放 `extensions.nvs`，
//     导入时优先读标准字段，缺失再从 extensions 兜底。
//   - 未知字段**保留**到 `extensions.nvs.raw`（不丢用户数据，便于后续补字段）。

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ─────────────── 导出：角色卡 V2 ───────────────

// CardV2 酒馆 V2 角色卡
type CardV2 struct {
	Spec        string   `json:"spec"`
	SpecVersion string   `json:"spec_version"`
	Data        CardData `json:"data"`
}

// CardData V2 数据段
type CardData struct {
	Name           string                 `json:"name"`
	Description    string                 `json:"description"`
	Personality    string                 `json:"personality"`
	Scenario       string                 `json:"scenario"`
	FirstMes       string                 `json:"first_mes"`
	MesExample     string                 `json:"mes_example"`
	CreatorNotes   string                 `json:"creator_notes"`
	SystemPrompt   string                 `json:"system_prompt"`
	PostHistory    string                 `json:"post_history_instructions"`
	AlternateGreet []string               `json:"alternate_greetings"`
	Tags           []string               `json:"tags"`
	Creator        string                 `json:"creator"`
	CharacterVer   string                 `json:"character_version"`
	Extensions     map[string]interface{} `json:"extensions"`
}

// ExportCard 导出为 V2 JSON（c 必须已通过归属校验）
func ExportCard(c *Character) (*CardV2, error) {
	if c == nil {
		return nil, errors.New("角色不存在")
	}
	greets := []string{}
	if strings.TrimSpace(c.Greetings) != "" {
		_ = json.Unmarshal([]byte(c.Greetings), &greets)
	}
	tags := []string{}
	for _, t := range strings.Split(c.Tags, ",") {
		if t = strings.TrimSpace(t); t != "" {
			tags = append(tags, t)
		}
	}
	// 世界书随卡导出（酒馆的 character_book）：只导出条目，不含其它角色的书
	var book interface{}
	if c.BookID > 0 {
		if b, err := GetWorldBook(c.BookID, c.UserID, false); err == nil && b != nil {
			entries, _ := ListEntries(b.ID)
			book = exportLorebook(b, entries)
		}
	}
	ext := map[string]interface{}{
		"nvs": map[string]interface{}{
			"id": c.ID, "appearance": c.Appearance, "speech": c.Speech,
			"mood": c.Mood, "affinity": c.Affinity, "energy": c.Energy,
			"emote_map": c.EmoteMap, "visibility": c.Visibility,
			"exported_at": time.Now().Format(time.RFC3339),
		},
	}
	if book != nil {
		ext["character_book"] = book
	}
	return &CardV2{
		Spec: "chara_card_v2", SpecVersion: "2.0",
		Data: CardData{
			Name: c.Name, Description: c.Persona,
			// 酒馆没有独立「外貌/说话风格」字段：合并进 personality，保持可读
			Personality: joinNonEmpty(c.Appearance, c.Speech, c.Persona),
			Scenario:    c.Scenario,
			FirstMes:    c.FirstMessage, MesExample: c.MesExample,
			CreatorNotes: c.CreatorNotes, SystemPrompt: c.SystemPrompt,
			PostHistory: c.PostHistory, AlternateGreet: greets, Tags: tags,
			Creator: "NVS", CharacterVer: "1.0",
			Extensions: ext,
		},
	}, nil
}

func joinNonEmpty(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, strings.TrimSpace(p))
		}
	}
	return strings.Join(out, "\n\n")
}

// ─────────────── 导入：兼容 V2 / V1 ───────────────

// ImportResult 导入结果
type ImportResult struct {
	Name         string          `json:"name"`
	Persona      string          `json:"persona"`
	Appearance   string          `json:"appearance"`
	Speech       string          `json:"speech"`
	Scenario     string          `json:"scenario"`
	FirstMes     string          `json:"first_mes"`
	MesExample   string          `json:"mes_example"`
	SystemPrompt string          `json:"system_prompt"`
	PostHistory  string          `json:"post_history"`
	CreatorNotes string          `json:"creator_notes"`
	Greetings    []string        `json:"greetings"`
	Tags         string          `json:"tags"`
	Book         *LorebookImport `json:"book,omitempty"`
	Raw          string          `json:"raw,omitempty"` // 未识别字段（json 字符串），便于后续补映射
}

// ParseCard 解析角色卡 JSON（V2 优先，V1 兼容）
func ParseCard(raw []byte) (*ImportResult, error) {
	if len(raw) == 0 {
		return nil, errors.New("文件为空")
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, errors.New("不是合法的 JSON 角色卡（若是 PNG 内嵌卡片，请先在酒馆导出 JSON）")
	}
	res := &ImportResult{}
	// V2
	if _, ok := probe["data"]; ok {
		var card CardV2
		if err := json.Unmarshal(raw, &card); err != nil {
			return nil, errors.New("V2 卡片解析失败：" + err.Error())
		}
		d := card.Data
		res.Name = strings.TrimSpace(d.Name)
		res.Persona = d.Description
		res.Scenario = d.Scenario
		res.FirstMes = d.FirstMes
		res.MesExample = d.MesExample
		res.SystemPrompt = d.SystemPrompt
		res.PostHistory = d.PostHistory
		res.CreatorNotes = d.CreatorNotes
		res.Greetings = d.AlternateGreet
		res.Tags = strings.Join(d.Tags, ",")
		if d.Extensions != nil {
			if nvs, ok := d.Extensions["nvs"].(map[string]interface{}); ok {
				res.Appearance = str(nvs["appearance"])
				res.Speech = str(nvs["speech"])
				// 我们自己导出的卡：personality 是外貌+说话+人设的合并，回填时纠正
				if res.Appearance != "" || res.Speech != "" {
					res.Persona = d.Description
				}
			}
			if cb, ok := d.Extensions["character_book"]; ok {
				if b, err := parseLorebook(cb); err == nil {
					res.Book = b
				}
			}
		}
	} else {
		// V1 扁平
		res.Name = str(probe["name"])
		res.Persona = firstNonEmpty(str(probe["description"]), str(probe["personality"]))
		res.Scenario = str(probe["scenario"])
		res.FirstMes = str(probe["first_mes"])
		res.MesExample = str(probe["mes_example"])
		res.CreatorNotes = str(probe["creator_notes"])
		res.SystemPrompt = str(probe["system_prompt"])
		if cb, ok := probe["character_book"]; ok {
			if b, err := parseLorebook(rawToAny(cb)); err == nil {
				res.Book = b
			}
		}
	}
	res.Name = strings.TrimSpace(res.Name)
	if res.Name == "" {
		return nil, errors.New("卡片缺少 name 字段")
	}
	// 保留未识别字段（不丢数据）
	known := map[string]bool{"spec": true, "spec_version": true, "data": true,
		"name": true, "description": true, "personality": true, "scenario": true,
		"first_mes": true, "mes_example": true, "creator_notes": true,
		"system_prompt": true, "post_history_instructions": true, "character_book": true}
	extra := map[string]json.RawMessage{}
	for k, v := range probe {
		if !known[k] {
			extra[k] = v
		}
	}
	if len(extra) > 0 {
		if b, err := json.Marshal(extra); err == nil {
			res.Raw = string(b)
		}
	}
	return res, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func str(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case json.RawMessage:
		var s string
		if json.Unmarshal(t, &s) == nil {
			return s
		}
		return string(t)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", t)
	}
}

func rawToAny(r json.RawMessage) interface{} {
	var v interface{}
	_ = json.Unmarshal(r, &v)
	return v
}

// ─────────────── Lorebook 互通 ───────────────

// LorebookImport 世界书导入结果（与 WorldEntry 一一对应）
type LorebookImport struct {
	Name        string           `json:"name"`
	Entries     []WorldEntryLite `json:"entries"`
	ScanDepth   int              `json:"scan_depth"`
	TokenBudget int              `json:"token_budget"`
}

// WorldEntryLite 中间态（不含 ID/BookID，落库时补）
type WorldEntryLite struct {
	Keys           []string `json:"keys"`
	SecondaryKeys  []string `json:"secondary_keys"`
	Content        string   `json:"content"`
	Comment        string   `json:"comment"`
	Constant       bool     `json:"constant"`
	Selective      bool     `json:"selective"`
	InsertionOrder int      `json:"insertion_order"`
	Priority       int      `json:"priority"`
	Position       int      `json:"position"`
	Depth          int      `json:"depth"`
	Probability    int      `json:"probability"`
	Enabled        bool     `json:"enabled"`
	CaseSensitive  bool     `json:"case_sensitive"`
	Group          string   `json:"group"`
}

// parseLorebook 解析酒馆 lorebook（兼容 entries 为 map 或数组）
func parseLorebook(v interface{}) (*LorebookImport, error) {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil, errors.New("世界书格式不支持")
	}
	out := &LorebookImport{}
	out.Name = str(m["name"])
	if sd, ok := m["scan_depth"]; ok {
		out.ScanDepth = toInt(sd)
	}
	if tb, ok := m["token_budget"]; ok {
		out.TokenBudget = toInt(tb)
	}
	rawEntries, ok := m["entries"]
	if !ok {
		return out, nil
	}
	add := func(e map[string]interface{}) {
		lite := WorldEntryLite{
			Keys:           toStrSlice(e["key"]),
			SecondaryKeys:  toStrSlice(e["keysecondary"]),
			Content:        str(e["content"]),
			Comment:        str(e["comment"]),
			Constant:       toBool(e["constant"]),
			Selective:      toBool(e["selective"]),
			InsertionOrder: toInt(e["insertion_order"]),
			Priority:       toInt(e["priority"]),
			Position:       toInt(e["position"]),
			Depth:          toInt(e["depth"]),
			Probability:    toInt(e["probability"]),
			Enabled:        true,
			CaseSensitive:  toBool(e["case_sensitive"]),
			Group:          str(e["group"]),
		}
		if _, has := e["enabled"]; has {
			lite.Enabled = toBool(e["enabled"])
		}
		if lite.Probability <= 0 {
			lite.Probability = 100
		}
		if lite.InsertionOrder == 0 {
			lite.InsertionOrder = 100
		}
		if lite.Priority == 0 {
			lite.Priority = 100
		}
		if strings.TrimSpace(lite.Content) == "" {
			return
		}
		out.Entries = append(out.Entries, lite)
	}
	switch es := rawEntries.(type) {
	case map[string]interface{}:
		for _, item := range es {
			if em, ok := item.(map[string]interface{}); ok {
				add(em)
			}
		}
	case []interface{}:
		for _, item := range es {
			if em, ok := item.(map[string]interface{}); ok {
				add(em)
			}
		}
	}
	return out, nil
}

// ParseLorebookJSON 供接口层直接调（导入世界书）
func ParseLorebookJSON(raw []byte) (*LorebookImport, error) {
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, errors.New("不是合法的 JSON 世界书")
	}
	// 允许包一层 {"data": ...}
	if m, ok := v.(map[string]interface{}); ok {
		if d, ok2 := m["data"].(map[string]interface{}); ok2 {
			if _, has := d["entries"]; has {
				v = d
			}
		}
	}
	return parseLorebook(v)
}

// ExportLorebookJSON 供接口层调用（导出世界书为酒馆 lorebook）
func ExportLorebookJSON(b *WorldBook, entries []WorldEntry) map[string]interface{} {
	if b == nil {
		return map[string]interface{}{}
	}
	return exportLorebook(b, entries)
}

// exportLorebook 导出为酒馆 lorebook（entries 用 map 便于互通）
func exportLorebook(b *WorldBook, entries []WorldEntry) map[string]interface{} {
	out := map[string]interface{}{
		"name": b.Title, "scan_depth": b.ScanDepth, "token_budget": b.TokenBudget,
	}
	es := map[string]interface{}{}
	for i, e := range entries {
		keys := splitTrim(e.Key)
		es[strconv.Itoa(i)] = map[string]interface{}{
			"uid": i, "key": keys, "keysecondary": splitTrim(e.SecondaryKeys),
			"content": e.Content, "comment": e.Kind + ":" + e.Key,
			"constant": e.Constant, "selective": e.Selective,
			"insertion_order": e.InsertionOrder, "priority": e.Priority,
			"position": e.Position, "depth": e.Depth, "probability": e.Probability,
			"enabled": e.Enabled, "case_sensitive": e.CaseSensitive, "group": e.Group,
		}
	}
	out["entries"] = es
	return out
}

func splitTrim(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func toStrSlice(v interface{}) []string {
	switch t := v.(type) {
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, x := range t {
			if s := str(x); strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if strings.TrimSpace(t) == "" {
			return nil
		}
		return splitTrim(t)
	}
	return nil
}

func toInt(v interface{}) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(t))
		return n
	case json.Number:
		n, _ := t.Int64()
		return int(n)
	}
	return 0
}

func toBool(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true" || t == "1"
	case float64:
		return t != 0
	}
	return false
}
