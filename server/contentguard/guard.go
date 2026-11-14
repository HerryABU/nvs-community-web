// Package contentguard 内容风控闸门：发布前对评论/帖子做编码混淆、敏感信息（PII）、
// 外链异常检测，产出 allow/review/block 决策，供评论与论坛链路统一接入。
//
// 处置原则：
//   - 编码层层包装且解码后仍无意义 → review/block，上报 AI + 管理员，且不外搜；
//   - 疑似泄露 PII → review（前端提醒用户确认后才可发送）；
//   - 外链异常 → review（前端弹窗「即将离开本站」）。
package contentguard

import (
	"regexp"
	"strings"

	"nvs-server/security"
)

// Decision 风控决策
type Decision struct {
	Action     string   `json:"action"` // allow | review | block
	Reason     string   `json:"reason,omitempty"`
	Decoded    string   `json:"decoded,omitempty"` // 解码后文本（供上报/审阅）
	Layers     []string `json:"layers,omitempty"`
	PIITypes   []string `json:"pii_types,omitempty"`
	Links      []string `json:"links,omitempty"`
	RiskyLinks []string `json:"risky_links,omitempty"`
	Suspicion  int      `json:"suspicion"`
}

// 常见短链/可疑域名
var shortLinkHosts = map[string]bool{
	"bit.ly": true, "t.cn": true, "tinyurl.com": true, "goo.gl": true,
	"is.gd": true, "v.gd": true, "ow.ly": true, "cutt.ly": true,
	"rebrand.ly": true, "buff.ly": true, "suo.im": true, "dwz.cn": true,
}

var suspiciousTLDs = []string{".top", ".xyz", ".tk", ".ml", ".ga", ".cf", ".gq", ".club", ".loan", ".cam", ".rest", ".icu", ".cyou", ".pw"}

var (
	reIDCard   = regexp.MustCompile(`\b\d{17}[\dXx]\b`)
	reMobile   = regexp.MustCompile(`\b1[3-9]\d{9}\b`)
	reEmail    = regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`)
	reBirth    = regexp.MustCompile(`\b(?:19|20)\d{2}\s*[年\-/.]\s*(?:0?[1-9]|1[0-2])\s*[月\-/.]\s*(?:0?[1-9]|[12]\d|3[01])`)
	reBank     = regexp.MustCompile(`\b(?:\d{4}[\s\-]?){3,4}\d{1,4}\b`)
	reAPIKey   = regexp.MustCompile(`(?i)\b(?:sk|pk|api|apikey|api_key|token|secret|passwd|password|access_key|private_key)[\-_\s]*[A-Za-z0-9+/_\-]{12,}`)
	reURL      = regexp.MustCompile(`(?i)(?:https?://|www\.)[^\s<>"']+`)
	reIPv4URL  = regexp.MustCompile(`(?i)(?:https?://)?\b\d{1,3}(?:\.\d{1,3}){3}(?::\d+)?(?:/[^\s]*)?`)
	reAddress  = regexp.MustCompile(`(?i)(?:省|市|区|县|路|街|号|栋|单元|室|小区|社区|村|镇|乡)`)
	reDateWord = regexp.MustCompile(`(?:住址|地址|现居|居住地|户籍)`)
)

// GuardText 对内容做风控检测（scene: comment/forum/blog/novel）
func GuardText(content, scene string) Decision {
	d := Decision{Action: "allow"}
	if strings.TrimSpace(content) == "" {
		return d
	}

	// 1) 编码混淆检测与递归解码（始终尝试：decodeOnce 只在「像编码」时才生效）
	original := content
	res := security.DecodeLayers(content, 5)
	decoded := res.Text
	layers := res.Layers
	if len(layers) > 0 {
		d.Decoded = decoded
		d.Layers = layers
	}
	suspicion := security.SuspicionScore(original, decoded, layers)

	// 2) 解码后仍不可读 → 尝试凯撒位移与倒序，看能否「恢复正常」
	if suspicion >= 20 && security.ReadabilityScore(decoded) < 30 {
		if c, score := security.TryCaesar(decoded); score > security.ReadabilityScore(decoded) {
			decoded = c
			layers = append(layers, "caesar")
			d.Layers = layers
			d.Decoded = decoded
			suspicion = security.SuspicionScore(original, decoded, layers)
		}
		rev := security.ReverseText(decoded)
		if security.ReadabilityScore(rev) > security.ReadabilityScore(decoded) {
			decoded = rev
			layers = append(layers, "reverse")
			d.Layers = layers
			d.Decoded = decoded
		}
	}

	// 3) 违禁词扫描（原文 + 解码后），命中即高危
	combined := original
	if decoded != original {
		combined = original + "\n" + decoded
	}
	hits := security.ScanBadWords(combined, scene)
	if len(hits) > 0 {
		// 解码后命中违禁词：高度可疑，直接屏蔽
		if decoded != original {
			d.Action = "block"
			d.Reason = "编码混淆后命中违禁规则（" + hits[0].RuleName + "）"
			d.Suspicion = 100
			return d
		}
		// 原文直接命中：交既有违禁词流程处理，这里标记 review
		d.Action = "review"
		d.Reason = "命中违禁规则（" + hits[0].RuleName + "）"
		d.Suspicion = max(d.Suspicion, 70)
	}

	// 4) 编码可疑但无法恢复 → 屏蔽 + 上报（不外搜）
	if len(layers) >= 2 && suspicion >= 45 {
		d.Action = "block"
		d.Reason = "多层编码混淆且无法正常还原（疑似绕过审核）"
		d.Suspicion = max(d.Suspicion, 85)
		return d
	}
	if len(layers) >= 1 && suspicion >= 60 && security.ReadabilityScore(decoded) < 20 {
		d.Action = "block"
		d.Reason = "编码内容无法还原为可读文本"
		d.Suspicion = max(d.Suspicion, 90)
		return d
	}

	// 5) PII 检测（原文 + 解码后）
	piiTypes := detectPII(combined)
	if len(piiTypes) > 0 {
		d.PIITypes = piiTypes
		d.Action = "review"
		d.Reason = "疑似泄露敏感信息（" + strings.Join(piiTypes, "、") + "）"
	}

	// 6) URL 检测与异常判断
	links, risky := detectRiskyLinks(original)
	d.Links = links
	d.RiskyLinks = risky
	if len(risky) > 0 {
		d.Action = "review"
		d.Reason = "包含异常外链（" + strings.Join(risky, "，") + "）"
		d.Suspicion = max(d.Suspicion, 65)
	}

	d.Suspicion = max(d.Suspicion, suspicion)
	if d.Action == "allow" && suspicion >= 40 {
		d.Action = "review"
		d.Reason = "内容存在可疑编码特征"
	}
	return d
}

// detectPII 检测敏感信息类型
func detectPII(text string) []string {
	var out []string
	if reIDCard.MatchString(text) {
		out = append(out, "身份证号")
	}
	if reMobile.MatchString(text) {
		out = append(out, "手机号")
	}
	if reEmail.MatchString(text) {
		out = append(out, "邮箱")
	}
	if reBirth.MatchString(text) {
		out = append(out, "生日")
	}
	if reBank.MatchString(text) {
		out = append(out, "银行卡号")
	}
	if reAPIKey.MatchString(text) {
		out = append(out, "APIKEY/密钥")
	}
	// 住址：地址词 + 数字/小区等
	if reAddress.MatchString(text) && reDateWord.MatchString(text) {
		out = append(out, "住址")
	}
	return out
}

// detectRiskyLinks 提取链接并判断异常
func detectRiskyLinks(text string) (links []string, risky []string) {
	matches := reURL.FindAllString(text, -1)
	for _, u := range matches {
		links = append(links, u)
	}
	// IP 直链：一律视为异常
	for _, m := range reIPv4URL.FindAllString(text, -1) {
		if !contains(links, m) {
			links = append(links, m)
		}
		risky = append(risky, m+"（IP 直链）")
	}
	for _, u := range links {
		lower := strings.ToLower(u)
		host := extractHost(lower)
		// 短链
		if shortLinkHosts[host] {
			risky = append(risky, u+"（短链）")
			continue
		}
		// 可疑 TLD
		for _, tld := range suspiciousTLDs {
			if strings.HasSuffix(host, tld) {
				risky = append(risky, u+"（可疑域名 "+tld+"）")
				break
			}
		}
	}
	return links, risky
}

func extractHost(u string) string {
	s := u
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	if i := strings.Index(s, "@"); i >= 0 {
		s = s[i+1:]
	}
	if i := strings.Index(s, ":"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimPrefix(s, "www.")
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
