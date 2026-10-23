// Package secret 凭据来源实现（需求文档 §6.1）。
// AI API Key 与证书私钥不得硬编码，经 SecretSource 按名取凭据。
package secret

import (
	"context"
	"fmt"
	"os"
)

// EnvSource 从环境变量读取凭据。
type EnvSource struct{}

// Get 读取环境变量 name；未设置或为空返回错误。
func (EnvSource) Get(_ context.Context, name string) (string, error) {
	v := os.Getenv(name)
	if v == "" {
		return "", fmt.Errorf("环境变量 %s 未设置或为空", name)
	}
	return v, nil
}
