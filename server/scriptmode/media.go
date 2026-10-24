package scriptmode

// ==================== 气泡内图片 ====================
// 语法：整行写 ![说明](图片地址) → 渲染为一个图片气泡（居中卡片）。
// 安全：URL 走白名单（http/https/站内相对路径），属性全部转义，禁止 javascript:/data: 等协议。

import (
	"regexp"
	"strings"
)

// 说明：URL 用 [^\s]+ 贪婪匹配并回退，保证 ![a](http://x/y(1).png) 这类含括号的地址也能正确收尾
var imageRe = regexp.MustCompile(`^!\[([^\]]{0,64})\]\((\S+)\)\s*$`)

// ParseImage 解析整行图片语法
func ParseImage(line string) (alt string, url string, ok bool) {
	m := imageRe.FindStringSubmatch(strings.TrimSpace(line))
	if len(m) != 3 {
		return "", "", false
	}
	return m[1], m[2], true
}

// SafeURL 图片地址白名单校验：
//   - 站内相对路径（/ 开头，或 ./ ../）
//   - http:// https://
//
// 其余（javascript:、data:、vbscript:、file: 等）一律拒绝
func SafeURL(u string) bool {
	u = strings.TrimSpace(u)
	if u == "" {
		return false
	}
	lower := strings.ToLower(u)
	// 显式拒绝危险协议（含大小写与常见伪装：java script: 之类）
	for _, bad := range []string{"javascript:", "data:", "vbscript:", "file:", "about:", "blob:"} {
		if strings.Contains(lower, bad) {
			return false
		}
	}
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return true
	}
	// 站内相对路径
	if strings.HasPrefix(u, "/") || strings.HasPrefix(u, "./") || strings.HasPrefix(u, "../") {
		return !strings.Contains(u, "://")
	}
	// 无协议的静态路径（如 uploads/a.png）
	if !strings.Contains(u, ":") && !strings.Contains(u, "//") {
		return true
	}
	return false
}
