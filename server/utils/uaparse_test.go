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

// ─────────────────────────────────────────────────────────────
// 内置浏览器 / 手机厂商浏览器识别（新增）
// 这些 UA 里通常同时带 Chrome 或 Safari 字样，顺序判错就永远统计不到微信来源。
// ─────────────────────────────────────────────────────────────

func TestParseUserAgent_InAppBrowsers(t *testing.T) {
	cases := []struct {
		name    string
		ua      string
		browser string
		device  string
		os      string
	}{
		{
			name: "微信 iOS",
			ua: "Mozilla/5.0 (iPhone; CPU iPhone OS 16_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) " +
				"Mobile/15E148 MicroMessenger/8.0.42(0x18002a2b) NetType/WIFI Language/zh_CN",
			browser: "WeChat", device: "phone", os: "iOS",
		},
		{
			name: "微信 Android（XWEB 内核，UA 里带 Chrome）",
			ua: "Mozilla/5.0 (Linux; Android 13; 22081212C Build/TKQ1.220829.002; wv) AppleWebKit/537.36 (KHTML, like Gecko) " +
				"Version/4.0 Chrome/107.0.5304.141 Mobile Safari/537.36 XWEB/5075 MMWEBSDK/20230805 " +
				"MicroMessenger/8.0.42.2460(0x28002A35) WeChat/arm64 Weixin NetType/WIFI Language/zh_CN ABI/arm64",
			browser: "WeChat", device: "phone", os: "Android",
		},
		{
			name: "企业微信",
			ua: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) " +
				"Mobile/15E148 wxwork/4.1.10 MicroMessenger/7.0.1 Language/zh ColorScheme/Light",
			browser: "WeCom", device: "phone", os: "iOS",
		},
		{
			name:    "QQ 浏览器",
			ua:      "Mozilla/5.0 (Linux; U; Android 12; zh-cn; V2134A Build/SP1A.210812.003) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/100.0.4896.58 MQQBrowser/14.6 Mobile Safari/537.36",
			browser: "QQBrowser", device: "phone", os: "Android",
		},
		{
			name:    "UC 浏览器",
			ua:      "Mozilla/5.0 (Linux; U; Android 12; zh-CN; SM-G9910 Build/SP1A.210812.016) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/78.0.3904.108 UCBrowser/15.5.5.131 Mobile Safari/537.36",
			browser: "UC", device: "phone", os: "Android",
		},
		{
			name:    "夸克",
			ua:      "Mozilla/5.0 (Linux; U; Android 13; PGT-AN10 Build/HONORPGT-AN10) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/111.0.0.0 Quark/5.9.0.230 Mobile Safari/537.36",
			browser: "Quark", device: "phone", os: "Android",
		},
		{
			name:    "华为浏览器",
			ua:      "Mozilla/5.0 (Linux; Android 12; ELS-AN00) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/99.0.4844.88 Mobile Safari/537.36 HuaweiBrowser/12.0.4.300",
			browser: "Huawei", device: "phone", os: "Android",
		},
		{
			name:    "小米浏览器",
			ua:      "Mozilla/5.0 (Linux; U; Android 13; 2211133C Build/TKQ1.221114.001) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/94.0.4606.85 Mobile Safari/537.36 XiaoMi/MiuiBrowser/17.3.8",
			browser: "MIUI", device: "phone", os: "Android",
		},
		{
			name:    "三星浏览器",
			ua:      "Mozilla/5.0 (Linux; Android 13; SM-S9180) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/21.0 Chrome/110.0.5481.154 Mobile Safari/537.36",
			browser: "Samsung", device: "phone", os: "Android",
		},
		{
			name:    "手机版 Edge（Android）",
			ua:      "Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Mobile Safari/537.36 EdgA/119.0.2151.82",
			browser: "Edge", device: "phone", os: "Android",
		},
		{
			name:    "手机版 Chrome（Android）",
			ua:      "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.6099.144 Mobile Safari/537.36",
			browser: "Chrome", device: "phone", os: "Android",
		},
		{
			name:    "桌面 Edge",
			ua:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.2210.91",
			browser: "Edge", device: "desktop", os: "Windows",
		},
	}
	for _, c := range cases {
		got := ParseUserAgent(c.ua)
		if got.BrowserName != c.browser {
			t.Errorf("%s: 浏览器 = %q，期望 %q", c.name, got.BrowserName, c.browser)
		}
		if got.DeviceType != c.device {
			t.Errorf("%s: 设备 = %q，期望 %q", c.name, got.DeviceType, c.device)
		}
		if got.OSName != c.os {
			t.Errorf("%s: 系统 = %q，期望 %q", c.name, got.OSName, c.os)
		}
	}
}

// 微信 UA 里含 Chrome 字样，必须守住「不能落到 Chrome/Edge/Safari」这条线。
func TestParseUserAgent_WeChatNotCountedAsChrome(t *testing.T) {
	ua := "Mozilla/5.0 (Linux; Android 13; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 " +
		"Chrome/107.0.5304.141 Mobile Safari/537.36 MicroMessenger/8.0.42"
	got := ParseUserAgent(ua)
	if got.BrowserName == "Chrome" || got.BrowserName == "Edge" || got.BrowserName == "Safari" {
		t.Fatalf("微信被误判为 %q —— 统计里将看不到微信来源", got.BrowserName)
	}
	if got.BrowserName != "WeChat" {
		t.Fatalf("应为 WeChat，实际 %q", got.BrowserName)
	}
	if got.BrowserVersion == "" {
		t.Fatalf("应解析出微信版本号")
	}
}
