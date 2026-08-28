package security

// 爬虫识别：区分「无头爬虫」「伪装头的爬虫」「正常匿名访问」。
// 识别依据（管理员可在「限流与 API Key」页调整）：
//   1. UA 命中内置/自定义爬虫关键词（无头浏览器、下载工具、商业 AI 采集器）
//   2. 伪装检测：UA 声称是浏览器但缺少所有 Sec-Fetch-* 头（现代浏览器必然携带）
//   3. 行为降级：同一 IP 在窗口内请求数超过阈值 → 自动降级为 bot 配额
// 识别结果只决定限流配额高低（bot 远低于匿名），不直接封禁。

import (
	"strings"
	"sync"
	"time"

	"nvs-server/models"
)

// ==================== 内置爬虫关键词 ====================

// builtinBotKeywords 内置爬虫 UA 关键词（小写子串匹配）。
// 覆盖：无头浏览器、自动化工具、通用下载/抓取库、商业 AI 采集器。
// 注意：主流搜索引擎（googlebot/bingbot/ia_archiver 等）不在此列 —— 对学术/搜索爬虫保持正常响应。
var builtinBotKeywords = []string{
	// 无头浏览器 & 自动化
	"headlesschrome", "headless", "phantomjs", "playwright", "puppeteer",
	"selenium", "webdriver", "chrome-lighthouse",
	// 通用下载 / 抓取工具与库
	"python-requests", "python-urllib", "aiohttp", "httpx", "scrapy",
	"node-fetch", "axios", "go-http-client", "okhttp", "httpclient",
	"curl/", "wget", "libwww", "python", "java/", "jdk", "winhttp",
	// 商业 AI / 数据采集爬虫（Meta 等）
	"meta-externalagent", "metawebindexer", "meta-webindexer",
	"gptbot", "chatgpt", "anthropic-ai", "claude", "bytespider",
	"ccbot", "claudebot", "perplexitybot", "amazonbot", "google-extended",
	"petalbot", "imagesiftbot", "omgili", "diffbot", "dataforseobot",
	"mj12bot", "ahrefsbot", "semrushbot", "screaming frog",
}

// browserUAKeywords UA 声称是浏览器的特征词（用于伪装检测）
var browserUAKeywords = []string{
	"chrome", "firefox", "safari", "edg/", "opera", "msie", "trident", "webkit",
}

// ==================== 自定义爬虫关键词（运行时缓存） ====================

var (
	botKeywordsMu     sync.RWMutex
	botCustomKeywords []string
	botSpoofCheck     bool
	botIPThreshold    int
	botIPTTLMin       int
)

// ReloadBotRules 从平台配置刷新爬虫规则（启动与管理端保存时调用）
func ReloadBotRules() {
	cfg := models.LoadAPIRateLimitConfig()
	botKeywordsMu.Lock()
	defer botKeywordsMu.Unlock()
	botCustomKeywords = cfg.BotUAKeywords
	botSpoofCheck = cfg.BotSpoofCheck
	botIPThreshold = cfg.BotIPThreshold
	botIPTTLMin = cfg.BotIPTTLMin
	if botIPTTLMin <= 0 {
		botIPTTLMin = 1
	}
}

// init 设置无 DB 依赖的默认规则（正式规则由 main 连库后 ReloadBotRules 从平台配置加载）
func init() {
	botCustomKeywords = nil
	botSpoofCheck = true
	botIPThreshold = 300
	botIPTTLMin = 1
}

// ==================== IP 行为统计（自动降级） ====================

type botIPEntry struct {
	count       int
	windowStart time.Time
}

var (
	botIPStore = make(map[string]*botIPEntry)
	botIPMu    sync.Mutex
)

// ipOverThreshold 单 IP 在窗口内的请求数是否超过降级阈值（超过 → 判为高频采集）
func ipOverThreshold(ip string) bool {
	botKeywordsMu.RLock()
	threshold := botIPThreshold
	ttlMin := botIPTTLMin
	botKeywordsMu.RUnlock()
	if threshold <= 0 {
		return false
	}

	now := time.Now()
	ttl := time.Duration(ttlMin) * time.Minute
	botIPMu.Lock()
	defer botIPMu.Unlock()
	e, ok := botIPStore[ip]
	if !ok || now.Sub(e.windowStart) > ttl {
		botIPStore[ip] = &botIPEntry{count: 1, windowStart: now}
		return false
	}
	e.count++
	return e.count > threshold
}

// ==================== 主识别函数 ====================

// ClassifyBot 判断请求是否为爬虫，返回 (isBot, reason)。
// 注意：此函数仅在「无有效 API Key」时调用 —— 携带 Key 的请求走 Key 配额，不判爬虫。
func ClassifyBot(ua string, hasSecFetch bool, ip string) (bool, string) {
	lowerUA := strings.ToLower(ua)
	if lowerUA == "" {
		// 无 UA：正常浏览器必带 UA，缺失视为无头客户端
		return true, "missing_ua"
	}

	// 1) UA 关键词命中（内置 + 自定义）
	botKeywordsMu.RLock()
	custom := botCustomKeywords
	spoofCheck := botSpoofCheck
	botKeywordsMu.RUnlock()

	for _, kw := range builtinBotKeywords {
		if strings.Contains(lowerUA, kw) {
			return true, "ua:" + kw
		}
	}
	for _, kw := range custom {
		kw = strings.ToLower(strings.TrimSpace(kw))
		if kw != "" && strings.Contains(lowerUA, kw) {
			return true, "ua_custom:" + kw
		}
	}

	// 2) 伪装检测：UA 声称是浏览器，但请求完全缺失 Sec-Fetch-* 头
	//    （现代浏览器/前端 fetch 必然携带 Sec-Fetch-Site/Mode/Dest；缺全部 → 大概率是伪造 UA 的脚本）
	if spoofCheck && !hasSecFetch {
		for _, bk := range browserUAKeywords {
			if strings.Contains(lowerUA, bk) {
				return true, "spoof_browser_ua"
			}
		}
	}

	// 3) 行为降级：单 IP 高频（窗口内超过阈值）
	if ipOverThreshold(ip) {
		return true, "ip_high_frequency"
	}

	return false, ""
}

// HasSecFetchHeaders 请求是否携带任何 Sec-Fetch-* 头
func HasSecFetchHeaders(secFetchSite, secFetchMode, secFetchDest string) bool {
	return secFetchSite != "" || secFetchMode != "" || secFetchDest != ""
}

// BuiltinBotKeywords 返回内置爬虫关键词列表（管理端展示，站长可在此基础上追加自定义关键词）
func BuiltinBotKeywords() []string {
	out := make([]string, len(builtinBotKeywords))
	copy(out, builtinBotKeywords)
	return out
}
