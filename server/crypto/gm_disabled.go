//go:build !gm

package crypto

// gm_disabled.go 默认（国际轨）构建：国密轨未编译进本二进制（N-01 零国密依赖）。
// 启用国密请以 make build-gm（纯 Go）或 make build-gm-cgo（Tongsuo CGO）重新构建。

import (
	"fmt"

	"nvs-server/config"
)

func newGMProvider(cfg *config.CryptoConfig) (Provider, error) {
	return nil, fmt.Errorf("国密轨（track=%s）未编译进本二进制：请以 make build-gm（纯 Go）或 make build-gm-cgo（Tongsuo CGO）重新构建", cfg.Track)
}

func compiledGMProviders() []string { return nil }
