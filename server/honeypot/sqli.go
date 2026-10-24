package honeypot

// ==================== SQL 注入蜜罐（gap-audit #3） ====================
// 思路：真实业务链路全部参数化查询，SQL 注入打不穿——所以「真注入」这条路本来就是死的。
// 那这条蜜罐做什么？做**钓鱼式假入口**：给攻击者一个看起来能注入的老接口，
// 回吐「预生成响应库」里的假数据（假表结构、假用户、假配置），
// 目的不是骗过谁，而是：
//   1. 低成本拿到完整攻击 payload（谁在测什么、用什么工具、试了哪些库）；
//   2. 拖时间（攻击者以为打穿了，会继续深入假库）；
//   3. 提供高质量特征反哺 WAF 规则（payload 里的函数名/注释符/编码手法）。
//
// 铁律：
//   - 假数据全部一次生成、与真实库零关联（假 email/password_hash/表名）；
//   - 不含任何真实用户信息，不做「真库脱敏导出」这种自欺欺人的事；
//   - 响应里埋金丝雀（`NVS-CANARY-SQLI-*`），一旦出现在外网即可确认泄露来源；
//   - 等级 off/low 时这些路径与普通 404 无差别（不暴露蜜罐存在）。

import (
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"

	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// EventSQLiHit 命中 SQL 注入诱饵（payload 已带注入特征）
// EventSQLiProbe 探路（访问了假 SQL 接口但 payload 无明显注入特征）
const (
	EventSQLiHit   = "sqli_hit"
	EventSQLiProbe = "sqli_probe"
)

// sqliPattern SQL 注入特征（union/时间盲注/报错注入/堆叠/注释绕过/信息库探测）
type sqliPattern struct {
	Name string
	Re   *regexp.Regexp
	Risk int
}

var sqliPatterns = []sqliPattern{
	{"union_select", regexp.MustCompile(`(?i)\bunion\b[\s/*!%]+(?:\ball\b[\s/*!%]+)?\bselect\b`), 95},
	{"time_blind", regexp.MustCompile(`(?i)\b(sleep|pg_sleep|benchmark|waitfor\s+delay)\s*\(`), 92},
	{"error_based", regexp.MustCompile(`(?i)\b(extractvalue|updatexml|exp|geometrycollection|polygon)\s*\(`), 90},
	{"stacked", regexp.MustCompile(`(?i);\s*(drop|delete|update|insert|alter|create|truncate)\b`), 88},
	{"schema_probe", regexp.MustCompile(`(?i)\b(information_schema|sysobjects|pg_catalog|sqlite_master|mysql\.user|all_tables)\b`), 85},
	{"file_access", regexp.MustCompile(`(?i)\b(load_file|into\s+outfile|into\s+dumpfile|xp_cmdshell|sp_executesql)\b`), 92},
	{"tautology", regexp.MustCompile(`(?i)(\bor\b|\band\b)\s+['"]?\d+['"]?\s*=\s*['"]?\d+`), 70},
	{"quote_break", regexp.MustCompile(`(?i)['"]\s*(or|and|--|#|/\*)`), 65},
	{"comment_bypass", regexp.MustCompile(`(?i)(--\s|#\s|/\*!|\*/|%27|%22|%23|%2d%2d)`), 55},
	{"boolean_probe", regexp.MustCompile(`(?i)\b(concat|char|ascii|substring|substr|mid|length|version|database|user)\s*\(`), 50},
}

// DetectSQLi 返回命中的特征名（按风险降序，去重）。
// 导出的原因是：反哺规则、风控联动、单测都要用同一套判定，不能各写一份。
func DetectSQLi(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	// 简单解码：常见 URL 编码与注释插入，避免最基础的绕过
	low := strings.ToLower(s)
	low = strings.ReplaceAll(low, "%20", " ")
	low = strings.ReplaceAll(low, "+", " ")
	out := make([]string, 0, 4)
	seen := map[string]bool{}
	for _, p := range sqliPatterns {
		if p.Re.MatchString(low) && !seen[p.Name] {
			seen[p.Name] = true
			out = append(out, p.Name)
		}
	}
	return out
}

// SQLiRisk 由命中特征算风险分（无命中=探路 40）
func SQLiRisk(hits []string) int {
	if len(hits) == 0 {
		return 40
	}
	max := 0
	for _, h := range hits {
		for _, p := range sqliPatterns {
			if p.Name == h && p.Risk > max {
				max = p.Risk
			}
		}
	}
	// 多特征叠加（说明在用工具，不是手滑）
	if len(hits) >= 3 && max+5 <= 100 {
		max += 5
	}
	return max
}

// ─────────────── 预生成响应库 ───────────────
// 全部为合成数据；金丝雀列用于泄露溯源。

var fakeTables = []string{
	"nvs_users", "nvs_orders", "nvs_payments", "nvs_api_keys",
	"nvs_ai_call_logs", "nvs_novels", "nvs_chapters", "nvs_sessions",
}

type fakeUser struct {
	ID       int
	Username string
	Email    string
	Hash     string
	Role     string
}

// fakeUsers 固定种子生成，保证同一部署每次看到的「库」一致（更像真的）
func fakeUsers(n int) []fakeUser {
	r := rand.New(rand.NewSource(20261024))
	names := []string{"admin", "ops", "editor01", "writer_lin", "reader_zhang", "mod_chen"}
	roles := []string{"admin", "author", "author", "reader", "reader", "editor"}
	out := make([]fakeUser, 0, n)
	for i := 0; i < n && i < len(names); i++ {
		out = append(out, fakeUser{
			ID:       i + 1,
			Username: names[i],
			Email:    fmt.Sprintf("%s@example.internal", names[i]),
			Hash:     fmt.Sprintf("$2a$10$%016x", r.Uint64()),
			Role:     roles[i],
		})
	}
	return out
}

// canaryToken 每次进程启动生成一次（同一次攻击会话内一致，便于关联）
var sqliCanary = fmt.Sprintf("NVS-CANARY-SQLI-%08x", uint32(time.Now().UnixNano()))

// ServeSQLi 处理一次注入请求，返回仿真结果并记事件。
func ServeSQLi(c *gin.Context, decoyID string) {
	set := Current()
	if !set.Enabled || set.Level == LevelOff || set.Level == LevelLow {
		utils.NotFound(c, "接口不存在")
		return
	}
	// payload 来源：query + body 摘要（不解析结构，只要字符串）
	payload := c.Request.URL.RawQuery
	if b := readBodySnippet(c.Request, 1024); b != "" {
		payload += " " + b
	}
	hits := DetectSQLi(payload)
	risk := SQLiRisk(hits)
	kind := EventSQLiHit
	if len(hits) == 0 {
		kind = EventSQLiProbe
	}
	recordSQLi(c, set, kind, decoyID, payload, hits, risk)

	if set.DelayMs > 0 {
		time.Sleep(time.Duration(minInt(set.DelayMs, 2000)) * time.Millisecond)
	}
	// 没命中特征也回一份「连上了」的假响应：让它以为这个入口有效
	c.Header("X-Powered-By", "PHP/7.4.3")
	c.Header("Server", "nginx/1.18.0")
	writeFakeResult(c, decoyID, hits)
}

// writeFakeResult 按诱饵类型回吐不同形态的假数据
func writeFakeResult(c *gin.Context, decoyID string, hits []string) {
	users := fakeUsers(6)
	switch decoyID {
	case "sqli_login":
		// 登录型：永远「成功」并给一个假管理员会话，诱导继续操作
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"msg":  "ok",
			"data": gin.H{
				"uid": users[0].ID, "username": users[0].Username, "role": "admin",
				"token": "hp." + sqliCanary, "expire": 7200,
			},
		})
	case "sqli_schema":
		cols := []gin.H{}
		for i, t := range fakeTables {
			cols = append(cols, gin.H{"table": t, "rows": 120 + i*37, "engine": "InnoDB"})
		}
		c.JSON(http.StatusOK, gin.H{
			"code": 0, "canary": sqliCanary,
			"data": gin.H{"database": "nvs_prod", "tables": cols},
		})
	case "sqli_dump":
		rows := make([]gin.H, 0, len(users))
		for _, u := range users {
			rows = append(rows, gin.H{
				"id": u.ID, "username": u.Username, "email": u.Email,
				"password": u.Hash, "role": u.Role, "canary": sqliCanary,
			})
		}
		c.JSON(http.StatusOK, gin.H{"code": 0, "count": len(rows), "rows": rows})
	default:
		// 通用查询型：仿 mysql 错误回显（报错注入会拿到「有用信息」的错觉）
		if len(hits) > 0 {
			c.JSON(http.StatusOK, gin.H{
				"code": 1064,
				"msg": fmt.Sprintf("You have an error in your SQL syntax near '%s' at line 1",
					truncate(strings.Join(hits, ","), 120)),
				"sqlstate": "42000", "canary": sqliCanary,
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"code": 0, "data": gin.H{"affected": 1, "server": "MySQL 5.7.42-log", "canary": sqliCanary},
		})
	}
}

// recordSQLi 记事件（复用 Record 的会话聚合；失败不影响响应）
func recordSQLi(c *gin.Context, set Setting, kind, decoyID, payload string, hits []string, risk int) {
	ev, err := Record(set, c, Decoy{ID: decoyID, Path: c.Request.URL.Path, Risk: risk})
	if err == nil && ev != nil {
		// 追加 payload / 特征（Record 只记通用字段），kind 改成 sqli_*，风险按特征重新算
		_ = db().Model(&Event{}).Where("id = ?", ev.ID).Updates(map[string]interface{}{
			"kind":         kind,
			"body_snippet": truncate(payload, 900),
			"risk":         risk,
			"decoy":        decoyID,
		}).Error
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ─────────────── 路由挂载 ───────────────

// sqliDecoys 假 SQL 入口（路径刻意像「老系统遗留」，不是 /api/v2 那种新前缀）
var sqliDecoys = []struct {
	Path string
	ID   string
}{
	{"/api/v1/query", "sqli_query"},
	{"/api/v1/login", "sqli_login"},
	{"/api/v1/admin/tables", "sqli_schema"},
	{"/api/v1/export/users", "sqli_dump"},
	{"/legacy/db.php", "sqli_query"},
}

// RegisterSQLi 挂载 SQL 注入诱饵（root 级，不经过鉴权）
func RegisterSQLi(r *gin.Engine) {
	for _, d := range sqliDecoys {
		decoy := d
		r.Any(decoy.Path, func(c *gin.Context) { ServeSQLi(c, decoy.ID) })
	}
	// 低等级只保留一个「探路」入口：/api/v1/query（出现在扫描器字典里，但 low 档也常见）
	r.Any("/db/query", func(c *gin.Context) { ServeSQLi(c, "sqli_query") })
}
