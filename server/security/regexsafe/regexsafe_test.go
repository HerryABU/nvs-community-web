package regexsafe

import "testing"

func TestValidateChapterSplitRule(t *testing.T) {
	cases := []struct {
		name    string
		rule    string
		wantErr bool
	}{
		{"空规则放行（走自动识别）", "", false},
		{"普通第X章", `第[0-9]+章`, false},
		{"内联多行标志 (?m)", `(?m)^-{3,}$`, false},
		{"内联忽略大小写 (?i)", `(?i)^chapter`, false},
		{"组合内联标志 (?is)", `(?is)^#{1,6}\s`, false},
		{"命名分组 (?P<name>", `(?P<no>第\d+章)`, true}, // <> 未列入白名单，拒绝
		{"环视 (?=", `(?=第)第X章`, true},
		{"负环视 (?!", `(?!第)`, true},
		{"后行环视 (?<=", `(?<=第)`, true},
		{"负后行环视 (?<!", `(?<!第)`, true},
		{"反向引用 \\1", `(第)\1`, true},
		{"非法字符 <", `<script>`, true},
		{"超长规则", string(make([]byte, MaxSplitRuleLen+1)), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateChapterSplitRule(c.rule)
			if c.wantErr && err == nil {
				t.Errorf("规则 %q 应被拒绝，实际通过", c.rule)
			}
			if !c.wantErr && err != nil {
				t.Errorf("规则 %q 应被放行，实际报错: %v", c.rule, err)
			}
		})
	}
}

// TestCompileAllowsInlineFlag 回归：含 (?m) 的合法规则必须能编译通过，
// 不再被误判为「环视」而回退自动识别（此前 strings.Contains(rule,"(?") 的 bug）。
func TestCompileAllowsInlineFlag(t *testing.T) {
	re, err := CompileChapterSplitRule(`(?m)^-{3,}$`)
	if err != nil {
		t.Fatalf("含 (?m) 的规则应编译通过，实际报错: %v", err)
	}
	locs := re.FindAllStringIndex("a\n---\nb\n---\nc", -1)
	if len(locs) != 2 {
		t.Fatalf("应匹配 2 处分割线，实际 %d", len(locs))
	}
}
