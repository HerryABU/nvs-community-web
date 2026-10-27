package honeypot

// ==================== 蜜罐 JWT（影子认证的密钥层） ====================
// 「先甜」的关键：诱饵 .env 里泄露的 JWT_SECRET 必须**真的能签出蜜罐认的 token**。
// 因此蜜罐使用**独立密钥**（data/honeypot/secret.key，0600，首次启动生成）：
//   - 与真实 config.JWTSecret 物理隔离，伪造 token 在主站一律无效；
//   - 诱饵 .env 暴露的就是这把蜜罐密钥，攻击者用它签出的 token 在蜜罐内验签通过 → 以为拿到了后台。
//
// 铁律：蜜罐 token 只在蜜罐路由内被接受，且没有任何「换出真实 session」的路径。

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"nvs-server/config"

	"github.com/golang-jwt/jwt/v5"
)

var (
	secretMu   sync.RWMutex
	shadowKey  []byte
	secretOnce sync.Once
)

// secretPath 蜜罐密钥落盘位置
func secretPath() string {
	base := filepath.Dir(config.DBPath)
	if base == "" || base == "." {
		base = "data"
	}
	return filepath.Join(base, "honeypot", "secret.key")
}

// loadShadowKey 读取或生成蜜罐密钥（0644→0600）
func loadShadowKey() ([]byte, error) {
	secretMu.RLock()
	if len(shadowKey) > 0 {
		k := shadowKey
		secretMu.RUnlock()
		return k, nil
	}
	secretMu.RUnlock()

	var out []byte
	var loadErr error
	secretOnce.Do(func() {
		p := secretPath()
		if b, err := os.ReadFile(p); err == nil && len(b) >= 32 {
			out = b
			return
		}
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			loadErr = err
			return
		}
		buf := make([]byte, 32)
		if _, err := rand.Read(buf); err != nil {
			loadErr = err
			return
		}
		if err := os.WriteFile(p, buf, 0600); err != nil {
			loadErr = err
			return
		}
		out = buf
	})
	if loadErr != nil {
		return nil, loadErr
	}
	secretMu.Lock()
	shadowKey = out
	secretMu.Unlock()
	return out, nil
}

// ShadowSecretB64 诱饵 .env 里要暴露的密钥（base64），攻击者据此伪造 token
func ShadowSecretB64() string {
	k, err := loadShadowKey()
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(k)
}

// ShadowClaims 影子 token 载荷（攻击者常改的就是 sub/role/is_admin）
type ShadowClaims struct {
	UserID  uint   `json:"user_id"`
	Email   string `json:"email"`
	Role    string `json:"role"`
	IsAdmin bool   `json:"is_admin"`
	HID     string `json:"hid"` // 蜜罐会话标识：把伪造 token 绑到会话上
	jwt.RegisteredClaims
}

// SignShadow 用于测试/内部造 token（生产上攻击者用泄露密钥自己签）
func SignShadow(c ShadowClaims) (string, error) {
	k, err := loadShadowKey()
	if err != nil {
		return "", err
	}
	if c.ExpiresAt == nil {
		c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(2 * time.Hour))
	}
	c.Issuer = "nvs-server"
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(k)
}

// VerifyShadow 验签（接受 HS256；alg=none 一律拒绝——这正是我们要记录的攻击特征）
func VerifyShadow(tokenStr string) (*ShadowClaims, error) {
	tokenStr = strings.TrimSpace(strings.TrimPrefix(tokenStr, "Bearer "))
	if tokenStr == "" {
		return nil, errors.New("缺少令牌")
	}
	k, err := loadShadowKey()
	if err != nil {
		return nil, err
	}
	claims := &ShadowClaims{}
	tok, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("非预期签名算法：%v", t.Header["alg"])
		}
		return k, nil
	})
	if err != nil {
		return nil, err
	}
	if !tok.Valid {
		return nil, errors.New("令牌无效")
	}
	return claims, nil
}

// TokenFromRequest 从 Authorization / query / cookie 里取 token
// （query 携带是攻击者常用手法，也顺带说明真实系统为何不该这么做）
func TokenFromRequest(c interface {
	GetHeader(string) string
	Query(string) string
}) string {
	if h := c.GetHeader("Authorization"); h != "" {
		return h
	}
	if q := c.Query("token"); q != "" {
		return q
	}
	return ""
}

// ==================== 蜜罐密钥：修改 / 重新生成 ====================
// 蜜罐密钥与真实 config.JWTSecret 物理隔离（data/honeypot/secret.key）。
// 站长可在后台「重新自动生成」（随机新密钥）或「手动修改」（自带密钥）；
// 重新生成后，旧诱饵 .env 暴露的值即失效，已签发的影子 token 全部作废。

// writeShadowKey 写入新密钥并热替换内存值（不触碰 secretOnce 之外的状态）
func writeShadowKey(raw []byte) (string, error) {
	if len(raw) < 32 {
		return "", errors.New("密钥过短（须 ≥ 32 字节）")
	}
	p := secretPath()
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return "", err
	}
	if err := os.WriteFile(p, raw, 0600); err != nil {
		return "", err
	}
	secretMu.Lock()
	shadowKey = raw
	secretMu.Unlock()
	return base64.StdEncoding.EncodeToString(raw), nil
}

// RegenerateShadowKey 生成新的随机蜜罐密钥（32 字节），落盘并热替换
func RegenerateShadowKey() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return writeShadowKey(buf)
}

// ModifyShadowKey 由站长手动指定蜜罐密钥（base64；解码后须 ≥ 32 字节）
func ModifyShadowKey(b64 string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		return "", errors.New("密钥不是合法 base64")
	}
	return writeShadowKey(raw)
}
