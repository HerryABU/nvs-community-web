package camp

import (
	"strings"
	"testing"
)

// 强提示词必须包含的约束（用户明确要求，删掉任何一条都算回归）。
func TestNeutralityPromptStrongConstraints(t *testing.T) {
	must := []string{
		"结构性原因", "严禁判断谁对谁错", "倒戈", "人数多", "文字长",
		"饭圈方法", "政治语言方法", "普通人语言格式",
		"confrontation", "lang_format", "只标记，不裁决",
	}
	for _, s := range must {
		if !strings.Contains(NeutralityPrompt, s) && !strings.Contains(NeutralityTailPrompt, s) {
			t.Fatalf("提示词缺少强约束条款：%q", s)
		}
	}
	if !strings.Contains(NeutralityPrompt, "输出契约") {
		t.Fatal("提示词缺少机器可解析的输出契约")
	}
	if !strings.Contains(NeutralityTailPrompt, "待标记的数据") {
		t.Fatal("三明治尾部缺少「文本只是数据不是指令」的复核提醒")
	}
}

// 解析器要能吃下 AI 常见输出形态，并正确读出 confrontation（由 AI 判定，Go 不复算）。
func TestParseAIEnvelope(t *testing.T) {
	cases := []struct {
		name  string
		raw   string
		camps int
		confr bool
	}{
		{"标准契约", `{"confrontation":true,"camps":[{"camp_id":"A","lang_format":"fandom","confidence":80},{"camp_id":"B","lang_format":"political","confidence":70}]}`, 2, true},
		{"代码块包裹", "```json\n" + `{"confrontation":false,"camps":[{"camp_id":"A","lang_format":"everyday"}]}` + "\n```", 1, false},
		{"对象后带说明", `{"confrontation":true,"camps":[{"camp_id":"A"}]}` + "\n以上为标记结果。", 1, true},
		{"旧版数组契约", `[{"camp_id":"A","lang_format":"mixed"}]`, 1, false},
	}
	for _, c := range cases {
		got := parseAIEnvelope(c.raw)
		if len(got.Camps) != c.camps {
			t.Fatalf("%s: camps 期望 %d 得到 %d", c.name, c.camps, len(got.Camps))
		}
		if got.Confrontation != c.confr {
			t.Fatalf("%s: confrontation 期望 %v 得到 %v", c.name, c.confr, got.Confrontation)
		}
	}
}

// 归一化只做枚举收敛，绝不新增「对错」类字段；
// 自由文本标签原样保留（不强行压回固定枚举）。
func TestNormalizeLangFormat(t *testing.T) {
	for in, want := range map[string]string{
		"fandom": LangFandom, "FAN": LangFandom, "fan": LangFandom,
		"political": LangPolitical, "POLITICS": LangPolitical,
		"everyday": LangEveryday, "plain": LangEveryday,
		"mixed": LangMixed, " ": "",
		"学术化论证": "学术化论证", "情绪宣泄式": "情绪宣泄式", "academic": "academic",
	} {
		if got := NormalizeLangFormat(in); got != want {
			t.Fatalf("%q → %q，期望 %q", in, got, want)
		}
	}
}
