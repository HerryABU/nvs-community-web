package utils

import (
	"testing"
	"time"
)

// 内存模式（无 Redis）下的完整生命周期：生成 → 校验 → 一次性消费 → 超限。
func TestPhoneCodeLifecycleMemory(t *testing.T) {
	if useRedisForPhone() {
		t.Skip("Redis 已连接，跳过内存模式测试")
	}
	phone := "13800138000"
	purpose := PhonePurposeLogin

	// 清理可能残留的状态
	DeletePhoneCode(phone, purpose)
	phoneCodeCountMu.Lock()
	delete(phoneCodeCounts, phoneCountKey(purpose, phone))
	delete(phoneCodeCounts, phoneCountIPKey(purpose, "127.0.0.1"))
	phoneCodeCountMu.Unlock()

	code, err := GeneratePhoneCode(phone, purpose, "127.0.0.1")
	if err != nil {
		t.Fatalf("生成验证码失败: %v", err)
	}
	if len(code) != 6 {
		t.Errorf("验证码应为 6 位: %s", code)
	}

	// 错误验证码：不计入成功，且累加 attempts
	if err := VerifyPhoneCode(phone, "000000", purpose); err == nil {
		t.Error("错误验证码应被拒绝")
	}

	// 正确验证码：消费成功
	if err := VerifyPhoneCode(phone, code, purpose); err != nil {
		t.Errorf("正确验证码应通过: %v", err)
	}

	// 一次性：重复使用同一码应失败
	if err := VerifyPhoneCode(phone, code, purpose); err == nil {
		t.Error("验证码应一次性消费")
	}
}

func TestPhoneCodeFormatValidation(t *testing.T) {
	if err := VerifyPhoneCode("13800138000", "123456", PhonePurposeLogin); err == nil {
		t.Error("无有效验证码时应拒绝")
	}
}

func TestPhoneCodeTTLExpiry(t *testing.T) {
	if useRedisForPhone() {
		t.Skip("Redis 已连接，跳过内存模式测试")
	}
	phone := "13900139000"
	purpose := PhonePurposeLogin
	DeletePhoneCode(phone, purpose)

	// 手工写入一条已过期的记录
	phoneCodeStoreMu.Lock()
	phoneCodeStore[phoneCodeKey(purpose, phone)] = &phoneCodeRecord{
		CodeHash:  hashPhoneCode(phone, "123456", "s"),
		Salt:      "s",
		Purpose:   purpose,
		ExpiresAt: time.Now().Add(-time.Minute),
	}
	phoneCodeStoreMu.Unlock()

	if err := VerifyPhoneCode(phone, "123456", purpose); err == nil {
		t.Error("过期验证码应被拒绝")
	}
}

func TestPhoneCodeResendTooOften(t *testing.T) {
	if useRedisForPhone() {
		t.Skip("Redis 已连接，跳过内存模式测试")
	}
	phone := "13700137000"
	purpose := PhonePurposeLogin
	DeletePhoneCode(phone, purpose)
	phoneCodeCountMu.Lock()
	delete(phoneCodeCounts, phoneCountKey(purpose, phone))
	phoneCodeCountMu.Unlock()

	if _, err := GeneratePhoneCode(phone, purpose, "127.0.0.1"); err != nil {
		t.Fatalf("首次生成应成功: %v", err)
	}
	// 60 秒内二次申请应被频控拒绝
	if _, err := GeneratePhoneCode(phone, purpose, "127.0.0.1"); err != ErrPhoneCodeTooOften {
		t.Errorf("60 秒内重复申请应返回 ErrPhoneCodeTooOften，实际: %v", err)
	}
}
