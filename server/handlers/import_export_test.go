package handlers

import (
	"testing"
)

// TestParseChapters 自动识别章节拆分：
// 第X章模式 / HTML h1-h6 / Markdown # 标题 / 无标记整篇
func TestParseChapters(t *testing.T) {
	// 1) 第X章模式
	ch := parseChapters("第一章 开端\n正文一\n第二章 发展\n正文二\n第三章 高潮\n正文三")
	if len(ch) != 3 {
		t.Fatalf("第X章模式应拆 3 章，实际 %d", len(ch))
	}
	if ch[0].title != "第一章 开端" || ch[0].content != "正文一" {
		t.Errorf("第一章内容不符: %+v", ch[0])
	}
	if ch[2].title != "第三章 高潮" || ch[2].content != "正文三" {
		t.Errorf("第三章内容不符: %+v", ch[2])
	}

	// 2) 数字章节号 + 阿拉伯数字
	ch = parseChapters("第1章 开始\n内容1\n第2章 继续\n内容2")
	if len(ch) != 2 || ch[0].title != "第1章 开始" || ch[1].title != "第2章 继续" {
		t.Errorf("数字章节拆分不符: %+v", ch)
	}

	// 3) HTML 标题模式
	ch = parseChapters("<h1>第一部分</h1>\n内容A\n<h2>子节</h2>\n内容B")
	if len(ch) != 2 {
		t.Fatalf("HTML 标题应拆 2 章，实际 %d", len(ch))
	}
	if ch[0].title != "第一部分" || ch[1].title != "子节" {
		t.Errorf("HTML 标题不符: %+v", ch)
	}

	// 4) Markdown 标题模式（# / ## / ###）
	ch = parseChapters("# 标题一\n内容1\n## 标题二\n内容2\n### 标题三\n内容3")
	if len(ch) != 3 {
		t.Fatalf("MD 标题应拆 3 章，实际 %d", len(ch))
	}
	if ch[0].title != "标题一" || ch[1].title != "标题二" || ch[2].title != "标题三" {
		t.Errorf("MD 标题不符: %+v", ch)
	}

	// 5) 无章节标记 → 整篇一章
	ch = parseChapters("没有标题标记\n的纯文本内容")
	if len(ch) != 1 || ch[0].title != "正文" || ch[0].content == "" {
		t.Errorf("无标记应整篇一章: %+v", ch)
	}

	// 6) 单个「第X章」不拆（不足 2 个标题时整篇一章）
	ch = parseChapters("第一章 只有一章\n内容")
	if len(ch) != 1 {
		t.Errorf("单章节标记不应拆分: %+v", ch)
	}
}

// TestParseChaptersWithRule 自定义正则拆分：自动区分「分割线」与「标题前缀」语义
func TestParseChaptersWithRule(t *testing.T) {
	// 1) 分割线规则（---）：匹配行清理后为空 → 分割线
	ch := parseChaptersWithRule("第一章内容\n---\n第二章内容\n---\n第三章内容", `(?m)^-{3,}$`)
	if len(ch) != 3 {
		t.Fatalf("分割线规则应拆 3 段，实际 %d", len(ch))
	}
	if ch[0].title != "第1部分" || ch[2].title != "第3部分" {
		t.Errorf("分割线段落标题不符: %+v", ch)
	}
	if ch[1].content != "第二章内容" {
		t.Errorf("分割线第 2 段内容不符: %+v", ch[1])
	}

	// 2) 标题前缀规则（# 标题）：匹配行清理后非空 → 标题
	ch = parseChaptersWithRule("# 标题A\n内容A\n# 标题B\n内容B", `(?m)^#\s+`)
	if len(ch) != 2 || ch[0].title != "标题A" || ch[1].title != "标题B" {
		t.Errorf("标题规则拆分不符: %+v", ch)
	}

	// 3) 非法正则 → 回退自动识别
	ch = parseChaptersWithRule("第一章 开端\n内容", `([unclosed`)
	if len(ch) != 1 {
		t.Errorf("非法正则应回退自动识别: %+v", ch)
	}

	// 4) 无匹配 → 回退自动识别
	ch = parseChaptersWithRule("普通文本内容", `(?m)^第X章`)
	if len(ch) != 1 {
		t.Errorf("无匹配应回退自动识别: %+v", ch)
	}
}

// TestCleanTitle 标题清理：# 标记 / ** 包裹 / 行首行尾分隔符；纯分隔符返回空
func TestCleanTitle(t *testing.T) {
	cases := map[string]string{
		"### 三级标题": "三级标题",
		"# 一级标题":   "一级标题",
		"*第X章*":    "第X章",
		"**加粗标题**": "加粗标题",
		"- 列表标题":   "列表标题",
		"---":      "",
		"***":      "",
		"  普通标题  ": "普通标题",
	}
	for in, want := range cases {
		if got := cleanTitle(in); got != want {
			t.Errorf("cleanTitle(%q) = %q，期望 %q", in, got, want)
		}
	}
}

// TestStripExt 文件名去扩展名（保留原始标题）
func TestStripExt(t *testing.T) {
	cases := map[string]string{
		"第一章 开端.md":   "第一章 开端",
		"正文.txt":      "正文",
		"noext":       "noext",
		"dir/第一章.doc": "第一章",
		"a.b.c.md":    "a.b.c",
		"":            "",
	}
	for in, want := range cases {
		if got := stripExt(in); got != want {
			t.Errorf("stripExt(%q) = %q，期望 %q", in, got, want)
		}
	}
}
