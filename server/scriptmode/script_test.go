package scriptmode

import (
	"strings"
	"testing"
)

func TestParseDialogueAndSides(t *testing.T) {
	src := "【清晨，街角】\n李明：早啊。\n李明：今天也要加油。\n旁白：他笑了笑。\n王芳：你也是。\n"
	nodes := Parse(src)
	if len(nodes) != 4 {
		t.Fatalf("期望 4 个节点（舞台+合并对话+旁白+对话），实际 %d：%+v", len(nodes), nodes)
	}
	if nodes[0].Type != NodeStage || nodes[0].Text != "清晨，街角" {
		t.Fatalf("舞台提示解析错误：%+v", nodes[0])
	}
	if nodes[1].Type != NodeDialogue || nodes[1].Speaker != "李明" {
		t.Fatalf("对话解析错误：%+v", nodes[1])
	}
	// 同一角色连续两句应合并到一个气泡
	if nodes[1].Text != "早啊。\n今天也要加油。" {
		t.Fatalf("同角色连续对话未合并：%q", nodes[1].Text)
	}
	if nodes[1].Side != "left" || nodes[3].Side != "right" {
		t.Fatalf("左右分侧错误：%s / %s", nodes[1].Side, nodes[3].Side)
	}
	if nodes[2].Type != NodeNarration {
		t.Fatalf("旁白解析错误：%+v", nodes[2])
	}
}

func TestParseNotDialogue(t *testing.T) {
	// URL 与超长前缀不应被当成角色名
	nodes := Parse("详情见 https://example.com/a:b 说明\n" +
		"这是一个非常非常长的句子没有冒号分隔的情况\n")
	for _, n := range nodes {
		if n.Type == NodeDialogue {
			t.Fatalf("误判为对话：%+v", n)
		}
	}
}

func TestRenderHTMLEscapes(t *testing.T) {
	out := RenderHTML(`小明：<script>alert(1)</script>`)
	if want := "&lt;script&gt;"; !strings.Contains(out, want) {
		t.Fatalf("未转义危险内容：%s", out)
	}
	if !strings.Contains(out, `class="script-bubble sb-left"`) {
		t.Fatalf("气泡结构缺失：%s", out)
	}
}

func TestSpeakersAndCount(t *testing.T) {
	src := "甲：一\n乙：二\n甲：三\n【转场】\n"
	sp := Speakers(src)
	if len(sp) != 2 || sp[0] != "甲" || sp[1] != "乙" {
		t.Fatalf("角色列表错误：%v", sp)
	}
	lines, speakers := CountDialogue(src)
	if lines != 3 || speakers != 2 {
		t.Fatalf("统计错误：lines=%d speakers=%d", lines, speakers)
	}
}

func TestEmoteAndImage(t *testing.T) {
	out := RenderHTML("【夜】\n小明：你来了[笑]\n![街景](/uploads/street.png)\n")
	if !strings.Contains(out, `class="sb-emote"`) || !strings.Contains(out, "😄") {
		t.Fatalf("表情未渲染: %s", out)
	}
	if !strings.Contains(out, `<img class="sb-image" src="/uploads/street.png"`) {
		t.Fatalf("图片未渲染: %s", out)
	}
	// 危险协议必须被拒，且不能出现在 src 里
	bad := RenderHTML("小明：点我\n![x](javascript:alert(1))\n")
	if strings.Contains(bad, "javascript:") || !strings.Contains(bad, "图片地址不被允许") {
		t.Fatalf("危险图片地址未被拦截: %s", bad)
	}
	// data: 协议同样拒绝
	if SafeURL("data:image/png;base64,AAAA") || SafeURL("jAvAsCrIpT:alert(1)") {
		t.Fatal("SafeURL 白名单失效")
	}
	if !SafeURL("https://example.com/a.png") || !SafeURL("./local.png") {
		t.Fatal("SafeURL 误拒合法地址")
	}
}

func TestParseMixed(t *testing.T) {
	src := "夜色沉沉，街灯一盏盏亮起来。\n\n连续的描写第二段。\n\n小明：你来了。\n【转场】\n小红：嗯。\n"
	nodes := ParseMixed(src)
	var prose, dialogue, stage int
	for _, n := range nodes {
		switch n.Type {
		case NodeProse:
			prose++
		case NodeDialogue:
			dialogue++
		case NodeStage:
			stage++
		}
	}
	if prose != 1 || dialogue != 2 || stage != 1 {
		t.Fatalf("混排解析错误: prose=%d dialogue=%d stage=%d (%+v)", prose, dialogue, stage, nodes)
	}
	html := RenderMixedHTML(src)
	// 注意：气泡的 class 是「script-bubble sb-left/sb-right」，断言不带右引号
	if !strings.Contains(html, `class="script-prose markdown-body"`) || !strings.Contains(html, `class="script-bubble sb-left"`) {
		t.Fatalf("混排渲染缺结构: %s", html)
	}
}
