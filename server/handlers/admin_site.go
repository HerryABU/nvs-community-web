package handlers

// 此文件包含 admin.go 中关于网站配置、隔离墙、远程站点、联邦作品的功能
// 从 admin.go 拆分出来，方便维护

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"nvs-server/config"
	"nvs-server/models"
	"nvs-server/security"
	"nvs-server/utils"
	"nvs-server/version"

	"github.com/gin-gonic/gin"
	"github.com/microcosm-cc/bluemonday"
)

// configEncryptSecret 用机器密钥 AES-256-GCM 加密敏感明文（联邦出站 Key 落库）
func configEncryptSecret(plain string) (string, error) {
	return config.EncryptSecret(plain)
}

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

// isSafeFederatedURL 🔒 SSRF 防护（安全审计 M5）：
// 仅允许 http/https；主机必须可解析，且解析结果不得为回环/内网/链路本地/CGNAT 等私网地址。
// 注意：无法完全防御 DNS rebinding，但可拦截字面内网 IP 与常见私网域名探测。
func isSafeFederatedURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	host := u.Hostname()
	if host == "" {
		return false
	}
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return false
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return false
		}
		if ip4 := ip.To4(); ip4 != nil {
			// 0.0.0.0/8、169.254.0.0/16（链路本地）、100.64.0.0/10（CGNAT）
			if ip4[0] == 0 || (ip4[0] == 169 && ip4[1] == 254) || (ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127) {
				return false
			}
		}
	}
	return true
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

// recordInbound 记录远程站点访问本站公开联邦接口的身份信息（谁连了我）。
// 若对方携带本站签发的联邦 API Key，同时记录 Key 前缀与签发方 —— 被链接的站
// 可以看到对方用的是哪一个 Key、是谁的（双向可见）。
func recordInbound(c *gin.Context, endpoint string) {
	name := c.Query("source_name")
	srcURL := c.Query("source_url")
	keyPrefix := ""
	issuer := ""
	if ident, ok := c.Get(security.CtxKeyIdentity); ok {
		if ki, ok2 := ident.(*models.KeyIdentity); ok2 && ki.ValidKey && ki.Type == "federated" {
			keyPrefix = ki.KeyPrefix
			issuer = ki.IssuerName
			if name == "" {
				name = ki.PeerName
			}
			if srcURL == "" {
				srcURL = ki.IssuerURL
			}
		}
	}
	models.RecordFederatedInbound(name, srcURL, c.ClientIP(), c.Request.UserAgent(), endpoint, keyPrefix, issuer)
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

	// 🔒 安全审计 M5：联邦同步 SSRF 防护——
	// 仅允许 http/https；禁止内网/回环/链路本地地址（防探测内网服务）；短超时防挂起。
	target := strings.TrimRight(site.APIURL, "/") + "/novels?" + query.Encode()
	if !isSafeFederatedURL(target) {
		utils.BadRequest(c, "远程站点地址非法：仅允许 http/https，且不得指向内网/回环地址")
		return
	}
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		utils.InternalError(c, "构造请求失败")
		return
	}
	// 携带本站的联邦出站 Key（对方站签发给本站的 Key）：对方据此识别本站身份并给高配额。
	// 未配置出站 Key 时仍可同步（走对方匿名通道，可能受限流约束）。
	outboundKeys := models.ActiveOutboundKeys(uint(id))
	var sentKey string
	for i := range outboundKeys {
		if plain, derr := outboundKeys[i].DecryptOutboundKey(); derr == nil && plain != "" {
			sentKey = plain
			break
		}
	}
	if sentKey != "" {
		req.Header.Set("X-API-Key", sentKey)
	}
	req.Header.Set("User-Agent", "NVS-Federated/"+version.AppVersion)
	resp, err := client.Do(req)
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

// ============ 联邦站点 API Key（双向可见） ============

// getFederatedSiteOr404 取站点，不存在则已响应 404 并返回 false
func getFederatedSiteOr404(c *gin.Context) (*models.FederatedSite, bool) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var site models.FederatedSite
	if err := models.DB.First(&site, id).Error; err != nil {
		utils.NotFound(c, "站点不存在")
		return nil, false
	}
	return &site, true
}

// ListFederatedSiteKeys GET /api/admin/sites/:id/keys
// inbound 只回前缀（明文仅生成时展示一次）；outbound 回前缀+签发方+状态（明文不回显）。
func ListFederatedSiteKeys(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	site, ok := getFederatedSiteOr404(c)
	if !ok {
		return
	}
	var keys []models.FederatedPeerKey
	models.DB.Where("site_id = ?", site.ID).Order("id DESC").Find(&keys)
	type keyView struct {
		ID          uint      `json:"id"`
		Direction   string    `json:"direction"`
		PeerName    string    `json:"peer_name"`
		KeyPrefix   string    `json:"key_prefix"`
		IssuerName  string    `json:"issuer_name"`
		QuotaPerMin int       `json:"quota_per_min"`
		IsActive    bool      `json:"is_active"`
		LastUsedAt  *time.Time `json:"last_used_at"`
		CreatedAt   time.Time `json:"created_at"`
	}
	views := make([]keyView, 0, len(keys))
	for _, k := range keys {
		views = append(views, keyView{
			ID: k.ID, Direction: k.Direction, PeerName: k.PeerName, KeyPrefix: k.KeyPrefix,
			IssuerName: k.IssuerName, QuotaPerMin: k.QuotaPerMin,
			IsActive: k.IsActive, LastUsedAt: k.LastUsedAt, CreatedAt: k.CreatedAt,
		})
	}
	utils.Success(c, gin.H{"site": site, "keys": views})
}

// CreateFederatedInboundKey POST /api/admin/sites/:id/keys/inbound
// 本站签发给对方站的 Key：对方访问本站时携带 → 本站识别身份并给高配额。
// 完整 Key 仅此一次返回，需管理员手动转交给对方站点。
func CreateFederatedInboundKey(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	site, ok := getFederatedSiteOr404(c)
	if !ok {
		return
	}
	var req struct {
		QuotaPerMin int    `json:"quota_per_min"`
		AllowedIPs  string `json:"allowed_ips"`
	}
	_ = c.ShouldBindJSON(&req)
	cfg := models.LoadAPIRateLimitConfig()
	quota := req.QuotaPerMin
	if quota <= 0 || quota > cfg.Federated.Req {
		quota = cfg.Federated.Req
	}
	full, prefix, hash, err := models.GenerateFederatedKey()
	if err != nil {
		utils.InternalError(c, "生成 Key 失败")
		return
	}
	rec := &models.FederatedPeerKey{
		SiteID:      site.ID,
		Direction:   "inbound",
		PeerName:    site.Name, // 本站视角：对方站名（谁在用这个 Key）
		KeyHash:     hash,
		KeyPrefix:   prefix,
		IssuerName:  models.GetSiteName(), // 签发方 = 本站
		IssuerURL:   models.GetPlatformConfig("site_url"),
		QuotaPerMin: quota,
		AllowedIPs:  strings.TrimSpace(req.AllowedIPs),
		IsActive:    true,
	}
	if err := models.DB.Create(rec).Error; err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.Success(c, gin.H{
		"message": "已生成本站签发给对方站的联邦 Key（完整 Key 仅此一次展示，请转交对方站并填入其「对方给本站的 Key」）",
		"key":     full, "key_prefix": prefix, "id": rec.ID,
		"issuer_name": rec.IssuerName, "quota_per_min": quota,
	})
}

// SaveFederatedOutboundKey POST /api/admin/sites/:id/keys/outbound
// 保存对方站签发给本站的 Key（本站访问对方站时携带，AES 加密落库）。
func SaveFederatedOutboundKey(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	site, ok := getFederatedSiteOr404(c)
	if !ok {
		return
	}
	var req struct {
		Key string `json:"key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请提供对方站签发给本站的 Key")
		return
	}
	plain := strings.TrimSpace(req.Key)
	if len(plain) < 16 || len(plain) > 256 {
		utils.BadRequest(c, "Key 长度非法")
		return
	}
	cipher, err := configEncryptSecret(plain)
	if err != nil {
		utils.InternalError(c, "加密失败")
		return
	}
	prefix := plain
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	rec := &models.FederatedPeerKey{
		SiteID:      site.ID,
		Direction:   "outbound",
		PeerName:    site.Name,
		KeyHash:     models.HashAPIKey(plain), // 唯一索引占位；出站 Key 由对方站验证，本站不做入站校验
		KeyPrefix:   prefix + "…",
		KeyCipher:   cipher,
		IssuerName:  site.Name, // 对方站签发给本站
		QuotaPerMin: 0,         // 出站 Key 由对方站决定配额；本站无本地配额
		IsActive:    true,
	}
	if err := models.DB.Create(rec).Error; err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.Success(c, gin.H{"message": "已保存对方站签发给本站的 Key（同步时自动携带）", "id": rec.ID})
}

// ToggleFederatedKey POST /api/admin/sites/:id/keys/:kid/toggle —— 启用/停用
func ToggleFederatedKey(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	site, ok := getFederatedSiteOr404(c)
	if !ok {
		return
	}
	kid, _ := strconv.ParseUint(c.Param("kid"), 10, 64)
	var key models.FederatedPeerKey
	if err := models.DB.Where("id = ? AND site_id = ?", kid, site.ID).First(&key).Error; err != nil {
		utils.NotFound(c, "Key 不存在")
		return
	}
	models.DB.Model(&key).Update("is_active", !key.IsActive)
	utils.Success(c, gin.H{"message": "已更新状态", "is_active": !key.IsActive})
}

// DeleteFederatedKey DELETE /api/admin/sites/:id/keys/:kid —— 吊销（立即失效）
func DeleteFederatedKey(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	site, ok := getFederatedSiteOr404(c)
	if !ok {
		return
	}
	kid, _ := strconv.ParseUint(c.Param("kid"), 10, 64)
	res := models.DB.Where("id = ? AND site_id = ?", kid, site.ID).Delete(&models.FederatedPeerKey{})
	if res.RowsAffected == 0 {
		utils.NotFound(c, "Key 不存在")
		return
	}
	utils.Success(c, gin.H{"message": "已吊销"})
}

func GetPublicZoneDetail(c *gin.Context) {
	zoneName := c.Param("zone")
	cfg := models.GetWallConfig()
	utils.Success(c, cfg.GetZoneDetail(zoneName))
}
