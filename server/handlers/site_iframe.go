package handlers

import (
	"net/url"
	"strings"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// site_iframe.go — G3 全站 opt-in 嵌入 iframe（站长 allowlist，沙箱渲染）。
//
// 安全模型：
//  1. 嵌入地址只允许 http/https 绝对地址，拒绝 javascript:/data:/file:/about: 等危险协议；
//  2. 地址串存于 platform_config（site_global_iframe_url / site_global_iframe_enabled），
//     任何人可读取「启用状态 + URL」，写入仅管理员；
//  3. 前端渲染时必须带 sandbox（见下，至多 allow-scripts allow-same-origin），绝不允许 admin 注入任意 HTML/JS；
//  4. 服务端只回显经校验合法的 URL，杜绝存储型脏数据被直接塞进 src。

const (
	SiteGlobalIframeURLKey     = "site_global_iframe_url"
	SiteGlobalIframeEnabledKey = "site_global_iframe_enabled"
)

// GetSiteIframe GET /api/site-iframe（公开）
// 返回全站 opt-in iframe 的启用状态与 allowlist URL（空 URL / 非法 URL 视作未配置）。
func GetSiteIframe(c *gin.Context) {
	enabled := models.GetPlatformConfig(SiteGlobalIframeEnabledKey) == "true"
	raw := models.GetPlatformConfig(SiteGlobalIframeURLKey)
	// 仅回显经服务端校验合法的 http/https URL；不合法一律视作未配置，enabled 退化为 false
	safe := ""
	if enabled && isValidIframeURL(raw) {
		safe = raw
	}
	utils.Success(c, gin.H{"enabled": safe != "", "url": safe})
}

// UpdateSiteIframe PUT /api/site-iframe（仅管理员）
// 配置单一 allowlist 嵌入 URL 与启用开关，服务端强制校验 URL 协议白名单。
func UpdateSiteIframe(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		URL     string `json:"url"`
		Enabled bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	req.URL = strings.TrimSpace(req.URL)
	if req.Enabled {
		if req.URL == "" {
			utils.BadRequest(c, "启用 iframe 时必须填写嵌入地址")
			return
		}
		if !isValidIframeURL(req.URL) {
			utils.BadRequest(c, "嵌入地址必须为 http/https 链接，且不可为 javascript:/data:/file: 等危险协议")
			return
		}
	}
	if err := models.SetPlatformConfig(SiteGlobalIframeEnabledKey, boolToStr(req.Enabled)); err != nil {
		utils.InternalError(c, "保存启用状态失败")
		return
	}
	if err := models.SetPlatformConfig(SiteGlobalIframeURLKey, req.URL); err != nil {
		utils.InternalError(c, "保存嵌入地址失败")
		return
	}
	safe := ""
	if req.Enabled && isValidIframeURL(req.URL) {
		safe = req.URL
	}
	utils.Success(c, gin.H{"enabled": safe != "", "url": safe})
}

// isValidIframeURL 仅放行 http/https 绝对地址（含主机名），拒绝一切危险协议与裸协议。
func isValidIframeURL(raw string) bool {
	if raw == "" {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	if u.Host == "" {
		return false
	}
	return true
}

func boolToStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
