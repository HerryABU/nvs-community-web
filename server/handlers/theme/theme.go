package theme

import (
	"nvs-server/handlers/admincommon"
	"nvs-server/handlers/hctx"
	"nvs-server/utils/infra"

	"github.com/gin-gonic/gin"
	themem "nvs-server/models/theme"
)

// theme.go — F12 用户自定义网站主题（alist 式）API。
// 安全模型：写入一律经服务端白名单消毒（sanitize.SanitizeCSS）后落库；
// 前端仅以 <style> textContent 注入，不使用 innerHTML。

// GetSiteTheme GET /api/theme/site（公开）
// 站点默认主题：管理员配置，对所有访客生效（登录用户的个人主题覆盖之）。
func GetSiteTheme(c *gin.Context) {
	js, mode, delay := themem.GetSiteThemeJSBundle()
	settingsRaw, settings := themem.GetSiteThemeSettings()
	infra.Success(c, gin.H{
		"css":          themem.GetSiteTheme(),
		"js":           js,
		"js_mode":      mode,
		"js_delay":     delay,
		"settings":     settingsRaw,
		"settings_obj": settings,
	})
}

// GetMyTheme GET /api/theme/me（需登录）
// 当前用户的个人主题 CSS/JS；未配置返回空串。
func GetMyTheme(c *gin.Context) {
	userID := hctx.CurrentUserID(c)
	if userID == 0 {
		infra.Unauthorized(c, "未登录")
		return
	}
	js, mode, delay := themem.GetUserThemeJSBundle(userID)
	infra.Success(c, gin.H{"css": themem.GetUserTheme(userID), "js": js, "js_mode": mode, "js_delay": delay})
}

// UpdateMyTheme PUT /api/theme/me（需登录）
// 保存个人主题：服务端强制消毒，返回消毒后的 CSS/JS 与被移除的规则列表。
func UpdateMyTheme(c *gin.Context) {
	userID := hctx.CurrentUserID(c)
	if userID == 0 {
		infra.Unauthorized(c, "未登录")
		return
	}
	var req struct {
		CSS     string `json:"css"`
		JS      string `json:"js"`
		JSMode  string `json:"js_mode"`  // immediate / idle / delayed（alist「延迟加载」）
		JSDelay int    `json:"js_delay"` // delayed 模式的延迟秒数
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	removed, err := themem.SetUserTheme(userID, req.CSS, req.JS, req.JSMode, req.JSDelay)
	if err != nil {
		infra.InternalError(c, "保存个人主题失败")
		return
	}
	js, mode, delay := themem.GetUserThemeJSBundle(userID)
	infra.Success(c, gin.H{"css": themem.GetUserTheme(userID), "js": js, "js_mode": mode, "js_delay": delay, "removed": removed})
}

// UpdateSiteTheme PUT /api/theme/site（仅管理员）
// 保存站点默认主题：对全部访客生效，需二次确认的醒目提示由前端负责。
func UpdateSiteTheme(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	var req struct {
		CSS     string `json:"css"`
		JS      string `json:"js"`
		JSMode  string `json:"js_mode"`  // immediate / idle / delayed（alist「延迟加载」）
		JSDelay int    `json:"js_delay"` // delayed 模式的延迟秒数
		// 结构化主题设置（可视化控件生成的 JSON 字符串；与原始 CSS 互补）
		Settings string `json:"settings"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	removed, err := themem.SetSiteTheme(req.CSS, req.JS, req.JSMode, req.JSDelay, req.Settings)
	if err != nil {
		infra.InternalError(c, "保存站点主题失败")
		return
	}
	js, mode, delay := themem.GetSiteThemeJSBundle()
	settingsRaw, _ := themem.GetSiteThemeSettings()
	infra.Success(c, gin.H{"css": themem.GetSiteTheme(), "js": js, "js_mode": mode, "js_delay": delay, "settings": settingsRaw, "removed": removed})
}
