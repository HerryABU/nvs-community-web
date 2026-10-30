package utils

import "testing"

func TestParseUserAgent(t *testing.T) {
	cases := []struct {
		name     string
		ua       string
		osName   string
		browser  string
		devType  string
		hasModel bool
	}{
		{
			name:    "Windows Chrome",
			ua:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			osName:  "Windows",
			browser: "Chrome",
			devType: "desktop",
		},
		{
			name:    "macOS Safari",
			ua:      "Mozilla/5.0 (Macintosh; Intel Mac OS X 13_5) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.5 Safari/605.1.15",
			osName:  "macOS",
			browser: "Safari",
			devType: "desktop",
		},
		{
			name:     "iPhone Safari",
			ua:       "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Mobile/15E148 Safari/604.1",
			osName:   "iOS",
			browser:  "Safari",
			devType:  "phone",
			hasModel: true, // iPhone
		},
		{
			name:     "Android Chrome",
			ua:       "Mozilla/5.0 (Linux; Android 13; SM-G991B Build/TP1A) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/115.0.0.0 Mobile Safari/537.36",
			osName:   "Android",
			browser:  "Chrome",
			devType:  "phone",
			hasModel: true, // SM-G991B
		},
		{
			name:     "iPad",
			ua:       "Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
			osName:   "iOS",
			browser:  "Safari",
			devType:  "tablet",
			hasModel: true, // iPad
		},
		{
			name:    "Edge on Windows",
			ua:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
			osName:  "Windows",
			browser: "Edge",
			devType: "desktop",
		},
		{
			name:    "Firefox Linux",
			ua:      "Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0",
			osName:  "Linux",
			browser: "Firefox",
			devType: "desktop",
		},
		{
			name:    "empty",
			ua:      "",
			osName:  "unknown",
			browser: "unknown",
			devType: "unknown",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			info := ParseUserAgent(c.ua)
			if info.OSName != c.osName {
				t.Errorf("OSName = %q, want %q", info.OSName, c.osName)
			}
			if info.BrowserName != c.browser {
				t.Errorf("BrowserName = %q, want %q", info.BrowserName, c.browser)
			}
			if info.DeviceType != c.devType {
				t.Errorf("DeviceType = %q, want %q", info.DeviceType, c.devType)
			}
			if c.hasModel && info.DeviceModel == "" {
				t.Errorf("expected non-empty DeviceModel for %q", c.name)
			}
			if !c.hasModel && c.ua != "" && info.DeviceModel != "" {
				// 桌面端不应有具体机型（除非是 iPhone/iPad 字面量）
				if c.devType == "desktop" {
					t.Errorf("desktop UA should have empty DeviceModel, got %q", info.DeviceModel)
				}
			}
		})
	}
}
