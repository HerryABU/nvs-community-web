package honeypot

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func mustRequest(t *testing.T) *http.Request {
	t.Helper()
	return httptest.NewRequest("GET", "/api/v1/query?sql=1", nil)
}

// TestDetectSQLi 注入特征识别（正例/反例都要有，避免「正常长文本也判成注入」）
func TestDetectSQLi(t *testing.T) {
	positives := []string{
		"id=1 UNION SELECT username,password FROM nvs_users",
		"id=1' AND SLEEP(5)--",
		"id=1; DROP TABLE users",
		"q=1' or '1'='1",
		"id=1 AND extractvalue(1,concat(0x7e,version()))",
		"file=../../etc/passwd' UNION/**/SELECT load_file('/etc/passwd')",
		"id=1' AND (SELECT 1 FROM information_schema.tables)",
	}
	for _, p := range positives {
		if hits := DetectSQLi(p); len(hits) == 0 {
			t.Fatalf("应识别为注入: %s", p)
		}
	}
	negatives := []string{
		"keyword=小说",
		"page=2&size=20",
		"title=The Union of Two Cities",
		"",
	}
	for _, n := range negatives {
		if hits := DetectSQLi(n); len(hits) > 0 {
			t.Fatalf("不应识别为注入: %q -> %v", n, hits)
		}
	}
}

// TestSQLiRiskScoring 风险分：无命中=40 探路；命中越高分越高；多特征叠加加分
func TestSQLiRiskScoring(t *testing.T) {
	if got := SQLiRisk(nil); got != 40 {
		t.Fatalf("无命中应为探路分 40，实际 %d", got)
	}
	one := SQLiRisk(DetectSQLi("id=1 or 1=1"))
	multi := SQLiRisk(DetectSQLi("id=1 UNION SELECT 1 FROM information_schema.tables--"))
	if multi < one {
		t.Fatalf("多特征风险应不低于单特征：one=%d multi=%d", one, multi)
	}
	if multi > 100 {
		t.Fatalf("风险分不应超过 100：%d", multi)
	}
}

// TestFakeResultShape 预生成响应库：金丝雀必须在，且不含真实数据
func TestFakeResultShape(t *testing.T) {
	if !strings.HasPrefix(sqliCanary, "NVS-CANARY-SQLI-") {
		t.Fatalf("金丝雀格式不对: %s", sqliCanary)
	}
	users := fakeUsers(6)
	if len(users) == 0 {
		t.Fatal("假用户库不应为空")
	}
	for _, u := range users {
		if !strings.HasSuffix(u.Email, "@example.internal") {
			t.Fatalf("假邮箱应使用 example.internal 域: %s", u.Email)
		}
	}
	// 固定种子：两次生成一致（更像同一个「库」）
	a, b := fakeUsers(6), fakeUsers(6)
	for i := range a {
		if a[i].Hash != b[i].Hash {
			t.Fatal("假数据应可复现（同一部署内一致）")
		}
	}
}

// TestCaptureTLSProxyHeader 反代透传时采集 JA3/JA4；缺失时标 missing 而不是伪造
func TestCaptureTLSProxyHeader(t *testing.T) {
	req := mustRequest(t)
	req.Header.Set("X-JA3-Hash", "cd08e31494f9531f560d64c695473da9")
	info := CaptureTLS(req)
	if info.Source != "proxy_header" || info.JA3 == "" || !info.ViaProxy {
		t.Fatalf("应采信反代头: %+v", info)
	}

	plain := mustRequest(t)
	info2 := CaptureTLS(plain)
	if info2.Source != "missing" || info2.JA3 != "" {
		t.Fatalf("缺失时必须标 missing 且不伪造: %+v", info2)
	}
}

func TestTLSStatsDiagnosis(t *testing.T) {
	if got := diagnoseJA3(0, 0); !strings.Contains(got, "还没有指纹") {
		t.Fatalf("空数据诊断不对: %s", got)
	}
	if got := diagnoseJA3(10, 0); !strings.Contains(got, "未透传") {
		t.Fatalf("零采集诊断不对: %s", got)
	}
	if got := diagnoseJA3(10, 10); !strings.Contains(got, "正常") {
		t.Fatalf("满采集诊断不对: %s", got)
	}
}
