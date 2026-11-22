package selfhost

import "testing"

func TestStripBase(t *testing.T) {
	cases := map[string]string{
		"/sh":             "/",
		"/sh/order":       "/order",
		"/sh/order/api/x": "/order/api/x",
		"/order":          "/order", // 内部直传（未走 /sh 挂载）保持原样
		"/shipped/order":  "/shipped/order",
	}
	for in, want := range cases {
		if got := stripBase(in); got != want {
			t.Errorf("stripBase(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestManagerMatchLongestPrefix(t *testing.T) {
	m := &Manager{entries: []*proxyEntry{
		{prefix: "/a"},
		{prefix: "/a/b"},
	}}
	for _, tc := range []struct{ path, want string }{
		{"/a", "/a"},
		{"/a/x", "/a"},
		{"/a/b", "/a/b"},
		{"/a/b/c", "/a/b"},
		{"/ab", ""},
		{"/z", ""},
	} {
		e := m.match(tc.path)
		got := ""
		if e != nil {
			got = e.prefix
		}
		if got != tc.want {
			t.Errorf("match(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}
