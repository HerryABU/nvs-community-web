//go:build gm

package cert

// dual_gm.go 国密轨双证书加载：经 gmsm/smx509 解析 SM2 证书与私钥，
// 通过 OID 1.2.156.10197.1.301 识别 SM2 公钥（标准库 crypto/x509 识别有限）。

import (
	"crypto/ecdsa"
	"crypto/tls"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/emmansun/gmsm/sm2"
	"github.com/emmansun/gmsm/smx509"
)

// LoadDualCertificate 从 PEM 文件加载 SM2 双证书（需求文档 §5.1）。
func LoadDualCertificate(signCert, signKey, encCert, encKey string) (*DualCertificate, error) {
	sign, err := loadSM2KeyPair(signCert, signKey)
	if err != nil {
		return nil, fmt.Errorf("SM2 签名证书加载失败: %w", err)
	}
	enc, err := loadSM2KeyPair(encCert, encKey)
	if err != nil {
		return nil, fmt.Errorf("SM2 加密证书加载失败: %w", err)
	}
	return &DualCertificate{Sign: sign, Enc: enc}, nil
}

// loadSM2KeyPair 加载一对 SM2 证书 + 私钥为 tls.Certificate。
func loadSM2KeyPair(certFile, keyFile string) (tls.Certificate, error) {
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("读取证书失败: %w", err)
	}
	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("读取私钥失败: %w", err)
	}

	leafDER := firstPEMBlock(certPEM, "CERTIFICATE")
	if leafDER == nil {
		return tls.Certificate{}, fmt.Errorf("%s 中未找到 CERTIFICATE 块", certFile)
	}
	leaf, err := smx509.ParseCertificate(leafDER)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("解析证书失败: %w", err)
	}
	// SM2 公钥识别（gmsm v0.45+）：SM2 公钥即标准 *ecdsa.PublicKey，曲线为 SM2，
	// 对应 OID 1.2.156.10197.1.301。标准库 x509 不识别该 OID，故经 smx509 解析。
	pub, ok := leaf.PublicKey.(*ecdsa.PublicKey)
	if !ok || pub.Curve != sm2.P256() {
		return tls.Certificate{}, fmt.Errorf("证书 %s 公钥不是 SM2（OID 1.2.156.10197.1.301）：%s", certFile, leaf.PublicKeyAlgorithm)
	}
	priv, err := parseSM2PrivateKeyPEM(keyPEM)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{
		Certificate: [][]byte{leafDER},
		PrivateKey:  priv,
	}, nil
}

// parseSM2PrivateKeyPEM 解析 SM2 私钥（SEC1「EC PRIVATE KEY」或 PKCS8「PRIVATE KEY」）。
func parseSM2PrivateKeyPEM(keyPEM []byte) (*sm2.PrivateKey, error) {
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, fmt.Errorf("私钥不是合法 PEM")
	}
	switch block.Type {
	case "EC PRIVATE KEY":
		return smx509.ParseSM2PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		key, err := smx509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		if k, ok := key.(*sm2.PrivateKey); ok {
			return k, nil
		}
		return nil, fmt.Errorf("PKCS8 私钥不是 SM2 类型（got %T）", key)
	}
	return nil, fmt.Errorf("不支持的私钥 PEM 类型: %s（支持 EC PRIVATE KEY / PRIVATE KEY）", block.Type)
}

// firstPEMBlock 返回 PEM 数据中第一个指定类型的 DER 块。
func firstPEMBlock(data []byte, blockType string) []byte {
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return nil
		}
		if block.Type == blockType {
			return block.Bytes
		}
	}
}
