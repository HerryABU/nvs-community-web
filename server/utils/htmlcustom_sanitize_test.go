package utils

import (
	"strings"
	"testing"
)

func TestSanitizeCustomHTML_BasicStructure(t *testing.T) {
	in := `<div class="wrap"><h1>标题</h1><p>正文 <b>加粗</b> 与 <i>斜体</i></p></div>`
	out, removed := SanitizeCustomHTML(in)
	if strings.Contains(out, "<script") {
		t.Fatalf("不应残留 script: %s", out)
	}
	for _, want := range []string{"<div", "<h1>", "<b>", "<i>"} {
		if !strings.Contains(out, want) {
			t.Errorf("应保留 %s，实际输出: %s (removed=%v)", want, out, removed)
		}
	}
}

func TestSanitizeCustomHTML_StripsScripts(t *testing.T) {
	in := `<div>hi<script>alert(1)</script></div><img src=x onerror=alert(2)>`
	out, _ := SanitizeCustomHTML(in)
	if strings.Contains(out, "alert") {
		t.Fatalf("脚本内容应被整体移除: %s", out)
	}
	if strings.Contains(out, "<script") {
		t.Fatalf("不应残留 <script: %s", out)
	}
	if strings.Contains(out, "onerror") {
		t.Fatalf("事件属性应被移除: %s", out)
	}
}

func TestSanitizeCustomHTML_StyleBlockSanitized(t *testing.T) {
	in := `<style>
.b{color:red;background:url(http://evil/x.png);position:fixed}
.c{font-size:14px}
</style><div class="b">x</div>`
	out, removed := SanitizeCustomHTML(in)
	// 安全 CSS 保留（SanitizeCSS 在冒号后加空格）
	if !strings.Contains(out, "14px") {
		t.Errorf("安全 CSS 应保留: %s (removed=%v)", out, removed)
	}
	// 危险 CSS（url 外链）移除
	if strings.Contains(out, "url(") {
		t.Errorf("url() 外链应被移除: %s", out)
	}
	// style 块必须存在（非彻底删除）
	if !strings.Contains(out, "<style>") {
		t.Errorf("style 块应保留（消毒后）: %s", out)
	}
}

func TestSanitizeCustomHTML_InlineStyleSanitized(t *testing.T) {
	in := `<div style="color:red;position:absolute;expression(alert(1))">x</div>`
	out, _ := SanitizeCustomHTML(in)
	if strings.Contains(out, "expression") {
		t.Errorf("expression 应被移除: %s", out)
	}
	// 安全 inline style 保留（冒号后有空格）
	if !strings.Contains(out, "color: red") {
		t.Errorf("安全 inline style 应保留: %s", out)
	}
}

func TestSanitizeCustomHTML_DangerousURLsRemoved(t *testing.T) {
	in := `<a href="javascript:alert(1)">x</a><a href="/safe">y</a><img src="https://ok.com/a.png">`
	out, _ := SanitizeCustomHTML(in)
	if strings.Contains(out, "javascript:") {
		t.Fatalf("javascript: 协议应被移除: %s", out)
	}
	if !strings.Contains(out, "/safe") {
		t.Errorf("相对链接应保留: %s", out)
	}
	if !strings.Contains(out, "https://ok.com/a.png") {
		t.Errorf("https 图片应保留: %s", out)
	}
}

func TestSanitizeCustomHTML_DangerTagsRemoved(t *testing.T) {
	in := `<iframe src="//x"></iframe><object data="x"></object><svg onload=alert(1)></svg><form><input></form>`
	out, _ := SanitizeCustomHTML(in)
	for _, bad := range []string{"iframe", "object", "svg", "form", "input"} {
		if strings.Contains(out, bad) {
			t.Errorf("危险标签 %s 应被移除: %s", bad, out)
		}
	}
}

func TestSanitizeCustomHTML_RawHTMLPassThroughForSelf(t *testing.T) {
	// 本函数用于「公开作者页」消毒（去脚本）。注意：原始未消毒的脚本不应通过。
	in := `<div onclick="steal()">click</div>`
	out, _ := SanitizeCustomHTML(in)
	if strings.Contains(out, "onclick") {
		t.Fatalf("onclick 应被移除: %s", out)
	}
	if !strings.Contains(out, "<div>") {
		t.Fatalf("div 应保留: %s", out)
	}
}

func TestSanitizeCustomHTML_EmptyAndOverlong(t *testing.T) {
	out, _ := SanitizeCustomHTML("")
	if out != "" {
		t.Fatalf("空输入应返回空")
	}
	big := strings.Repeat("<div>a</div>", 200000)
	out, removed := SanitizeCustomHTML(big)
	if out != "" || len(removed) == 0 {
		t.Fatalf("超长输入应整体拒绝")
	}
}
