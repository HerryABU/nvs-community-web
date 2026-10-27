package ipgeo

import "testing"

// TestLookupKnownIPs 用可公开复核的 IP 校验解析结果。
// 期望值取自 ip2region 官方样例的语义，而不是「跑出什么就算什么」——
// 否则读表算法错了（比如把绝对偏移当相对偏移）也会一路「通过」。
func TestLookupKnownIPs(t *testing.T) {
	if !Available() {
		t.Fatal("ip2region 数据未嵌入，Available()=false")
	}
	size, ver := Stats()
	t.Logf("xdb 大小 %d 字节，version=%d", size, ver)

	cases := []struct {
		ip            string
		wantCountry   string
		wantProvince  string
		wantCountryCD string
	}{
		{"8.8.8.8", "United States", "California", "US"},
		{"114.114.114.114", "中国", "江苏省", "CN"},
		{"223.5.5.5", "中国", "浙江省", "CN"},
		{"39.156.66.10", "中国", "北京市", "CN"},
		{"202.96.128.86", "中国", "广东省", "CN"},
	}
	for _, c := range cases {
		r := Lookup(c.ip)
		if r.Country != c.wantCountry || r.Province != c.wantProvince {
			t.Errorf("%s => %s；期望 %s|%s", c.ip, r.String(), c.wantCountry, c.wantProvince)
			continue
		}
		if r.CountryCD != c.wantCountryCD {
			t.Errorf("%s 国家代码 => %q；期望 %q", c.ip, r.CountryCD, c.wantCountryCD)
		}
		t.Logf("%s => %s", c.ip, r.String())
	}
}

// TestLookupReserved 内网/保留地址必须被判为 Reserved，不能被算到某个省头上。
func TestLookupReserved(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "192.168.1.1", "10.0.0.1"} {
		r := Lookup(ip)
		if !r.IsReserved() {
			t.Errorf("%s 应为保留地址，实际 %s", ip, r.String())
		}
	}
}

// TestLookupIPv6Unsupported IPv6 不应被 IPv4 段误判。
func TestLookupIPv6Unsupported(t *testing.T) {
	r := Lookup("2400:3200::1")
	if !r.IsReserved() {
		t.Errorf("IPv6 应返回空结果（本库只含 IPv4），实际 %s", r.String())
	}
}

// TestLookupInvalid 非法输入不 panic。
func TestLookupInvalid(t *testing.T) {
	for _, ip := range []string{"", "not-an-ip", "999.1.1.1"} {
		_ = Lookup(ip)
	}
}

// TestNormalizeProvince 省名归一：xdb 简称 → GeoJSON 全称。
func TestNormalizeProvince(t *testing.T) {
	cases := map[string]string{
		"广东":  "广东省",
		"广东省": "广东省",
		"北京":  "北京市",
		"内蒙古": "内蒙古自治区",
		"香港":  "香港特别行政区",
		"":    "",
		"0":   "",
	}
	for in, want := range cases {
		if got := NormalizeProvince(in); got != want {
			t.Errorf("NormalizeProvince(%q) = %q；期望 %q", in, got, want)
		}
	}
}
