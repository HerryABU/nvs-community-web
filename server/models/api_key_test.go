package models

// API Key 模型纯函数单元测试（不依赖 DB）

import (
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestGenerateAPIKey(t *testing.T) {
	full, prefix, hash, err := GenerateAPIKey("reader")
	if err != nil {
		t.Fatal(err)
	}
	if len(full) != len("nvs_rdr_")+32 {
		t.Fatalf("Key 长度异常: %d", len(full))
	}
	if prefix[:len("nvs_rdr_")] != "nvs_rdr_" {
		t.Fatalf("前缀错误: %s", prefix)
	}
	if HashAPIKey(full) != hash {
		t.Fatal("哈希不一致")
	}
	// 不同角色前缀
	if p, _, _, _ := GenerateAPIKey("author"); p[:len("nvs_aut_")] != "nvs_aut_" {
		t.Fatal("author 前缀错误")
	}
	if p, _, _, _ := GenerateAPIKey("admin"); p[:len("nvs_adm_")] != "nvs_adm_" {
		t.Fatal("admin 前缀错误")
	}
	if p, _, _, _ := GenerateFederatedKey(); p[:len("nvs_fed_")] != "nvs_fed_" {
		t.Fatal("federated 前缀错误")
	}
}

func TestSanitizeQuotaConfig(t *testing.T) {
	def := DefaultAPIRateLimitConfig()
	// 零值补全
	cfg := APIRateLimitConfig{}
	cfg = sanitizeQuotaConfig(cfg, def)
	if cfg.Admin.Req != def.Admin.Req || cfg.Reader.Req != def.Reader.Req || cfg.Bot.Req != def.Bot.Req {
		t.Fatal("零值应回退默认配额")
	}
	// 超上限夹取
	cfg.Reader.Req = 99999999
	cfg = sanitizeQuotaConfig(cfg, def)
	if cfg.Reader.Req > 1000000 {
		t.Fatal("配额应被夹取到上限内")
	}
}

func TestRoleQuotaFor(t *testing.T) {
	cfg := DefaultAPIRateLimitConfig()
	if cfg.RoleQuotaFor("admin").Req != cfg.Admin.Req {
		t.Fatal("admin 配额错误")
	}
	if cfg.RoleQuotaFor("author").Req != cfg.Author.Req {
		t.Fatal("author 配额错误")
	}
	if cfg.RoleQuotaFor("reader").Req != cfg.Reader.Req {
		t.Fatal("reader 配额错误")
	}
	if cfg.RoleQuotaFor("unknown").Req != cfg.Reader.Req {
		t.Fatal("未知角色应回退 reader 配额")
	}
}

// TestDefaultQuotaOrdering 校验配额层级关系：
//
//	Key（admin/author/reader，均远高于无 Key 通道） >> 匿名 > 爬虫
//
// 设计意图：无 Key 的匿名访问只给低配额，被识别为爬虫的给「更低」配额；
// 爬虫脚本想提速的唯一途径是申请并在请求中携带 API Key。
func TestDefaultQuotaOrdering(t *testing.T) {
	c := DefaultAPIRateLimitConfig()

	// 1) 任何 Key 身份都必须显著高于匿名
	for _, role := range []string{"admin", "author", "reader"} {
		q := c.RoleQuotaFor(role)
		if q.Req <= c.Anonymous.Req {
			t.Fatalf("%s Key 配额(%d) 必须高于匿名(%d)", role, q.Req, c.Anonymous.Req)
		}
	}
	// 2) 匿名（无 Key 普通访问）必须高于爬虫（无 Key 且被识别为爬虫）
	if c.Anonymous.Req <= c.Bot.Req {
		t.Fatalf("匿名配额(%d) 必须高于爬虫配额(%d)", c.Anonymous.Req, c.Bot.Req)
	}
	// 3) 爬虫配额必须小到「无法用于批量抓取」，但非零（完全封死会让站长失去观测能力）
	if c.Bot.Req <= 0 {
		t.Fatal("爬虫配额应 > 0（保留低速率可用性）")
	}
	if c.Bot.Req > c.Anonymous.Req/4 {
		t.Fatalf("爬虫配额(%d) 应远低于匿名（≤ 匿名/4 = %d）", c.Bot.Req, c.Anonymous.Req/4)
	}
	// 4) 窗口单位一致（避免误配成 1 秒级导致实际放行量翻倍）
	if c.Anonymous.Window <= 0 || c.Bot.Window <= 0 {
		t.Fatal("配额窗口必须为正数")
	}
}

// TestFederatedQuotaHigherThanAnonymous 联邦 Key 配额必须远高于匿名（站点间同步需要吞吐）
func TestFederatedQuotaHigherThanAnonymous(t *testing.T) {
	c := DefaultAPIRateLimitConfig()
	if c.Federated.Req <= c.Anonymous.Req {
		t.Fatalf("联邦 Key 配额(%d) 必须高于匿名(%d)", c.Federated.Req, c.Anonymous.Req)
	}
}

// ==================== 双数据库兼容：字段长度契约 ====================
//
// SQLite 不校验 varchar 长度，MySQL 严格模式下超长直接报 "Data too long"，
// 且无长度的 string 在 MySQL 会被建为 longtext（不能建索引/默认值受限）。
// 以下两个测试把「为什么是 512/1024/128/64/32」固化成断言，防止后人随手改小。

// TestNewTablesMySQLColumnTypes 用 MySQL 方言推导新增三表的列类型：
//   - 所有 string 列必须是 varchar(N)（不能退化成 longtext）
//   - 建索引的列必须可索引（varchar + 长度合理）
//   - 单行 utf8mb4 字节数不得超过 MySQL 65535 行大小上限
func TestNewTablesMySQLColumnTypes(t *testing.T) {
	dialector := mysql.New(mysql.Config{SkipInitializeWithVersion: true})
	ns := schema.NamingStrategy{}

	tables := []struct {
		name  string
		model interface{}
	}{
		{"api_keys", &APIKey{}},
		{"federated_peer_keys", &FederatedPeerKey{}},
		{"api_access_logs", &ApiAccessLog{}},
	}

	const rowLimit = 65535 // MySQL InnoDB 单行最大字节数
	for _, tb := range tables {
		s, err := schema.Parse(tb.model, &sync.Map{}, ns)
		if err != nil {
			t.Fatalf("%s schema 解析失败: %v", tb.name, err)
		}
		var rowBytes int
		for _, f := range s.Fields {
			if f.DataType != schema.String {
				continue
			}
			colType := dialector.DataTypeOf(f)
			if !strings.HasPrefix(colType, "varchar(") {
				t.Errorf("%s.%s 应为 varchar(N)，实际 %s（无长度 string 在 MySQL 会退化成长文本，无法建索引）",
					tb.name, f.DBName, colType)
				continue
			}
			n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(colType, "varchar("), ")"))
			if err != nil {
				t.Fatalf("解析列类型失败: %s", colType)
			}
			if n <= 0 {
				t.Errorf("%s.%s varchar 长度非法: %d", tb.name, f.DBName, n)
			}
			// utf8mb4 每字符最多 4 字节
			rowBytes += n * 4
			if f.UniqueIndex != "" || f.TagSettings["INDEX"] != "" || f.PrimaryKey {
				if n*4 > 3072 {
					t.Errorf("%s.%s 建了索引但 varchar(%d) 在 utf8mb4 下 %d 字节，超过 InnoDB 索引长度上限",
						tb.name, f.DBName, n, n*4)
				}
			}
		}
		if rowBytes > rowLimit {
			t.Errorf("%s 单行 utf8mb4 上限 %d 字节 > MySQL 行大小上限 %d", tb.name, rowBytes, rowLimit)
		}
	}
}

// TestNewTablesFieldSizeContract 逐个锁定关键字段的最小长度（写下推导依据）
func TestNewTablesFieldSizeContract(t *testing.T) {
	ns := schema.NamingStrategy{}
	parse := func(model interface{}) map[string]int {
		s, err := schema.Parse(model, &sync.Map{}, ns)
		if err != nil {
			t.Fatalf("schema 解析失败: %v", err)
		}
		out := map[string]int{}
		for _, f := range s.Fields {
			out[f.DBName] = f.Size
		}
		return out
	}

	// APIKey
	ak := parse(&APIKey{})
	checkSize(t, "api_keys", ak, map[string]int{
		"name":        64,  // 备注名，中文 20 字左右
		"role":        16,  // admin/author/reader
		"key_hash":    64,  // sha256 hex 定长 64
		"key_prefix":  24,  // nvs_rdr_ + 8 位随机 = 16，留余量
		"allowed_ips": 512, // 多个 IPv6 + CIDR 逗号分隔
	})

	// FederatedPeerKey
	fp := parse(&FederatedPeerKey{})
	checkSize(t, "federated_peer_keys", fp, map[string]int{
		"direction":   8,    // inbound / outbound
		"peer_name":   128,  // 站点名
		"key_hash":    64,   // sha256 hex 定长 64
		"key_prefix":  24,   // nvs_fed_ + 8 位随机
		"key_cipher":  1024, // hex = 2 × (12 nonce + N + 16 tag) = 2N+56；N=256 → 568；取 1024 兼容最长 484 字符的对端 Key
		"issuer_name": 128,
		"issuer_url":  512, // 站点地址 + 可能的子路径
		"allowed_ips": 512,
	})

	// ApiAccessLog
	al := parse(&ApiAccessLog{})
	checkSize(t, "api_access_logs", al, map[string]int{
		"identity_type": 16, // federated/admin/author/reader/anonymous/bot/blocked
		"key_prefix":    24,
		"method":        8,   // GET/POST/PUT/DELETE
		"path":          512, // URL + 长查询串（列表分页/筛选参数可能很长）
		"request_ip":    64,  // IPv6 及 IPv4-mapped IPv6 最长 45 字符，留余量
		"user_agent":    512, // 现代浏览器 UA 通常 120~250，爬虫/工具链可能更长，超长由代码截断到 500
		"bot_reason":    128, // 如 block_ua_custom:xxx / spoof_browser_ua
	})
}

func checkSize(t *testing.T, table string, got map[string]int, want map[string]int) {
	t.Helper()
	for col, min := range want {
		if v, ok := got[col]; !ok {
			t.Errorf("%s 缺少列 %s", table, col)
		} else if v < min {
			t.Errorf("%s.%s size=%d < 要求的最小长度 %d（MySQL 严格模式会 Data too long）", table, col, v, min)
		}
	}
}

// TestNewTablesMigrateOnSQLite 全模型 AutoMigrate 到内存 SQLite，并按上界长度写入后读回，
// 验证三张新表在 SQLite 侧建表正常、无截断。
func TestNewTablesMigrateOnSQLite(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:apikey_schema_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存测试库失败: %v", err)
	}
	if err := AutoMigrateAll(db); err != nil {
		t.Fatalf("迁移测试库失败: %v", err)
	}
	for _, tb := range []string{"api_keys", "federated_peer_keys", "api_access_logs"} {
		if !db.Migrator().HasTable(tb) {
			t.Fatalf("表 %s 未创建", tb)
		}
	}

	long512 := strings.Repeat("a", 512)
	long1024 := strings.Repeat("b", 1024)

	// 用户级 Key
	if err := db.Create(&APIKey{
		UserID: 1, Name: strings.Repeat("n", 64), Role: "reader",
		KeyHash: strings.Repeat("c", 64), KeyPrefix: "nvs_rdr_12345678",
		QuotaPerMin: 60, AllowedIPs: long512,
	}).Error; err != nil {
		t.Fatalf("写入 api_keys 失败: %v", err)
	}
	var ak APIKey
	db.First(&ak)
	if len(ak.AllowedIPs) != 512 || len(ak.Name) != 64 {
		t.Fatalf("api_keys 数据被截断: name=%d allowed_ips=%d", len(ak.Name), len(ak.AllowedIPs))
	}

	// 联邦 Key（出站密文按上界长度写）
	if err := db.Create(&FederatedPeerKey{
		SiteID: 1, Direction: "outbound", PeerName: strings.Repeat("p", 128),
		KeyHash: strings.Repeat("d", 64), KeyPrefix: "nvs_fed_12345678",
		KeyCipher: long1024, IssuerName: strings.Repeat("e", 128), IssuerURL: long512,
		QuotaPerMin: 5000, AllowedIPs: long512,
	}).Error; err != nil {
		t.Fatalf("写入 federated_peer_keys 失败: %v", err)
	}
	var fp FederatedPeerKey
	db.First(&fp)
	if len(fp.KeyCipher) != 1024 || len(fp.IssuerURL) != 512 {
		t.Fatalf("federated_peer_keys 数据被截断: key_cipher=%d issuer_url=%d", len(fp.KeyCipher), len(fp.IssuerURL))
	}

	// 访问日志（含被阻断身份）
	if err := db.Create(&ApiAccessLog{
		IdentityType: "blocked", IdentityID: 0, KeyPrefix: "",
		Method: "GET", Path: long512, StatusCode: 403,
		RequestIP: strings.Repeat("f", 64), UserAgent: long512,
		BotReason: "block_ua:" + strings.Repeat("g", 100),
	}).Error; err != nil {
		t.Fatalf("写入 api_access_logs 失败: %v", err)
	}
	var al ApiAccessLog
	db.First(&al)
	if len(al.Path) != 512 || len(al.UserAgent) != 512 || al.IdentityType != "blocked" {
		t.Fatalf("api_access_logs 数据异常: path=%d ua=%d type=%s", len(al.Path), len(al.UserAgent), al.IdentityType)
	}

	if sqlDB, e := db.DB(); e == nil {
		_ = sqlDB.Close()
	}
}

func TestMatchAllowedIP(t *testing.T) {
	// 空白名单 = 放行
	if !MatchAllowedIP("", "1.2.3.4") {
		t.Fatal("空白名单应放行")
	}
	// 精确 IP
	if !MatchAllowedIP("1.2.3.4, 5.6.7.8", "5.6.7.8") {
		t.Fatal("应命中精确 IP")
	}
	if MatchAllowedIP("1.2.3.4", "9.9.9.9") {
		t.Fatal("不应命中未列入 IP")
	}
	// CIDR
	if !MatchAllowedIP("10.0.0.0/8", "10.1.2.3") {
		t.Fatal("应命中 CIDR 网段")
	}
	if MatchAllowedIP("10.0.0.0/8", "11.1.2.3") {
		t.Fatal("不应命中网段外 IP")
	}
}
