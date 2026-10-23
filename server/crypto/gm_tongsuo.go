//go:build gm && tongsuo

package crypto

// gm_tongsuo.go Tongsuo CGO 国密轨（-tags gm,tongsuo，官方绑定，推荐路径）。
//
// 依赖 Tongsuo C 库（编译时需 --enable-ntls），经 github.com/tongsuo-project/tongsuo-go-sdk
// 绑定。ctx 风格 API：NewCtxWithVersion(NTLS) + 签名/加密双证书四件套。
//
// dual 轨：Tongsuo SDK v1.0.0 未提供混合模式 API，故在连接准入层按 ClientHello
// 记录层协议版本嗅探分流——0x0101（TLCP）→ Tongsuo NTLS，0x03xx（标准 TLS）→ crypto/tls。
// 与纯 Go 路径的 NewBasicAutoSwitchConfig 判断依据一致（需求文档 §5.2 isGMClientHello）。
//
// ⚠️ 本文件仅在站长侧装有 Tongsuo C 库的环境可编译验证；无 CGO 环境请用 purego 路径。

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"os"
	"sync"

	ts "github.com/tongsuo-project/tongsuo-go-sdk"
	tscrypto "github.com/tongsuo-project/tongsuo-go-sdk/crypto"
	"nvs-server/config"
	"nvs-server/tlsconfig"
)

type tongsuoProvider struct{}

func init() {
	gmProviderFactory = newTongsuoProvider
	gmProviderName = config.GMProviderTongsuo
}

func newTongsuoProvider(cfg *config.CryptoConfig) (Provider, error) {
	if cfg.GMProvider != config.GMProviderTongsuo {
		return nil, fmt.Errorf("此二进制编译的是 tongsuo（CGO）国密实现，但 NVS_CRYPTO_GM_PROVIDER=%q；请改用 tongsuo，或以 -tags gm 重新构建（purego 纯 Go 实现）", cfg.GMProvider)
	}
	// TONGSUO_HOME 校验（需求文档 §5.3：路径错误时明确报错，不崩溃）
	if home := os.Getenv("TONGSUO_HOME"); home != "" {
		if _, err := os.Stat(home); err != nil {
			return nil, fmt.Errorf("TONGSUO_HOME=%s 不可访问: %w（请修正路径或改用 purego 实现）", home, err)
		}
	} else {
		log.Printf("[crypto] 未设置 TONGSUO_HOME，按系统默认库路径加载 Tongsuo")
	}
	// 启动即构建 ctx 验证双证书（fail-fast）
	if _, err := buildTongsuoCtx(cfg); err != nil {
		return nil, err
	}
	return tongsuoProvider{}, nil
}

func (tongsuoProvider) Name() string { return "tongsuo" }

// SupportedCipherSuites 国密套件（OpenSSL 风格名称 → 国标套件 ID）。
func (tongsuoProvider) SupportedCipherSuites() []uint16 {
	return []uint16{CipherSuiteGM_GCM, CipherSuiteGM_CBC}
}

// BuildServerTLSConfig Tongsuo 的配置承载于 *ts.Ctx（非标准 *tls.Config），
// 装配须经 BuildListener。
func (tongsuoProvider) BuildServerTLSConfig(cfg *config.CryptoConfig) (*tls.Config, error) {
	return nil, ErrUseBuildListener
}

// BuildListener gm 轨 → 整监听器 NTLS；dual 轨 → 记录层嗅探分流。
func (tongsuoProvider) BuildListener(inner net.Listener, cfg *config.CryptoConfig) (net.Listener, error) {
	ctx, err := buildTongsuoCtx(cfg)
	if err != nil {
		return nil, err
	}
	if cfg.Track != config.TrackDual {
		return ts.NewListener(inner, ctx), nil
	}

	intlConf, err := intlProvider{}.BuildServerTLSConfig(cfg)
	if err != nil {
		return nil, err
	}
	if intlConf == nil {
		return nil, fmt.Errorf("dual 轨需要国际轨证书（NVS_CRYPTO_INTL_CERT/NVS_CRYPTO_INTL_KEY）")
	}
	return &dualRouterListener{inner: inner, gmCtx: ctx, intlConf: intlConf}, nil
}

// buildTongsuoCtx 构建 NTLS 上下文并加载 SM2 双证书四件套。
func buildTongsuoCtx(cfg *config.CryptoConfig) (*ts.Ctx, error) {
	ctx, err := ts.NewCtxWithVersion(ts.NTLS)
	if err != nil {
		return nil, fmt.Errorf("创建 Tongsuo NTLS 上下文失败（Tongsuo C 库是否已安装并 enable-ntls？）: %w", err)
	}
	pair := tscrypto.GMDoubleCertKey{
		SignCertFile: cfg.SMSignCertFile,
		SignKeyFile:  cfg.SMSignKeyFile,
		EncCertFile:  cfg.SMEncCertFile,
		EncKeyFile:   cfg.SMEncKeyFile,
	}
	if err := loadDoubleCert(ctx, pair); err != nil {
		return nil, err
	}
	// 国密套件：ECDHE-SM2-WITH-SM4-SM3（0xE051 类）+ ECC-SM2-WITH-SM4-SM3（0xE011 类）
	if err := ctx.SetCipherList("ECDHE-SM2-WITH-SM4-SM3:ECC-SM2-WITH-SM4-SM3"); err != nil {
		return nil, fmt.Errorf("设置国密套件失败: %w", err)
	}
	return ctx, nil
}

// loadDoubleCert 将签名/加密双证书四件套载入 ctx。
func loadDoubleCert(ctx *ts.Ctx, pair tscrypto.GMDoubleCertKey) error {
	load := func(certFile, keyFile string,
		useCert func(*tscrypto.Certificate) error,
		useKey func(tscrypto.PrivateKey) error) error {
		certPEM, err := tscrypto.LoadPEMFromFile(certFile)
		if err != nil {
			return fmt.Errorf("读取证书 %s 失败: %w", certFile, err)
		}
		c, err := tscrypto.LoadCertificateFromPEM(certPEM)
		if err != nil {
			return fmt.Errorf("解析证书 %s 失败: %w", certFile, err)
		}
		if err := useCert(c); err != nil {
			return fmt.Errorf("载入证书 %s 失败: %w", certFile, err)
		}
		keyPEM, err := os.ReadFile(keyFile)
		if err != nil {
			return fmt.Errorf("读取私钥 %s 失败: %w", keyFile, err)
		}
		k, err := tscrypto.LoadPrivateKeyFromPEM(keyPEM)
		if err != nil {
			return fmt.Errorf("解析私钥 %s 失败: %w", keyFile, err)
		}
		if err := useKey(k); err != nil {
			return fmt.Errorf("载入私钥 %s 失败: %w", keyFile, err)
		}
		return nil
	}
	if err := load(pair.SignCertFile, pair.SignKeyFile, ctx.UseSignCertificate, ctx.UseSignPrivateKey); err != nil {
		return err
	}
	return load(pair.EncCertFile, pair.EncKeyFile, ctx.UseEncryptCertificate, ctx.UseEncryptPrivateKey)
}

// ==================== dual 轨：记录层嗅探分流 ====================

// dualRouterListener 按 ClientHello 记录层协议版本分流：
//   - 0x16 0x01 0x01（TLCP，VersionGMSSL）→ Tongsuo NTLS 握手
//   - 0x16 0x03 xx（标准 TLS）→ crypto/tls 握手
//   - 其他（非 TLS 流量，如健康探测）→ 原样透传
//
// 隔离性（N-02）：国密分支 panic 由 recover 捕获并记录，不影响国际 TLS 路径。
type dualRouterListener struct {
	inner    net.Listener
	gmCtx    *ts.Ctx
	intlConf *tls.Config
}

func (l *dualRouterListener) Accept() (net.Conn, error) {
	c, err := l.inner.Accept()
	if err != nil {
		return nil, err
	}
	br := bufio.NewReaderSize(c, 512)
	// 嗅探前 3 字节记录头（阻塞直到客户端首发数据；TLS 客户端必先发 ClientHello）
	head, err := br.Peek(3)
	if err != nil || len(head) < 3 || head[0] != 0x16 {
		// 非握手记录：透传（由上层按普通连接处理）
		return &peekedConn{Conn: c, br: br}, nil
	}
	if head[1] == 0x01 && head[2] == 0x01 { // TLCP
		return l.wrapGM(c, br)
	}
	return &peekedConn{Conn: c, br: br, tlsConf: l.intlConf}, nil
}

// wrapGM Tongsuo NTLS 分支（panic 隔离，N-02）。
func (l *dualRouterListener) wrapGM(c net.Conn, br *bufio.Reader) (conn net.Conn, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[crypto] 国密握手路径异常（已隔离，国际 TLS 不受影响）: %v", r)
			_ = c.Close()
			conn, err = nil, fmt.Errorf("国密握手路径异常: %v", r)
		}
	}()
	tsConn, err := ts.Server(&peekedConn{Conn: c, br: br}, l.gmCtx)
	if err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("Tongsuo NTLS 包装失败: %w", err)
	}
	return &observedTongsuoConn{Conn: tsConn}, nil
}

// peekedConn 带缓冲嗅探的连接；tlsConf 非空时首次 I/O 前升级为 TLS。
type peekedConn struct {
	net.Conn
	br      *bufio.Reader
	tlsConf *tls.Config
	once    sync.Once
	tlsConn net.Conn
}

func (c *peekedConn) upgrade() {
	c.once.Do(func() {
		if c.tlsConf == nil {
			return
		}
		c.tlsConn = tls.Server(c, c.tlsConf)
	})
}

func (c *peekedConn) Read(b []byte) (int, error) {
	c.upgrade()
	if c.tlsConn != nil {
		return c.tlsConn.Read(b)
	}
	return c.br.Read(b)
}

func (c *peekedConn) Write(b []byte) (int, error) {
	c.upgrade()
	if c.tlsConn != nil {
		return c.tlsConn.Write(b)
	}
	return c.Conn.Write(b)
}

func (c *peekedConn) Close() error { return c.Conn.Close() }

// observedTongsuoConn 握手完成后记录协议/套件（Tongsuo 经 GetVersion/CurrentCipher）。
type observedTongsuoConn struct {
	net.Conn // *ts.Conn
	once sync.Once
}

func (c *observedTongsuoConn) Read(b []byte) (int, error) {
	c.observe()
	return c.Conn.Read(b)
}

func (c *observedTongsuoConn) Write(b []byte) (int, error) {
	c.observe()
	return c.Conn.Write(b)
}

func (c *observedTongsuoConn) observe() {
	c.once.Do(func() {
		tc, ok := c.Conn.(*ts.Conn)
		if !ok {
			return
		}
		_ = tc.Handshake()
		ver, _ := tc.GetVersion()
		cipher, _ := tc.CurrentCipher()
		proto := "tlcp"
		if ver != "" && ver != "NTLS" {
			proto = tlsconfig.ProtocolName(stdVersionOf(ver))
		}
		tlsconfig.Record(tlsconfig.HandshakeRecord{
			Protocol:    proto,
			CipherSuite: tongsuoCipherID(cipher),
		})
	})
}

// stdVersionOf Tongsuo 版本字符串 → 标准库版本号（尽力而为）。
func stdVersionOf(ver string) uint16 {
	switch ver {
	case "TLSv1":
		return tls.VersionTLS10
	case "TLSv1.1":
		return tls.VersionTLS11
	case "TLSv1.2":
		return tls.VersionTLS12
	case "TLSv1.3":
		return tls.VersionTLS13
	}
	return 0
}

// tongsuoCipherID Tongsuo 套件名 → 国标套件 ID（未知返回 0）。
func tongsuoCipherID(name string) uint16 {
	switch name {
	case "ECC-SM2-WITH-SM4-SM3":
		return CipherSuiteGM_CBC
	case "ECDHE-SM2-WITH-SM4-SM3":
		return CipherSuiteGM_GCM
	}
	return 0
}
