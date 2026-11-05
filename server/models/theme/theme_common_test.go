package theme

import "testing"

// TestNormalizeThemeJSMode 加载模式归一化：只有 idle / delayed 被接受，其余一律回落 immediate。
func TestNormalizeThemeJSMode(t *testing.T) {
	cases := map[string]string{
		"immediate": ThemeJSModeImmediate,
		"idle":      ThemeJSModeIdle,
		"delayed":   ThemeJSModeDelayed,
		"IDLE":      ThemeJSModeIdle,    // 大写也认
		" delayed ": ThemeJSModeDelayed, // 带空格也认
		"":          ThemeJSModeImmediate,
		"bogus":     ThemeJSModeImmediate,
		"none":      ThemeJSModeImmediate,
	}
	for in, want := range cases {
		if got := NormalizeThemeJSMode(in); got != want {
			t.Errorf("NormalizeThemeJSMode(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestNormalizeThemeJSDelay 延迟秒数归一化：默认 3、上限 600。
func TestNormalizeThemeJSDelay(t *testing.T) {
	cases := map[int]int{
		0:   3,
		-5:  3,
		1:   1,
		5:   5,
		600: 600,
		999: 600,
	}
	for in, want := range cases {
		if got := NormalizeThemeJSDelay(in); got != want {
			t.Errorf("NormalizeThemeJSDelay(%d) = %d, want %d", in, got, want)
		}
	}
}
