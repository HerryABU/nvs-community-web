package crypto

// intl.go 国际轨实现（需求文档 §4.2）。
//
// 说明：本文件不加 //go:build !gm 标签——dual（双轨自适应）构建下，
// 普通 TLS 客户端的回退路径同样需要国际轨能力；且本文件仅导入标准库，
// 不影响默认构建的「零国密依赖」要求（N-01）。

import (
	"crypto/tls"
	"fmt"
	"net"

	"nvs-server/config"
	"nvs-server/tlsconfig"
)

type intlProvider struct{}

func (intlProvider) Name() string { return "intl" }

// SupportedCipherSuites 返回 nil = 使用标准库默认套件表（RSA/ECC + AES）。
func (intlProvider) SupportedCipherSuites() []uint16 { return nil }

// BuildServerTLSConfig 构建标准 *tls.Config。
// 未配置国际轨证书时返回 (nil, nil)——调用方按纯 HTTP 处理（反向代理终止 TLS 的部署形态）。
func (intlProvider) BuildServerTLSConfig(cfg *config.CryptoConfig) (*tls.Config, error) {
	if cfg == nil || !cfg.IntlTLSConfigured() {
		return nil, nil
	}
	keyPair, err := tls.LoadX509KeyPair(cfg.IntlCertFile, cfg.IntlKeyFile)
	if err != nil {
		return nil, fmt.Errorf("加载国际轨证书失败（%s/%s）: %w", cfg.IntlCertFile, cfg.IntlKeyFile, err)
	}
	return &tls.Config{
		MinVersion:             tls.VersionTLS12,
		Certificates:           []tls.Certificate{keyPair},
		SessionTicketsDisabled: cfg.SessionTicketsDisabled,
		// 握手结果可观测（需求文档 §5.4）：握手完成后记录协议/套件/SNI
		VerifyConnection: func(cs tls.ConnectionState) error {
			tlsconfig.Record(tlsconfig.HandshakeRecord{
				Protocol:    tlsconfig.ProtocolName(cs.Version),
				CipherSuite: cs.CipherSuite,
				ServerName:  cs.ServerName,
			})
			return nil
		},
	}, nil
}

// BuildListener 国际轨：配证书 → tls.NewListener；未配 → 原样返回（纯 HTTP）。
func (intlProvider) BuildListener(inner net.Listener, cfg *config.CryptoConfig) (net.Listener, error) {
	tlsConf, err := intlProvider{}.BuildServerTLSConfig(cfg)
	if err != nil {
		return nil, err
	}
	if tlsConf == nil {
		return inner, nil
	}
	return tls.NewListener(inner, tlsConf), nil
}
