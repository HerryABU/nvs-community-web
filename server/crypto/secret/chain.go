package secret

import (
	"context"
	"fmt"
)

// Source SecretSource 别名（便于链式组合）。
type Source interface {
	Get(ctx context.Context, name string) (string, error)
}

// ChainSource 按顺序尝试多个来源，首个成功即返回。
type ChainSource struct {
	Sources []Source
}

// Get 依序尝试各来源。
func (c ChainSource) Get(ctx context.Context, name string) (string, error) {
	for _, s := range c.Sources {
		if s == nil {
			continue
		}
		if v, err := s.Get(ctx, name); err == nil {
			return v, nil
		}
	}
	return "", fmt.Errorf("所有凭据来源均无法提供 %q", name)
}

// EnvThenFile 先环境变量、后凭据文件（文件名 = name 视为路径）的常用组合。
func EnvThenFile() ChainSource {
	return ChainSource{Sources: []Source{EnvSource{}, FileSource{}}}
}
