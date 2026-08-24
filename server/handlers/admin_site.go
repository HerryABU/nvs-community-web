package handlers

// 此文件包含 admin.go 中关于网站配置、隔离墙、远程站点、联邦作品的功能
// 从 admin.go 拆分出来，方便维护

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	"github.com/microcosm-cc/bluemonday"
)

// ============ 站长配置 ============

func GetPlatformConfigs(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	utils.Success(c, models.GetAllPlatformConfigs())
}

func UpdatePlatformConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	allowed := map[string]bool{
		"site_name": true, "vip_enabled": true, "categories": true,
		"email_verify": true, "captcha_enabled": true,
		"smtp_host": true, "smtp_port": true, "smtp_user": true,
		"smtp_password": true, "smtp_from": true,
		"smtp_insecure_skip_verify": true, // 跳过 TLS 证书校验（内网/自签名 SMTP，默认关闭）
		"robots_txt":                true,
		"guidelines":                true, // 平台指南（注册页展示，支持基础 HTML 排版）
		"email_domain_whitelist":    true, "email_domain_blacklist": true,
		// 联网备案（176号令 第七条第一项）
		"site_icp": true, "site_psb": true, "site_psb_link": true,
	}
	for k, v := range req {
		if !allowed[k] {
			continue
		}
		// 平台指南是 HTML 内容，保存时用宽松策略消毒，防存储型 XSS
		if k == "guidelines" {
			policy := bluemonday.UGCPolicy()
			policy.AllowElements("h1", "h2", "h3", "h4", "h5", "p", "ul", "ol", "li", "strong", "b", "em", "i", "u", "br", "hr", "blockquote", "a")
			policy.AllowAttrs("href", "target", "rel").OnElements("a")
			v = policy.Sanitize(v)
		}
		if err := models.SetPlatformConfig(k, v); err != nil {
			utils.InternalError(c, "保存配置失败: "+k)
			return
		}
	}
	utils.Success(c, gin.H{"message": "配置已更新"})
}

// ============ SMTP 测试与诊断 ============

// TestSMTP 测试 SMTP 配置并逐阶段诊断（管理员专用）。
// POST /api/admin/smtp/test
// 入参：to 必填（测试收件邮箱）；host/port/user/password/from 可选，
// 为空时使用已保存的平台配置（password 为空时沿用已保存的授权码，不回显）。
func TestSMTP(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		To       string `json:"to" binding:"required,email"`
		Host     string `json:"host"`
		Port     string `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
		From     string `json:"from"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请提供有效的测试收件邮箱")
		return
	}

	// 优先取已保存的平台配置，请求内的非空字段覆盖之
	host, port, user, pass, from := models.GetSMTPConfigFromDB()
	if req.Host != "" {
		host = req.Host
	}
	if req.Port != "" {
		port = req.Port
	}
	if req.User != "" {
		user = req.User
	}
	if req.Password != "" {
		pass = req.Password
	}
	if req.From != "" {
		from = req.From
	}
	portInt := 587
	if p, err := strconv.Atoi(port); err == nil {
		portInt = p
	}
	cfg := utils.SMTPConfig{
		Host: host, Port: fmt.Sprintf("%d", portInt), User: user, Password: pass, From: from,
		InsecureSkipVerify: models.GetPlatformConfig("smtp_insecure_skip_verify") == "true",
	}

	steps, ok := utils.DiagnoseSMTP(cfg, strings.TrimSpace(strings.ToLower(req.To)))
	msg := "SMTP 测试通过，邮件已发送"
	if !ok {
		msg = "SMTP 测试未通过，请根据下方诊断定位问题"
	}
	utils.Success(c, gin.H{"ok": ok, "steps": steps, "message": msg})
}

// ============ 隔离墙配置 ============

func GetWallConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	utils.Success(c, models.GetWallConfig())
}

func UpdateWallConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req models.WallConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := models.SetWallConfig(req); err != nil {
		utils.InternalError(c, "保存配置失败")
		return
	}
	utils.Success(c, gin.H{"message": "隔离墙配置已更新"})
}

// ============ 远程站点互通 ============

func ListFederatedSites(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var sites []models.FederatedSite
	models.DB.Order("id ASC").Find(&sites)
	utils.Success(c, sites)
}

func CreateFederatedSite(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		Name        string `json:"name" binding:"required"`
		URL         string `json:"url" binding:"required"`
		APIURL      string `json:"api_url" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请填写完整信息")
		return
	}
	policy := bluemonday.StrictPolicy()
	site := &models.FederatedSite{Name: policy.Sanitize(req.Name), URL: policy.Sanitize(req.URL), APIURL: policy.Sanitize(req.APIURL), Description: policy.Sanitize(req.Description), Status: "active"}
	if err := models.DB.Create(site).Error; err != nil {
		utils.InternalError(c, "添加失败")
		return
	}
	utils.Success(c, site)
}

func UpdateFederatedSite(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var site models.FederatedSite
	if err := models.DB.First(&site, id).Error; err != nil {
		utils.NotFound(c, "站点不存在")
		return
	}
	var req struct{ Name, URL, APIURL, Description, Status *string }
	c.ShouldBindJSON(&req)
	policy := bluemonday.StrictPolicy()
	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = policy.Sanitize(*req.Name)
	}
	if req.URL != nil {
		updates["url"] = policy.Sanitize(*req.URL)
	}
	if req.APIURL != nil {
		updates["api_url"] = policy.Sanitize(*req.APIURL)
	}
	if req.Description != nil {
		updates["description"] = policy.Sanitize(*req.Description)
	}
	if req.Status != nil {
		updates["status"] = policy.Sanitize(*req.Status)
	}
	if len(updates) > 0 {
		models.DB.Model(&site).Updates(updates)
	}
	utils.Success(c, gin.H{"message": "更新成功"})
}

func DeleteFederatedSite(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	models.DB.Where("site_id = ?", id).Delete(&models.FederatedNovel{})
	models.DB.Delete(&models.FederatedSite{}, id)
	utils.Success(c, gin.H{"message": "已删除"})
}

// selfPublicURL 返回本站对外访问地址（供联邦互通报给对方识别是谁在连）。
// 优先使用站长配置的 site_url，否则从当前请求的 Host + Scheme 推导。
func selfPublicURL(c *gin.Context) string {
	if cfg := models.GetPlatformConfig("site_url"); cfg != "" {
		return strings.TrimRight(cfg, "/")
	}
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if xfp := c.GetHeader("X-Forwarded-Proto"); xfp != "" {
		scheme = xfp
	}
	host := c.Request.Host
	if host == "" {
		host = "localhost"
	}
	return scheme + "://" + host
}

// recordInbound 记录远程站点访问本站公开联邦接口的身份信息（谁连了我）
func recordInbound(c *gin.Context, endpoint string) {
	name := c.Query("source_name")
	srcURL := c.Query("source_url")
	models.RecordFederatedInbound(name, srcURL, c.ClientIP(), c.Request.UserAgent(), endpoint)
}

func SyncFederatedSite(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var site models.FederatedSite
	if err := models.DB.First(&site, id).Error; err != nil {
		utils.NotFound(c, "站点不存在")
		return
	}
	// 向对方自报身份，让被连的站能看到是谁在连它
	query := url.Values{}
	query.Set("page_size", "50")
	query.Set("source_name", models.GetSiteName())
	query.Set("source_url", selfPublicURL(c))
	// 同步对方站的「真实作品列表」：调用其公开接口 /api/novels（返回 author 为 User 对象）。
	// API 地址应为对方站 API 根地址（以 /api 结尾），此处拼接 /novels 即命中 /api/novels。
	resp, err := http.Get(strings.TrimRight(site.APIURL, "/") + "/novels?" + query.Encode())
	if err != nil {
		utils.InternalError(c, "连接远程站点失败: "+err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		utils.InternalError(c, fmt.Sprintf("远程站点返回异常状态码 %d（请确认 API 地址填到对方站 /api 根地址）", resp.StatusCode))
		return
	}
	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			List []struct {
				ID       uint   `json:"id"`
				Title    string `json:"title"`
				Category string `json:"category"`
				Summary  string `json:"summary"`
				CoverURL string `json:"cover_url"`
				Author   struct {
					Nickname string `json:"nickname"`
				} `json:"author"`
			} `json:"list"`
			Total int64 `json:"total"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		utils.InternalError(c, "解析远程数据失败: "+err.Error())
		return
	}
	if result.Code != 0 {
		msg := result.Message
		if msg == "" {
			msg = "远程站点返回错误"
		}
		utils.InternalError(c, msg)
		return
	}
	synced := 0
	for _, item := range result.Data.List {
		if item.ID == 0 {
			continue
		}
		var existing models.FederatedNovel
		if models.DB.Where("site_id = ? AND remote_id = ?", id, item.ID).First(&existing).Error == nil {
			continue
		}
		authorName := ""
		if item.Author.Nickname != "" {
			authorName = item.Author.Nickname
		}
		models.DB.Create(&models.FederatedNovel{SiteID: uint(id), RemoteID: item.ID, Title: item.Title, Category: item.Category, Author: authorName, Summary: item.Summary, CoverURL: item.CoverURL, SourceURL: fmt.Sprintf("%s/novel/%d", site.URL, item.ID), CachedAt: time.Now()})
		synced++
	}
	now := time.Now()
	count := int64(0)
	models.DB.Model(&models.FederatedNovel{}).Where("site_id = ?", id).Count(&count)
	models.DB.Model(&site).Updates(map[string]interface{}{"last_sync_at": now, "novel_count": count})
	utils.Success(c, gin.H{"message": "同步完成", "synced": synced, "total": count})
}

func ListPublicSites(c *gin.Context) {
	recordInbound(c, "federated/sites")
	var sites []models.FederatedSite
	models.DB.Where("status = ?", "active").Order("id ASC").Find(&sites)
	utils.Success(c, sites)
}

func ListFederatedNovels(c *gin.Context) {
	recordInbound(c, "federated/novels")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	siteID, _ := strconv.ParseUint(c.Query("site_id"), 10, 64)
	query := models.DB.Model(&models.FederatedNovel{})
	if siteID > 0 {
		query = query.Where("site_id = ?", siteID)
	}
	var total int64
	query.Count(&total)
	var list []models.FederatedNovel
	query.Order("cached_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list)
	utils.Success(c, gin.H{"list": list, "total": total})
}

// ============ 公共 API ============

func GetSiteInfo(c *gin.Context) {
	recordInbound(c, "site-info")
	wallCfg := models.GetWallConfig()
	// 平台指南：公开可读（注册页展示），消毒后返回
	guidelines := models.GetPlatformConfig("guidelines")
	if guidelines != "" {
		guidelines = bluemonday.UGCPolicy().Sanitize(guidelines)
	}
	// 联网备案信息（176号令 第七条第一项；已配置才返回）
	icp, psb, psbLink := models.GetSiteFiling()
	utils.Success(c, gin.H{
		"site_name": models.GetSiteName(), "wall_zones": wallCfg.Zones, "wall_enabled": wallCfg.Enabled,
		"email_verify": models.IsEmailVerifyEnabled(), "captcha_enabled": models.IsCaptchaEnabled(),
		"guidelines": guidelines,
		"site_icp":   icp, "site_psb": psb, "site_psb_link": psbLink,
	})
}

// ListInboundSites 列出「谁连接了我」的入站记录（管理员）
func ListInboundSites(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	list, err := models.ListFederatedInbound(200)
	if err != nil {
		utils.InternalError(c, "获取入站连接记录失败")
		return
	}
	utils.Success(c, list)
}

func GetPublicZoneDetail(c *gin.Context) {
	zoneName := c.Param("zone")
	cfg := models.GetWallConfig()
	utils.Success(c, cfg.GetZoneDetail(zoneName))
}
