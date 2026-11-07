package sentiment

import (
	"testing"

	"nvs-server/sentiment/guard"
	"nvs-server/sentiment/lexicon"
	"nvs-server/sentiment/topic"
)

// 阵营对抗「不一刀切」：多争议话题 + 纯立场命中 → block 应降级为 flag。
// 这里不判断谁对谁错，只验证「系统在此不具备裁决资格」的编排结果。
func TestCampConfrontationDowngradesBlock(t *testing.T) {
	d := guard.Decision{
		Action: guard.ActionBlock,
		Groups: []topic.Group{topic.GroupGender, topic.GroupPolitics},
		Hits: []lexicon.Hit{
			{Word: "立场", Cat: lexicon.CatSlang, Severity: 1},
		},
		Blocked: []string{"立场"},
	}
	if !isCampConfrontationBlock(d) {
		t.Fatal("纯立场之争的多话题 block 应被识别为需降级")
	}
}

// 真实违规不放松：强制词 / 辱骂 / 群体蔑称 / 刷屏出征都不降级。
func TestCampConfrontationKeepsRealViolations(t *testing.T) {
	base := guard.Decision{
		Action: guard.ActionBlock,
		Groups: []topic.Group{topic.GroupGender, topic.GroupPolitics},
		Hits:   []lexicon.Hit{{Word: "xxx", Cat: lexicon.CatSlang}},
	}
	cases := []struct {
		name string
		mut  func(*guard.Decision)
	}{
		{"强制拦截词", func(d *guard.Decision) { d.Hits = []lexicon.Hit{{Word: "禁词", ForceBlock: true}} }},
		{"人身攻击", func(d *guard.Decision) { d.Hits = []lexicon.Hit{{Word: "傻逼", Cat: lexicon.CatAbuse, Severity: 3}} }},
		{"群体蔑称", func(d *guard.Decision) { d.Hits = []lexicon.Hit{{Word: "xx", Cat: lexicon.CatBigotry}} }},
		{"刷屏出征", func(d *guard.Decision) { d.Flood = lexicon.Flood{Mentions: 6} }},
		{"单一话题", func(d *guard.Decision) { d.Groups = []topic.Group{topic.GroupGender} }},
	}
	for _, c := range cases {
		d := base
		d.Hits = append([]lexicon.Hit{}, base.Hits...)
		d.Groups = append([]topic.Group{}, base.Groups...)
		c.mut(&d)
		if isCampConfrontationBlock(d) {
			t.Fatalf("%s 不应被降级为标记", c.name)
		}
	}
}
