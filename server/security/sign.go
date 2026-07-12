package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// GenerateSigningKey 生成32字节 AES-256 签名密钥，返回 base64 编码
func GenerateSigningKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

// SignContent 用签名密钥对内容进行 HMAC-SHA256 签名
func SignContent(content string, signingKeyBase64 string) string {
	if signingKeyBase64 == "" {
		return ""
	}
	key, err := base64.StdEncoding.DecodeString(signingKeyBase64)
	if err != nil {
		return ""
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(content))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature 验证内容签名是否匹配
func VerifySignature(content string, signature string, signingKeyBase64 string) bool {
	if signingKeyBase64 == "" || signature == "" {
		return false
	}
	expected := SignContent(content, signingKeyBase64)
	return hmac.Equal([]byte(expected), []byte(signature))
}
