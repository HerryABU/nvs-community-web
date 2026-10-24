//go:build gm

package cert

import (
	"crypto/rand"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/emmansun/gmsm/sm2"
	"github.com/emmansun/gmsm/smx509"
)

// genSM2CertPEM 生成自签 SM2 证书与私钥 PEM（仅测试用）
func genSM2CertPEM(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()
	priv, err := sm2.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("生成 SM2 密钥失败：%v", err)
	}
	tpl := &smx509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: "nvs-gm-test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              smx509.KeyUsageDigitalSignature | smx509.KeyUsageKeyEncipherment,
		BasicConstraintsValid: true,
	}
	der, err := smx509.CreateCertificate(rand.Reader, tpl, tpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("签发 SM2 证书失败：%v", err)
	}
	keyDER, err := smx509.MarshalSM2PrivateKey(priv)
	if err != nil {
		t.Fatalf("序列化 SM2 私钥失败：%v", err)
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM
}

// TestLoadDualCertificateGM 国密轨：SM2 双证书（签名 + 加密）应能加载并通过 OID 校验
func TestLoadDualCertificateGM(t *testing.T) {
	dir := t.TempDir()
	signCert, signKey := genSM2CertPEM(t)
	encCert, encKey := genSM2CertPEM(t)

	paths := map[string][]byte{
		"sign.crt": signCert, "sign.key": signKey,
		"enc.crt": encCert, "enc.key": encKey,
	}
	for name, data := range paths {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatalf("写临时证书失败：%v", err)
		}
	}

	dual, err := LoadDualCertificate(
		filepath.Join(dir, "sign.crt"), filepath.Join(dir, "sign.key"),
		filepath.Join(dir, "enc.crt"), filepath.Join(dir, "enc.key"),
	)
	if err != nil {
		t.Fatalf("加载 SM2 双证书失败：%v", err)
	}
	if dual == nil || len(dual.Sign.Certificate) == 0 || len(dual.Enc.Certificate) == 0 {
		t.Fatalf("双证书内容为空：%+v", dual)
	}
	if dual.Sign.PrivateKey == nil || dual.Enc.PrivateKey == nil {
		t.Fatalf("私钥未加载")
	}
}

// TestLoadDualCertificateGM_RejectRSA 非 SM2 证书必须被拒绝（OID 校验生效）
func TestLoadDualCertificateGM_RejectRSA(t *testing.T) {
	dir := t.TempDir()
	// 用标准库生成 RSA 证书
	certPath, keyPath := genRSAPEM(t, dir)
	if _, err := LoadDualCertificate(certPath, keyPath, certPath, keyPath); err == nil {
		t.Fatalf("RSA 证书不应通过 SM2 校验")
	}
}
