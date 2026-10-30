package utils

import (
	"regexp"
	"strings"
)

// UserAgentInfo 从 User-Agent 解析出的结构化客户端画像。
// 用于「用户画像 / 客户端分布」可视化（功能二：网络诊断面板）。
// 与 middleware.classifyUAFamily（粗分类，用于 bot 判定）互补：
// 这里给出更细的 OS / 浏览器 / 设备型号，供运营做设备维度统计。
type UserAgentInfo struct {
	OSName         string `json:"os_name"`         // Windows / iOS / Android / macOS / Linux / unknown
	OSVersion      string `json:"os_version"`      // 如 10/11、16.0、13.5
	BrowserName    string `json:"browser_name"`    // Chrome / Edge / Firefox / Safari / Opera / unknown
	BrowserVersion string `json:"browser_version"` // 如 120.0.0.0
	DeviceType     string `json:"device_type"`     // desktop / phone / tablet / unknown
	DeviceModel    string `json:"device_model"`    // iPhone / iPad / 具体 Android 机型（未知时为空）
}

var (
	reWindowsNT    = regexp.MustCompile(`windows nt ([0-9.]+)`)
	reIOS          = regexp.MustCompile(`(?:iphone os|cpu os) ([0-9_]+) like mac os x`)
	reAndroidVer   = regexp.MustCompile(`android[ /]([0-9]+(?:\.[0-9]+){0,2})`)
	reAndroidModel = regexp.MustCompile(`(?i);\s*([^;]+?)\s+build/`)
	reMacOS        = regexp.MustCompile(`mac os x ([0-9_]+)`)
	reOpera        = regexp.MustCompile(`opr/([0-9.]+)`)
	reEdg          = regexp.MustCompile(`edg(?:e|e|a)?/([0-9.]+)`)
	reChrome       = regexp.MustCompile(`(?:chrome|crios)/([0-9.]+)`)
	reFirefox      = regexp.MustCompile(`(?:firefox|fxios)/([0-9.]+)`)
	reSafari       = regexp.MustCompile(`version/([0-9.]+)\s+.*safari`)
)

// ParseUserAgent 解析 User-Agent 字符串为结构化客户端画像。
// 纯字符串 + 正则，无外部依赖；对任何非法 UA 都返回 unknown 而非 panic。
func ParseUserAgent(ua string) UserAgentInfo {
	info := UserAgentInfo{DeviceType: "unknown", OSName: "unknown", BrowserName: "unknown"}
	if strings.TrimSpace(ua) == "" {
		return info
	}
	lower := strings.ToLower(ua)

	// ── 设备类型 + 机型 ──
	switch {
	case strings.Contains(lower, "iphone"):
		info.DeviceType = "phone"
		info.DeviceModel = "iPhone"
	case strings.Contains(lower, "ipad"):
		info.DeviceType = "tablet"
		info.DeviceModel = "iPad"
	case strings.Contains(lower, "android"):
		info.DeviceType = "phone"
		if m := reAndroidModel.FindStringSubmatch(ua); m != nil {
			info.DeviceModel = strings.TrimSpace(m[1])
		}
	case strings.Contains(lower, "macintosh"), strings.Contains(lower, "windows"),
		strings.Contains(lower, "x11"), strings.Contains(lower, "linux"):
		info.DeviceType = "desktop"
	default:
		if strings.Contains(lower, "mobile") {
			info.DeviceType = "phone"
		}
	}

	// ── 操作系统 ──
	switch {
	case strings.Contains(lower, "windows nt"):
		info.OSName = "Windows"
		if m := reWindowsNT.FindStringSubmatch(lower); m != nil {
			info.OSVersion = ntToName(m[1])
		}
	case strings.Contains(lower, "cpu os") || strings.Contains(lower, "iphone os"):
		info.OSName = "iOS"
		if m := reIOS.FindStringSubmatch(lower); m != nil {
			info.OSVersion = strings.ReplaceAll(m[1], "_", ".")
		}
	case strings.Contains(lower, "android"):
		info.OSName = "Android"
		if m := reAndroidVer.FindStringSubmatch(lower); m != nil {
			info.OSVersion = m[1]
		}
	case strings.Contains(lower, "mac os x"):
		info.OSName = "macOS"
		if m := reMacOS.FindStringSubmatch(lower); m != nil {
			info.OSVersion = strings.ReplaceAll(m[1], "_", ".")
		}
	case strings.Contains(lower, "linux"):
		info.OSName = "Linux"
	}

	// ── 浏览器（注意顺序：Opera/Edge 的 UA 也含 Chrome 字样，须先于 Chrome 判定）──
	if m := reOpera.FindStringSubmatch(lower); m != nil {
		info.BrowserName = "Opera"
		info.BrowserVersion = m[1]
	} else if m := reEdg.FindStringSubmatch(lower); m != nil {
		info.BrowserName = "Edge"
		info.BrowserVersion = m[1]
	} else if m := reChrome.FindStringSubmatch(lower); m != nil {
		info.BrowserName = "Chrome"
		info.BrowserVersion = m[1]
	} else if m := reFirefox.FindStringSubmatch(lower); m != nil {
		info.BrowserName = "Firefox"
		info.BrowserVersion = m[1]
	} else if strings.Contains(lower, "safari") {
		// iOS / macOS 上的 Safari；CriOS/Edg 等已在前面优先匹配
		info.BrowserName = "Safari"
		if m := reSafari.FindStringSubmatch(lower); m != nil {
			info.BrowserVersion = m[1]
		}
	}
	return info
}

// ntToName 把 Windows NT 内核版本映射为常见发行名
func ntToName(v string) string {
	switch v {
	case "10.0":
		return "10/11"
	case "6.3":
		return "8.1"
	case "6.2":
		return "8"
	case "6.1":
		return "7"
	default:
		return v
	}
}
