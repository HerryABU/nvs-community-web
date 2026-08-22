package security

import "testing"

// TestBadWordsEngine 违禁词引擎冒烟测试：
// 覆盖 literal/regex 双匹配、mark/delete/replace 三动作、scope 范围过滤
func TestBadWordsEngine(t *testing.T) {
	SetDynamicBadWords([]DynamicBadWord{
		{ID: 1, Name: "涉赌词", MatchType: "literal", Pattern: "赌博", Action: "mark", Scope: "all"},
		{ID: 2, Name: "广告词", MatchType: "literal", Pattern: "加微信", Action: "replace", Replacement: "***", Scope: "comment,forum"},
		{ID: 3, Name: "脏话", MatchType: "regex", Pattern: "(?i)fuck", Action: "delete", Scope: "chapter"},
		{ID: 4, Name: "无效正则", MatchType: "regex", Pattern: "([unclosed", Action: "delete", Scope: "all"},
	})

	// 用例 1：comment 范围 → replace 生效（加微信→***）、mark 命中（赌博）、fuck 不属于 comment 范围不生效
	cleaned, hits := ApplyBadWords("今天聊赌博和加微信，fuck you", "comment")
	if cleaned != "今天聊赌博和***，fuck you" {
		t.Errorf("comment 净化结果不符: %q", cleaned)
	}
	if !hasHit(hits, "赌博", "mark") {
		t.Errorf("comment 应命中 mark 动作的「赌博」: %+v", hits)
	}
	if hasHit(hits, "fuck") {
		t.Errorf("comment 范围不应命中 chapter 范围的 fuck: %+v", hits)
	}

	// 用例 2：chapter 范围 → delete 生效（fuck 删除）、加微信 replace 不属于 chapter 不生效
	cleaned, hits = ApplyBadWords("聊赌博，加微信，fuck you", "chapter")
	if cleaned != "聊赌博，加微信， you" {
		t.Errorf("chapter 净化结果不符: %q", cleaned)
	}
	if !hasHit(hits, "fuck") {
		t.Errorf("chapter 应命中 delete 动作的 fuck: %+v", hits)
	}

	// 用例 3：纯扫描（mark 不删不改）
	cleaned, hits = ApplyBadWords("赌博", "blog")
	if cleaned != "赌博" {
		t.Errorf("mark 动作不应修改文本: %q", cleaned)
	}
	if len(hits) != 1 || hits[0].RuleName != "涉赌词" {
		t.Errorf("blog 范围应命中 mark 的「涉赌词」: %+v", hits)
	}

	// 用例 4：正则大小写不敏感（(?i)fuck）
	if _, hits := ApplyBadWords("FUCK 大写", "chapter"); !hasHit(hits, "FUCK") {
		t.Errorf("(?i) 正则应命中大写 FUCK: %+v", hits)
	}
}

func hasHit(hits []BadWordHit, matched string, actions ...string) bool {
	for _, h := range hits {
		if h.Matched == matched {
			if len(actions) == 0 {
				return true
			}
			for _, a := range actions {
				if h.Action == a {
					return true
				}
			}
		}
	}
	return false
}
