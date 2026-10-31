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

	// ── 内置浏览器 / 手机厂商浏览器 ──
	// 这些 UA 里通常同时带 Chrome 或 Safari 字样（微信 X5/WKWebView、UC、夸克等），
	// 所以必须**先于** Chrome/Safari 判定，否则微信会被统计成 Chrome，
	// 「多少人从微信里打开」这个关键问题就永远看不到。
	reWeCom        = regexp.MustCompile(`wxwork/([0-9.]+)`)
	reWeChat       = regexp.MustCompile(`micromessenger/([0-9.]+)`)
	reQQBrowser    = regexp.MustCompile(`mqqbrowser/([0-9.]+)`)
	reQQApp        = regexp.MustCompile(`\bqq/([0-9.]+)`)
	reUCBrowser    = regexp.MustCompile(`ucbrowser/([0-9.]+)`)
	reQuark        = regexp.MustCompile(`quark/([0-9.]+)`)
	reBaiduBrowser = regexp.MustCompile(`(?:baidubrowser|bidubrowser|baiduboxapp)/([0-9.]+)`)
	reSogou        = regexp.MustCompile(`(?:sogoumobilebrowser|metasr)/([0-9.]+)`)
	reHuawei       = regexp.MustCompile(`(?:huaweibrowser|hbpc)/([0-9.]+)`)
	reMIUI         = regexp.MustCompile(`miuibrowser/([0-9.]+)`)
	reSamsung      = regexp.MustCompile(`samsungbrowser/([0-9.]+)`)
	reVivo         = regexp.MustCompile(`vivobrowser/([0-9.]+)`)
	reOppo         = regexp.MustCompile(`(?:heytapbrowser|oppobrowser)/([0-9.]+)`)
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

	// ── 浏览器 ──
	// 顺序即优先级，三条铁律：
	//   1. 应用内置浏览器（微信 / 企业微信 / QQ / UC / 夸克 / 百度 / 搜狗 / 手机厂商）
	//      必须在 Chrome/Safari 之前判 —— 它们的 UA 里都带 Chrome 或 Safari 字样；
	//   2. Opera / Edge 也带 Chrome 字样，同样先于 Chrome；
	//   3. 都不命中才落到 Chrome / Firefox / Safari。
	// 站点运营最关心的「微信打开占比」全靠第 1 条，判错就永远看不到。
	switch {
	case reWeCom.MatchString(lower):
		info.BrowserName, info.BrowserVersion = "WeCom", reWeCom.FindStringSubmatch(lower)[1]
	case reWeChat.MatchString(lower):
		info.BrowserName, info.BrowserVersion = "WeChat", reWeChat.FindStringSubmatch(lower)[1]
	case reQQBrowser.MatchString(lower):
		info.BrowserName, info.BrowserVersion = "QQBrowser", reQQBrowser.FindStringSubmatch(lower)[1]
	case reUCBrowser.MatchString(lower):
		info.BrowserName, info.BrowserVersion = "UC", reUCBrowser.FindStringSubmatch(lower)[1]
	case reQuark.MatchString(lower):
		info.BrowserName, info.BrowserVersion = "Quark", reQuark.FindStringSubmatch(lower)[1]
	case reBaiduBrowser.MatchString(lower):
		info.BrowserName, info.BrowserVersion = "Baidu", reBaiduBrowser.FindStringSubmatch(lower)[1]
	case reSogou.MatchString(lower):
		info.BrowserName, info.BrowserVersion = "Sogou", reSogou.FindStringSubmatch(lower)[1]
	case reHuawei.MatchString(lower):
		info.BrowserName, info.BrowserVersion = "Huawei", reHuawei.FindStringSubmatch(lower)[1]
	case reMIUI.MatchString(lower):
		info.BrowserName, info.BrowserVersion = "MIUI", reMIUI.FindStringSubmatch(lower)[1]
	case reVivo.MatchString(lower):
		info.BrowserName, info.BrowserVersion = "vivo", reVivo.FindStringSubmatch(lower)[1]
	case reOppo.MatchString(lower):
		info.BrowserName, info.BrowserVersion = "OPPO", reOppo.FindStringSubmatch(lower)[1]
	case reSamsung.MatchString(lower):
		info.BrowserName, info.BrowserVersion = "Samsung", reSamsung.FindStringSubmatch(lower)[1]
	case reQQApp.MatchString(lower) && info.DeviceType != "desktop":
		info.BrowserName, info.BrowserVersion = "QQ", reQQApp.FindStringSubmatch(lower)[1]
	}
	if info.BrowserName != "unknown" {
		return info
	}

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
