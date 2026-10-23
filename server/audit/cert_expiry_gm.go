//go:build gm

package audit

// cert_expiry_gm.go 国密构建：经 gmsm/smx509 解析 SM2 证书有效期。

import (
	"encoding/pem"
	"os"

	"github.com/emmansun/gmsm/smx509"

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
		der := firstCertDER(data)
		if der == nil {
			present = false
			expiry[label] = "未找到 CERTIFICATE 块"
			return
		}
		c, err := smx509.ParseCertificate(der)
		if err != nil {
			present = false
			expiry[label] = "解析失败: " + err.Error()
			return
		}
		expiry[label] = c.NotAfter.Format("2006-01-02T15:04:05Z07:00")
	}
	check("签名证书 Sign", cfg.SMSignCertFile)
	check("加密证书 Enc", cfg.SMEncCertFile)
	return present, expiry
}

func firstCertDER(data []byte) []byte {
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
