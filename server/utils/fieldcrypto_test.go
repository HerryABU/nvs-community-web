package utils

import (
	"strings"
	"testing"
)

// 字段加密 round-trip + 错误密钥拒绝 + 空串直通
func TestFieldCryptoRoundTrip(t *testing.T) {
	ResetFieldCryptoKeyForTest()
	plain := "张三"
	enc, err := EncryptField(plain)
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}
	if enc == plain || strings.Contains(enc, plain) {
		t.Error("密文不应包含明文")
	}
	dec, err := DecryptField(enc)
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}
	if dec != plain {
		t.Errorf("解密结果不匹配: %s", dec)
	}

	// 空串直通（不加密）
	if e, _ := EncryptField(""); e != "" {
		t.Error("空串应直通")
	}
	if d, _ := DecryptField(""); d != "" {
		t.Error("空串应直通")
	}

	// 非法密文应报错
	if _, err := DecryptField("not-base64!!!"); err == nil {
		t.Error("非法密文应报错")
	}
}

// 不同明文应产生不同密文（nonce 随机性）
func TestFieldCryptoNonceRandom(t *testing.T) {
	ResetFieldCryptoKeyForTest()
	a, _ := EncryptField("李四")
	b, _ := EncryptField("李四")
	if a == b {
		t.Error("同一明文两次加密应产生不同密文（nonce 随机）")
	}
}
