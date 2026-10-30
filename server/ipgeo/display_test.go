package ipgeo

import "testing"

// DisplayName 是对外展示口径：国内只到省份（去掉「省/市/自治区」后缀），
// 境外只到国家，内网/保留一律不展示。
func TestDisplayName(t *testing.T) {
	if !Available() {
		t.Skip("ip2region 数据未嵌入")
	}
	cases := []struct{ ip, want string }{
		{"114.114.114.114", "江苏"},    // 江苏省 → 江苏
		{"223.5.5.5", "浙江"},          // 浙江省 → 浙江
		{"39.156.66.10", "北京"},       // 北京市 → 北京
		{"202.96.128.86", "广东"},      // 广东省 → 广东
		{"8.8.8.8", "United States"}, // 境外只到国家
		{"127.0.0.1", ""},            // 回环 → 不展示
		{"10.0.0.5", ""},             // 私网 → 不展示
		{"", ""},                     // 空 → 不展示
	}
	for _, c := range cases {
		if got := DisplayName(c.ip); got != c.want {
			t.Errorf("DisplayName(%q) = %q，期望 %q", c.ip, got, c.want)
		}
	}
}
