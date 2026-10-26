package updater

import "testing"

// 模型输出经常套 markdown 代码块或加解释，解析必须能捞出来。
func TestParseCandidates(t *testing.T) {
	raw := "```json\n[{\"word\":\"典中典\",\"cat\":\"slang\",\"tags\":[\"亚文化\"],\"risk\":\"mid\",\"confidence\":55,\"basis\":\"多用于嘲讽\"}]\n```"
	got := parseCandidates(raw)
	if len(got) != 1 || got[0].Word != "典中典" {
		t.Fatalf("解析失败: %+v", got)
	}
	if got[0].Confidence != 55 {
		t.Fatalf("置信度丢失: %+v", got[0])
	}

	// 空数组表示「判断不出是黑话」，允许且不应报错
	if got := parseCandidates("[]"); len(got) != 0 {
		t.Fatalf("空结果应返回 0 条，得到 %d", len(got))
	}

	// 完全不成形 → 空（宁可当天不更新，也不乱入库）
	if got := parseCandidates("我觉得这个词挺常见的，不算黑话。"); len(got) != 0 {
		t.Fatalf("无 JSON 时应返回空，得到 %+v", got)
	}

	// 超长词过滤（>12 字基本是句子，不是黑话）
	long := "[{\"word\":\"这是一个非常长的句子不可能是词\",\"cat\":\"slang\",\"confidence\":80}]"
	if got := parseCandidates(long); len(got) != 0 {
		t.Fatalf("超长词应被过滤，得到 %+v", got)
	}
}

func TestSeverityOf(t *testing.T) {
	cases := map[string]int{"high": 3, "midhigh": 2, "mid": 2, "low": 1, "": 1, "unknown": 1}
	for risk, want := range cases {
		if got := severityOf(risk); got != want {
			t.Fatalf("risk=%q got=%d want=%d", risk, got, want)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("短", 10); got != "短" {
		t.Fatalf("未超长不该截断: %q", got)
	}
	got := truncate("一二三四五", 3)
	if []rune(got)[0] != '一' || len([]rune(got)) != 4 { // 3 字 + 省略号
		t.Fatalf("按字符截断错误: %q", got)
	}
}
