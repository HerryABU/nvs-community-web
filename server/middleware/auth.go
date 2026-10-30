package middleware

import (
	"net/http"
	"strconv"

	"nvs-server/config"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// DeviceFingerprintHeader 前端采集的设备指纹通过该请求头上报（不参与签名密钥，
// 仅用于把访问令牌与设备绑定，防 JWT 窃取后异地/异设备盗用）。
const DeviceFingerprintHeader = "X-Device-Fingerprint"

// tokenBindingMatches 校验访问令牌的绑定签名是否与「当前请求的设备指纹 + 登录地」一致。
// 返回 true 表示放行：
//   - 令牌无绑定（升级前遗留令牌）一律放行（向后兼容）；
//   - 当前设备指纹 + 登录地与令牌签发时一致 → 放行；
//   - 登录地解析失败（geo 抖动）时，放宽到「仅比对设备指纹」以降低误伤。
func tokenBindingMatches(claims *utils.Claims, c *gin.Context) bool {
	if claims.Binding == "" {
		return true // 遗留令牌：未绑定，向后兼容放行
	}
	fp := c.GetHeader(DeviceFingerprintHeader)
	region := utils.RegionForBinding(c.ClientIP())
	if utils.ComputeBinding(fp, region) == claims.Binding {
		return true
	}
	// geo 抖动兜底：本次 IP 归属地解析失败（region 为空），放宽到仅比对指纹，避免误踢在线用户。
	if region == "" && utils.ComputeBinding(fp, "") == claims.Binding {
		return true
	}
	return false
}

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := c.Cookie("token")
		if err != nil {
			utils.Unauthorized(c, "未登录")
			c.Abort()
			return
		}

		claims, err := utils.ParseToken(tokenString)
		if err != nil {
			utils.Unauthorized(c, "Token 无效或已过期")
			c.Abort()
			return
		}

		// 🔒 令牌绑定校验：访问令牌与「设备指纹 + 登录地」强绑定，
		// 窃取到的令牌在异设备 / 异省份使用会被 401 拒绝（防 JWT 盗用）。
		if !tokenBindingMatches(claims, c) {
			utils.Unauthorized(c, "登录环境已变更（设备/地区），请重新登录")
			c.Abort()
			return
		}

		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxUserEmail, claims.Email)
		c.Set(CtxUserRole, claims.Role)
		c.Next()
	}
}

// OptionalAuth 可选登录中间件：解析 token 并设置用户上下文（user id/email/role），
// 解析失败不拦截请求。用于公开路由（如论坛详情）识别登录态，
// 使管理员/创建者等权限判断在公开接口中也能生效（如管理员进论坛免密码，176号令合规）。
// 绑定校验失败时当作匿名（不设置上下文、不拦截），公开页不应因绑定不匹配而整页报错。
func OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if tokenString, err := c.Cookie("token"); err == nil && tokenString != "" {
			if claims, perr := utils.ParseToken(tokenString); perr == nil {
				if tokenBindingMatches(claims, c) {
					c.Set(CtxUserID, claims.UserID)
					c.Set(CtxUserEmail, claims.Email)
					c.Set(CtxUserRole, claims.Role)
				}
			}
		}
		c.Next()
	}
}

// AuthorRequired 要求作者角色（author / vip_author）
func AuthorRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		roleStr := UserRole(c)
		if roleStr == "" {
			utils.Forbidden(c, "需要作者权限")
			c.Abort()
			return
		}

		if roleStr != "author" && roleStr != "vip_author" && roleStr != "admin" {
			utils.Forbidden(c, "需要作者权限")
			c.Abort()
			return
		}
		c.Next()
	}
}

// AdminRequired 要求管理员角色（admin）。用于 /api/admin 组级纵深防御，
// 防止「新增 handler 漏写 ensureAdmin」导致越权。需配合 AuthRequired 或 OptionalAuth 使用
// （上下文里的 user id/role 由前置中间件写入）。
func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		role := UserRole(c)
		if role != "admin" {
			utils.Forbidden(c, "需要管理员权限")
			c.Abort()
			return
		}
		c.Next()
	}
}

// PoliceRequired 要求警方或管理员角色（police / admin）。用于 /api/police 取管台组级纵深防御。
// 管理员默认具备警方台全部查看能力（仅取证/查看软删，无普通管理写权限由具体 handler 控制）。
func PoliceRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		role := UserRole(c)
		if role != "admin" && role != "police" {
			utils.Forbidden(c, "需要警方或管理员权限")
			c.Abort()
			return
		}
		c.Next()
	}
}

// SetTokenCookie 设置 JWT Cookie
func SetTokenCookie(c *gin.Context, token string) {
	// 访问令牌短期有效（默认 15 分钟，见 JWTAccessExpireMinutes），Cookie 生命周期与令牌保持一致，
	// 过期后由 HttpOnly 刷新令牌无感续期；即便令牌被窃取也仅在极短窗口内可用。
	expireMin, err := strconv.Atoi(config.JWTAccessExpireMinutes)
	if err != nil || expireMin <= 0 {
		expireMin = 15
	}
	// 安全审计 L3：Secure 由 .env 的 COOKIE_SECURE 控制（生产 HTTPS 下应开启，防明文截获）
	c.SetCookie(
		"token",
		token,
		expireMin*60, // 分钟 → 秒
		"/",
		"",
		config.ForceCookieSecure, // Secure: 生产 HTTPS 下由 COOKIE_SECURE=true 开启
		true,                     // HttpOnly
	)
	c.SetSameSite(http.SameSiteLaxMode)
}

// ClearTokenCookie 清除 JWT Cookie
func ClearTokenCookie(c *gin.Context) {
	c.SetCookie(
		"token",
		"",
		-1,
		"/",
		"",
		config.ForceCookieSecure,
		true, // HttpOnly
	)
	c.SetSameSite(http.SameSiteLaxMode)
}

// SetRefreshCookie 设置长期刷新令牌 Cookie（HttpOnly + SameSite=Lax，与访问令牌同安全属性）。
// 有效期取 JWTRefreshExpireHours（默认 720h），仅用于 /api/auth/refresh 无感续期。
func SetRefreshCookie(c *gin.Context, token string) {
	expireHours, err := strconv.Atoi(config.JWTRefreshExpireHours)
	if err != nil || expireHours <= 0 {
		expireHours = 720
	}
	c.SetCookie(
		"refresh_token",
		token,
		expireHours*3600,
		"/",
		"",
		config.ForceCookieSecure, // Secure: 生产 HTTPS 下由 COOKIE_SECURE=true 开启
		true,                     // HttpOnly：禁止 JS 读取，防 XSS 窃取
	)
	c.SetSameSite(http.SameSiteLaxMode)
}

// ClearRefreshCookie 清除刷新令牌 Cookie（登出 / 刷新令牌失效时调用）
func ClearRefreshCookie(c *gin.Context) {
	c.SetCookie(
		"refresh_token",
		"",
		-1,
		"/",
		"",
		config.ForceCookieSecure,
		true, // HttpOnly
	)
	c.SetSameSite(http.SameSiteLaxMode)
}
