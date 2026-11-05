package aiadmin

import "testing"

// TestNormalizeMonitorInputs 渠道监控的两种高频填错必须被纠正，而不是报「令牌无效」
func TestNormalizeMonitorInputs(t *testing.T) {
	const sk = "sk-EXAMPLE0000000000000000000000000000"
	const nap = "nap_EXAMPLE00000000000000000000000"

	// ① 站长实际填法：地址填成 /api/user/self，用户 ID 填 sk_，模型密钥填 56347
	got := normalizeMonitorInputs("https://api.hcnsec.cn/api/user/self", nap, sk, "56347")
	if got.Base != "https://api.hcnsec.cn" {
		t.Fatalf("地址未归一化：%q", got.Base)
	}
	if got.Token != nap {
		t.Fatalf("令牌应保持 nap_，实际 %q", got.Token)
	}
	if got.UserID != "56347" {
		t.Fatalf("模型密钥里的数字应被当成用户 ID，实际 %q", got.UserID)
	}
	if got.ModelKey != "" {
		t.Fatalf("模型密钥里的纯数字应被清空，实际 %q", got.ModelKey)
	}
	if len(got.Warnings) == 0 {
		t.Fatal("应给出纠正说明，否则站长不知道自己填错了")
	}

	// ② 令牌填了数字、用户 ID 空 → 按用户 ID 处理
	got = normalizeMonitorInputs("https://api.x.com", "56347", "", "")
	if got.UserID != "56347" || got.Token != "" {
		t.Fatalf("token 里的数字应转成用户 ID，实际 token=%q uid=%q", got.Token, got.UserID)
	}

	// ③ 用户 ID 里填了令牌、令牌空 → 搬过去
	got = normalizeMonitorInputs("https://api.x.com", "", sk, "")
	if got.Token != sk || got.UserID != "" {
		t.Fatalf("用户 ID 里的令牌应搬到令牌位，实际 token=%q uid=%q", got.Token, got.UserID)
	}

	// ④ 用户 ID 里填了令牌、令牌也有值 → 忽略用户 ID 那个错值
	got = normalizeMonitorInputs("https://api.x.com", nap, sk, "")
	if got.Token != nap || got.UserID != "" {
		t.Fatalf("应保留正确令牌并忽略错的用户 ID，实际 token=%q uid=%q", got.Token, got.UserID)
	}

	// ⑤ 本来就填对：一个字都不许改
	got = normalizeMonitorInputs("https://api.hcnsec.cn", nap, "56347", sk)
	if got.Base != "https://api.hcnsec.cn" || got.Token != nap || got.UserID != "56347" || got.ModelKey != sk {
		t.Fatalf("正确输入被改动：%+v", got)
	}
	if len(got.Warnings) != 0 {
		t.Fatalf("正确输入不该有警告，实际 %v", got.Warnings)
	}
}
