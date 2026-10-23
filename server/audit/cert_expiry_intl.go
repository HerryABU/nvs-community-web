//go:build !gm

package audit

// cert_expiry_intl.go 国际轨构建：标准库 x509 无法解析 SM2 公钥 OID，
// 如实报告「需要国密构建」，避免误导站长。

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"time"

	"nvs-server/config"
)

func inspectSM2Certs(cfg *config.CryptoConfig) (bool, map[string]string) {
	expiry := map[string]string{}
	present := true
	check := func(label, certFile string) {
		if certFile == "" {
			present = false
			expiry[label] = "未配置"
			return
		}
		data, err := os.ReadFile(certFile)
		if err != nil {
			present = false
			expiry[label] = "读取失败: " + err.Error()
			return
		}
		der := firstCertDERStd(data)
		if der == nil {
			present = false
			expiry[label] = "未找到 CERTIFICATE 块"
			return
		}
		c, err := x509.ParseCertificate(der)
		if err != nil {
			// 标准库不支持 SM2 OID（预期）：提示改用国密构建的二进制
			present = false
			expiry[label] = "标准库无法解析（SM2 证书需 -tags gm 构建的 gm-audit）"
			return
		}
		expiry[label] = c.NotAfter.Format(time.RFC3339)
	}
	check("签名证书 Sign", cfg.SMSignCertFile)
	check("加密证书 Enc", cfg.SMEncCertFile)
	return present, expiry
}

func firstCertDERStd(data []byte) []byte {
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return nil
		}
		if block.Type == "CERTIFICATE" {
			return block.Bytes
		}
	}
}
