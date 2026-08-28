package security

// 爬虫识别 & 限流核心逻辑单元测试（无 DB 依赖部分）

import (
	"testing"
)

func TestClassifyBot_UAKeywords(t *testing.T) {
	cases := []struct {
		name string
		ua   string
		want bool
	}{
		{"无头 Chrome", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/120.0 Safari/537.36", true},
		{"python-requests", "python-requests/2.31.0", true},
		{"curl", "curl/8.1.2", true},
		{"Meta 商业爬虫", "Mozilla/5.0 (compatible; MetaWebIndexer/1.0)", true},
		{"GPTBot", "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko); compatible; GPTBot/1.0; +https://openai.com/gptbot", true},
		{"正常 Chrome（带 Sec-Fetch）", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36", false},
		{"正常 Firefox（带 Sec-Fetch）", "Mozilla/5.0 (Windows NT 10.0; rv:127.0) Gecko/20100101 Firefox/127.0", false},
		{"主流搜索引擎（放行）", "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := ClassifyBot(tc.ua, true, "1.2.3.4")
			if got != tc.want {
				t.Fatalf("期望 isBot=%v 实际 %v (reason=%s)", tc.want, got, reason)
			}
		})
	}
}

func TestClassifyBot_SpoofedBrowserUA(t *testing.T) {
	// 伪装检测：浏览器 UA 但完全缺失 Sec-Fetch-* 头 → 判 bot
	botSpoofCheck = true
	browserUA := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
	if got, _ := ClassifyBot(browserUA, false, "1.2.3.4"); !got {
		t.Fatal("浏览器 UA 但无 Sec-Fetch 头应判为伪装爬虫")
	}
	// 关闭伪装检测后放行
	botSpoofCheck = false
	if got, _ := ClassifyBot(browserUA, false, "1.2.3.4"); got {
		t.Fatal("关闭伪装检测后浏览器 UA（无 Sec-Fetch）不应判 bot")
	}
	botSpoofCheck = true
}

func TestClassifyBot_EmptyUA(t *testing.T) {
	if got, _ := ClassifyBot("", true, "1.2.3.4"); !got {
		t.Fatal("空 UA 应判为无头客户端")
	}
}

func TestClassifyBot_IPHighFrequency(t *testing.T) {
	botKeywordsMu.Lock()
	botIPThreshold = 5
	botIPTTLMin = 1
	botKeywordsMu.Unlock()

	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36"
	// 前 5 次不判，第 6 次起判（阈值 5）
	becameBot := false
	for i := 0; i < 8; i++ {
		if bot, _ := ClassifyBot(ua, true, "9.9.9.9"); bot {
			becameBot = true
			break
		}
	}
	if !becameBot {
		t.Fatal("单 IP 高频请求应自动降级为 bot")
	}

	botKeywordsMu.Lock()
	botIPThreshold = 300
	botKeywordsMu.Unlock()
}

func TestKeyRateLimit(t *testing.T) {
	// 配额 3 次/60 秒：前 3 次放行，第 4 次拒绝
	key := "test-key-1"
	for i := 1; i <= 3; i++ {
		if !KeyRateLimit(key, 3, 60) {
			t.Fatalf("第 %d 次应放行", i)
		}
	}
	if KeyRateLimit(key, 3, 60) {
		t.Fatal("第 4 次应被限流")
	}
}

func TestIsSensitivePath(t *testing.T) {
	if !isSensitivePath("/api/auth/login") {
		t.Fatal("登录接口应保留 IP 限流")
	}
	if isSensitivePath("/api/novels") {
		t.Fatal("读取接口不应视为敏感路径")
	}
}
