package utils

// TruncateRunes 按字符（rune）数截断字符串，超出部分以省略号结尾。
// 从 handlers/admin 迁移至 utils，避免 admin ↔ emergency 循环依赖。
func TruncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
