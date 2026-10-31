package middleware

import (
	"crypto/tls"
	"net/http"
	"strings"
	"testing"
)

// effectiveHost 决定「访问域名」这一列记的是不是用户真正看到的地址。
// cpolar / nginx 转发时 Host 常被改写成上游，所以必须优先信 X-Forwarded-Host。
func TestEffectiveHost(t *testing.T) {
	cases := []struct {
		name    string
		host    string
		headers map[string]string
		tls     bool
		wantH   string
		wantS   string
	}{
		{
			name: "直连：用 Host",
			host: "192.168.1.3:8000", wantH: "192.168.1.3:8000", wantS: "http",
		},
		{
			name:    "nginx 透传域名（优先于被改写的 Host）",
			host:    "127.0.0.1:8000",
			headers: map[string]string{"X-Forwarded-Host": "nvs.example.com"},
			wantH:   "nvs.example.com", wantS: "http",
		},
		{
			name: "cpolar 域名 + https",
			host: "127.0.0.1:8000",
			headers: map[string]string{
				"X-Forwarded-Host":  "3f2a-1-2-3-4.ngrok-free.app",
				"X-Forwarded-Proto": "https",
			},
			wantH: "3f2a-1-2-3-4.ngrok-free.app", wantS: "https",
		},
		{
			name:    "多级代理逗号列表取第一个",
			host:    "127.0.0.1:8000",
			headers: map[string]string{"X-Forwarded-Host": "a.com, b.com, c.com"},
			wantH:   "a.com", wantS: "http",
		},
		{
			name:    "只有 X-Real-Host 时也能取到",
			host:    "127.0.0.1:8000",
			headers: map[string]string{"X-Real-Host": "real.example.com"},
			wantH:   "real.example.com", wantS: "http",
		},
		{
			name:  "大小写与结尾点归一",
			host:  "NVS.Example.COM.",
			wantH: "nvs.example.com", wantS: "http",
		},
		{
			name:    "个别反代把 scheme 写进 host",
			host:    "127.0.0.1:8000",
			headers: map[string]string{"X-Forwarded-Host": "https://tunnel.example.com"},
			wantH:   "tunnel.example.com", wantS: "http",
		},
		{
			name: "TLS 直连识别为 https",
			host: "nvs.example.com", tls: true,
			wantH: "nvs.example.com", wantS: "https",
		},
		{
			name:    "X-Forwarded-Ssl: on",
			host:    "nvs.example.com",
			headers: map[string]string{"X-Forwarded-Ssl": "on"},
			wantH:   "nvs.example.com", wantS: "https",
		},
	}
	for _, c := range cases {
		r := &http.Request{Host: c.host, Header: http.Header{}}
		for k, v := range c.headers {
			r.Header.Set(k, v)
		}
		if c.tls {
			r.TLS = &tls.ConnectionState{}
		}
		gotH, gotS := effectiveHost(r)
		if gotH != c.wantH || gotS != c.wantS {
			t.Errorf("%s: 得到 (%q, %q)，期望 (%q, %q)", c.name, gotH, gotS, c.wantH, c.wantS)
		}
	}
}

// 超长 Host 必须截断到列宽，否则写库会失败/被静默截断。
func TestEffectiveHost_Truncates(t *testing.T) {
	r := &http.Request{Host: strings.Repeat("a", 300), Header: http.Header{}}
	h, _ := effectiveHost(r)
	if len(h) != 128 {
		t.Fatalf("期望截断到 128 字节，实际 %d", len(h))
	}
}

// nil 请求不能 panic（中间件理论上拿不到 nil，但不该靠这个假设）。
func TestEffectiveHost_Nil(t *testing.T) {
	h, s := effectiveHost(nil)
	if h != "" || s != "http" {
		t.Fatalf("nil 请求应返回空 host + http，实际 (%q, %q)", h, s)
	}
}
