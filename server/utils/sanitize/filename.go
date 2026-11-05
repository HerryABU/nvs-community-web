package sanitize

import "strings"

// SanitizeFilename 将文件名中的非法/危险字符替换为下划线，并做长度与首尾裁剪。
// 从 handlers/web 迁移至 utils，避免 admin ↔ web 循环依赖。
func SanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_",
		"?", "_", "\"", "_", "<", "_", ">", "_",
		"|", "_", " ", "_",
	)
	safe := replacer.Replace(name)
	safe = strings.Trim(safe, "._")
	if safe == "" {
		safe = "untitled"
	}
	if len(safe) > 64 {
		safe = safe[:64]
	}
	return safe
}
