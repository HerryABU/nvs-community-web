package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"sync"
	"sync/atomic"
)

// ==================== 建站向导状态 ====================
// 状态不存数据库（向导本身要配置数据库），改用磁盘标记：
//   - data/secure-config.json   存在 = 已完成安装（加密的数据库配置，向导永久关闭）
//   - data/.setup_key           一次性安装密钥（首次运行生成，完成后删除）
//
// 严防死守：
//   - 未安装时服务端只挂载安装路由，业务 API 由门禁中间件返回 503；
//   - 安装接口仅接受「安装密钥」（只在服务器控制台打印一次），且受 sync.Once 安装锁保护；
//   - 安装完成后安装路由在代码层面注销（内存原子标志，404），连判断文件存在的机会都不给。

const (
	setupKeyFile = "data/.setup_key"
)

// 缓存安装状态（进程内只读一次磁盘，安装完成时原子置位）
var (
	installedOnce  sync.Once
	installedState atomic.Bool
)

// IsInstalled 系统是否已完成建站向导（内存原子标志，无文件 I/O）
func IsInstalled() bool {
	installedOnce.Do(func() {
		installedState.Store(SecureConfigExists())
	})
	return installedState.Load()
}

// GetOrCreateSetupKey 获取或生成一次性安装密钥（写入 data/.setup_key）
func GetOrCreateSetupKey() string {
	if b, err := os.ReadFile(setupKeyFile); err == nil && len(b) > 0 {
		return string(b)
	}
	key := randomHex(32)
	_ = os.MkdirAll("data", 0755)
	_ = os.WriteFile(setupKeyFile, []byte(key), 0600)
	return key
}

// VerifySetupKey 校验一次性安装密钥
func VerifySetupKey(key string) bool {
	if key == "" {
		return false
	}
	b, err := os.ReadFile(setupKeyFile)
	if err != nil {
		return false
	}
	return key == string(b)
}

// MarkInstalled 删除一次性安装密钥并置位安装状态（向导永久关闭）。
// 注意：加密配置需在此之前已由 WriteSecureConfig 写入。
func MarkInstalled() error {
	_ = os.Remove(setupKeyFile)
	installedState.Store(true)
	return nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "setup-key-fallback-please-restart"
	}
	return hex.EncodeToString(b)
}
