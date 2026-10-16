package slidecaptcha

import (
	"testing"
	"time"
)

// TestVerifyPassTokenRoundTrip 回归测试：修复前 signPass 用 id|correctX|ts 签名，
// 而 VerifyPassToken 用 id|ts 校验，二者永远不匹配，导致开启滑块后发码/注册被全拒。
// 本测试确保「滑块通过 → 签发 pass token → 校验通过」链路闭环。
func TestVerifyPassTokenRoundTrip(t *testing.T) {
	id := "test-challenge-1"
	correctX := 123
	store.Store(id, &challenge{id: id, correctX: correctX, createdAt: time.Now()})

	token, ok := Verify(id, correctX)
	if !ok || token == "" {
		t.Fatalf("Verify 应通过滑块并签发 token，实际 ok=%v token=%q", ok, token)
	}

	if !VerifyPassToken(token) {
		t.Fatalf("修复后 VerifyPassToken 应通过合法 token，但返回 false（HMAC 不匹配 bug 复现）")
	}

	// 一次性：同一 token 复用应失效
	if VerifyPassToken(token) {
		t.Fatalf("pass token 应一次性使用，复用不应通过")
	}
}

// TestVerifyPassTokenRejectsTampered 非法 / 篡改 token 必须拒绝。
func TestVerifyPassTokenRejectsTampered(t *testing.T) {
	cases := []string{"", "not-a-token", "123.abc.deadbeef", "9999999999.foo.ffffff"}
	for _, tc := range cases {
		if VerifyPassToken(tc) {
			t.Fatalf("非法 token %q 应被拒绝", tc)
		}
	}
}

// TestVerifyRejectsWrongPosition 滑块位置偏差超过容差应失败。
func TestVerifyRejectsWrongPosition(t *testing.T) {
	id := "test-challenge-2"
	correctX := 200
	store.Store(id, &challenge{id: id, correctX: correctX, createdAt: time.Now()})

	if _, ok := Verify(id, correctX+50); ok {
		t.Fatalf("偏差 50px 远超容差，Verify 应失败")
	}
}
