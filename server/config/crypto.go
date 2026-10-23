package config

import (
	"fmt"
	"log"
	"strings"
)

// ==================== 国密（SM）双轨密码配置 ====================
//
// 框架级可选能力：默认国际轨（intl，标准 crypto/tls，零国密依赖）；
// 站长显式开启国密轨（gm）或双轨自适应（dual）后，框架经 crypto.Provider
// 抽象接入 Tongsuo（CGO）或纯 Go gmtls 实现。
//
// 责任边界：框架不代做等保定级、密评或内容安全评估；是否启用、如何满足合规
// 由站长自行决定并承担相应责任（见 docs/gm-mapping.md）。

// CryptoTrack 密码轨道。
type CryptoTrack string

const (
	// TrackIntl 国际轨（默认）：RSA/ECC + AES，标准 crypto/tls。
	TrackIntl CryptoTrack = "intl"
	// TrackGM 纯国密轨：SM2/SM3/SM4 + TLCP（GB/T 38636-2020）。
	TrackGM CryptoTrack = "gm"
	// TrackDual 双轨自适应：按 ClientHello 在国密 TLS 与国际 TLS 间协商。
	TrackDual CryptoTrack = "dual"
)

// 国密实现路径（GMProvider 取值）。
const (
	// GMProviderTongsuo Tongsuo-Go-SDK（CGO，官方绑定，推荐）。
	GMProviderTongsuo = "tongsuo"
	// GMProviderPureGo 纯 Go GM-TLS 实现（无 CGO，可退路）。
	GMProviderPureGo = "purego"
)

// CryptoConfig 国密双轨配置（需求文档 §4.1）。
// 全部字段来自 .env，启动时集中校验、失败即退出（fail-fast）。
type CryptoConfig struct {
	// Track 轨道：intl（默认）/ gm / dual。
	Track CryptoTrack `json:"track"`

	// 国际轨证书（track=intl/dual 时使用）
	IntlCertFile string `json:"intl_cert_file"`
	IntlKeyFile  string `json:"intl_key_file"`

	// 国密双证书：签名证书 + 加密证书（TLCP 要求两套）
	SMSignCertFile string `json:"sm_sign_cert_file"`
	SMSignKeyFile  string `json:"sm_sign_key_file"`
	SMEncCertFile  string `json:"sm_enc_cert_file"`
	SMEncKeyFile   string `json:"sm_enc_key_file"`

	// GMProvider 国密实现路径："tongsuo"（CGO，推荐）或 "purego"（无 CGO）。
	GMProvider string `json:"gm_provider"`

	// SessionTicketsDisabled 禁用 TLS 会话复用（映射 tls.Config.SessionTicketsDisabled）。
	SessionTicketsDisabled bool `json:"session_tickets_disabled"`

	// HandshakeLogPath 握手记录 JSONL 输出路径（空 = 关闭，需求文档 §5.4）。
	HandshakeLogPath string `json:"handshake_log_path"`
}

// 环境变量名（.env）。
const (
	envCryptoTrack          = "NVS_CRYPTO_TRACK"
	envCryptoIntlCert       = "NVS_CRYPTO_INTL_CERT"
	envCryptoIntlKey        = "NVS_CRYPTO_INTL_KEY"
	envCryptoSMSignCert     = "NVS_CRYPTO_SM_SIGN_CERT"
	envCryptoSMSignKey      = "NVS_CRYPTO_SM_SIGN_KEY"
	envCryptoSMEncCert      = "NVS_CRYPTO_SM_ENC_CERT"
	envCryptoSMEncKey       = "NVS_CRYPTO_SM_ENC_KEY"
	envCryptoGMProvider     = "NVS_CRYPTO_GM_PROVIDER"
	envCryptoTicketsDisable = "NVS_CRYPTO_SESSION_TICKETS_DISABLED"
	envTLSHandshakeLog      = "NVS_TLS_HANDSHAKE_LOG"
)

var (
	// CryptoTrackSetting 轨道设置（包级变量，与既有配置风格一致）。
	CryptoTrackSetting = TrackIntl
	// CryptoCfg 加载后的国密配置快照（Init 后可用）。
	CryptoCfg *CryptoConfig
	// TLSHandshakeLogPath 握手记录输出路径（空 = 关闭）。
	TLSHandshakeLogPath string
)

// initCrypto 由 config.Init() 调用：读取国密相关环境变量。
func initCrypto() {
	track := strings.ToLower(strings.TrimSpace(getEnv(envCryptoTrack, string(TrackIntl))))
	switch CryptoTrack(track) {
	case TrackIntl, TrackGM, TrackDual:
		CryptoTrackSetting = CryptoTrack(track)
	case "":
		CryptoTrackSetting = TrackIntl
	default:
		// 保留原值由 Validate 报错（fail-fast 在 main 中触发）
		CryptoTrackSetting = CryptoTrack(track)
	}

	CryptoCfg = &CryptoConfig{
		Track:                  CryptoTrackSetting,
		IntlCertFile:           strings.TrimSpace(getEnv(envCryptoIntlCert, "")),
		IntlKeyFile:            strings.TrimSpace(getEnv(envCryptoIntlKey, "")),
		SMSignCertFile:         strings.TrimSpace(getEnv(envCryptoSMSignCert, "")),
		SMSignKeyFile:          strings.TrimSpace(getEnv(envCryptoSMSignKey, "")),
		SMEncCertFile:          strings.TrimSpace(getEnv(envCryptoSMEncCert, "")),
		SMEncKeyFile:           strings.TrimSpace(getEnv(envCryptoSMEncKey, "")),
		GMProvider:             strings.ToLower(strings.TrimSpace(getEnv(envCryptoGMProvider, ""))),
		SessionTicketsDisabled: getEnvBool(envCryptoTicketsDisable, false),
		HandshakeLogPath:       strings.TrimSpace(getEnv(envTLSHandshakeLog, "")),
	}
	TLSHandshakeLogPath = CryptoCfg.HandshakeLogPath

	if CryptoTrackSetting == TrackIntl {
		if CryptoCfg.IntlCertFile != "" && CryptoCfg.IntlKeyFile != "" {
			log.Printf("[config] 密码轨道: intl（国际轨 TLS：%s）", CryptoCfg.IntlCertFile)
		}
		return
	}
	log.Printf("[config] 密码轨道: %s（国密实现: %s）", CryptoTrackSetting, CryptoCfg.GMProvider)
}

// Validate 校验国密配置（需求文档 §4.1：gm/dual 必须显式选择实现路径）。
// 返回错误时 main 应 fail-fast 退出，避免带病启动。
func (c *CryptoConfig) Validate() error {
	if c == nil {
		return nil
	}
	switch c.Track {
	case TrackIntl, "":
		// 国际轨：证书要么都不配（纯 HTTP，反向代理场景），要么成对配置
		if (c.IntlCertFile == "") != (c.IntlKeyFile == "") {
			return fmt.Errorf("NVS_CRYPTO_INTL_CERT 与 NVS_CRYPTO_INTL_KEY 必须成对配置")
		}
		return nil
	case TrackGM, TrackDual:
	default:
		return fmt.Errorf("NVS_CRYPTO_TRACK 取值非法: %q（允许 intl/gm/dual）", c.Track)
	}

	if c.GMProvider == "" {
		return fmt.Errorf("NVS_CRYPTO_TRACK=%s 时必须显式指定 NVS_CRYPTO_GM_PROVIDER（tongsuo=推荐 CGO 官方绑定 / purego=纯 Go 无 CGO）", c.Track)
	}
	if c.GMProvider != GMProviderTongsuo && c.GMProvider != GMProviderPureGo {
		return fmt.Errorf("NVS_CRYPTO_GM_PROVIDER 取值非法: %q（允许 tongsuo/purego）", c.GMProvider)
	}

	// 国密双证书四件套必填（TLCP 签名+加密两套）
	for _, f := range []struct{ name, val string }{
		{"NVS_CRYPTO_SM_SIGN_CERT", c.SMSignCertFile},
		{"NVS_CRYPTO_SM_SIGN_KEY", c.SMSignKeyFile},
		{"NVS_CRYPTO_SM_ENC_CERT", c.SMEncCertFile},
		{"NVS_CRYPTO_SM_ENC_KEY", c.SMEncKeyFile},
	} {
		if f.val == "" {
			return fmt.Errorf("NVS_CRYPTO_TRACK=%s 时 %s 不能为空（TLCP 要求签名+加密双证书）", c.Track, f.name)
		}
	}

	// dual 轨额外要求国际轨证书（普通 TLS 客户端回退路径）
	if c.Track == TrackDual {
		if c.IntlCertFile == "" || c.IntlKeyFile == "" {
			return fmt.Errorf("NVS_CRYPTO_TRACK=dual 时必须同时配置 NVS_CRYPTO_INTL_CERT/NVS_CRYPTO_INTL_KEY（普通客户端回退 TLS1.3）")
		}
	}
	return nil
}

// GMEnabled 是否启用国密轨（gm/dual）。
func (c *CryptoConfig) GMEnabled() bool {
	return c != nil && (c.Track == TrackGM || c.Track == TrackDual)
}

// IntlTLSConfigured 国际轨 TLS 是否已配置证书（intl 轨且证书成对）。
func (c *CryptoConfig) IntlTLSConfigured() bool {
	return c != nil && c.IntlCertFile != "" && c.IntlKeyFile != ""
}
