package utils

import (
	"strings"
	"testing"
)

// containsAny 断言 sanitized 中不含任一危险串
func containsAny(s string, subs ...string) bool {
	low := strings.ToLower(s)
	for _, sub := range subs {
		if strings.Contains(low, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}

// TestSanitizeCSSAtImport @import 一律移除（外链样式表是 XSS/挂马经典载体）
func TestSanitizeCSSAtImport(t *testing.T) {
	out, removed := SanitizeCSS(`@import url("https://evil.example/x.css");`)
	if containsAny(out, "import", "evil") {
		t.Fatalf("@import 应被移除，实际输出: %q", out)
	}
	if len(removed) == 0 {
		t.Fatal("应记录被移除的 @import 规则")
	}
}

// TestSanitizeCSSURL 所有 url(...) 一律移除（background/src/list-style-image 等）
func TestSanitizeCSSURL(t *testing.T) {
	cases := []string{
		`.a { background: url(https://evil.example/x.png); }`,
		`.a { background-image: url("https://evil.example/x.png"); }`,
		`.a { list-style-image: url(//evil.example/x.svg); }`,
		`.a { cursor: url(https://evil.example/cur.cur), pointer; }`,
		`.a { mask-image: url(https://evil.example/m.svg); }`,
		`.a { background: url(javascript:alert(1)); }`,
	}
	for _, css := range cases {
		out, removed := SanitizeCSS(css)
		if containsAny(out, "url(", "javascript:", "evil") {
			t.Fatalf("url() 应被移除，实际输出: %q", out)
		}
		if len(removed) == 0 {
			t.Fatalf("应记录被移除规则: %q", css)
		}
	}
}

// TestSanitizeCSSExpression expression() 移除（含注释混淆变形）
func TestSanitizeCSSExpression(t *testing.T) {
	cases := []string{
		`.a { width: expression(alert(1)); }`,
		`.a { width: ex/**/pression(alert(1)); }`,
		`.a { width: expr/*x*/ession(alert(1)); }`,
	}
	for _, css := range cases {
		out, removed := SanitizeCSS(css)
		if containsAny(out, "expression(", "alert") {
			t.Fatalf("expression() 应被移除，实际输出: %q", out)
		}
		if len(removed) == 0 {
			t.Fatalf("应记录被移除规则: %q", css)
		}
	}
}

// TestSanitizeCSSBehaviorBinding behavior:/-moz-binding 移除（IE/Firefox 脚本绑定）
func TestSanitizeCSSBehaviorBinding(t *testing.T) {
	out, removed := SanitizeCSS(`.a { behavior: url(x.htc); -moz-binding: url(x.xml); color: red; }`)
	if containsAny(out, "behavior", "binding", "url(") {
		t.Fatalf("behavior/-moz-binding 应被移除，实际输出: %q", out)
	}
	if !strings.Contains(out, "color: red") {
		t.Fatalf("合法声明应保留，实际输出: %q", out)
	}
	if len(removed) < 2 {
		t.Fatalf("应记录两条被移除规则，实际 %d 条: %v", len(removed), removed)
	}
}

// TestSanitizeCSSJavaScript javascript:/vbscript: 协议值移除
func TestSanitizeCSSJavaScript(t *testing.T) {
	cases := []string{
		`.a { background: javascript:alert(1); }`,
		`.a { behavior: javascript:alert(1); }`,
		`.a { background: vbscript:msgbox(1); }`,
	}
	for _, css := range cases {
		out, removed := SanitizeCSS(css)
		if containsAny(out, "javascript:", "vbscript:", "alert", "msgbox") {
			t.Fatalf("脚本协议应被移除，实际输出: %q", out)
		}
		if len(removed) == 0 {
			t.Fatalf("应记录被移除规则: %q", css)
		}
	}
}

// TestSanitizeCSSCommentObfuscation 注释混淆：@imp/**/ort 等拼接还原后必须被拒
func TestSanitizeCSSCommentObfuscation(t *testing.T) {
	cases := []string{
		`@imp/**/ort url("https://evil.example/x.css");`,
		`@media screen { .a { col/**/or: expression(alert(1)); } }`,
		`.a { background: ur/**/l(https://evil.example/x.png); }`,
	}
	for _, css := range cases {
		out, removed := SanitizeCSS(css)
		if containsAny(out, "import", "expression(", "url(", "evil") {
			t.Fatalf("注释混淆攻击应被移除，实际输出: %q", out)
		}
		if len(removed) == 0 {
			t.Fatalf("应记录被移除规则: %q", css)
		}
	}
}

// TestSanitizeCSSAtRuleWhitelist @font-face/@supports/@charset/@namespace 移除，
// @media/@keyframes 保留且内部同样净化
func TestSanitizeCSSAtRuleWhitelist(t *testing.T) {
	css := `
@charset "utf-8";
@import url("https://evil.example/x.css");
@font-face { font-family: evil; src: url(https://evil.example/f.woff); }
@supports (display: grid) { .a { color: blue; } }
@namespace svg url(http://www.w3.org/2000/svg);
@media (max-width: 768px) {
  .a { color: red; background: url(https://evil.example/x.png); }
}
@keyframes pulse {
  from { opacity: 0; }
  50% { opacity: .5; }
  to { opacity: 1; background: url(https://evil.example/x.png); }
}
.a { color: green; }
`
	out, removed := SanitizeCSS(css)
	// 白名单外 @ 规则必须全部移除
	for _, banned := range []string{"@charset", "@import", "@font-face", "@supports", "@namespace", "evil", "src:"} {
		if containsAny(out, banned) {
			t.Fatalf("%s 应被移除，实际输出: %q", banned, out)
		}
	}
	// @media/@keyframes 保留，内部危险声明被剥掉
	if !strings.Contains(out, "@media (max-width: 768px)") {
		t.Fatalf("@media 应保留，实际输出: %q", out)
	}
	if !strings.Contains(out, "@keyframes pulse") {
		t.Fatalf("@keyframes 应保留，实际输出: %q", out)
	}
	if !strings.Contains(out, "from") || !strings.Contains(out, "50%") {
		t.Fatalf("keyframes 百分比选择器应保留，实际输出: %q", out)
	}
	if containsAny(out, "url(") {
		t.Fatalf("@media/@keyframes 内部的 url() 应被剥掉，实际输出: %q", out)
	}
	if !strings.Contains(out, "color: red") || !strings.Contains(out, "color: green") {
		t.Fatalf("合法声明应保留，实际输出: %q", out)
	}
	if len(removed) < 5 {
		t.Fatalf("应记录至少 5 条移除记录，实际 %d 条: %v", len(removed), removed)
	}
}

// TestSanitizeCSSKeyframeSelectorRestriction keyframes 内部仅允许 from/to/百分比
func TestSanitizeCSSKeyframeSelectorRestriction(t *testing.T) {
	out, removed := SanitizeCSS(`@keyframes x { .evil { color: red; } from { opacity: 0; } }`)
	if containsAny(out, ".evil") {
		t.Fatalf("keyframes 内的类选择器应被移除，实际输出: %q", out)
	}
	if !strings.Contains(out, "from") {
		t.Fatalf("from 选择器应保留，实际输出: %q", out)
	}
	if len(removed) == 0 {
		t.Fatal("应记录被移除的非法 keyframe 选择器")
	}
}

// TestSanitizeCSSNormalDeclarations 正常声明全部保留（含 var(--x) 引用）
func TestSanitizeCSSNormalDeclarations(t *testing.T) {
	css := `
:root { --primary-color: #ff0000; --my-card-radius: 12px; }
body { color: var(--text-color); background-color: var(--bg-color); margin: 0 auto; }
.card { padding: 16px 20px; border-radius: var(--radius-md); box-shadow: var(--shadow-md);
  font-family: "Inter", "PingFang SC", sans-serif; transition: all var(--transition-fast); }
.gradient { background: linear-gradient(135deg, #6366f1 0%, #8b5cf6 100%); }
.el-button--primary { background: var(--gradient-primary) !important; }
`
	out, removed := SanitizeCSS(css)
	if len(removed) != 0 {
		t.Fatalf("正常 CSS 不应有任何移除记录，实际: %v", removed)
	}
	for _, keep := range []string{
		"--primary-color: #ff0000", "--my-card-radius: 12px",
		"color: var(--text-color)", "background-color: var(--bg-color)", "margin: 0 auto",
		"padding: 16px 20px", "border-radius: var(--radius-md)", "box-shadow: var(--shadow-md)",
		"linear-gradient(135deg, #6366f1 0%, #8b5cf6 100%)", "!important",
	} {
		if !strings.Contains(out, keep) {
			t.Fatalf("正常声明 %q 应保留，实际输出: %q", keep, out)
		}
	}
}

// TestSanitizeCSSEmpty 空 CSS（含纯空白）原样返回空
func TestSanitizeCSSEmpty(t *testing.T) {
	for _, css := range []string{"", "   \n\t  "} {
		out, removed := SanitizeCSS(css)
		if out != "" {
			t.Fatalf("空 CSS 应返回空串，实际: %q", out)
		}
		if len(removed) != 0 {
			t.Fatalf("空 CSS 不应有移除记录，实际: %v", removed)
		}
	}
}

// TestSanitizeCSSOversize 超长 CSS 整体拒绝（防拖垮解析与入库体积失控）
func TestSanitizeCSSOversize(t *testing.T) {
	css := strings.Repeat(".a { color: red; }\n", 20000) // 约 340KB > 128KB 上限
	out, removed := SanitizeCSS(css)
	if out != "" {
		t.Fatalf("超长 CSS 应整体拒绝，实际输出 %d 字节", len(out))
	}
	if len(removed) == 0 || !strings.Contains(removed[0], "128KB") {
		t.Fatalf("超长应记录原因，实际: %v", removed)
	}
}

// TestSanitizeCSSOversizeWithinLimit 上限之内的超长合法 CSS 正常保留
func TestSanitizeCSSOversizeWithinLimit(t *testing.T) {
	css := strings.Repeat(".a { color: red; }\n", 5000) // 约 85KB < 128KB 上限
	out, removed := SanitizeCSS(css)
	if len(removed) != 0 {
		t.Fatalf("合法长 CSS 不应有移除记录，实际: %v", removed)
	}
	if strings.Count(out, "color: red") != 5000 {
		t.Fatalf("5000 条声明应全部保留，实际 %d 条", strings.Count(out, "color: red"))
	}
}

// TestSanitizeCSSEscapeAndMalformed 反斜杠转义/未闭合块/孤立分号等畸形输入
func TestSanitizeCSSEscapeAndMalformed(t *testing.T) {
	cases := []struct {
		name string
		css  string
	}{
		{"反斜杠转义属性名", `.a { col\6f r: red; }`},
		{"反斜杠转义值", `.a { background: \75 rl(https://evil.example/x.png); }`},
		{"未闭合规则块", `.a { color: red;`},
		{"孤立分号文本", `.a color: red;`},
		{"选择器注入尖括号", `a < img { color: red; }`},
		{"IE progid 滤镜", `.a { filter: progid:DXImageTransform.Microsoft.gradient(startColorstr='#ff0000'); }`},
	}
	for _, c := range cases {
		out, removed := SanitizeCSS(c.css)
		if containsAny(out, "url(", "\\6f", "progid", "<") {
			t.Fatalf("%s：危险内容应被移除，实际输出: %q", c.name, out)
		}
		if len(removed) == 0 {
			t.Fatalf("%s：应记录被移除规则", c.name)
		}
	}
}

// TestSanitizeCSSKitchenSink 混合攻击向量大锅饭：输出中不得出现任何危险串
func TestSanitizeCSSKitchenSink(t *testing.T) {
	css := `
@import url(https://evil.example/a.css);
@font-face { src: url(https://evil.example/f.woff); }
@supports (x: y) { .a { behavior: url(x.htc); } }
.a {
  behavior: url(x.htc);
  -moz-binding: url(https://evil.example/b.xml);
  width: ex/**/pression(alert(1));
  background: url(https://evil.example/x.png);
  color: expression(alert(2));
  filter: progid:DXImageTransform.Microsoft.gradient();
  content: "javascript:alert(3)";
  margin: 0 auto;
  col\6f r: red;
}
@media screen and (max-width: 768px) { .b { background: url(x); color: blue; } }
@keyframes spin { from { transform: rotate(0); } to { transform: rotate(360deg); } }
`
	out, removed := SanitizeCSS(css)
	for _, banned := range []string{
		"url(", "expression(", "javascript:", "behavior", "-moz-binding", "progid",
		"@import", "@font-face", "@supports", "\\6f", "evil", "alert",
	} {
		if containsAny(out, banned) {
			t.Fatalf("输出不得包含危险串 %q，实际输出: %q", banned, out)
		}
	}
	// 合法内容必须存活
	for _, keep := range []string{"margin: 0 auto", "@media screen and (max-width: 768px)", "@keyframes spin", "color: blue", "rotate(0)"} {
		if !strings.Contains(out, keep) {
			t.Fatalf("合法内容 %q 应保留，实际输出: %q", keep, out)
		}
	}
	if len(removed) < 8 {
		t.Fatalf("应记录至少 8 条移除记录，实际 %d 条: %v", len(removed), removed)
	}
}

// TestSanitizeCSSNestedMedia 嵌套 @media 保留且逐层净化
func TestSanitizeCSSNestedMedia(t *testing.T) {
	out, removed := SanitizeCSS(`@media screen { @media (max-width: 640px) { .a { color: red; background: url(x); } } }`)
	if !strings.Contains(out, "@media screen") || !strings.Contains(out, "max-width: 640px") {
		t.Fatalf("嵌套 @media 应保留，实际输出: %q", out)
	}
	if containsAny(out, "url(") {
		t.Fatalf("嵌套 @media 内部的 url() 应被剥掉，实际输出: %q", out)
	}
	if len(removed) == 0 {
		t.Fatal("应记录被移除的 url() 声明")
	}
}
