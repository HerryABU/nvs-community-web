package topic

import "testing"

func TestClassifyAndRisk(t *testing.T) {
	cases := []struct {
		name   string
		tags   []string
		want   Risk
		groups []Group
	}{
		{"无关标签不抬风险", []string{"玄幻", "连载中"}, RiskLow, nil},
		{"亚文化单独=中高", []string{"二次元", "同人"}, RiskMidHigh, []Group{GroupSubculture}},
		{"性别议题单独=高", []string{"性别议题", "彩礼"}, RiskHigh, []Group{GroupGender}},
		{"政治批判单独=中", []string{"时政评论"}, RiskMid, []Group{GroupPolitics}},
		{"资本批判单独=中", []string{"资本家", "996"}, RiskMid, []Group{GroupCapital}},
		// 用户明确要求：亚文化 + 政治批判 → 高风险
		{"亚文化+政治批判=高", []string{"二次元", "时政"}, RiskHigh, []Group{GroupSubculture, GroupPolitics}},
		{"亚文化+性别=高", []string{"动漫", "女权"}, RiskHigh, []Group{GroupGender, GroupSubculture}},
		{"政治+资本=中高", []string{"政策", "资本家"}, RiskMidHigh, []Group{GroupPolitics, GroupCapital}},
		{"地域+政治=高", []string{"地域黑", "体制"}, RiskHigh, []Group{GroupPolitics, GroupRegion}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := Assess(c.tags, "一段中性的评论内容")
			if a.Risk != c.want {
				t.Fatalf("风险等级不符: got=%s want=%s (groups=%v reasons=%v)",
					a.Risk, c.want, a.Groups, a.Reasons)
			}
			if len(c.groups) != len(a.Groups) {
				t.Fatalf("话题组数量不符: got=%v want=%v", a.Groups, c.groups)
			}
		})
	}
}

func TestAssessSeverityBump(t *testing.T) {
	// 政治批判单独是「中」，但出现组织化动员就该抬到中高
	a := Assess([]string{"时政"}, "大家一起去举报他，出征！")
	if a.Risk <= RiskMid {
		t.Fatalf("有动员信号却没升档: risk=%s reasons=%v", a.Risk, a.Reasons)
	}
	if a.PublicOpinionScore < 40 {
		t.Fatalf("舆论可能性分数过低: %d", a.PublicOpinionScore)
	}

	// 同类信号只升一档，不能连升
	b := Assess([]string{"时政"}, "出征，冲了他，挂人，人肉，开盒")
	if b.Risk == RiskHigh && len(b.Severity) > 4 {
		t.Fatalf("同类信号重复升档过多: risk=%s sev=%d", b.Risk, len(b.Severity))
	}

	// 高风险话题 + 动员 + 群体对立 → 舆论高风险
	c := Assess([]string{"性别议题"}, "女拳都该死，大家出征举报她")
	if !c.PublicOpinionHigh {
		t.Fatalf("应判定为高风险舆论: score=%d risk=%s sev=%v", c.PublicOpinionScore, c.Risk, c.Severity)
	}
}

func TestParseTagsJSON(t *testing.T) {
	if got := ParseTagsJSON(`["二次元","时政"]`); len(got) != 2 {
		t.Fatalf("解析失败: %v", got)
	}
	if got := ParseTagsJSON("坏的 json"); got != nil {
		t.Fatalf("坏数据应返回 nil，得到 %v", got)
	}
	if got := ParseTagsJSON("[]"); got != nil {
		t.Fatalf("空数组应返回 nil，得到 %v", got)
	}
}
