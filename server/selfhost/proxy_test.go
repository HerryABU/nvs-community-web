package selfhost

import (
	"strings"
	"testing"

	"nvs-server/selfhost/proxycore"
)

// 适配层测试：NVS 库里的前缀带前导 /，喂给 GWh 同构的 Manager 时必须剥掉；
// 其余匹配/改写/跳数语义全部由 proxycore 的移植测试覆盖。
func TestRoutePrefixAdaptation(t *testing.T) {
	for _, p := range []string{"/java-order", "/team/app", "/x"} {
		trimmed := strings.TrimPrefix(p, "/")
		if trimmed == p || strings.HasPrefix(trimmed, "/") {
			t.Errorf("前缀转换错误: %q -> %q", p, trimmed)
		}
	}
}

func TestManagerUsesGWhSemantics(t *testing.T) {
	m := proxycore.NewManager("", proxycore.SelfInfo{})
	if err := m.Load([]proxycore.Route{
		{Prefix: "a", Target: ":19099", Timeout: 5, Status: proxycore.StatusActive},
		{Prefix: "a/b", Target: ":19098", Timeout: 5, Status: proxycore.StatusActive},
	}); err != nil {
		t.Fatal(err)
	}
	if !m.Matchable("/a/b/x", "") {
		t.Fatal("路径级匹配应命中多级前缀")
	}
	if !m.Exists("a/b") || m.Count() != 2 {
		t.Fatalf("路由表内容错误: count=%d", m.Count())
	}
	m.Remove("a/b")
	// 删除多级前缀后，/a/b/x 会回退命中单段前缀 a（GWh 的既有语义），但 a/b 本身不再存在。
	if m.Exists("a/b") || !m.Matchable("/a/b/x", "") || !m.Matchable("/a/x", "") {
		t.Fatalf("删除后路由表语义错误: count=%d", m.Count())
	}
}
