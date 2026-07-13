// email.go 保留为 package utils 的占位文件。
// SMTP 发送与配置 → email_smtp.go
// 邮件 HTML 模板     → email_template.go
// 本文件保留验证码存储/校验逻辑。
package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
	"time"
)

// ============================================================
// 验证码存储与校验
// ============================================================

// VerificationCode 邮箱验证码记录
type VerificationCode struct {
	Code      string
	ExpiresAt time.Time
}

var (
	codeStore   = make(map[string]VerificationCode)
	codeStoreMu sync.RWMutex
)

// GenerateVerificationCode 生成6位数字验证码
func GenerateVerificationCode() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	return fmt.Sprintf("%06d", n.Int64())
}

// StoreVerificationCode 存储验证码（10分钟有效）
func StoreVerificationCode(email, code string) {
	codeStoreMu.Lock()
	defer codeStoreMu.Unlock()
	codeStore[email] = VerificationCode{
		Code:      code,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
}

// VerifyCode 验证验证码
func VerifyCode(email, code string) bool {
	codeStoreMu.RLock()
	defer codeStoreMu.RUnlock()
	vc, ok := codeStore[email]
	if !ok {
		return false
	}
	if time.Now().After(vc.ExpiresAt) {
		delete(codeStore, email)
		return false
	}
	return vc.Code == code
}

// DeleteVerificationCode 删除验证码
func DeleteVerificationCode(email string) {
	codeStoreMu.Lock()
	defer codeStoreMu.Unlock()
	delete(codeStore, email)
}
