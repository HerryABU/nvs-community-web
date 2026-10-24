package honeypot

// ==================== 影子认证（M2：先甜后卡） ====================
// 攻击者用诱饵 .env 里的密钥伪造 token → 蜜罐内验签通过 → 返回合成数据（甜）；
// 一旦碰写入/越权/提权 → 返回「检测到异地登录，请重新验证」并**恒失败**（卡），
// 同时把「改了什么字段（sub/role/is_admin）、用什么算法」记进事件，作为取证素材。
//
// 路由前缀用 /api/v2/*：真实站点没有 v2，不会与主站路由冲突；
// 对攻击者而言「发现新版 API」本身就是甜头。

import (
	"net/http"
	"strings"
	"time"

	"nvs-server/models"

	"github.com/gin-gonic/gin"
)

// 事件类型
const (
	EventJWTForged  = "jwt_forged"    // 伪造/复用 token
	EventPrivilege  = "privilege_ops" // 越权/写入/提权尝试
	EventBlocked    = "blocked_action"
	EventVerifyPage = "verify_page"
	EventVerifyTry  = "verify_attempt"
)

// ShadowRoute 影子路由注册
func RegisterShadow(r *gin.Engine) {
	g := r.Group("/api/v2")
	g.Use(func(c *gin.Context) {
		set := Current()
		if !set.Enabled || set.Level == LevelOff {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"code": 4, "message": "接口不存在"})
			return
		}
		c.Next()
	})

	// 甜：读接口返回合成数据
	g.GET("/user/me", shadowMe)
	g.GET("/user/profile", shadowMe)
	g.GET("/admin/stats", shadowStats)
	g.GET("/admin/users", shadowUsers)

	// 卡：写入/提权 → 身份异常 + 验证页
	g.POST("/admin/users", shadowDangerous("create_user"))
	g.PUT("/admin/users/:id", shadowDangerous("update_user"))
	g.POST("/admin/users/:id/role", shadowDangerous("grant_role"))
	g.DELETE("/admin/users/:id", shadowDangerous("delete_user"))
	g.POST("/admin/settings", shadowDangerous("update_settings"))
	g.POST("/admin/export", shadowDangerous("export_data"))
	g.POST("/user/password", shadowDangerous("change_password"))
}

// shadowMe 甜头：返回合成用户（字段像真的，但没有一条能用于真实站点）
func shadowMe(c *gin.Context) {
	tok := TokenFromRequest(c)
	claims, err := VerifyShadow(tok)
	if err != nil {
		// 没带/带错：也算探测，记一条但不拦（保持「服务是活的」错觉）
		recordShadow(c, EventJWTForged, "invalid_token", 45, err.Error())
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": gin.H{"user_id": 1, "username": "admin", "email": "admin@localhost", "role": "user"},
		})
		return
	}
	forged := claims.HID == "" || claims.Role == "admin" || claims.IsAdmin
	risk := 50
	if forged {
		risk = 65 // 改了 role/is_admin 或没有会话标记 → 明显的伪造特征
	}
	recordShadow(c, EventJWTForged, "shadow_login", risk,
		"role="+claims.Role+" is_admin="+boolStr(claims.IsAdmin))

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"user_id":    claims.UserID,
			"username":   "admin",
			"email":      claims.Email,
			"role":       claims.Role,
			"is_admin":   true,
			"last_login": time.Now().Add(-3 * time.Hour).Format(time.RFC3339),
			"notice":     "系统将于今日 23:00 例行维护",
		},
	})
}

// shadowStats 甜：合成统计
func shadowStats(c *gin.Context) {
	recordShadow(c, EventJWTForged, "admin_stats", 60, "读取后台统计")
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"users": 18342, "posts": 92311, "pending_review": 17, "today_new": 63,
	}})
}

// shadowUsers 甜：合成用户列表（假邮箱，避免误伤真实用户）
func shadowUsers(c *gin.Context) {
	recordShadow(c, EventJWTForged, "admin_users", 62, "列举用户")
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"list": []gin.H{
			{"id": 1, "username": "admin", "email": "admin@example.com", "role": "admin"},
			{"id": 2, "username": "editor01", "email": "editor01@example.com", "role": "editor"},
			{"id": 3, "username": "user_1024", "email": "user_1024@example.com", "role": "user"},
		},
		"total": 18342,
	}})
}

// shadowDangerous 卡口：任何写入/提权都返回「身份异常，请重新验证」，并给验证页地址
func shadowDangerous(action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		recordShadow(c, EventPrivilege, action, 80, "触及写入/提权接口")
		c.JSON(http.StatusForbidden, gin.H{
			"code":    3,
			"message": "检测到异地登录，请重新验证身份",
			"data":    gin.H{"verify_url": "/hp/verify?next=" + c.Request.URL.Path, "action": action},
		})
	}
}

// recordShadow 记录影子认证事件（复用会话聚合）
func recordShadow(c *gin.Context, kind, decoy string, risk int, note string) {
	set := Current()
	if !set.Enabled || set.Level == LevelOff {
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
		SessionID: sess.ID, Kind: kind, Decoy: decoy,
		Method: c.Request.Method, Path: truncate(c.Request.URL.Path, 512),
		Query: fields.Query, UA: fields.UA, Referer: fields.Referer, XFF: fields.XFF,
		Headers: fields.Headers, BodySnippet: truncate(note+" | "+fields.BodySnippet, 600),
		Risk: risk, Action: "record", CreatedAt: now,
	}
	_ = models.DB.Create(ev)
	_ = updateSessionStats(sess, risk, now)

	// 高等级 + 自动封禁：提权尝试直接封
	if set.AutoBan && set.Level == LevelHigh && risk >= 80 {
		Ban(ip, set.BanMinutes)
	}
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// HasForgedAdminClaim 判断 token 是否自称管理员（后台展示用）
func HasForgedAdminClaim(tok string) bool {
	cl, err := VerifyShadow(tok)
	if err != nil {
		return false
	}
	return cl.IsAdmin || strings.EqualFold(cl.Role, "admin")
}
