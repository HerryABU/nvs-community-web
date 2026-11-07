package theme

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestNormalizedSettingsJSON 回归：SetSiteTheme 曾把 ParseThemeSettings 的两个返回值写反，
// 将 removed 列表 Marshal 成 "null" 落库，导致可视化配色永远不生效。
// 这里保证「规范化后的设置」序列化为含全部字段的 JSON 对象（而非 null）。
func TestNormalizedSettingsJSON(t *testing.T) {
	raw := `{"background":{"type":"color","value":""},"glass":{"enabled":true,"blur":20,"opacity":0.6,"tint":"#ffffff"},"button":{"radius":16,"primary":"#e11d48"},"accent":{"color":"#f43f5e"},"surface":{"color":"#fff1f2"},"mascot":{"enabled":false,"script_url":""}}`
	normalized, removed := ParseThemeSettings(raw)
	if len(removed) != 0 {
		t.Fatalf("合法设置不应有回退提示，得到 %v", removed)
	}
	b, err := json.Marshal(normalized)
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	s := string(b)
	if strings.TrimSpace(s) == "null" || !strings.HasPrefix(s, "{") {
		t.Fatalf("规范化设置必须序列化为 JSON 对象，得到 %q", s)
	}
	for _, key := range []string{`"button"`, `"accent"`, `"surface"`, `"#e11d48"`, `"#f43f5e"`, `"#fff1f2"`} {
		if !strings.Contains(s, key) {
			t.Fatalf("序列化结果缺少 %s: %q", key, s)
		}
	}
}

func TestParseThemeSettings(t *testing.T) {
	def := DefaultThemeSettings()

	// 空串 → 默认
	s, removed := ParseThemeSettings("")
	if removed != nil {
		t.Fatalf("empty: expected no removed, got %v", removed)
	}
	if s.Glass.Enabled != def.Glass.Enabled || s.Glass.Blur != def.Glass.Blur {
		t.Fatalf("empty: expected default glass, got %+v", s.Glass)
	}

	// 完整合法 JSON
	raw := `{"background":{"type":"gradient","value":"linear-gradient(135deg,#6366f1,#ec4899)"},"glass":{"enabled":true,"blur":30,"opacity":0.5,"tint":"#222222"},"button":{"radius":8,"primary":"#ff0000"},"mascot":{"enabled":true,"script_url":"https://cdn.example.com/a.js"}}`
	s, removed = ParseThemeSettings(raw)
	if removed != nil {
		t.Fatalf("valid: expected no removed, got %v", removed)
	}
	if s.Background.Type != "gradient" || s.Background.Value != "linear-gradient(135deg,#6366f1,#ec4899)" {
		t.Fatalf("valid: background not parsed: %+v", s.Background)
	}
	if s.Glass.Blur != 30 || s.Glass.Opacity != 0.5 || s.Glass.Tint != "#222222" {
		t.Fatalf("valid: glass not parsed: %+v", s.Glass)
	}
	if s.Button.Radius != 8 || s.Button.Primary != "#ff0000" {
		t.Fatalf("valid: button not parsed: %+v", s.Button)
	}
	if !s.Mascot.Enabled || s.Mascot.ScriptURL != "https://cdn.example.com/a.js" {
		t.Fatalf("valid: mascot not parsed: %+v", s.Mascot)
	}

	// 非法 JSON → 回退默认 + 提示
	s, removed = ParseThemeSettings("{not json")
	if len(removed) == 0 {
		t.Fatalf("invalid json: expected a removed hint")
	}
	if s.Glass.Blur != def.Glass.Blur {
		t.Fatalf("invalid json: expected default glass blur, got %v", s.Glass.Blur)
	}

	// 越界数值 → 回退 + 提示
	s, removed = ParseThemeSettings(`{"glass":{"blur":999,"opacity":5}}`)
	if len(removed) == 0 {
		t.Fatalf("out-of-range: expected removed hints")
	}
	if s.Glass.Blur != def.Glass.Blur || s.Glass.Opacity != def.Glass.Opacity {
		t.Fatalf("out-of-range: expected default glass values, got %+v", s.Glass)
	}

	// 非法颜色 → 忽略 + 提示
	s, removed = ParseThemeSettings(`{"button":{"primary":"notacolor"},"glass":{"tint":"xyz"}}`)
	if len(removed) == 0 {
		t.Fatalf("bad color: expected removed hints")
	}
	if s.Button.Primary != "" || s.Glass.Tint != def.Glass.Tint {
		t.Fatalf("bad color: expected defaults, got primary=%q tint=%q", s.Button.Primary, s.Glass.Tint)
	}

	// 危险 mascot 协议 → 忽略 + 提示
	s, removed = ParseThemeSettings(`{"mascot":{"enabled":true,"script_url":"javascript:alert(1)"}}`)
	if len(removed) == 0 {
		t.Fatalf("bad url: expected removed hint")
	}
	if s.Mascot.ScriptURL != "" {
		t.Fatalf("bad url: expected empty script_url, got %q", s.Mascot.ScriptURL)
	}
}
