// tokenvault.go —— 令牌（上游 API Key）加密密码：独立于 JWT 的第二把锁。
//
// 背景与红线：
//   - 上游 API Key（以及实名信息等敏感字段）落库必须加密；
//   - **严禁**用 JWT_SECRET 派生加密密钥 —— 轮换 JWT 就把所有密钥废掉，
//     而且 JWT 密钥本来是为会话签名用的，不该兼任数据加密；
//   - 密钥必须来自**站长自己设置的密码**，忘记密码时用命令行
//     `nvs-server --pwd <新密码> --scope token` 重设。
//
// 设计取舍（单文件部署产品的现实约束）：
//
//	站长的密码经 PBKDF2-HMAC-SHA256（随机 salt，20 万轮）派生出 32 字节 AES 密钥，
//	派生结果存到 data/token-vault.key（0600）。这样：
//	  · 密码只在「设置/更换」时需要，重启服务不需要人工输入（否则无人值守重启会挂）；
//	  · 更改密码会**自动用旧密钥解密、再用新密钥重新加密**现有密文，不会丢数据；
//	  · 密钥文件与 JWT 密钥、蜜罐密钥三者互不复用。
//
// 若从未设置过该密码，行为保持向后兼容：仍走旧的字段加密密钥（见 fieldcrypto.go），
// 解密时会自动尝试两把钥匙，老数据不会读不出来。
package secure

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	tokenVaultFileName = "token-vault.key"
	tokenVaultIters    = 200000
)

var (
	tokenVaultOnce sync.Once
	tokenVaultKey  []byte
	// ErrTokenVaultMissing 未设置令牌加密密码
	ErrTokenVaultMissing = errors.New("尚未设置令牌加密密码，请运行 nvs-server --pwd <密码> --scope token")
)

// TokenVaultConfigured 是否已设置令牌加密密码
func TokenVaultConfigured() bool {
	return len(loadTokenVaultKey()) == 32
}

// loadTokenVaultKey 读取已派生的密钥（进程内缓存）。
func loadTokenVaultKey() []byte {
	tokenVaultOnce.Do(func() {
		path := filepath.Join("data", tokenVaultFileName)
		raw, err := os.ReadFile(path)
		if err != nil {
			return
		}
		b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
		if err == nil && len(b) == 32 {
			tokenVaultKey = b
		}
	})
	return tokenVaultKey
}

// SetTokenVaultPassphrase 用站长密码设置/更换令牌加密密钥。
// 返回重新加密的字段数量，便于命令行提示「已迁移 N 条」。
//
// reencrypt 由调用方注入（避免 utils 反向依赖 models）：
// 传入 (oldKey, newKey) 让它把现有密文迁移到新密钥；首次设置时 oldKey 为 nil。
func SetTokenVaultPassphrase(password string, reencrypt func(oldKey, newKey []byte) (int, error)) (int, error) {
	password = strings.TrimSpace(password)
	if len(password) < 8 {
		return 0, errors.New("令牌加密密码至少 8 位")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return 0, fmt.Errorf("生成盐失败: %w", err)
	}
	newKey := pbkdf2SHA256([]byte(password), salt, tokenVaultIters, 32)

	oldKey := loadTokenVaultKey()
	moved := 0
	if reencrypt != nil && len(oldKey) == 32 {
		n, err := reencrypt(oldKey, newKey)
		if err != nil {
			return 0, fmt.Errorf("迁移既有密文失败（未改动任何数据）: %w", err)
		}
		moved = n
	}

	// 落盘：salt|key 一起存（salt 用于将来再次由密码重算校验）
	blob := append(append([]byte{}, salt...), newKey...)
	if err := os.MkdirAll("data", 0o700); err != nil {
		return 0, err
	}
	path := filepath.Join("data", tokenVaultFileName)
	if err := os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(blob)), 0o600); err != nil {
		return 0, err
	}
	// 热替换缓存
	tokenVaultKey = newKey
	tokenVaultOnce = sync.Once{}
	tokenVaultOnce.Do(func() {})
	return moved, nil
}

// pbkdf2SHA256 标准库实现 PBKDF2-HMAC-SHA256（避免为一把钥匙引入新依赖）。
func pbkdf2SHA256(password, salt []byte, iter, keyLen int) []byte {
	hLen := sha256.Size
	blocks := (keyLen + hLen - 1) / hLen
	out := make([]byte, 0, blocks*hLen)
	for block := 1; block <= blocks; block++ {
		mac := hmac.New(sha256.New, password)
		mac.Write(salt)
		mac.Write([]byte{byte(block >> 24), byte(block >> 16), byte(block >> 8), byte(block)})
		u := mac.Sum(nil)
		t := append([]byte{}, u...)
		for i := 1; i < iter; i++ {
			mac = hmac.New(sha256.New, password)
			mac.Write(u)
			u = mac.Sum(nil)
			for j := range t {
				t[j] ^= u[j]
			}
		}
		out = append(out, t...)
	}
	return out[:keyLen]
}
