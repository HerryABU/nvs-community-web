package audit

// gmaudit.go 国密自检报告（需求文档 §8.1）。
//
// 报告仅汇总框架侧可观测数据，不生成密评结论；
// 站长可自行提交给测评机构参考（docs/gm-mapping.md）。

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"nvs-server/config"
	"nvs-server/crypto"
	"nvs-server/tlsconfig"
)

// HandshakeStats 握手统计（需求文档 §8.1，从日志聚合 + 内存快照）。
type HandshakeStats struct {
	Total   int64            `json:"total"`
	Failed  int64            `json:"failed"`
	ByProto map[string]int64 `json:"by_proto"`
	BySuite map[string]int64 `json:"by_suite"`
}

// GMAuditReport 国密自检报告（需求文档 §8.1 字段全集）。
type GMAuditReport struct {
	Track             string            `json:"track"`
	GMProvider        string            `json:"gm_provider,omitempty"`
	TongsuoVersion    string            `json:"tongsuo_version,omitempty"` // CGO 路径
	PureGoVersion     string            `json:"purego_version,omitempty"`  // 纯 Go 路径
	SM2CertPresent    bool              `json:"sm2_cert_present"`
	CertExpiry        map[string]string `json:"cert_expiry"` // 签名/加密证书有效期
	HandshakeStats    HandshakeStats    `json:"handshake_stats"`
	SessionReuseOn    bool              `json:"session_reuse_on"`
	CompiledProviders []string          `json:"compiled_providers"`
	GeneratedAt       time.Time         `json:"generated_at"`
}

// BuildGMAuditReport 依据当前配置构建自检报告。
func BuildGMAuditReport(cfg *config.CryptoConfig) GMAuditReport {
	rep := GMAuditReport{
		Track:             string(cfg.Track),
		GMProvider:        cfg.GMProvider,
		CertExpiry:        map[string]string{},
		CompiledProviders: crypto.CompiledGMProviders(),
		SessionReuseOn:    !cfg.SessionTicketsDisabled,
		GeneratedAt:       time.Now(),
	}
	if cfg.Track == config.TrackGM || cfg.Track == config.TrackDual {
		rep.SM2CertPresent, rep.CertExpiry = inspectSM2Certs(cfg)
	}
	// 实现版本（debug.ReadBuildInfo 反映实际构建的模块版本）
	for mod, ver := range moduleVersions() {
		switch {
		case strings.Contains(mod, "tongsuo-go-sdk"):
			rep.TongsuoVersion = mod + " " + ver
		case strings.Contains(mod, "gmsm"):
			rep.PureGoVersion = mod + " " + ver
		case strings.Contains(mod, "zhenyi-base"):
			if rep.PureGoVersion == "" {
				rep.PureGoVersion = mod + " " + ver
			} else {
				rep.PureGoVersion += "; " + mod + " " + ver
			}
		}
	}
	rep.HandshakeStats = aggregateHandshakes(cfg.HandshakeLogPath)
	return rep
}

// moduleVersions 构建信息中国密相关模块 → 版本。
func moduleVersions() map[string]string {
	out := map[string]string{}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return out
	}
	for _, d := range bi.Deps {
		if strings.Contains(d.Path, "tongsuo") || strings.Contains(d.Path, "gmsm") || strings.Contains(d.Path, "zhenyi") {
			out[d.Path] = d.Version
		}
	}
	return out
}

// aggregateHandshakes 聚合握手记录：JSONL 日志文件（全量）+ 内存快照（本次进程）。
func aggregateHandshakes(logPath string) HandshakeStats {
	stats := HandshakeStats{ByProto: map[string]int64{}, BySuite: map[string]int64{}}
	if logPath != "" {
		if f, err := os.Open(logPath); err == nil {
			sc := bufio.NewScanner(f)
			sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
			for sc.Scan() {
				var rec tlsconfig.HandshakeRecord
				if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
					continue
				}
				stats.Total++
				if rec.Err != "" {
					stats.Failed++
					continue
				}
				stats.ByProto[rec.Protocol]++
				stats.BySuite[fmt.Sprintf("0x%04X", rec.CipherSuite)]++
			}
			_ = f.Close()
		}
	}
	mem := tlsconfig.StatsSnapshot()
	stats.Total += mem.Total
	stats.Failed += mem.Failed
	for k, v := range mem.ByProto {
		stats.ByProto[k] += v
	}
	for k, v := range mem.BySuite {
		stats.BySuite[k] += v
	}
	return stats
}

// RenderMarkdown 渲染 Markdown 报告（需求文档 §8.1：输出为合法 Markdown；
// N-03：中英双语，lang = "zh"（默认）/ "en"）。
func (r GMAuditReport) RenderMarkdown(lang string) string {
	if lang == "en" {
		return r.renderEN()
	}
	return r.renderZH()
}

func (r GMAuditReport) renderZH() string {
	var b strings.Builder
	b.WriteString("# 国密（SM）自检报告（gm-audit）\n\n")
	b.WriteString(fmt.Sprintf("- 生成时间：%s\n", r.GeneratedAt.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("- 密码轨道（track）：`%s`\n", r.Track))
	b.WriteString(fmt.Sprintf("- 国密实现（gm_provider）：`%s`\n", orDash(r.GMProvider)))
	b.WriteString(fmt.Sprintf("- 编译进本二进制的国密实现：%s\n", joinOr(r.CompiledProviders, "无（默认国际轨构建）")))
	b.WriteString(fmt.Sprintf("- Tongsuo 版本（CGO 路径）：%s\n", orDash(r.TongsuoVersion)))
	b.WriteString(fmt.Sprintf("- 纯 Go 实现版本：%s\n", orDash(r.PureGoVersion)))
	b.WriteString(fmt.Sprintf("- SM2 证书就绪：%v\n", r.SM2CertPresent))
	b.WriteString(fmt.Sprintf("- 会话复用（SessionTickets）：%s\n", onOff(r.SessionReuseOn)))
	b.WriteString("\n## 证书有效期\n\n")
	b.WriteString("| 证书 | NotAfter |\n|---|---|\n")
	if len(r.CertExpiry) == 0 {
		b.WriteString("| （未配置 SM2 证书） | — |\n")
	}
	for k, v := range r.CertExpiry {
		b.WriteString(fmt.Sprintf("| %s | %s |\n", k, v))
	}
	b.WriteString("\n## 握手统计\n\n")
	b.WriteString(fmt.Sprintf("- 总握手：%d（失败 %d）\n", r.HandshakeStats.Total, r.HandshakeStats.Failed))
	b.WriteString("- 按协议：\n")
	writeKV(&b, r.HandshakeStats.ByProto)
	b.WriteString("- 按套件：\n")
	writeKV(&b, r.HandshakeStats.BySuite)
	b.WriteString("\n---\n\n")
	b.WriteString("> 本报告仅汇总框架侧可观测数据，**不构成密评结论**；密评结论以测评机构正式报告为准（GB/T 39786-2021 / GM/T 0115-2021）。\n")
	return b.String()
}

func (r GMAuditReport) renderEN() string {
	var b strings.Builder
	b.WriteString("# GM (SM) Self-Audit Report (gm-audit)\n\n")
	b.WriteString(fmt.Sprintf("- Generated: %s\n", r.GeneratedAt.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("- Crypto track: `%s`\n", r.Track))
	b.WriteString(fmt.Sprintf("- GM provider: `%s`\n", orDash(r.GMProvider)))
	b.WriteString(fmt.Sprintf("- Compiled GM providers: %s\n", joinOr(r.CompiledProviders, "none (default intl build)")))
	b.WriteString(fmt.Sprintf("- Tongsuo version (CGO): %s\n", orDash(r.TongsuoVersion)))
	b.WriteString(fmt.Sprintf("- Pure Go version: %s\n", orDash(r.PureGoVersion)))
	b.WriteString(fmt.Sprintf("- SM2 certificates ready: %v\n", r.SM2CertPresent))
	b.WriteString(fmt.Sprintf("- Session reuse (SessionTickets): %s\n", onOff(r.SessionReuseOn)))
	b.WriteString("\n## Certificate expiry\n\n")
	b.WriteString("| Certificate | NotAfter |\n|---|---|\n")
	if len(r.CertExpiry) == 0 {
		b.WriteString("| (no SM2 certificates configured) | — |\n")
	}
	for k, v := range r.CertExpiry {
		b.WriteString(fmt.Sprintf("| %s | %s |\n", k, v))
	}
	b.WriteString("\n## Handshake stats\n\n")
	b.WriteString(fmt.Sprintf("- Total: %d (failed %d)\n", r.HandshakeStats.Total, r.HandshakeStats.Failed))
	b.WriteString("- By protocol:\n")
	writeKV(&b, r.HandshakeStats.ByProto)
	b.WriteString("- By cipher suite:\n")
	writeKV(&b, r.HandshakeStats.BySuite)
	b.WriteString("\n---\n\n")
	b.WriteString("> This report aggregates framework-side observable data only and **is NOT a compliance (miping) conclusion**; conclusions come from the formal assessment report (GB/T 39786-2021 / GM/T 0115-2021).\n")
	return b.String()
}

func writeKV(b *strings.Builder, m map[string]int64) {
	if len(m) == 0 {
		b.WriteString("  - （无记录 / none）\n")
		return
	}
	for k, v := range m {
		b.WriteString(fmt.Sprintf("  - `%s`: %d\n", k, v))
	}
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return "`" + s + "`"
}

func joinOr(list []string, fallback string) string {
	if len(list) == 0 {
		return fallback
	}
	return "`" + strings.Join(list, "`, `") + "`"
}

func onOff(on bool) string {
	if on {
		return "开启（on）"
	}
	return "关闭（off）"
}
