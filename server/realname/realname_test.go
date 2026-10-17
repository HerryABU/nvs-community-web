package realname

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ============ 腾讯云 TC3 签名与全链路 ============

func TestTencentVerifyViaMockServer(t *testing.T) {
	var gotAuth, gotAction, gotVersion, gotTS, gotBody string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAction = r.Header.Get("X-TC-Action")
		gotVersion = r.Header.Get("X-TC-Version")
		gotTS = r.Header.Get("X-TC-Timestamp")
		buf := make([]byte, r.ContentLength)
		r.Body.Read(buf)
		gotBody = string(buf)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"Response":{"Result":"0","Description":"姓名和身份证号一致","RequestId":"req-1"}}`))
	}))
	defer srv.Close()

	// 把腾讯域名指向本地测试服务（TLS 客户端信任 httptest 证书）
	origHost, origClient := tcHost, httpClient
	tcHost = strings.TrimPrefix(srv.URL, "https://")
	httpClient = srv.Client()
	defer func() { tcHost = origHost; httpClient = origClient }()

	res, err := tencentProvider{}.Verify(context.Background(), Config{
		Tencent: TencentCfg{SecretID: "AKIDtest", SecretKey: "secretkey"},
	}, "张三", "11010119900307851X")
	if err != nil {
		t.Fatalf("Verify 失败: %v", err)
	}
	if !res.Matched {
		t.Errorf("应核验一致，实际 %+v", res)
	}
	if gotAction != "IdCardVerification" || gotVersion != "2018-03-01" {
		t.Errorf("Action/Version 头错误: %s/%s", gotAction, gotVersion)
	}
	if gotTS == "" {
		t.Errorf("X-TC-Timestamp 头缺失")
	}
	// Authorization 三段结构校验
	if !strings.HasPrefix(gotAuth, "TC3-HMAC-SHA256 Credential=AKIDtest/") ||
		!strings.Contains(gotAuth, ", SignedHeaders=content-type;host, Signature=") {
		t.Errorf("Authorization 结构错误: %s", gotAuth)
	}
	// 签名段为 64 位十六进制
	sig := gotAuth[strings.Index(gotAuth, "Signature=")+10:]
	if len(sig) != 64 {
		t.Errorf("签名长度应为 64 hex，实际 %d", len(sig))
	}
	// 请求体携带 IdCard/Name
	if !strings.Contains(gotBody, `"IdCard":"11010119900307851X"`) || !strings.Contains(gotBody, `"Name":"张三"`) {
		t.Errorf("请求体内容错误: %s", gotBody)
	}
}

func TestTencentResultCodes(t *testing.T) {
	cases := []struct {
		body    string
		matched bool
		isErr   bool
	}{
		{`{"Response":{"Result":"0"}}`, true, false},
		{`{"Response":{"Result":"-1","Description":"不一致"}}`, false, false},
		{`{"Response":{"Result":"-2","Description":"非法身份证号"}}`, false, true},
		{`{"Response":{"Error":{"Code":"AuthFailure","Message":"签名错误"}}}`, false, true},
	}
	for _, c := range cases {
		var resp tcResponse
		res, err := parseTencentResp([]byte(c.body), &resp)
		if c.isErr {
			if err == nil {
				t.Errorf("%s 应返回错误", c.body)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s 不应出错: %v", c.body, err)
			continue
		}
		if res.Matched != c.matched {
			t.Errorf("%s matched 应=%v，实际 %v", c.body, c.matched, res.Matched)
		}
	}
}

// ============ 网易易盾签名 ============

func TestDunSignature(t *testing.T) {
	// 同输入必须可重现；不同算法输出长度符合预期
	got := dunSignature("MD5", "k", map[string]string{"a": "1", "b": "2"})
	if len(got) != 32 {
		t.Errorf("MD5 签名应 32 hex，实际 %q", got)
	}
	if again := dunSignature("MD5", "k", map[string]string{"b": "2", "a": "1"}); again != got {
		t.Errorf("签名不可重现: %s vs %s", got, again)
	}
	// SHA1/SHA256 长度校验
	if s := dunSignature("SHA1", "k", map[string]string{"a": "1"}); len(s) != 40 {
		t.Errorf("SHA1 应 40 hex，实际 %q", s)
	}
	if s := dunSignature("SHA256", "k", map[string]string{"a": "1"}); len(s) != 64 {
		t.Errorf("SHA256 应 64 hex，实际 %q", s)
	}
	// 密钥参与签名：换密钥结果必须不同
	if other := dunSignature("MD5", "k2", map[string]string{"a": "1", "b": "2"}); other == got {
		t.Errorf("更换 secretKey 后签名不应相同")
	}
}

// ============ 宽松响应解析 ============

func TestParseLooseResult(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		matched bool
		wantErr bool
	}{
		{"通用code=0", `{"code":0,"message":"一致"}`, true, false},
		{"通用code=1", `{"code":1,"message":"不一致"}`, false, false},
		{"status字符串一致", `{"status":"match"}`, true, false},
		{"status中文", `{"status":"一致"}`, true, false},
		{"result布尔true", `{"result":true}`, true, false},
		{"阿里data.result=1", `{"code":200,"data":{"result":1}}`, true, false},
		{"阿里data.result=2", `{"code":200,"data":{"result":2}}`, false, false},
		{"网易data.status=1", `{"code":200,"data":{"status":1}}`, true, false},
		{"无法识别", `{"foo":"bar"}`, false, true},
		{"非JSON", `not json`, false, true},
	}
	for _, c := range cases {
		res, err := parseLooseResult([]byte(c.body))
		if c.wantErr {
			if err == nil {
				t.Errorf("%s 应报错", c.name)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s 不应出错: %v", c.name, err)
			continue
		}
		if res.Matched != c.matched {
			t.Errorf("%s matched 应=%v，实际=%v（code=%s）", c.name, c.matched, res.Matched, res.Code)
		}
	}
}

// ============ 华为云 ============

func TestRegionFromEndpoint(t *testing.T) {
	if r := regionFromEndpoint("ivs.cn-north-4.myhuaweicloud.com"); r != "cn-north-4" {
		t.Errorf("region 提取错误: %s", r)
	}
	if r := regionFromEndpoint("ivs.ap-southeast-1.myhuaweicloud.com"); r != "ap-southeast-1" {
		t.Errorf("region 提取错误: %s", r)
	}
}

func TestParseHuaweiResult(t *testing.T) {
	cases := []struct {
		body    string
		matched bool
		wantErr bool
	}{
		{`{"verify_result":"VALID"}`, true, false},
		{`{"verify_result":"INVALID"}`, false, false},
		{`{"result":{"verify_result":"01"}}`, true, false},
		{`{"error_code":"IVS.0001","error_msg":"参数错误"}`, false, true},
		{`{"result":1}`, true, false}, // 退化通用解析
	}
	for _, c := range cases {
		res, err := parseHuaweiResult([]byte(c.body))
		if c.wantErr {
			if err == nil {
				t.Errorf("%s 应报错", c.body)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s 不应出错: %v", c.body, err)
			continue
		}
		if res.Matched != c.matched {
			t.Errorf("%s matched 应=%v，实际=%v", c.body, c.matched, res.Matched)
		}
	}
}

// ============ 注册表与路由 ============

func TestProviderRegistry(t *testing.T) {
	for _, name := range []string{"national", "tencent", "alibaba", "huawei", "netease"} {
		if _, ok := Get(name); !ok {
			t.Errorf("提供商 %s 未注册", name)
		}
	}
	// 未配置
	if _, err := Verify(context.Background(), Config{}, "张三", "11010119900307851X"); err != ErrNotConfigured {
		t.Errorf("空 provider 应返回 ErrNotConfigured，实际 %v", err)
	}
	// 未知提供商
	if _, err := Verify(context.Background(), Config{Provider: "unknown"}, "a", "b"); err == nil {
		t.Errorf("未知 provider 应报错")
	}
}

// ============ 各提供商未配置哨兵 ============

func TestProvidersNotConfigured(t *testing.T) {
	ctx := context.Background()
	if _, err := (nationalProvider{}).Verify(ctx, Config{}, "a", "b"); err != ErrNotConfigured {
		t.Errorf("national 空配置应 ErrNotConfigured")
	}
	if _, err := (tencentProvider{}).Verify(ctx, Config{}, "a", "b"); err != ErrNotConfigured {
		t.Errorf("tencent 空配置应 ErrNotConfigured")
	}
	if _, err := (aliyunProvider{}).Verify(ctx, Config{}, "a", "b"); err != ErrNotConfigured {
		t.Errorf("alibaba 空配置应 ErrNotConfigured")
	}
	if _, err := (huaweiProvider{}).Verify(ctx, Config{}, "a", "b"); err != ErrNotConfigured {
		t.Errorf("huawei 空配置应 ErrNotConfigured")
	}
	if _, err := (neteaseProvider{}).Verify(ctx, Config{}, "a", "b"); err != ErrNotConfigured {
		t.Errorf("netease 空配置应 ErrNotConfigured")
	}
}

// ============ JSON 辅助 ============

func TestNumOfAndFirstHelpers(t *testing.T) {
	var v interface{}
	json.Unmarshal([]byte(`{"a":1,"b":"2","c":true,"d":null,"e":1.5}`), &v)
	m := v.(map[string]interface{})
	if f, ok := numOf(m["a"]); !ok || f != 1 {
		t.Errorf("numOf float64 失败")
	}
	if f, ok := numOf(m["b"]); !ok || f != 2 {
		t.Errorf("numOf 字符串失败")
	}
	if _, ok := numOf(m["c"]); ok {
		t.Errorf("numOf 布尔应失败")
	}
	if f, ok := numOf(m["e"]); !ok || f != 1.5 {
		t.Errorf("numOf 小数失败")
	}
	if s := firstString(m, "x", "b"); s != "2" {
		t.Errorf("firstString 失败: %s", s)
	}
	if _, ok := firstNumber(m, "c", "a"); !ok {
		t.Errorf("firstNumber 失败")
	}
	if got := trimFloat(2); got != "2" {
		t.Errorf("trimFloat(2)=%s", got)
	}
	if got := trimFloat(1.25); got != "1.25" {
		t.Errorf("trimFloat(1.25)=%s", got)
	}
}
