package honeypot

// ==================== 数据连接（支持独立库） ====================
// 两种部署形态：
//   1) 内嵌（默认）：蜜罐表放主库，零配置；
//   2) 独立库：设 NVS_HONEYPOT_DB_PATH（如 data/honeypot/honeypot.db）→ 蜜罐只写这个库，
//      主站管理接口读的也是它（所以后台照样能看），但蜜罐数据与业务库**物理隔离**。
// 独立进程 `nvs-honeypot` 复用同一路径：受害面最小的隔离形态（崩溃/被写入都影响不到主站）。
//
// 铁律：独立库路径不得与主库路径相同（启动断言），否则「隔离」只是心理安慰。

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var (
	dbMu    sync.RWMutex
	ownDB   *gorm.DB
	ownPath string
)

// SetDB 切换到独立库（主站或独立进程启动时调用）
func SetDB(conn *gorm.DB, path string) error {
	if conn == nil {
		return errors.New("数据库连接为空")
	}
	if strings.TrimSpace(path) != "" {
		if same, err := sameFile(path, mainDBPath()); err == nil && same {
			return errors.New("蜜罐库不能与主库同路径（隔离失效）")
		}
	}
	dbMu.Lock()
	ownDB, ownPath = conn, path
	dbMu.Unlock()
	ResetCache()
	return nil
}

// DBPath 当前蜜罐库路径（空 = 用主库）
func DBPath() string {
	dbMu.RLock()
	defer dbMu.RUnlock()
	return ownPath
}

// Isolated 是否使用独立库
func Isolated() bool {
	dbMu.RLock()
	defer dbMu.RUnlock()
	return ownDB != nil
}

// mainDBPath 主库路径（用于同路径断言）
func mainDBPath() string {
	if p := strings.TrimSpace(os.Getenv("DB_PATH")); p != "" {
		if abs, err := filepath.Abs(p); err == nil {
			return abs
		}
		return p
	}
	if abs, err := filepath.Abs("data/nvs.db"); err == nil {
		return abs
	}
	return "data/nvs.db"
}

func sameFile(a, b string) (bool, error) {
	if a == "" || b == "" {
		return false, nil
	}
	aa, err := filepath.Abs(a)
	if err != nil {
		return false, err
	}
	bb, err := filepath.Abs(b)
	if err != nil {
		return false, err
	}
	return aa == bb, nil
}

// db 当前使用的连接：优先独立库，其次主库（未初始化返回 nil，调用方需容忍）
func db() *gorm.DB {
	dbMu.RLock()
	if ownDB != nil {
		conn := ownDB
		dbMu.RUnlock()
		return conn
	}
	dbMu.RUnlock()
	return modelsDB()
}

// Bootstrap 主站启动时调用：
//   - 未设 NVS_HONEYPOT_DB_PATH → 蜜罐表建在主库（内嵌形态）
//   - 设了 → 单独打开该库、建表、并切到它（蜜罐数据与业务库物理隔离，后台仍可读）
func Bootstrap(mainDB *gorm.DB) error {
	path := strings.TrimSpace(os.Getenv("NVS_HONEYPOT_DB_PATH"))
	if path == "" {
		return Migrate(mainDB)
	}
	return OpenIsolated(path)
}

// OpenIsolated 打开（或创建）独立蜜罐库并切换
func OpenIsolated(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	conn, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return err
	}
	if err := Migrate(conn); err != nil {
		return err
	}
	return SetDB(conn, path)
}

// ResetDB 复位到主库（测试与手动切换用）；会关闭独立库连接，避免文件句柄泄漏
func ResetDB() {
	dbMu.Lock()
	if ownDB != nil {
		if sqlDB, err := ownDB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
	ownDB, ownPath = nil, ""
	dbMu.Unlock()
	ResetCache()
}
