package theme

import "testing"

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
