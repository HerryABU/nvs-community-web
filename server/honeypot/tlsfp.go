package honeypot

// ==================== TLS 指纹（JA3 / JA4）与 WebRTC 内网 IP（gap-audit #4） ====================
// 现实约束（必须先说清楚，否则就是骗人）：
//   - JA3/JA4 来自 TLS ClientHello，**只有终止 TLS 的那一层算得出来**。
//     本进程通常跑在反代/网关之后，看到的是明文 HTTP，拿不到 ClientHello。
//   - 所以这里提供两条采集路径，按部署形态选：
//       A. 反代透传（推荐、零成本）：nginx / Caddy / 网关算出 JA3/JA4 后放头
//          `X-JA3-Hash` / `X-JA4`（或 `X-TLS-Fingerprint`），本包直接采信并落库；
//       B. 本进程直接终止 TLS（不走反代）时，`c.Request.TLS` 可用，
//          但 Go 标准库不暴露 ClientHello 原始字节，故只记「TLS 版本 + 套件」，
//          JA3 标记为 `direct-no-ja3`（明确「没算」而不是「算不出来乱写」）。
//   - 反代没配时不静默：落 `ja3_missing`，并在后台给出配置模板，避免「以为在采其实没采」。
//
// WebRTC 内网 IP 走验证页 JS（RTCPeerConnection 收集 ICE candidate 里的 host 候选）：
// Chrome/Safari 现代版本默认 mDNS 混淆（xxx.local），能拿到真内网 IP 的场景在减少，
// 所以这里只做「有就记、没有就空」，不作为判定依据，只作为关联线索。

import (
	"net/http"
	"strings"
)

// TLSInfo 一次请求的 TLS 层信息
type TLSInfo struct {
	JA3       string `json:"ja3"`
	JA4       string `json:"ja4"`
	Version   string `json:"tls_version"`
	Source    string `json:"source"` // proxy_header / direct_tls / missing
	ViaProxy  bool   `json:"via_proxy"`
	TermLocal bool   `json:"terminated_locally"`
}

// tlsHeaderCandidates 反代常用头（按优先级）
var ja3Headers = []string{"X-JA3-Hash", "X-JA3", "X-TLS-JA3", "X-SSL-JA3"}
var ja4Headers = []string{"X-JA4", "X-JA4-Hash", "X-TLS-JA4"}

// CaptureTLS 采集 TLS 指纹（缺失时明确标 missing，不伪造）
func CaptureTLS(c *http.Request) TLSInfo {
	info := TLSInfo{Source: "missing"}
	for _, h := range ja3Headers {
		if v := strings.TrimSpace(c.Header.Get(h)); v != "" {
			info.JA3 = truncate(v, 64)
			info.Source = "proxy_header"
			info.ViaProxy = true
			break
		}
	}
	for _, h := range ja4Headers {
		if v := strings.TrimSpace(c.Header.Get(h)); v != "" {
			info.JA4 = truncate(v, 64)
			info.Source = "proxy_header"
			info.ViaProxy = true
			break
		}
	}
	// 兼容通用头（部分网关只给一个合并指纹）
	if info.JA3 == "" && info.JA4 == "" {
		if v := strings.TrimSpace(c.Header.Get("X-TLS-Fingerprint")); v != "" {
			info.JA3 = truncate(v, 64)
			info.Source = "proxy_header"
			info.ViaProxy = true
		}
	}
	if c.TLS != nil {
		info.TermLocal = true
		info.Version = tlsVersionName(c.TLS.Version)
		if info.Source == "missing" {
			// 本进程终止 TLS 但无法算 JA3：明确记录，不冒充
			info.Source = "direct_no_ja3"
		}
	}
	return info
}

func tlsVersionName(v uint16) string {
	switch v {
	case 0x0304:
		return "TLS1.3"
	case 0x0303:
		return "TLS1.2"
	case 0x0302:
		return "TLS1.1"
	case 0x0301:
		return "TLS1.0"
	}
	return "unknown"
}

// tlsHint 反代配置模板（后台直接展示，省得用户去翻文档）
func tlsHint() map[string]string {
	return map[string]string{
		"nginx": `# 需要 ngx_http_ssl_fingerprint 或 lua-resty-ja3 模块算出指纹后放头
map $ssl_ja3_hash $ja3_out { default $ssl_ja3_hash; "" "missing"; }
add_header X-JA3-Hash $ja3_out always;
proxy_set_header X-JA3-Hash $ja3_out;`,
		"caddy": `# Caddy 需插件（caddy-ja3 / caddy-tlsfingerprint）
header_up X-JA3-Hash {http.request.tls.ja3_hash}`,
		"note": "反代未透传时，蜜罐指纹里 ja3 会记为 missing（不会伪造）。本进程直接终止 TLS 时只记版本，不记 JA3。",
	}
}

// markTLSMissingStat 统计用（轻量：只统计会话里 ja3 缺失比例，供后台提示）
func TLSStats() map[string]interface{} {
	var total, withJA3, withJA4, withWebRTC int64
	db().Model(&Fingerprint{}).Count(&total)
	db().Model(&Fingerprint{}).Where("ja3 <> '' AND ja3 <> 'missing'").Count(&withJA3)
	db().Model(&Fingerprint{}).Where("ja4 <> ''").Count(&withJA4)
	db().Model(&Fingerprint{}).Where("webrtc_ips <> ''").Count(&withWebRTC)
	ratio := 0.0
	if total > 0 {
		ratio = float64(withJA3) / float64(total) * 100
	}
	return map[string]interface{}{
		"fingerprints": total,
		"with_ja3":     withJA3,
		"with_ja4":     withJA4,
		"with_webrtc":  withWebRTC,
		"ja3_ratio":    ratio,
		"hint":         tlsHint(),
		// 采集率低不是 bug，是反代没配；这里明说，避免被当成功能失效
		"diagnosis": diagnoseJA3(total, withJA3),
	}
}

func diagnoseJA3(total, withJA3 int64) string {
	switch {
	case total == 0:
		return "还没有指纹数据（蜜罐未被访问或等级为 off）"
	case withJA3 == 0:
		return "反代未透传 JA3/JA4：请按 hint 配置，或在蜜罐独立进程直接终止 TLS"
	case float64(withJA3)/float64(total) < 0.5:
		return "部分请求缺 JA3：可能有多个入口（只有经反代的那条会带指纹）"
	}
	return "JA3 采集正常"
}
