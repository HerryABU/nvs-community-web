// email.go 保留为 package utils 的占位文件。
// SMTP 发送与配置 → email_smtp.go
// 邮件 HTML 模板     → email_template.go
// 本文件保留验证码存储/校验逻辑。
package utils

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var redisCtx = context.Background()

// ============================================================
// 验证码存储与校验
// ============================================================

const (
	// verificationCodeTTL 验证码有效期（10 分钟）
	verificationCodeTTL = 10 * time.Minute
	// verificationResendInterval 同一邮箱两次发送的最小间隔（防滥用）
	verificationResendInterval = 60 * time.Second
	// verificationMaxAttempts 单条验证码最大尝试次数（防爆破）
	verificationMaxAttempts = 5

	// redisVerifyPrefix 验证码在 Redis 中的 key 前缀（仅 MySQL 模式启用 Redis 时使用）
	redisVerifyPrefix = "nvs:verify:"
)

// VerificationCode 邮箱验证码记录
type VerificationCode struct {
	Code       string
	ExpiresAt  time.Time
	LastSentAt time.Time // 上次发送时间，用于发送频率限制
	Attempts   int       // 已尝试次数，用于防爆破
}

var (
	codeStore   = make(map[string]*VerificationCode)
	codeStoreMu sync.RWMutex
)

// useRedis 是否使用 Redis 存储验证码（MySQL 模式且 Redis 已就绪时启用）。
// SQLite 模式下 RedisClient 为 nil，自动回退到内存临时存储。
func useRedis() bool {
	return RedisClient != nil
}

// loadCode 读取验证码（Redis 或内存）。
func loadCode(email string) (*VerificationCode, bool) {
	if useRedis() {
		raw, err := RedisClient.Get(redisCtx, redisVerifyPrefix+email).Result()
		if err == redis.Nil {
			return nil, false
		}
		if err != nil {
			// Redis 异常时回退内存，避免阻断业务流程
			return nil, false
		}
		var vc VerificationCode
		if err := json.Unmarshal([]byte(raw), &vc); err != nil {
			return nil, false
		}
		// 双保险：若已过期直接视为不存在
		if time.Now().After(vc.ExpiresAt) {
			_ = RedisClient.Del(redisCtx, redisVerifyPrefix+email).Err()
			return nil, false
		}
		return &vc, true
	}
	codeStoreMu.RLock()
	vc, ok := codeStore[email]
	codeStoreMu.RUnlock()
	return vc, ok
}

// saveCode 写入验证码（Redis 或内存）。写入时按剩余有效期设置 TTL，保证 Redis 与原结构体一致。
func saveCode(email string, vc *VerificationCode) error {
	if useRedis() {
		ttl := time.Until(vc.ExpiresAt)
		if ttl <= 0 {
			_ = RedisClient.Del(redisCtx, redisVerifyPrefix+email).Err()
			return nil
		}
		b, err := json.Marshal(vc)
		if err != nil {
			return err
		}
		return RedisClient.Set(redisCtx, redisVerifyPrefix+email, string(b), ttl).Err()
	}
	codeStoreMu.Lock()
	codeStore[email] = vc
	codeStoreMu.Unlock()
	return nil
}

// delCode 删除验证码（Redis 或内存）。
func delCode(email string) {
	if useRedis() {
		_ = RedisClient.Del(redisCtx, redisVerifyPrefix+email).Err()
		return
	}
	codeStoreMu.Lock()
	delete(codeStore, email)
	codeStoreMu.Unlock()
}

// GenerateVerificationCode 生成6位数字验证码
func GenerateVerificationCode() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	return fmt.Sprintf("%06d", n.Int64())
}

// StoreVerificationCode 存储验证码（10分钟有效）。
// 同一邮箱在 verificationResendInterval 内重复发送会返回错误，防止验证码被频繁覆盖与邮件轰炸。
func StoreVerificationCode(email, code string) error {
	vc, exists := loadCode(email)
	if exists && time.Since(vc.LastSentAt) < verificationResendInterval {
		remain := verificationResendInterval - time.Since(vc.LastSentAt)
		return fmt.Errorf("发送过于频繁，请 %d 秒后重试", int(remain.Seconds())+1)
	}
	return saveCode(email, &VerificationCode{
		Code:       code,
		ExpiresAt:  time.Now().Add(verificationCodeTTL),
		LastSentAt: time.Now(),
		Attempts:   0,
	})
}

// VerifyCode 验证验证码。
// 使用写锁（RLock 下执行 delete 会造成并发数据竞争）；过期或尝试超限会删除记录以释放内存。
func VerifyCode(email, code string) bool {
	vc, ok := loadCode(email)
	if !ok {
		return false
	}
	if time.Now().After(vc.ExpiresAt) {
		delCode(email)
		return false
	}
	if vc.Attempts >= verificationMaxAttempts {
		delCode(email)
		return false
	}
	if vc.Code != code {
		vc.Attempts++
		_ = saveCode(email, vc)
		return false
	}
	// 验证成功：删除记录，保证一次性使用语义（调用方无需再显式删除）
	delCode(email)
	return true
}

// DeleteVerificationCode 删除验证码
func DeleteVerificationCode(email string) {
	delCode(email)
}
