// textlimit.go — 定长字段安全工具（SQLite→MySQL 迁移强类型兼容）。
// MySQL 严格模式下 VARCHAR(N) 按「字符数」校验（多字节中文按 1 字符计），超长直接报错；
// SQLite 不校验长度，放任不管会把问题留到迁移时才爆。所有用户输入的定长字段
// 必须在写入前按「字符数」截断，并在前端给出 maxlength 提示。
package utils

import "unicode/utf8"

// RuneLen 返回字符串的字符数（rune 数，非字节数）。
func RuneLen(s string) int { return utf8.RuneCountInString(s) }

// ClampString 按字符数截断到 maxRunes（maxRunes<=0 表示不限制）。
// 截断在 rune 边界进行，不会产生非法 UTF-8 半个汉字。
func ClampString(s string, maxRunes int) string {
	if maxRunes <= 0 || utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	// 定位第 maxRunes 个字符的字节偏移
	count := 0
	for i := range s {
		if count == maxRunes {
			return s[:i]
		}
		count++
	}
	return s
}

// ClampStrings 批量截断（键为字段名，值为 maxRunes）。
func ClampStrings(m map[string]string, limits map[string]int) {
	for k, max := range limits {
		if v, ok := m[k]; ok {
			m[k] = ClampString(v, max)
		}
	}
}

// ExceedsLimit 字符串是否超过字符数限制（用于校验提示）。
func ExceedsLimit(s string, maxRunes int) bool {
	return maxRunes > 0 && utf8.RuneCountInString(s) > maxRunes
}

// 常见定长字段的字符上限（与 models gorm:size 对应，前后端共用同一组常量语义）。
// 前端 maxlength 与后端 ClampString 必须使用同一组数值。
const (
	MaxUsername   = 64  // users.username
	MaxEmail      = 128 // users.email
	MaxNickname   = 64  // users.nickname
	MaxSignature  = 256 // users.signature
	MaxTitle      = 256 // novel/blog/thread/chapter title
	MaxForumName  = 128 // forums.name
	MaxReason     = 256 // 举报/禁言/封禁原因
	MaxTagName    = 32  // blog_tags.name
	MaxTopicName  = 48  // blog_topics.name
	MaxNoteBody   = 2000 // reading_notes.content
	MaxAnnTitle   = 255 // announcements.title
	MaxSoftReason = 512 // softdel_*.reason
)
