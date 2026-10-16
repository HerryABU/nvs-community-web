// jssanitize.go — 用户自定义主题 JS 的注入安全处理（F12 alist 式扩展）。
//
// 设计取舍（务必先读，再改）：
//   - 与 CSS 不同，JS 无法做「白名单式消毒」还能保留可用性：任何有意义的 JS
//     都必须能调用 DOM / fetch / 等 API。因此我们不试图理解或裁剪用户的 JS 逻辑，
//     只做一件事——**防止它从注入载体里逃逸出来**。
//   - 注入方式：前端以 <script> 元素的 textContent 写入（而非 innerHTML），
//     脚本内容作为纯文本插入后由浏览器执行。这种注入的唯一逃逸通道是字面量
//     `</script>`，它会提前闭合我们的 <script> 标签。所以消毒器只需剥离这个序列。
//   - 信任模型：站点 JS 仅管理员可写（handlers 已 ensureAdmin 门禁），个人 JS
//     仅对本人登录态生效（自 XSS，无横向危害）。JS 本身的能力由「谁能写」来约束，
//     而非由消毒器约束。这等同于 alist 的自定义 JS 模型。
//   - 长度上限：64KB，防超长输入拖慢解析/入库。
package utils

import (
	"strings"
)

// MaxJSLength 主题自定义 JS 长度上限（64KB）。
const MaxJSLength = 64 * 1024

// SanitizeJS 对自定义 JS 做注入安全处理，返回（处理后的 JS, 被移除项原因列表）。
// removed 恒非 nil，便于 JSON 序列化为 []。
//
// 处理规则（只防逃逸，不限制逻辑）：
//  1. 长度超过 MaxJSLength 直接整体拒绝；
//  2. 移除所有 </script（不区分大小写、允许后续空白/斜杠）以防止闭合注入载体；
//  3. 移除裸 <script 开标签（防止嵌套脚本节点，干净起见）；
//  4. 移除 HTML 注释定界符 <!-- / -->（HTML5 中 <script> 为原始文本，本非必须，
//     但 alist 主题常用注释占位，移除可避免与外部解析器意外交互）。
func SanitizeJS(js string) (string, []string) {
	removed := []string{}
	if len(js) > MaxJSLength {
		return "", append(removed, "JS 长度超过 64KB 上限 已整体移除（请拆分或精简）")
	}
	if js == "" {
		return "", removed
	}

	low := strings.ToLower(js)
	var (
		hadClose bool
		hadOpen  bool
		hadCmt   bool
	)

	// 1) 剥离 </script（含 </SCRIPT、</script >、</script/> 等变体）
	for {
		i := strings.Index(low, "</script")
		if i < 0 {
			break
		}
		hadClose = true
		// 跳到下一个 '>' 之后，删掉这一整段 </script...>
		end := strings.IndexByte(low[i:], '>')
		if end < 0 {
			js = js[:i]
			low = low[:i]
			break
		}
		js = js[:i] + js[i+end+1:]
		low = low[:i] + low[i+end+1:]
	}

	// 2) 剥离裸 <script 开标签（防止嵌套节点）
	for {
		i := strings.Index(low, "<script")
		if i < 0 {
			break
		}
		hadOpen = true
		end := strings.IndexByte(low[i:], '>')
		if end < 0 {
			js = js[:i]
			low = low[:i]
			break
		}
		js = js[:i] + js[i+end+1:]
		low = low[:i] + low[i+end+1:]
	}

	// 3) 剥离 HTML 注释定界符
	for {
		i := strings.Index(low, "<!--")
		if i < 0 {
			break
		}
		hadCmt = true
		js = js[:i] + js[i+4:]
		low = low[:i] + low[i+4:]
	}
	for {
		i := strings.Index(low, "-->")
		if i < 0 {
			break
		}
		hadCmt = true
		js = js[:i] + js[i+3:]
		low = low[:i] + low[i+3:]
	}

	if hadClose {
		removed = append(removed, "</script> 已被剥离（防止提前闭合注入载体）")
	}
	if hadOpen {
		removed = append(removed, "<script> 开标签已被剥离（禁止嵌套脚本节点）")
	}
	if hadCmt {
		removed = append(removed, "HTML 注释定界符 <!-- --> 已被剥离")
	}

	return js, removed
}
