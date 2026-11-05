package aiworkbench

import "testing"

// TestResolveContextLimit 上下文条数：用户只能调小，不能突破站点上限；关掉记忆就是 0
func TestResolveContextLimit(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }

	cases := []struct {
		name   string
		flags  int
		user   int
		memory *bool
		want   int
	}{
		{"默认走站点上限", 20, 0, nil, 20},
		{"站点没配时兜底 20", 0, 0, nil, 20},
		{"用户调小生效", 20, 6, nil, 6},
		{"用户调大不生效（防成本被顶爆）", 20, 200, nil, 20},
		{"关掉记忆 → 0 条历史", 20, 10, boolPtr(false), 0},
		{"显式开启记忆 → 按设置", 20, 10, boolPtr(true), 10},
		{"站点上限很小时用户仍不能突破", 4, 30, nil, 4},
	}
	for _, c := range cases {
		if got := resolveContextLimit(c.flags, c.user, c.memory); got != c.want {
			t.Errorf("%s: resolveContextLimit(%d,%d,%v) = %d, want %d", c.name, c.flags, c.user, c.memory, got, c.want)
		}
	}
}
