package honeypot

// ==================== 诱饵路由 ====================
// 全部诱饵路径**无条件注册**（gin 路由表必须静态），命中后再按当前等级决定是否生效：
//   - 等级不覆盖该诱饵 → 当作普通 404（不暴露蜜罐存在）
//   - 覆盖 → 记录事件并返回仿真内容
// 注意：诱饵响应没有任何「成功」分支，也不提供可交互的真实端点。

import (
	"net/http"
	"strings"
	"time"

	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// probePaths 常见扫描器探测路径（不在诱饵表内的 404 探路，仅记录不响应内容）
var probePaths = []string{
	"/.git/HEAD", "/.svn/entries", "/.DS_Store", "/config.php", "/wp-admin/",
	"/phpmyadmin/", "/admin/login", "/adminer.php", "/swagger-ui.html",
	"/actuator/health", "/api/v1/users", "/graphql", "/server-status",
	"/trace.axd", "/.aws/credentials", "/docker-compose.yml", "/.env.local",
}

// Register 挂载诱饵路由（root 级，不经过 /api 鉴权中间件）
func Register(r *gin.Engine) {
	for _, d := range allDecoys() {
		decoy := d // 闭包捕获
		r.Any(decoy.Path, func(c *gin.Context) {
			serveDecoy(c, decoy)
		})
	}
	for _, p := range probePaths {
		path := p
		r.Any(path, func(c *gin.Context) {
			serveProbe(c, path)
		})
	}
}

// serveDecoy 返回诱饵内容（按等级）
func serveDecoy(c *gin.Context, d Decoy) {
	set := Current()
	if !set.Enabled || set.Level == LevelOff {
		utils.NotFound(c, "页面不存在")
		return
	}
	// 等级是否覆盖该诱饵
	covered := false
	for _, x := range decoysForLevel(set.Level) {
		if x.ID == d.ID {
			covered = true
			break
		}
	}
	if !covered {
		utils.NotFound(c, "页面不存在")
		return
	}

	// 已封禁 IP：直接断开式响应（不再重复记录，避免放大日志）
	if IsBanned(c.ClientIP()) {
		c.Status(http.StatusTooManyRequests)
		return
	}

	_, _ = Record(set, c, d)

	// 高等级可配置延迟（拖住自动化；上限 2s，避免被用来耗 goroutine）
	if set.Level == LevelHigh && set.DelayMs > 0 {
		d2 := slowDelay(set)
		if d2 > 2*time.Second {
			d2 = 2 * time.Second
		}
		select {
		case <-time.After(d2):
		case <-c.Request.Context().Done():
			return
		}
	}

	for k, v := range d.Headers {
		c.Header(k, v)
	}
	c.Header("X-Robots-Tag", "noindex")
	c.String(http.StatusOK, d.Content)
}

// serveProbe 探测路径：记录但不给内容（与真实 404 一致）
func serveProbe(c *gin.Context, path string) {
	set := Current()
	if set.Enabled && set.Level != LevelOff && set.Level != LevelLow {
		ip := c.ClientIP()
		if IsBanned(ip) {
			c.Status(http.StatusTooManyRequests)
			return
		}
		fields := BuildLogFields(set, c)
		now := time.Now()
		sess, err := ensureSession(ip, fields.UA, fields.Referer, fields.XFF, now)
		if err == nil {
			ev := &Event{
				SessionID: sess.ID, Kind: EventDecoyProbe, Decoy: "probe",
				Method: c.Request.Method, Path: truncate(path, 512),
				Query: fields.Query, UA: fields.UA, Referer: fields.Referer, XFF: fields.XFF,
				Headers: fields.Headers, BodySnippet: fields.BodySnippet,
				Risk: 40, Action: "record", CreatedAt: now,
			}
			_ = db().Create(ev)
			_ = updateSessionStats(sess, 40, now)
		}
	}
	utils.NotFound(c, "页面不存在")
}

// ProbeNotFound SPA 兜底前的探路记录（NoRoute 里调用；不改变原响应）
func ProbeNotFound(c *gin.Context) {
	set := Current()
	if !set.Enabled || set.Level == LevelOff {
		return
	}
	p := c.Request.URL.Path
	if strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/assets/") {
		return
	}
	// 只有「像在探路」的路径才记（含扩展名或敏感关键字），避免把正常 404 全记下来
	suspicious := false
	for _, kw := range []string{".php", ".env", ".git", "admin", "backup", "config", "actuator", "swagger", "phpmyadmin", "shell", "upload"} {
		if strings.Contains(strings.ToLower(p), kw) {
			suspicious = true
			break
		}
	}
	if !suspicious {
		return
	}
	ip := c.ClientIP()
	if IsBanned(ip) {
		return
	}
	fields := BuildLogFields(set, c)
	now := time.Now()
	sess, err := ensureSession(ip, fields.UA, fields.Referer, fields.XFF, now)
	if err != nil {
		return
	}
	ev := &Event{
		SessionID: sess.ID, Kind: EventDecoyProbe, Decoy: "not_found",
		Method: c.Request.Method, Path: truncate(p, 512),
		Query: fields.Query, UA: fields.UA, Referer: fields.Referer, XFF: fields.XFF,
		Headers: fields.Headers, Risk: 35, Action: "record", CreatedAt: now,
	}
	_ = db().Create(ev)
	_ = updateSessionStats(sess, 35, now)
}
