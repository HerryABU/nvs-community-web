package models

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"nvs-server/utils"
)

// DBConfig 描述一个数据库连接目标（迁移/安装时由前端传入）
type DBConfig struct {
	Driver   string `json:"driver"` // "sqlite" | "mysql"
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Name     string `json:"name"`
	Path     string `json:"path"` // sqlite 文件路径，留空默认 data/nvs.db
}

// AllModels 所有需要建表与迁移的模型（与 main.go 的 AutoMigrate 保持一致）
func AllModels() []interface{} {
	return []interface{}{
		&User{}, &Novel{}, &NovelCategory{}, &Chapter{}, &Comment{}, &Rating{},
		&Forum{}, &Thread{}, &Post{}, &ForumMute{}, &VipApplication{}, &Report{},
		&WithdrawalRequest{}, &EarningsRecord{}, &BlacklistIP{}, &PlatformConfig{},
		&FederatedSite{}, &FederatedNovel{}, &FederatedInbound{}, &BookShelf{}, &Follow{}, &AuthorBlog{},
		&UserFrame{}, &UserHTML{}, &XssRule{}, &EmergencyAuditLog{}, &UserAuditLog{},
		&BadWord{}, &BadWordFlag{}, // 违禁词库：规则 + 标记待审记录
		&BlogCategory{}, &BlogLike{}, // 博客分类（一/二级）+ 博客点赞
		&ZoneAccessLog{}, // 隔离墙（敏感分区）进入记录，176号令合规
		&Notification{}, // 站内消息通知
		&ReadingNote{},  // 精读：书签 + 批注（含公开/私密）
		// 软删除留证表（softdel_*，合规证据留存）
		&SoftDelComment{}, &SoftDelNovel{}, &SoftDelChapter{}, &SoftDelUser{},
		&SoftDelThread{}, &SoftDelPost{}, &SoftDelBlog{}, &SoftDelForum{},
	}
}

// BuildDialector 根据配置构建 GORM 方言
func BuildDialector(cfg DBConfig) (gorm.Dialector, error) {
	switch cfg.Driver {
	case "mysql":
		host := cfg.Host
		if host == "" {
			host = "127.0.0.1"
		}
		port := cfg.Port
		if port == "" {
			port = "3306"
		}
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.User, cfg.Password, host, port, cfg.Name)
		return mysql.Open(dsn), nil
	case "sqlite":
		path := cfg.Path
		if path == "" {
			path = "data/nvs.db"
		}
		// _pragma=foreign_keys(1)：每个新连接自动启用外键约束（数据一致性保障）
		// _pragma=busy_timeout(5000)：写锁等待 5s，避免并发写 SQLite 直接报 database is locked
		dsn := path
		if strings.Contains(dsn, "?") {
			dsn += "&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
		} else {
			dsn += "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
		}
		return sqlite.Open(dsn), nil
	default:
		return nil, fmt.Errorf("不支持的数据库驱动: %s", cfg.Driver)
	}
}

// OpenDB 打开一个数据库连接（不迁移，仅连接）
func OpenDB(cfg DBConfig) (*gorm.DB, error) {
	d, err := BuildDialector(cfg)
	if err != nil {
		return nil, err
	}
	if cfg.Driver == "sqlite" {
		// 确保 sqlite 文件父目录存在
		path := cfg.Path
		if path == "" {
			path = "data/nvs.db"
		}
		if dir := filepath.Dir(path); dir != "." && dir != "" {
			_ = os.MkdirAll(dir, 0755)
		}
	}
	db, err := gorm.Open(d, &gorm.Config{
		Logger: utils.NewDBLogger(),
	})
	if err != nil {
		return nil, err
	}
	if cfg.Driver == "mysql" {
		if sqlDB, e := db.DB(); e == nil {
			sqlDB.SetMaxIdleConns(10)
			sqlDB.SetMaxOpenConns(100)
			sqlDB.SetConnMaxLifetime(time.Hour)
		}
	}
	// 双保险：当前连接显式开启外键（DSN 参数保证连接池内所有连接均开启）
	if cfg.Driver == "sqlite" {
		if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
			return nil, fmt.Errorf("启用外键约束失败: %w", err)
		}
		var fk int
		_ = db.Raw("PRAGMA foreign_keys").Scan(&fk)
		if fk != 1 {
			return nil, fmt.Errorf("外键约束未生效（PRAGMA foreign_keys = %d）", fk)
		}
	}
	return db, nil
}

// AutoMigrateAll 迁移所有模型表结构
func AutoMigrateAll(db *gorm.DB) error {
	if err := db.AutoMigrate(AllModels()...); err != nil {
		return err
	}
	// 方案A：移除 posts.deleted_at 列（统一走 softdel_posts 留证，幂等）
	// 注意顺序：先 AutoMigrate（建表），再移除历史遗留列
	if err := DropPostDeletedAt(db); err != nil {
		return err
	}
	// 清理被新复合索引替代的旧单列索引（冗余索引会造成写放大）
	return DropLegacyIndexes(db)
}

// CopyAllTables 将 src 的所有表数据完整复制到 dst（dst 需已 AutoMigrate）。
// 通过 SELECT * → map 扫描逐表复制，保留主键 ID 与时间戳，外键关系保持一致性。
// 复制期间临时关闭外键检查（防止表复制顺序触发外键失败），完成后恢复。
// onTable 回调用于上报每张表的进度（table 名、行数）。
func CopyAllTables(src, dst *gorm.DB, onTable func(table string, rows int64)) (map[string]int64, error) {
	// 复制期间临时关闭目标库外键检查（SQLite / MySQL）
	restoreFK := func() {}
	if dst.Dialector.Name() == "sqlite" {
		_ = dst.Exec("PRAGMA foreign_keys = OFF").Error
		restoreFK = func() { _ = dst.Exec("PRAGMA foreign_keys = ON").Error }
	} else {
		_ = dst.Exec("SET FOREIGN_KEY_CHECKS = 0").Error
		restoreFK = func() { _ = dst.Exec("SET FOREIGN_KEY_CHECKS = 1").Error }
	}
	defer restoreFK()

	tables, err := src.Migrator().GetTables()
	if err != nil {
		return nil, err
	}
	result := make(map[string]int64, len(tables))
	for _, t := range tables {
		// 跳过 SQLite 内部序列表（只读，不可 INSERT，且 MySQL 无对应表）
		if strings.HasPrefix(strings.ToLower(t), "sqlite_") {
			continue
		}
		var rows []map[string]interface{}
		if err := src.Table(t).Find(&rows).Error; err != nil {
			return result, fmt.Errorf("读取表 %s 失败: %w", t, err)
		}
		// 事务内批量插入，提升跨库复制速度
		err := dst.Transaction(func(tx *gorm.DB) error {
			const batch = 500
			for i := 0; i < len(rows); i += batch {
				end := i + batch
				if end > len(rows) {
					end = len(rows)
				}
				if err := tx.Table(t).Create(rows[i:end]).Error; err != nil {
					return fmt.Errorf("写入表 %s 失败: %w", t, err)
				}
			}
			return nil
		})
		if err != nil {
			return result, err
		}
		result[t] = int64(len(rows))
		if onTable != nil {
			onTable(t, int64(len(rows)))
		}
	}
	return result, nil
}

// dbSwapMu 保护全局 DB 指针的热切换（迁移/安装后替换 models.DB）
var dbSwapMu sync.RWMutex

// ReconnectDatabase 将全局 models.DB 热切换到新连接（无需重启）。
// 调用方需先完成数据迁移，本函数只负责「打开新连接 → 替换全局指针 → 关闭旧连接」。
func ReconnectDatabase(cfg DBConfig) error {
	newDB, err := OpenDB(cfg)
	if err != nil {
		return err
	}
	dbSwapMu.Lock()
	old := DB
	DB = newDB
	dbSwapMu.Unlock()

	if old != nil {
		if s, e := old.DB(); e == nil && s != nil {
			_ = s.Close()
		}
	}
	return nil
}

// CurrentDBSummary 返回当前数据库连接摘要（密码脱敏），供管理面板展示
func CurrentDBSummary() map[string]interface{} {
	driver := "unknown"
	if DB != nil {
		driver = DB.Dialector.Name()
		// glebarez/sqlite 返回 "sqlite"，mysql 返回 "mysql"
	}
	return map[string]interface{}{
		"driver": driver,
	}
}
