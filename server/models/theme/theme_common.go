package theme

import "strings"

// 主题自定义 JS 的加载模式（alist「延迟加载」的等价实现）。
//
// 为什么需要：自定义 JS 目前是随主题「立即注入」的，若脚本较重或依赖 DOM，
// 会与首屏渲染抢主线程，甚至在 DOM 就绪前执行报错。
// alist 提供了「延迟加载」选项，这里对齐为三种模式：
//
//	immediate —— 立即注入（默认，保持既有行为）
//	idle      —— 等首屏渲染完成（window load）后，在浏览器空闲时注入
//	delayed   —— 延迟 N 秒后注入（适合埋点、统计、第三方挂件这类"不急"的脚本）
const (
	ThemeJSModeImmediate = "immediate"
	ThemeJSModeIdle      = "idle"
	ThemeJSModeDelayed   = "delayed"
)

// NormalizeThemeJSMode 归一化加载模式；非法值一律回落到 immediate。
func NormalizeThemeJSMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case ThemeJSModeIdle:
		return ThemeJSModeIdle
	case ThemeJSModeDelayed:
		return ThemeJSModeDelayed
	default:
		return ThemeJSModeImmediate
	}
}

// NormalizeThemeJSDelay 归一化延迟秒数：仅 delayed 模式有意义，范围 1~600，默认 3。
func NormalizeThemeJSDelay(delay int) int {
	if delay <= 0 {
		return 3
	}
	if delay > 600 {
		return 600
	}
	return delay
}
