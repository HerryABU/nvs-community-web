package middleware

import "testing"

// 白名单必须「只放该放的」：任何一条漏网都等于给警方开了 admin 全量。
func TestPoliceGovernanceWhitelist(t *testing.T) {
	allowed := []struct{ method, path string }{
		{"GET", "/api/admin/moderation/records"},
		{"GET", "/api/admin/moderation/stats"},
		{"POST", "/api/admin/moderation/records/12/decide"},
		{"GET", "/api/admin/reports"},
		{"POST", "/api/admin/reports/3/handle"},
		{"GET", "/api/admin/novels"},
		{"GET", "/api/admin/novels/7/chapters"},
		{"POST", "/api/admin/softdel"},
		{"POST", "/api/admin/softdel/restore"},
		{"GET", "/api/admin/hotwords"},
	}
	for _, c := range allowed {
		if !policeAllowed(c.method, c.path) {
			t.Fatalf("应放行却被拒: %s %s", c.method, c.path)
		}
	}

	denied := []struct{ method, path string }{
		// 合规与安全：警方一律不可见
		{"GET", "/api/admin/compliance"},
		{"GET", "/api/admin/compliance/bundle"},
		{"GET", "/api/admin/realname/records"},
		{"GET", "/api/admin/redact/config"},
		{"GET", "/api/admin/audit-logs"},
		// 密钥 / 支付 / 角色提权
		{"GET", "/api/admin/apikeys"},
		{"POST", "/api/admin/users/1/promote-police"},
		{"GET", "/api/admin/users"},
		{"GET", "/api/admin/payment/orders"},
		// AI 配置与舆情配置（站长决策，不给警方改）
		{"PUT", "/api/admin/sentiment/config"},
		{"GET", "/api/admin/ai/models"},
		{"PUT", "/api/admin/ai/safety/config"},
		// 方法不匹配：同一路径 GET 放行，DELETE 不该放行
		{"DELETE", "/api/admin/reports/3/handle"},
		// 前缀混淆应被拒（正则锚定，不接受子路径拼接）
		{"GET", "/api/admin/moderation/records/export"},
		{"GET", "/api/admin/novels/7/chapters/1"},
	}
	for _, c := range denied {
		if policeAllowed(c.method, c.path) {
			t.Fatalf("不该放行却放行了: %s %s", c.method, c.path)
		}
	}
}

// 尾部斜杠不能成为绕过手段。
func TestPoliceGovernanceTrailingSlash(t *testing.T) {
	if !policeAllowed("GET", "/api/admin/reports/") {
		t.Fatalf("尾部斜杠应归一化后放行")
	}
	// 双重斜杠不做归一化 → 应当被拒（宁可 403，也不放宽匹配）
	if policeAllowed("GET", "/api/admin//reports") {
		t.Fatalf("双斜杠不应被放行")
	}
}
