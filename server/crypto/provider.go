// Package crypto 国密（SM）双轨密码提供者抽象（需求文档 §4）。
//
// 双轨设计：
//   - 国际轨（默认）：intlProvider 直接委托 crypto/tls 标准库，零国密依赖；
//   - 国密轨（可选）：gm 构建标签下经 GMProvider 选择 Tongsuo（CGO，推荐）
//     或纯 Go gmtls 实现（无 CGO，可退路）。
//
// 默认构建（无 -tags gm）不导入任何国密模块（N-01）。
package crypto

import (
	"context"
	"crypto/tls"
	"encoding/asn1"
	"errors"
	"fmt"
	"net"

	"nvs-server/config"
)

// 国密 TLS 密码套件（需求文档 §2 / §12）。
const (
	// CipherSuiteGM_CBC GMTLS_ECDHE_SM2_WITH_SM4_CBC_SM3。
	CipherSuiteGM_CBC = 0xE011
	// CipherSuiteGM_GCM GMTLS_ECDHE_SM2_WITH_SM4_GCM_SM3。
	CipherSuiteGM_GCM = 0xE051
)

// OID（需求文档 §12，证书解析识别用）。
var (
	// OIDSM2Curve SM2 曲线 OID 1.2.156.10197.1.301。
	OIDSM2Curve = asn1.ObjectIdentifier{1, 2, 156, 10197, 1, 301}
	// OIDSM3 SM3 哈希 OID 1.2.156.10197.1.401。
	OIDSM3 = asn1.ObjectIdentifier{1, 2, 156, 10197, 1, 401}
	// OIDSM2WithSM3 SM2/SM3 签名算法 OID 1.2.156.10197.1.501。
	OIDSM2WithSM3 = asn1.ObjectIdentifier{1, 2, 156, 10197, 1, 501}
)

// 标准编号（文档 / 审计用，需求文档 §12）。
const (
	StandardTLCP        = "GB/T 38636-2020"
	StandardSSLVPN      = "GM/T 0024-2014"
	StandardCryptoModule = "GM/T 0028-2014"
	StandardMiPing      = "GB/T 39786-2021"
)

// Provider 国际轨与国密轨的统一抽象（需求文档 §4.2）。
type Provider interface {
	// Name 返回 "intl" / "tongsuo" / "purego"，用于审计日志。
	Name() string

	// BuildServerTLSConfig 根据 CryptoTrack 构建服务端 *tls.Config。
	// 国际轨直接返回标准库配置；国密轨返回 ErrUseBuildListener
	// （其 TLS 配置由库自有类型承载，须经 BuildListener 装配）。
	BuildServerTLSConfig(cfg *config.CryptoConfig) (*tls.Config, error)

	// SupportedCipherSuites 返回该提供者支持的密码套件 ID 列表。
	SupportedCipherSuites() []uint16

	// BuildListener 在标准 net.Listener 上叠加本提供者的 TLS 能力。
	// 国际轨未配置证书时原样返回（纯 HTTP，反向代理场景）。
	BuildListener(inner net.Listener, cfg *config.CryptoConfig) (net.Listener, error)
}

// SecretSource 密钥 / 凭据来源（需求文档 §6.1）。
// AI API Key 与证书私钥不得硬编码，经此接口按名取凭据。
type SecretSource interface {
	Get(ctx context.Context, name string) (string, error)
}

// ErrUseBuildListener 国密轨的 TLS 配置须经 BuildListener 装配（需求文档 §5.2 封装层）。
var ErrUseBuildListener = errors.New("国密轨 TLS 配置由库自有类型承载，请使用 BuildListener 装配监听器")

// New 按轨道构建 Provider。轨道非法或国密轨未编译进本二进制时返回错误。
func New(cfg *config.CryptoConfig) (Provider, error) {
	if cfg == nil {
		cfg = &config.CryptoConfig{Track: config.TrackIntl}
	}
	switch cfg.Track {
	case config.TrackIntl, "":
		return intlProvider{}, nil
	case config.TrackGM, config.TrackDual:
		return newGMProvider(cfg)
	default:
		return nil, fmt.Errorf("未知密码轨道: %q（允许 intl/gm/dual）", cfg.Track)
	}
}

// CompiledGMProviders 返回本二进制编译进的国密实现（默认构建为空）。
func CompiledGMProviders() []string {
	return compiledGMProviders()
}
