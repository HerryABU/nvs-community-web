package utils

import (
	"path/filepath"
	"strings"
)

// ResolveContentPath 规范化内容文件路径，兼容历史脏数据（跨平台迁移场景）。
// 旧数据在 Windows 上运行时，content_path 落库的是反斜杠分隔符
// （如 data\novels\authors\2\9\1.html），部署到 Linux 后 os.ReadFile 直接按
// 反斜杠查找必然失败（文件实际为 data/novels/authors/2/9/1.html）。
// 这里把反斜杠统一替换为正斜杠，再按当前平台分隔符解析：
//   - Linux:  data\novels\... → data/novels/...
//   - Windows: data\novels\... → data\novels\...（filepath.FromSlash 转回反斜杠，行为不变）
func ResolveContentPath(p string) string {
	if p == "" {
		return p
	}
	return filepath.FromSlash(strings.ReplaceAll(p, "\\", "/"))
}
