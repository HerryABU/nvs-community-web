package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ==================== 加密的数据库配置存储 ====================
// 数据库连接信息（尤其是密码）不写入明文 .env（容易被扫描器命中），
// 而是用 AES-256-GCM 加密后写入 data/secure-config.json；
// 加密密钥保存在 data/.secret.key（0600 权限，仅本机可读），首次运行时生成。
//
// 状态锁：secure-config.json 是否存在 = 是否已完成安装（向导永久关闭）。

const (
	secureConfigFile = "data/secure-config.json"
	secretKeyFile    = "data/.secret.key"
)

// SecureDBConfig 持久化的数据库配置（加密存储）
type SecureDBConfig struct {
	Driver   string `json:"driver"` // "sqlite" | "mysql"
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Name     string `json:"name"`
	Path     string `json:"path"` // sqlite 文件路径
}

// SecureConfigExists 是否已存在加密配置（= 是否已完成安装）
func SecureConfigExists() bool {
	_, err := os.Stat(secureConfigFile)
	return err == nil
}

// GetOrCreateSecretKey 读取或生成 AES-256 机器密钥（32 字节，hex 存储）
func GetOrCreateSecretKey() ([]byte, error) {
	if b, err := os.ReadFile(secretKeyFile); err == nil {
		if key, err := hex.DecodeString(string(b)); err == nil && len(key) == 32 {
			return key, nil
		}
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll("data", 0755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(secretKeyFile, []byte(hex.EncodeToString(key)), 0600); err != nil {
		return nil, err
	}
	return key, nil
}

func encryptAES(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func decryptAES(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(ciphertext) < ns {
		return nil, fmt.Errorf("密文长度非法")
	}
	return gcm.Open(nil, ciphertext[:ns], ciphertext[ns:], nil)
}

// WriteSecureConfig 加密写入数据库配置
func WriteSecureConfig(sc SecureDBConfig) error {
	key, err := GetOrCreateSecretKey()
	if err != nil {
		return err
	}
	data, err := json.Marshal(sc)
	if err != nil {
		return err
	}
	enc, err := encryptAES(key, data)
	if err != nil {
		return err
	}
	if err := os.MkdirAll("data", 0755); err != nil {
		return err
	}
	return os.WriteFile(secureConfigFile, enc, 0600)
}

// LoadSecureConfig 读取并解密数据库配置
func LoadSecureConfig() (*SecureDBConfig, error) {
	enc, err := os.ReadFile(secureConfigFile)
	if err != nil {
		return nil, err
	}
	key, err := GetOrCreateSecretKey()
	if err != nil {
		return nil, err
	}
	data, err := decryptAES(key, enc)
	if err != nil {
		return nil, err
	}
	var sc SecureDBConfig
	if err := json.Unmarshal(data, &sc); err != nil {
		return nil, err
	}
	return &sc, nil
}

// ApplySecureConfig 将加密配置覆盖到内存中的配置变量（运行时生效）
func ApplySecureConfig(sc *SecureDBConfig) {
	if sc == nil {
		return
	}
	DBDriver = sc.Driver
	DBHost = sc.Host
	DBPort = sc.Port
	DBUser = sc.User
	DBPassword = sc.Password
	DBName = sc.Name
	DBPath = sc.Path
}

// PersistSecureConfig 写入加密配置并立即应用到内存变量
func PersistSecureConfig(sc SecureDBConfig) error {
	if err := WriteSecureConfig(sc); err != nil {
		return err
	}
	ApplySecureConfig(&sc)
	StripEnvDBCredentials()
	return nil
}

// StripEnvDBCredentials 从 .env 移除所有 DB_* 数据库凭据行（已迁移到加密配置，避免明文残留被扫描）
func StripEnvDBCredentials() {
	envPath := ".env"
	raw, err := os.ReadFile(envPath)
	if err != nil {
		return
	}
	lines := strings.Split(string(raw), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			out = append(out, line)
			continue
		}
		idx := strings.Index(line, "=")
		if idx == -1 {
			out = append(out, line)
			continue
		}
		key := strings.TrimSpace(line[:idx])
		if strings.HasPrefix(key, "DB_") {
			continue // 移除所有数据库凭据
		}
		out = append(out, line)
	}
	_ = os.WriteFile(envPath, []byte(strings.Join(out, "\n")), 0644)
}
