// Package cert SM2 双证书加载与 OID 识别（需求文档 §5.1）。
//
// TLCP（GB/T 38636-2020）要求签名证书 + 加密证书两套。
// SM2 公钥 OID 为 1.2.156.10197.1.301（见 crypto 包常量）。
package cert

import "crypto/tls"

// DualCertificate SM2 双证书：签名证书 + 加密证书。
type DualCertificate struct {
	// Sign SM2 签名证书（握手签名用）。
	Sign tls.Certificate
	// Enc SM2 加密证书（密钥交换用，ECC 静态双证书模式必需）。
	Enc tls.Certificate
}
