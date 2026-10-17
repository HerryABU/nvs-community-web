package security

import (
	"strings"
	"testing"
)

func TestValidateUserPattern(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		wantErr bool
	}{
		{"普通字面", `广告`, false},
		{"合法正则", `(?i)(微信|QQ)[:：]?\s*\d{5,}`, false},
		{"空串拒绝", "", true},
		{"超长拒绝", strings.Repeat("a", MaxUserPatternLen+1), true},
		{"未闭合括号拒绝", `(abc`, true},
		{"环视 RE2 编译失败", `(?=第)第`, true},
		{"反向引用 RE2 编译失败", `(第)\1`, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateUserPattern(c.pattern)
			if c.wantErr && err == nil {
				t.Errorf("pattern %q 应被拒绝，实际通过", c.pattern)
			}
			if !c.wantErr && err != nil {
				t.Errorf("pattern %q 应放行，实际报错: %v", c.pattern, err)
			}
		})
	}
}

func TestUserPatternHintNestedQuantifier(t *testing.T) {
	if hint := UserPatternHint(`(a+)+b`); hint == "" {
		t.Error("(a+)+ 应给出嵌套量词提示")
	}
	if hint := UserPatternHint(`(?:第)[0-9]+章`); hint != "" {
		t.Errorf("普通非捕获分组+量词不应提示，实际: %s", hint)
	}
	if hint := UserPatternHint(`(?i)广告`); hint != "" {
		t.Errorf("普通 (?i) 规则不应提示，实际: %s", hint)
	}
}

func TestTruncateForScan(t *testing.T) {
	small := strings.Repeat("x", 1024)
	got, truncated := TruncateForScan(small)
	if truncated || got != small {
		t.Error("小于上限的内容不应被截断")
	}
	big := strings.Repeat("y", MaxScanBytes+100)
	got, truncated = TruncateForScan(big)
	if !truncated || len(got) != MaxScanBytes {
		t.Errorf("超过上限应截断到 %d，实际 len=%d truncated=%v", MaxScanBytes, len(got), truncated)
	}
}

func TestScanBadWordsTruncatesLargeInput(t *testing.T) {
	// 构造一条会命中末尾的规则，验证 1MB 后的内容不被扫描（截断方向正确）
	SetDynamicBadWords([]DynamicBadWord{
		{ID: 1, Name: "测试规则", MatchType: "literal", Pattern: "炸弹词", Action: "mark", Scope: "all"},
	})
	big := strings.Repeat("a", MaxScanBytes) + "炸弹词"
	hits := ScanBadWords(big, "all")
	if len(hits) != 0 {
		t.Errorf("1MB 边界外的命中不应被扫描到，实际 hits=%d", len(hits))
	}
	inside := strings.Repeat("a", 100) + "炸弹词"
	hits = ScanBadWords(inside, "all")
	if len(hits) != 1 {
		t.Errorf("上限内的命中应被扫描到，实际 hits=%d", len(hits))
	}
	SetDynamicBadWords(nil)
}
