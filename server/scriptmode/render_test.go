package scriptmode

import (
	"strings"
	"testing"
)

// TestBubbleDataAttrs 气泡需带 data-* 供前端长按互动（gap-audit #22）
func TestBubbleDataAttrs(t *testing.T) {
	html := RenderHTML("小明：你来了。\n小红：嗯。\n")
	if !strings.Contains(html, `data-bubble="1"`) || !strings.Contains(html, `data-speaker="小明"`) {
		t.Fatalf("气泡缺少 data 属性: %s", html)
	}
	if !strings.Contains(html, `data-bubble="2"`) {
		t.Fatalf("第二个气泡序号应为 2: %s", html)
	}
	if !strings.Contains(html, `data-text="你来了。"`) {
		t.Fatalf("data-text 应含原文: %s", html)
	}
}

// TestBubbleDataAttrsEscaped 原文含引号必须转义，不能破结构（否则等于 HTML 注入）
func TestBubbleDataAttrsEscaped(t *testing.T) {
	src := "小明：他说\"别动\"，然后就走了。\n"
	html := RenderHTML(src)
	if strings.Count(html, `data-text="`) != 1 {
		t.Fatalf("data-text 只应出现一次: %s", html)
	}
	if strings.Contains(html, `data-text="他说"`) {
		t.Fatalf("引号未转义，属性被截断: %s", html)
	}
	if !strings.Contains(html, "&#34;") && !strings.Contains(html, "&#39;") {
		t.Logf("提示：转义形式可能不同，实际 HTML=%s", html)
	}
}

// TestStageNarrationNoBubbleIndex 舞台/旁白不占气泡序号
func TestStageNarrationNoBubbleIndex(t *testing.T) {
	src := "【雨夜】\n旁白一句。\n小明：台词。\n"
	html := RenderHTML(src)
	if !strings.Contains(html, `data-bubble="1"`) {
		t.Fatalf("第一个气泡应为 1: %s", html)
	}
	if strings.Contains(html, `data-bubble="2"`) {
		t.Fatalf("舞台/旁白不应占号: %s", html)
	}
}
