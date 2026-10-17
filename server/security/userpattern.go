package security

import (
	"errors"
	"regexp"
	"strings"
)

// ═══ 用户自定义正则统一防护（违禁词 / XSS 规则后台共用）═══
//
// 背景：管理员可在后台编写正则型规则（badword regex 模式、xss_rule）。
// Go 标准库 regexp 基于 RE2（Thompson NFA），线性时间、无灾难性回溯，
// 经典 ReDoS 在 Go 侧不存在；但任意用户正则仍有资源耗尽面：
//   1) 超长/病态 pattern 的编译成本与后续每次匹配的固定开销；
//   2) 线性扫描 × 规则数 × 超大正文 的累计 CPU（如 5MB 章节 × 50 条规则）。
// 本文件提供统一防线：长度上限 + 编译校验 + 嵌套量词误用提示 + 扫描输入截断。
// （章节拆分用户正则另有更严格的 regexsafe 包白名单约束，不适用本文件。）

// MaxUserPatternLen 用户自定义正则的字节上限。
// 256 字节足以表达任何合理的内容审核规则，同时约束编译成本。
const MaxUserPatternLen = 256

// MaxScanBytes 单次规则扫描的正文截断上限（1MB）。
// 超出部分不参与匹配（保守方向：宁可漏检不可耗尽 CPU），
// 调用方可通过返回的 truncated 标记决定是否提示。
const MaxScanBytes = 1 << 20

// 嵌套量词启发式：形如 (…+)+ / (…*)* / (…{n,}){m,} 的写法。
// RE2 线性不会爆炸，但这类结构几乎总是误用（本意通常是单个量词），
// 保存时给管理员明确提示，避免「以为在匹配 X 实际语义完全不同」。
var nestedQuantifierRe = regexp.MustCompile(`([([][^)\]]*[+*][^)\]]*[)])[+*]|([([][^)\]]*\{[0-9]+(,[0-9]*)?\}[^)\]]*[)])[+*]`)

// ValidateUserPattern 校验用户提交的自定义正则：长度上限 + RE2 可编译。
// 通过则返回 nil；拒绝时返回中文错误信息（直接呈现给管理员）。
func ValidateUserPattern(pattern string) error {
	if pattern == "" {
		return errors.New("正则表达式不能为空")
	}
	if len(pattern) > MaxUserPatternLen {
		return errors.New("正则长度超过 256 字节上限，请拆分多条规则")
	}
	if _, err := regexp.Compile(pattern); err != nil {
		return errors.New("正则表达式无效: " + err.Error())
	}
	return nil
}

// UserPatternHint 返回正则的误用提示（可保存，仅提醒）；无问题返回空串。
func UserPatternHint(pattern string) string {
	if pattern == "" || len(pattern) > MaxUserPatternLen {
		return ""
	}
	if _, err := regexp.Compile(pattern); err != nil {
		return ""
	}
	if nestedQuantifierRe.MatchString(pattern) {
		return "检测到嵌套量词（如 (a+)+），RE2 引擎不会指数爆炸，但该写法通常是误用，请确认语义"
	}
	return ""
}

// TruncateForScan 将待扫描正文截断到 MaxScanBytes。
// 返回截断后的内容与是否发生了截断。
func TruncateForScan(content string) (string, bool) {
	if len(content) <= MaxScanBytes {
		return content, false
	}
	return content[:MaxScanBytes], true
}

// ScanNote 截断提示语（供 handler 附加到保存/审核结果中）。
func ScanNote() string {
	return "正文超过 1MB，仅扫描前 1MB"
}

// countQuantifierGroups 仅供测试使用：统计 pattern 中疑似嵌套量词出现次数。
func countQuantifierGroups(pattern string) int {
	return len(nestedQuantifierRe.FindAllString(pattern, -1))
}

// hasLookaround 提示环视结构：RE2 不支持环视（编译期直接报错），
// 这里仅用于测试断言「含 (?= 的 pattern 必然编译失败」这一契约。
func hasLookaround(pattern string) bool {
	return strings.Contains(pattern, "(?=") || strings.Contains(pattern, "(?!") ||
		strings.Contains(pattern, "(?<=") || strings.Contains(pattern, "(?<!")
}
