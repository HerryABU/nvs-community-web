//go:build !gm

package cert

// dual_intl.go 国际轨构建下不编译国密证书解析（gmsm 未引入）。
// 启用国密轨请以 make build-gm（纯 Go）或 make build-gm-cgo（Tongsuo CGO）构建。

import "fmt"

// LoadDualCertificate 在国际轨构建下不可用。
func LoadDualCertificate(signCert, signKey, encCert, encKey string) (*DualCertificate, error) {
	return nil, fmt.Errorf("SM2 双证书加载需要国密构建（-tags gm）：请以 make build-gm 或 make build-gm-cgo 重新构建")
}
