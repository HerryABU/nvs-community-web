//go:build gm

package crypto

// gm.go 国密轨构建（-tags gm）：Provider 工厂注册与分发。
// 具体实现由构建子标签选择：
//   - gm && !tongsuo → gm_purego.go（纯 Go，无 CGO，可退路）
//   - gm && tongsuo  → gm_tongsuo.go（Tongsuo CGO，官方绑定，推荐）

import (
	"fmt"

	"nvs-server/config"
)

// gmProviderFactory / gmProviderName 由具体实现文件在 init 中注册。
var (
	gmProviderFactory func(*config.CryptoConfig) (Provider, error)
	gmProviderName    string
)

// newGMProvider 构建国密轨 Provider（供 provider.go 的 New 调用）。
func newGMProvider(cfg *config.CryptoConfig) (Provider, error) {
	if gmProviderFactory == nil {
		return nil, fmt.Errorf("国密轨已启用（track=%s），但本二进制未编译任何国密实现：purego 用 -tags gm，tongsuo 用 -tags gm,tongsuo", cfg.Track)
	}
	return gmProviderFactory(cfg)
}

// compiledGMProviders 本二进制编译进的国密实现名。
func compiledGMProviders() []string {
	if gmProviderName == "" {
		return nil
	}
	return []string{gmProviderName}
}
