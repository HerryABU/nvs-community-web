// phonecode.go — 手机验证码存储与校验（Redis 模式 + 内存模式双支持）。
// 与 email.go 的邮箱验证码同构：Redis 就绪时（MySQL 模式）落 Redis（SETEX TTL），
// 否则回退进程内内存存储（SQLite 单机模式）。验证码一律哈希存储，绝不保存明文。
package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// PhoneCodeTTL 手机验证码有效期（5 分钟，短于邮箱的 10 分钟）
	PhoneCodeTTL = 5 * time.Minute
	// PhoneCodeResendInterval 同一手机号+用途两次发送最小间隔（防轰炸/覆盖）
	PhoneCodeResendInterval = 60 * time.Second
	// PhoneCodeMaxAttempts 单条验证码最大尝试次数（防在线爆破）
	PhoneCodeMaxAttempts = 5
	// PhoneCodeMaxPerDay 同一手机号+用途每日申请上限
	PhoneCodeMaxPerDay = 10
	// PhoneCodeMaxPerIPPerDay 同一 IP 每日申请上限（防换号轰炸）
	PhoneCodeMaxPerIPPerDay = 30

	redisPhoneCodePrefix = "nvs:phonecode:"   // <purpose>:<phone> → 验证码记录 JSON
	redisPhoneCntPrefix   = "nvs:phonecount:" // <purpose>:<phone> / ip:<purpose>:<ip> → 当日计数
)

// 手机验证码用途
const (
	PhonePurposeLogin  = "login"  // 登录/注册
	PhonePurposeReset  = "reset"  // 找回密码
	PhonePurposeVerify = "verify" // 绑定/换绑验证
)

var (
	// ErrPhoneCodeNotFound 无有效验证码（不存在/已过期/已消费/尝试超限）
	ErrPhoneCodeNotFound = errors.New("验证码不存在或已过期")
	// ErrPhoneCodeMismatch 验证码不匹配
	ErrPhoneCodeMismatch = errors.New("验证码错误")
	// ErrPhoneCodeTooManyAttempts 尝试次数超限
	ErrPhoneCodeTooManyAttempts = errors.New("验证码错误次数过多，请重新获取")
	// ErrPhoneCodeTooOften 发送过于频繁
	ErrPhoneCodeTooOften = errors.New("发送过于频繁，请稍后再试")
	// ErrPhoneCodeQuota 当日申请次数超限
	ErrPhoneCodeQuota = errors.New("当日验证码申请次数已达上限，请明天再试")
)

// phoneCodeRecord 手机验证码记录（Redis JSON / 内存结构共用）
type phoneCodeRecord struct {
	CodeHash   string    `json:"code_hash"`   // sha256(phone|code|salt)
	Salt       string    `json:"salt"`        // 每码随机盐
	Purpose    string    `json:"purpose"`     // login/reset/verify
	Attempts   int       `json:"attempts"`    // 已尝试次数
	Consumed   bool      `json:"consumed"`    // 一次性消费标记
	ExpiresAt  time.Time `json:"expires_at"`  // 过期时间
	LastSentAt time.Time `json:"last_sent_at"` // 上次发送时间（60s 频控）
}

// 内存模式存储（SQLite 单机）
var (
	phoneCodeStore   = make(map[string]*phoneCodeRecord)
	phoneCodeStoreMu sync.RWMutex
	phoneCodeCounts  = make(map[string][]time.Time) // 频控计数：key → 申请时间列表
	phoneCodeCountMu sync.Mutex
)

func useRedisForPhone() bool { return RedisClient != nil }

func phoneCodeKey(purpose, phone string) string {
	return redisPhoneCodePrefix + purpose + ":" + phone
}

func phoneCountKey(purpose, phone string) string {
	return redisPhoneCntPrefix + purpose + ":" + phone
}

func phoneCountIPKey(purpose, ip string) string {
	return redisPhoneCntPrefix + "ip:" + purpose + ":" + ip
}

func hashPhoneCode(phone, code, salt string) string {
	sum := sha256.Sum256([]byte(phone + "|" + code + "|" + salt))
	return hex.EncodeToString(sum[:])
}

func generatePhoneCode() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	}
	return fmt.Sprintf("%06d", n.Int64())
}

func generatePhoneSalt() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

// GeneratePhoneCode 生成并存储手机验证码，返回明文码（由调用方经短信网关发送）。
// 60 秒内重复申请返回 ErrPhoneCodeTooOften；当日超限返回 ErrPhoneCodeQuota。
func GeneratePhoneCode(phone, purpose, ip string) (string, error) {
	phone = strings.TrimSpace(phone)
	purpose = strings.TrimSpace(purpose)
	if purpose == "" {
		purpose = PhonePurposeLogin
	}

	// 当日配额频控（手机号 + IP 双维度）
	dayAgo := time.Now().Add(-24 * time.Hour)
	nPhone, err := phoneCodeCount(purpose, phone, dayAgo)
	if err != nil {
		return "", err
	}
	if nPhone >= PhoneCodeMaxPerDay {
		return "", ErrPhoneCodeQuota
	}
	if ip != "" {
		nIP, err := phoneCodeCountIP(purpose, ip, dayAgo)
		if err != nil {
			return "", err
		}
		if nIP >= PhoneCodeMaxPerIPPerDay {
			return "", ErrPhoneCodeQuota
		}
	}

	// 60 秒发送频控：已存在未过期记录且距上次发送不足 60s → 拒绝（防短信轰炸）
	if existing, lerr := loadPhoneCode(purpose, phone); lerr == nil && existing != nil && !existing.Consumed {
		if time.Since(existing.LastSentAt) < PhoneCodeResendInterval {
			return "", ErrPhoneCodeTooOften
		}
	}

	code := generatePhoneCode()
	salt := generatePhoneSalt()
	rec := &phoneCodeRecord{
		CodeHash:   hashPhoneCode(phone, code, salt),
		Salt:       salt,
		Purpose:    purpose,
		ExpiresAt:  time.Now().Add(PhoneCodeTTL),
		LastSentAt: time.Now(),
	}

	if err := savePhoneCode(purpose, phone, rec); err != nil {
		return "", err
	}
	// 记录申请时间用于频控
	bumpPhoneCount(purpose, phone)
	if ip != "" {
		bumpPhoneCountIP(purpose, ip)
	}
	return code, nil
}

// VerifyPhoneCode 校验并消费验证码（一次性）。区分 不存在/超尝试/不匹配。
func VerifyPhoneCode(phone, code, purpose string) error {
	phone = strings.TrimSpace(phone)
	code = strings.TrimSpace(code)
	purpose = strings.TrimSpace(purpose)
	if purpose == "" {
		purpose = PhonePurposeLogin
	}

	rec, err := loadPhoneCode(purpose, phone)
	if err != nil || rec == nil {
		return ErrPhoneCodeNotFound
	}
	if rec.Consumed || time.Now().After(rec.ExpiresAt) {
		return ErrPhoneCodeNotFound
	}
	if rec.Attempts >= PhoneCodeMaxAttempts {
		return ErrPhoneCodeTooManyAttempts
	}
	if hashPhoneCode(phone, code, rec.Salt) != rec.CodeHash {
		rec.Attempts++
		_ = savePhoneCode(purpose, phone, rec)
		return ErrPhoneCodeMismatch
	}
	rec.Consumed = true
	_ = savePhoneCode(purpose, phone, rec)
	return nil
}

// DeletePhoneCode 主动作废验证码（换绑/改密成功后清理）。
func DeletePhoneCode(phone, purpose string) {
	deletePhoneCode(purpose, phone)
}

// ---------------- 存储后端（Redis / 内存） ----------------

func savePhoneCode(purpose, phone string, rec *phoneCodeRecord) error {
	if useRedisForPhone() {
		b, err := json.Marshal(rec)
		if err != nil {
			return err
		}
		// TTL = 记录剩余有效期（至少 1 秒，避免 SETEX 0 报错）
		ttl := time.Until(rec.ExpiresAt)
		if ttl < time.Second {
			ttl = time.Second
		}
		return RedisClient.Set(redisCtx, phoneCodeKey(purpose, phone), b, ttl).Err()
	}
	phoneCodeStoreMu.Lock()
	phoneCodeStore[phoneCodeKey(purpose, phone)] = rec
	phoneCodeStoreMu.Unlock()
	return nil
}

func loadPhoneCode(purpose, phone string) (*phoneCodeRecord, error) {
	if useRedisForPhone() {
		raw, err := RedisClient.Get(redisCtx, phoneCodeKey(purpose, phone)).Result()
		if err == redis.Nil {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		var rec phoneCodeRecord
		if err := json.Unmarshal([]byte(raw), &rec); err != nil {
			return nil, err
		}
		return &rec, nil
	}
	phoneCodeStoreMu.RLock()
	rec := phoneCodeStore[phoneCodeKey(purpose, phone)]
	phoneCodeStoreMu.RUnlock()
	if rec != nil && time.Now().After(rec.ExpiresAt) {
		// 惰性清理过期记录
		phoneCodeStoreMu.Lock()
		delete(phoneCodeStore, phoneCodeKey(purpose, phone))
		phoneCodeStoreMu.Unlock()
		return nil, nil
	}
	return rec, nil
}

func deletePhoneCode(purpose, phone string) {
	if useRedisForPhone() {
		_ = RedisClient.Del(redisCtx, phoneCodeKey(purpose, phone)).Err()
		return
	}
	phoneCodeStoreMu.Lock()
	delete(phoneCodeStore, phoneCodeKey(purpose, phone))
	phoneCodeStoreMu.Unlock()
}

// ---------------- 频控计数（Redis INCR+TTL / 内存时间列表） ----------------

func phoneCodeCount(purpose, phone string, since time.Time) (int, error) {
	if useRedisForPhone() {
		// Redis 计数 key 按天 TTL，incr 由发送方执行；这里直接读
		n, err := RedisClient.Get(redisCtx, phoneCountKey(purpose, phone)).Int()
		if err == redis.Nil {
			return 0, nil
		}
		return n, err
	}
	phoneCodeCountMu.Lock()
	defer phoneCodeCountMu.Unlock()
	return len(pruneCountLocked(phoneCountKey(purpose, phone), since)), nil
}

func phoneCodeCountIP(purpose, ip string, since time.Time) (int, error) {
	if useRedisForPhone() {
		n, err := RedisClient.Get(redisCtx, phoneCountIPKey(purpose, ip)).Int()
		if err == redis.Nil {
			return 0, nil
		}
		return n, err
	}
	phoneCodeCountMu.Lock()
	defer phoneCodeCountMu.Unlock()
	return len(pruneCountLocked(phoneCountIPKey(purpose, ip), since)), nil
}

func bumpPhoneCount(purpose, phone string) {
	key := phoneCountKey(purpose, phone)
	if useRedisForPhone() {
		// INCR + ExpireNX：首次计数设置 24h TTL，后续只累加不续期（保证窗口固定）
		if err := RedisClient.Incr(redisCtx, key).Err(); err != nil {
			return
		}
		_ = RedisClient.ExpireNX(redisCtx, key, 24*time.Hour).Err()
		return
	}
	phoneCodeCountMu.Lock()
	phoneCodeCounts[key] = append(phoneCodeCounts[key], time.Now())
	phoneCodeCountMu.Unlock()
}

func bumpPhoneCountIP(purpose, ip string) {
	key := phoneCountIPKey(purpose, ip)
	if useRedisForPhone() {
		if err := RedisClient.Incr(redisCtx, key).Err(); err != nil {
			return
		}
		_ = RedisClient.ExpireNX(redisCtx, key, 24*time.Hour).Err()
		return
	}
	phoneCodeCountMu.Lock()
	phoneCodeCounts[key] = append(phoneCodeCounts[key], time.Now())
	phoneCodeCountMu.Unlock()
}

// pruneCountLocked 清理 24h 前的时间戳（调用方须持锁）
func pruneCountLocked(key string, since time.Time) []time.Time {
	list := phoneCodeCounts[key]
	kept := list[:0]
	for _, t := range list {
		if t.After(since) {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(phoneCodeCounts, key)
		return nil
	}
	phoneCodeCounts[key] = kept
	return kept
}

// CleanupPhoneCodes 清理过期的内存验证码与计数（定时任务调用；Redis 模式为空操作）。
func CleanupPhoneCodes() {
	if useRedisForPhone() {
		return // Redis TTL 自动过期
	}
	now := time.Now()
	phoneCodeStoreMu.Lock()
	for k, rec := range phoneCodeStore {
		if now.After(rec.ExpiresAt) {
			delete(phoneCodeStore, k)
		}
	}
	phoneCodeStoreMu.Unlock()
	dayAgo := now.Add(-24 * time.Hour)
	phoneCodeCountMu.Lock()
	for k := range phoneCodeCounts {
		pruneCountLocked(k, dayAgo)
	}
	phoneCodeCountMu.Unlock()
}

// PhoneCodeBackend 当前验证码存储后端（"redis" / "memory"，供健康检查展示）。
func PhoneCodeBackend() string {
	if useRedisForPhone() {
		return "redis"
	}
	return "memory"
}
