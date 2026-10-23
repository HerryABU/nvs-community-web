package secret

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// FileSource 从文件读取凭据（权限要求 0600，需求文档 §6.1）。
type FileSource struct{}

// Get 读取凭据文件 name；文件权限过于宽松（组/其他人可读写）时拒绝。
func (FileSource) Get(_ context.Context, name string) (string, error) {
	info, err := os.Stat(name)
	if err != nil {
		return "", fmt.Errorf("读取凭据文件失败: %w", err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		return "", fmt.Errorf("凭据文件 %s 权限过于宽松（%04o），要求 0600（chmod 600 %s）", name, perm, name)
	}
	data, err := os.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("读取凭据文件失败: %w", err)
	}
	v := strings.TrimSpace(string(data))
	if v == "" {
		return "", fmt.Errorf("凭据文件 %s 为空", name)
	}
	return v, nil
}
