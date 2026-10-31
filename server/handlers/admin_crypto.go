package handlers

// admin_crypto.go 密码轨道（国际轨 / 国密轨 / 双轨）状态查询。
//
// 设计取向：**只读**。
// TLS 证书与密码套件必须在监听器创建前就位（provider → BuildListener），
// 运行期热改 .env 不会生效。与其给一个「点了保存但什么都不变」的表单，
// 不如把真实状态摊开、把「改哪些键 + 要重启」写清楚。
//
// 数据来源优先复用既有能力：
//   - audit.BuildGMAuditReport —— 轨道 / provider / 证书有效期 / 握手统计 / 会话复用
//   - crypto.CompiledGMProviders —— 当前二进制实际编进了哪些国密实现
// 这样后台看到的数字和 `nvs-server gm-audit` 的 CLI 报告永远同源。

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"time"

	"nvs-server/audit"
	"nvs-server/config"
	"nvs-server/crypto"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// cryptoCertInfo 一张证书的实际磁盘状态。
type cryptoCertInfo struct {
	Role     string `json:"role"`  // intl / sm_sign / sm_enc
	Label    string `json:"label"` // 展示名
	Path     string `json:"path"`
	Present  bool   `json:"present"`
	Readable bool   `json:"readable"`
	NotAfter string `json:"not_after,omitempty"`
	DaysLeft int    `json:"days_left,omitempty"`
	Expired  bool   `json:"expired"`
	SelfSign bool   `json:"self_signed,omitempty"`
	Subject  string `json:"subject,omitempty"`
	Error    string `json:"error,omitempty"`
}

// cryptoTrackDoc 一档轨道的说明（前端直接渲染，避免文案写死在 vue 里）。
type cryptoTrackDoc struct {
	Value    string   `json:"value"`
	Label    string   `json:"label"`
	Summary  string   `json:"summary"`
	Certs    []string `json:"certs"`    // 需要哪些证书
	BestFor  string   `json:"best_for"` // 适用场景
	Requires string   `json:"requires"` // 额外条件
}

// AdminCryptoStatus GET /api/admin/crypto/status
func AdminCryptoStatus(c *gin.Context) {
	cfg := config.CryptoCfg
	rep := audit.BuildGMAuditReport(cfg)

	// ── 证书明细：国际轨单独解析（标准库 x509），SM2 两张用 audit 已算好的有效期 ──
	certs := []cryptoCertInfo{
		inspectIntlCert(cfg.IntlCertFile),
		{
			Role:     "sm_sign",
			Label:    "SM2 签名证书",
			Path:     cfg.SMSignCertFile,
			Present:  cryptoFileExists(cfg.SMSignCertFile),
			Readable: cryptoFileExists(cfg.SMSignCertFile),
			NotAfter: rep.CertExpiry["sign"],
			Expired:  certExpired(rep.CertExpiry["sign"]),
		},
		{
			Role:     "sm_enc",
			Label:    "SM2 加密证书",
			Path:     cfg.SMEncCertFile,
			Present:  cryptoFileExists(cfg.SMEncCertFile),
			Readable: cryptoFileExists(cfg.SMEncCertFile),
			NotAfter: rep.CertExpiry["enc"],
			Expired:  certExpired(rep.CertExpiry["enc"]),
		},
	}
	for i := range certs {
		if certs[i].NotAfter != "" {
			certs[i].DaysLeft = daysUntil(certs[i].NotAfter)
		}
		// 私钥存在性并入「就绪」判断：只有证书没有私钥，TLS 起不来
		if certs[i].Role == "intl" && cryptoFileExists(cfg.IntlKeyFile) {
			certs[i].Readable = true
		}
	}

	tracks := []cryptoTrackDoc{
		{
			Value:    "intl",
			Label:    "国际轨（默认）",
			Summary:  "只用标准 TLS（RSA/ECDSA + AES-GCM/ChaCha20）。二进制里不含任何国密模块。",
			Certs:    []string{"国际证书 + 私钥（或使用内置自签）"},
			BestFor:  "绝大多数站点。浏览器兼容性最好，零额外依赖。",
			Requires: "无需额外依赖",
		},
		{
			Value:    "gm",
			Label:    "国密轨（纯国密）",
			Summary:  "只接受 TLCP 国密握手（SM2 双证书 + SM4/SM3）。普通浏览器（标准 TLS）无法连接。",
			Certs:    []string{"SM2 签名证书 + 私钥", "SM2 加密证书 + 私钥"},
			BestFor:  "纯内网、专网、已全面部署国密客户端的场景。",
			Requires: "以 -tags gm 构建（纯 Go，无 CGO）",
		},
		{
			Value:    "dual",
			Label:    "双轨（国密 + 国际并行）",
			Summary:  "按 ClientHello 自动分流：国密客户端走 TLCP，普通浏览器走标准 TLS，两套证书同时加载。",
			Certs:    []string{"国际证书 + 私钥", "SM2 签名证书 + 私钥", "SM2 加密证书 + 私钥"},
			BestFor:  "既要过密评、又不能影响普通访客 —— 这是绝大多数站长的落地选择。",
			Requires: "以 -tags gm 构建（纯 Go）或 -tags \"gm tongsuo\"（CGO）",
		},
	}

	utils.Success(c, gin.H{
		"track":              string(cfg.Track),
		"gm_provider":        cfg.GMProvider,
		"compiled_providers": crypto.CompiledGMProviders(),
		"purego_version":     rep.PureGoVersion,
		"tongsuo_version":    rep.TongsuoVersion,
		"gm_compiled":        len(crypto.CompiledGMProviders()) > 0,
		"session_reuse_on":   rep.SessionReuseOn,
		"handshake_log_path": cfg.HandshakeLogPath,
		"handshake_log_on":   cfg.HandshakeLogPath != "",
		"sm2_cert_present":   rep.SM2CertPresent,
		"certs":              certs,
		"handshake":          rep.HandshakeStats,
		"tracks":             tracks,
		"generated_at":       rep.GeneratedAt,
	})
}

// inspectIntlCert 解析国际轨证书（PEM，取第一张 X.509）。
// 不存在 / 解析失败不算错误 —— 未配置时 TLS 会走内置自签，这里如实显示「未配置」。
func inspectIntlCert(path string) cryptoCertInfo {
	info := cryptoCertInfo{
		Role:  "intl",
		Label: "国际轨证书",
		Path:  path,
	}
	if path == "" {
		return info
	}
	info.Present = cryptoFileExists(path)
	if !info.Present {
		return info
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		info.Error = "读取失败：" + err.Error()
		return info
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		info.Error = "不是 PEM 格式"
		return info
	}
	crt, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		info.Error = "解析失败：" + err.Error()
		return info
	}
	info.Readable = true
	info.NotAfter = crt.NotAfter.Format(time.RFC3339)
	info.Subject = crt.Subject.CommonName
	info.Expired = crt.NotAfter.Before(time.Now())
	info.DaysLeft = int(time.Until(crt.NotAfter).Hours() / 24)
	info.SelfSign = crt.Subject.CommonName == crt.Issuer.CommonName
	return info
}

func cryptoFileExists(p string) bool {
	if p == "" {
		return false
	}
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// certExpired 依据 RFC3339 字符串判断是否过期（解析不出来就当作没过期，避免误报红）。
func certExpired(notAfter string) bool {
	if notAfter == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, notAfter)
	if err != nil {
		return false
	}
	return t.Before(time.Now())
}

func daysUntil(notAfter string) int {
	t, err := time.Parse(time.RFC3339, notAfter)
	if err != nil {
		return 0
	}
	return int(time.Until(t).Hours() / 24)
}
