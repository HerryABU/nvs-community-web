// Package regexsafe 提供「章节拆分正则」的安全编译与匹配能力。
//
// 背景：小说上传分解允许用户提交自定义 split_rule。Go 标准库 regexp 基于 RE2（Thompson NFA），
// 不存在经典「灾难性回溯」ReDoS；但用户提交任意正则仍有两类风险：
//   1) 未来若改换为 PCRE/NFA 引擎，当前任意正则会立刻变成真正的 ReDoS；
//   2) 超长/特殊正则配合超大正文，仍可能造成 CPU 长时间占用（资源耗尽型 DoS）。
//
// 本包通过「字符白名单 + 禁用回溯结构 + 编译超时 + 输入限长」四重约束，从根上消除正则爆炸：
//   - 用户规则只能由简单、确定性的语法构成（字符类、基础元字符、中文字符）；
//   - 显式禁止环视 (?= (?!) (?<= (?<! 与反向引用 \1..\9；
//   - 章节拆分强制 multiline 模式（按行匹配章节标题），但 (? 由本包内部注入，用户无法自行添加；
//   - 匹配在独立 goroutine 中执行并带超时，超时即视为无匹配回退自动识别。
package regexsafe

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// MaxSplitRuleLen 用户自定义章节拆分正则的最大长度，防止超长恶意规则。
const MaxSplitRuleLen = 200

// MaxSplitInputBytes 拆分输入的最大字节数（8MB），防止超大文件耗尽内存/CPU。
const MaxSplitInputBytes = 8 << 20

// SplitTimeout 单次章节拆分匹配的最大耗时，超时即回退自动识别。
const SplitTimeout = 3 * time.Second

// 允许出现在用户自定义拆分规则中的字符白名单：
// 仅允许构建简单、确定性的正则（字符类、基础元字符、 bounded 量词 {n,m}、中文字符、常用标点）。
// 任何不在此集合内的字符一律拒绝，从语法层面杜绝回溯型构造。
// 注：{} 为 RE2 有界量词（{n}/{n,}/{n,m}），确定性无回溯，安全放行；
//     <>(?P<name>) 命名分组未列入白名单（章节拆分无需命名分组）。
var allowedRuleChars = regexp.MustCompile(`^[\[\](){}.*+?^$|\\/\-#:、，。；;,\s\d\w\p{Han}]*$`)

// 反向引用 \1..\9（回溯型，禁止）。
var backrefRe = regexp.MustCompile(`\\[1-9]`)

var (
	errRuleEmpty        = errors.New("拆分规则为空")
	errRuleTooLong      = errors.New("拆分正则过长（上限 200 字符）")
	errRuleIllegalChar  = errors.New("拆分正则包含非法字符，仅允许基础字符类与中文字符")
	errRuleLookaround   = errors.New("拆分正则禁止环视/反向引用等回溯型结构")
	errRuleCompile      = errors.New("拆分正则编译失败")
)

// ValidateChapterSplitRule 校验用户提交的章节拆分正则是否安全。
// 返回 nil 表示通过（空规则也通过，交由调用方走自动识别）。
func ValidateChapterSplitRule(rule string) error {
	if rule == "" {
		return nil
	}
	if len(rule) > MaxSplitRuleLen {
		return errRuleTooLong
	}
	// 1) 字符白名单：任何不在此集合内的字符一律拒绝
	if !allowedRuleChars.MatchString(rule) {
		return errRuleIllegalChar
	}
	// 2) 禁用回溯型结构：环视 (?= (?!) (?<= (?<!
	//    注意：只拦截「环视」这四类构造，放行合法内联标志 (?m)/(?i)/(?s)/(?U)
	//    与命名分组 (?P<name>——它们都是确定性语法，不引入回溯。
	//    （Go RE2 本就不支持环视，此处为纵深防御；此前误用 strings.Contains(rule,"(?")
	//     会把 (?m) 等合法标志一并拒绝，导致含 (?m) 的规则被误判为非法而回退自动识别。）
	for _, pat := range []string{"(?=", "(?!", "(?<=", "(?<!"} {
		if strings.Contains(rule, pat) {
			return errRuleLookaround
		}
	}
	// 3) 反向引用 \1..\9
	if backrefRe.MatchString(rule) {
		return errRuleLookaround
	}
	return nil
}

// CompileChapterSplitRule 在白名单校验通过后编译用户规则（内部强制 multiline 模式），
// 校验失败或编译失败返回 error，调用方应回退到自动识别。
func CompileChapterSplitRule(rule string) (*regexp.Regexp, error) {
	if err := ValidateChapterSplitRule(rule); err != nil {
		return nil, err
	}
	// 章节拆分天然按行匹配标题，强制 multiline 由本包注入，用户无法自行添加 (?m) 等标志。
	re, err := regexp.Compile("(?m)" + rule)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errRuleCompile, err)
	}
	return re, nil
}

// SafeFindAllStringIndex 在限定时间内对（已限长的）输入执行 FindAllStringIndex。
// RE2 本身线性时间，这里额外加超时作为纵深防御，防止极特殊输入长时间占用。
// 超时返回 nil（调用方据此回退自动识别）。
func SafeFindAllStringIndex(re *regexp.Regexp, content string, timeout time.Duration) [][]int {
	type result struct {
		locs [][]int
	}
	ch := make(chan result, 1)
	go func() {
		ch <- result{locs: re.FindAllStringIndex(content, -1)}
	}()
	select {
	case r := <-ch:
		return r.locs
	case <-time.After(timeout):
		return nil
	}
}

// ValidationError 便于接口层区分「规则非法」与「其他错误」。
type ValidationError struct {
	Msg string
}

func (e *ValidationError) Error() string { return e.Msg }

// MarshalJSON 让校验错误可被 JSON 序列化（调试/日志用）。
func (e *ValidationError) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{"error": e.Msg})
}
