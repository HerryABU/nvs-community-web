// fieldcrypto.go — 敏感字段落库加密（AES-256-GCM）。
// 用于实名姓名等 PII：密文 base64 存储，明文只在需要时解密展示（管理员取证）。
// 密钥来源：config.RealNameEncKey（hex/base64 32 字节），缺省由 JWT_SECRET 派生
// （保证重启不丢失；轮换 JWT_SECRET 前须先显式配置独立密钥）。
package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"nvs-server/config"
)

// fieldCryptoKeyFile 独立字段加密密钥文件（与 JWT_SECRET 物理隔离）。
// ⚠️ 除登录令牌外，任何「需要存储内容的加密」都不得用 JWT 密钥派生——
// JWT 只用于认证，密钥泄露面必须彼此隔离。
const fieldCryptoKeyFile = "data/.field_crypto.key"

var (
	// ErrFieldCryptoNoKey 未配置加密密钥
	ErrFieldCryptoNoKey = errors.New("未配置字段加密密钥")
	// ErrFieldCryptoFormat 密文格式非法
	ErrFieldCryptoFormat = errors.New("密文格式不正确")
)

// fieldCryptoKey 派生/解析 AES-256 密钥（进程级缓存）。
var fieldCryptoKey []byte

func getFieldCryptoKey() ([]byte, error) {
	if fieldCryptoKey != nil {
		return fieldCryptoKey, nil
	}
	raw := strings.TrimSpace(config.RealNameEncKey)
	var key []byte
	if raw != "" {
		// 优先 hex 解码
		if b, err := hex.DecodeString(raw); err == nil && len(b) == 32 {
			key = b
		} else if b, err := base64.StdEncoding.DecodeString(raw); err == nil && len(b) == 32 {
			key = b
		}
	}
	if key == nil {
		// 缺省：独立随机密钥（持久化 data/.field_crypto.key），与 JWT 完全隔离
		if k, err := loadOrCreateFieldKey(); err == nil && len(k) == 32 {
			key = k
		} else {
			// 兼容历史密文：仅在无法生成独立密钥时回退 JWT 派生
			key = jwtDerivedFieldKey()
		}
	}
	fieldCryptoKey = key
	return key, nil
}

// loadOrCreateFieldKey 读取/生成独立字段加密密钥（32 字节 hex，0600 持久化）
func loadOrCreateFieldKey() ([]byte, error) {
	if b, err := os.ReadFile(fieldCryptoKeyFile); err == nil {
		if raw, err := hex.DecodeString(strings.TrimSpace(string(b))); err == nil && len(raw) == 32 {
			return raw, nil
		}
	}
	buf := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return nil, err
	}
	if err := os.MkdirAll("data", 0700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(fieldCryptoKeyFile, []byte(hex.EncodeToString(buf)), 0600); err != nil {
		return nil, err
	}
	return buf, nil
}

// jwtDerivedFieldKey 历史兼容用：JWT 派生密钥（仅用于解密旧密文）
func jwtDerivedFieldKey() []byte {
	sum := sha256.Sum256([]byte("nvs-field-crypto|" + config.JWTSecret))
	return sum[:]
}

// fieldCryptoCandidates 返回可用的加密密钥候选（按优先级）。
//
// 第一位是**令牌加密密码**派生的密钥（见 tokenvault.go）—— 站长自己设的密码，
// 与 JWT 完全无关；未设置时回退到旧密钥（config.RealNameEncKey 或 JWT 派生）。
// 解密时按顺序尝试，因此历史密文不会因为启用保险箱而读不出来。
func fieldCryptoCandidates() [][]byte {
	var keys [][]byte
	seen := map[string]bool{}
	add := func(k []byte) {
		if len(k) != 32 {
			return
		}
		sig := string(k)
		if seen[sig] {
			return
		}
		seen[sig] = true
		keys = append(keys, k)
	}
	// 1) 令牌保险箱密钥（站长自设，最高优先）
	add(loadTokenVaultKey())
	// 2) 独立字段密钥 / 显式 RealNameEncKey
	if k, err := getFieldCryptoKey(); err == nil {
		add(k)
	}
	// 3) 历史兼容：JWT 派生密钥（仅解密旧密文用，新加密不再使用）
	add(jwtDerivedFieldKey())
	return keys
}

// EncryptField 加密明文字段，返回 base64(nonce|ciphertext) （含 GCM tag）。
func EncryptField(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	cands := fieldCryptoCandidates()
	if len(cands) == 0 {
		return "", ErrFieldCryptoNoKey
	}
	key := cands[0]
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("创建加密器失败: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建 GCM 失败: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// DecryptField 解密 EncryptField 产出的密文。
func DecryptField(ciphertext string) (string, error) {
	ciphertext = strings.TrimSpace(ciphertext)
	if ciphertext == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", ErrFieldCryptoFormat
	}
	cands := fieldCryptoCandidates()
	if len(cands) == 0 {
		return "", ErrFieldCryptoNoKey
	}
	var lastErr error
	for _, key := range cands {
		if plain, e := decryptWithKey(key, raw); e == nil {
			return plain, nil
		} else {
			lastErr = e
		}
	}
	return "", lastErr
}

// decryptWithKey 用指定密钥解一段 nonce|ciphertext（GCM）。
func decryptWithKey(key, raw []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("创建解密器失败: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建 GCM 失败: %w", err)
	}
	if len(raw) < gcm.NonceSize() {
		return "", ErrFieldCryptoFormat
	}
	nonce, sealed := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", fmt.Errorf("解密失败（密钥可能已轮换）: %w", err)
	}
	return string(plain), nil
}

// ResetFieldCryptoKeyForTest 仅测试用：清空密钥缓存。
func ResetFieldCryptoKeyForTest() { fieldCryptoKey = nil }

// EncryptFieldWithKey 用**指定密钥**加密（密钥轮换/迁移专用：换令牌加密密码时使用）。
func EncryptFieldWithKey(key []byte, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if len(key) != 32 {
		return "", ErrFieldCryptoNoKey
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("创建加密器失败: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建 GCM 失败: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(plaintext), nil)), nil
}

// DecryptFieldWithKey 用**指定密钥**解密（密钥轮换/迁移专用）。
func DecryptFieldWithKey(key []byte, ciphertext string) (string, error) {
	ciphertext = strings.TrimSpace(ciphertext)
	if ciphertext == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", ErrFieldCryptoFormat
	}
	return decryptWithKey(key, raw)
}
