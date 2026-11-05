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

	"nvs-server/config"
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

// DomainModelProviders 由各域子包在 init 中注册，聚合其自有模型用于统一迁移。
// 这样根包无需 import 任何域子包，避免「根↔域」导入循环。
var DomainModelProviders []func() []interface{}

// RegisterDomainModels 域子包在 init 调用，登记自身模型列表。
func RegisterDomainModels(p func() []interface{}) {
	DomainModelProviders = append(DomainModelProviders, p)
}

// 跨域解耦钩子：由各域子包在 init 注册，根包仅持有函数变量、不回引域子包，避免导入循环。
// BlogCommentNotifier 新评论触达博客作者时通知；BlogFeedProvider 特别关注动态流的博客部分。
var BlogCommentNotifier func(actorID, blogAuthorID uint, blogTitle, link string) error
var BlogFeedProvider func(ids []uint, window int) ([]FeedItem, int64, error)

// AllModels 所有需要建表与迁移的模型（与 main.go 的 AutoMigrate 保持一致）。
// 仅列出根包自身的模型；域子包模型由 DomainModelProviders 在运行时聚合。
// 已下沉到域子包的模型由各自 init() 注册，不再在此重复列出：
//
//	models/blog      —— AuthorBlog / BlogCategory / BlogTag / BlogTopic 等
//	models/security  —— BadWord / BadWordFlag / Blacklist / CrisisEvent / XssRule / Vulnerability
//	models/theme     —— SiteTheme / UserTheme / AuthorTheme / HomeBlock / HomeCustomize 系列
//	models/siteops   —— Announcement / SystemSetting / EmergencyAuditLog / NavMenu /
//	                    Carousel / CheckInRecord / RageConfig / RefreshState
//	models/search    —— SearchLog（热词聚合 / 标签热词）
//	models/aicost    —— ChannelBalanceSample / UserAiKey / CostRate 等成本与用户密钥
//	models/aiops     —— AiPrompt / AiVendor / AiNovelMemory / AigcAsset 等 AI 运营模型
//	models/aihub     —— AiChannelKey / AiModelKey / AiStyle / UserAiFreeQuota 等渠道与工作台模型
//	models/ai        —— AiModel / AiCallLog / AiAgent / AiMediaTask / AgentSubmission 等核心模型
//	models/quota     —— UserWebQuota / WebAccessLog / UserPublishLog / HTMLQuotaOverride 等额度模型
//	models/wallet    —— TokenLedger / UserTokenBalance / CardKey / RechargeOrder 等代币与支付模型
//	models/softdel   —— SoftDel* 留证表 / UserFrame / UserHTML（并注册迁移辅助钩子）
//	models/apikey    —— APIKey / ApiAccessLog / ApiKeyAlarm / UserAuditLog 等密钥与审计模型
//	models/platform  —— FederatedSite / VipApplication / Report / ZoneAccessLog 等平台治理模型
func AllModels() []interface{} {
	list := []interface{}{
		&User{}, &Novel{}, &NovelCategory{}, &Chapter{}, &Comment{}, &Rating{},
		// 作品角色表（话本气泡体的角色管理：头像/主题色/默认左右）
		&NovelCharacter{},
		&Forum{}, &Thread{}, &Post{}, &ForumMute{}, &PlatformConfig{},
		&BookShelf{}, &Follow{},
		&Notification{}, // 站内消息通知
		&ReadingNote{},  // 精读：书签 + 批注（含公开/私密）
		// 自建论坛额度：按人覆盖（管理员可削减/放开某人的建版数）
		&UserForumQuota{},
		// 私信（F20：会话消息，互关才可发第 3 条）
		&Message{},
	}
	for _, p := range DomainModelProviders {
		list = append(list, p()...)
	}
	return list
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
			// 优先用 DB_PATH（容器挂载卷 / 自定义目录），否则回落默认。
			// 原来这里硬编码 data/nvs.db，导致设了 DB_PATH 的部署在建站向导阶段
			// 仍把库写进工作目录 —— 容器重启即丢数据，且与运行时连接的库不是同一个。
			path = config.DBPath
			if path == "" {
				path = "data/nvs.db"
			}
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
		// 确保 sqlite 文件父目录存在（默认值与 BuildDialector 保持同一口径）
		path := cfg.Path
		if path == "" {
			path = config.DBPath
			if path == "" {
				path = "data/nvs.db"
			}
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
	// 这些辅助实现已下沉到 models/softdel，由该子包 init() 注册；未加载时为 nil，
	// 跳过即可（main.go / 测试入口均已 blank import 保证生产与测试行为不变）。
	if DropPostDeletedAtHook != nil {
		if err := DropPostDeletedAtHook(db); err != nil {
			return err
		}
	}
	// 清理被新复合索引替代的旧单列索引（冗余索引会造成写放大）
	if DropLegacyIndexesHook != nil {
		if err := DropLegacyIndexesHook(db); err != nil {
			return err
		}
	}
	// 楼层模型：存量评论/回帖补 root_id 与 floor_no（幂等，仅缺失时回填）
	// 注意：回填必须用传入的 db，AutoMigrateAll 阶段全局 DB 可能尚未就绪（测试场景）
	if BackfillCommentFloorsHook != nil {
		BackfillCommentFloorsHook(db)
	}
	if BackfillPostFloorsHook != nil {
		BackfillPostFloorsHook(db)
	}
	// 应用扩展：存量已解压扩展补 is_extracted（新列默认 false，需按 extract_dir 回填）
	if BackfillUserHTMLExtractedHook != nil {
		BackfillUserHTMLExtractedHook(db)
	}
	return nil
}

// mysqlJSONColumns 记录从 SQLite 复制到 MySQL 时需要归一化为合法 JSON 的列（type:json 字段）。
// SQLite 允许这些列存空字符串，而 MySQL 的 JSON 列拒绝空串（Error 3140: Invalid JSON text: "The document is empty."），
// 复制时统一改写为 "[]"，与 CreateNovel 的代码兜底保持一致。
var mysqlJSONColumns = map[string][]string{
	"novels": {"tags"},
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
		// MySQL 目标：JSON 列空字符串归一化为 "[]"（SQLite 允许空串，MySQL JSON 列拒绝 → Error 3140）
		if dst.Dialector.Name() == "mysql" {
			if jsonCols, ok := mysqlJSONColumns[t]; ok && len(rows) > 0 {
				for _, row := range rows {
					for _, col := range jsonCols {
						if v, ok := row[col].(string); ok && v == "" {
							row[col] = "[]"
						}
					}
				}
			}
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
