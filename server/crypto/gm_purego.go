//go:build gm && !tongsuo

package crypto

// gm_purego.go 纯 Go 国密轨（-tags gm，无 CGO，可退路）。
//
// TLCP 握手由 github.com/aiyang-zh/zhenyi-base/zgmtls 实现（GM/T 0024-2014 风格，
// 版本标识 VersionGMSSL=0x0101）；SM2 证书解析经 gmsm/smx509（OID 1.2.156.10197.1.301）。
// dual 轨使用 zgmtls.NewBasicAutoSwitchConfig：按 ClientHello 的 SupportedVersions
// 是否含 VersionGMSSL 决定进入国密握手还是标准 TLS 握手（需求文档 §5.2）。

import (
	"crypto/tls"
	"fmt"
	"net"
	"sync"

	"github.com/aiyang-zh/zhenyi-base/zgmtls"
	"nvs-server/cert"
	"nvs-server/config"
	"nvs-server/tlsconfig"
)

type pureGoProvider struct{}

func init() {
	gmProviderFactory = newPureGoProvider
	gmProviderName = config.GMProviderPureGo
}

func newPureGoProvider(cfg *config.CryptoConfig) (Provider, error) {
	if cfg.GMProvider != config.GMProviderPureGo {
		return nil, fmt.Errorf("此二进制编译的是 purego（纯 Go，无 CGO）国密实现，但 NVS_CRYPTO_GM_PROVIDER=%q；请改用 purego，或以 -tags gm,tongsuo 重新构建（Tongsuo CGO）", cfg.GMProvider)
	}
	// 启动即验证双证书可加载（fail-fast，避免带病启动）
	if _, err := cert.LoadDualCertificate(cfg.SMSignCertFile, cfg.SMSignKeyFile, cfg.SMEncCertFile, cfg.SMEncKeyFile); err != nil {
		return nil, err
	}
	return pureGoProvider{}, nil
}

func (pureGoProvider) Name() string { return "purego" }

// SupportedCipherSuites 国密套件：ECDHE-SM2-WITH-SM4-SM3（0xE051 类）优先，
// 静态 ECC 双证书 GMTLS_SM2_WITH_SM4_SM3（0xE011 类）兜底。
func (pureGoProvider) SupportedCipherSuites() []uint16 {
	return []uint16{
		zgmtls.GMTLS_ECDHE_SM2_WITH_SM4_SM3,
		zgmtls.GMTLS_SM2_WITH_SM4_SM3,
	}
}

// BuildServerTLSConfig zgmtls.Config 为 crypto/tls.Config 的国密扩展分支
// （含 GMSupport / GetKECertificate），装配须经 BuildListener → zgmtls.NewListener
// （需求文档 §5.2 封装层；避免 unsafe 指针转换带来的布局兼容风险）。
func (pureGoProvider) BuildServerTLSConfig(cfg *config.CryptoConfig) (*tls.Config, error) {
	return nil, ErrUseBuildListener
}

// BuildListener 构建国密监听器：gm 轨纯国密；dual 轨双证书自适应协商。
func (pureGoProvider) BuildListener(inner net.Listener, cfg *config.CryptoConfig) (net.Listener, error) {
	dual, err := cert.LoadDualCertificate(cfg.SMSignCertFile, cfg.SMSignKeyFile, cfg.SMEncCertFile, cfg.SMEncKeyFile)
	if err != nil {
		return nil, err
	}
	signCert := toGMTLSCert(dual.Sign)
	encCert := toGMTLSCert(dual.Enc)

	var gmCfg *zgmtls.Config
	if cfg.Track == config.TrackDual {
		// 双轨：标准证书走 zgmtls.LoadX509KeyPair（RSA/ECC）
		stdCert, err := zgmtls.LoadX509KeyPair(cfg.IntlCertFile, cfg.IntlKeyFile)
		if err != nil {
			return nil, fmt.Errorf("加载国际轨证书失败（dual 回退路径）: %w", err)
		}
		gmCfg, err = zgmtls.NewBasicAutoSwitchConfig(&signCert, &encCert, &stdCert)
		if err != nil {
			return nil, fmt.Errorf("构建双轨自适应配置失败: %w", err)
		}
	} else {
		// 纯国密：签名证书在前、加密证书在后（TLCP 双证书）
		gmCfg = &zgmtls.Config{
			GMSupport:    zgmtls.NewGMSupport(),
			Certificates: []zgmtls.Certificate{signCert, encCert},
			CipherSuites: []uint16{
				zgmtls.GMTLS_ECDHE_SM2_WITH_SM4_SM3,
				zgmtls.GMTLS_SM2_WITH_SM4_SM3,
			},
			MinVersion: zgmtls.VersionGMSSL,
			MaxVersion: zgmtls.VersionGMSSL,
			// 显式证书回调：静态 ECC 双证书套件（0xE011 类）需要加密证书参与密钥交换
			GetCertificate:   func(*zgmtls.ClientHelloInfo) (*zgmtls.Certificate, error) { return &signCert, nil },
			GetKECertificate: func(*zgmtls.ClientHelloInfo) (*zgmtls.Certificate, error) { return &encCert, nil },
		}
	}
	if cfg.SessionTicketsDisabled {
		gmCfg.SessionTicketsDisabled = true
	}
	// zgmtls.Config 无 VerifyConnection 回调，握手完成后经 ConnectionState 观测（§5.4）
	return &recordedListener{Listener: zgmtls.NewListener(inner, gmCfg)}, nil
}

// toGMTLSCert std tls.Certificate → zgmtls.Certificate（字段同名同构；
// SM2 私钥为 gmsm *sm2.PrivateKey，zgmtls 同样使用 gmsm，类型互通）。
func toGMTLSCert(c tls.Certificate) zgmtls.Certificate {
	return zgmtls.Certificate{Certificate: c.Certificate, PrivateKey: c.PrivateKey}
}

// recordedListener 握手观测包装（需求文档 §5.4 / N-02 隔离）。
type recordedListener struct{ net.Listener }

func (l *recordedListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &recordedGMConn{Conn: c}, nil
}

type recordedGMConn struct {
	net.Conn
	once sync.Once
}

func (c *recordedGMConn) Read(b []byte) (int, error) {
	c.observe()
	return c.Conn.Read(b)
}

func (c *recordedGMConn) Write(b []byte) (int, error) {
	c.observe()
	return c.Conn.Write(b)
}

// observe 首次 I/O（握手已完成）时记录协商结果。
func (c *recordedGMConn) observe() {
	c.once.Do(func() {
		gc, ok := c.Conn.(*zgmtls.Conn)
		if !ok {
			return
		}
		cs := gc.ConnectionState()
		tlsconfig.Record(tlsconfig.HandshakeRecord{
			Protocol:    tlsconfig.ProtocolName(cs.Version),
			CipherSuite: cs.CipherSuite,
			ServerName:  cs.ServerName,
		})
	})
}
