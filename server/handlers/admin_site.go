package handlers

// 此文件包含 admin.go 中关于网站配置、隔离墙、远程站点、联邦作品的功能
// 从 admin.go 拆分出来，方便维护

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	"github.com/microcosm-cc/bluemonday"
)

// ============ 站长配置 ============

func GetPlatformConfigs(c *gin.Context) {
	if !ensureAdmin(c) { return }
	utils.Success(c, models.GetAllPlatformConfigs())
}

func UpdatePlatformConfig(c *gin.Context) {
	if !ensureAdmin(c) { return }
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil { utils.BadRequest(c, "参数错误"); return }
	allowed := map[string]bool{
		"site_name": true, "vip_enabled": true, "categories": true,
		"email_verify": true, "captcha_enabled": true,
		"smtp_host": true, "smtp_port": true, "smtp_user": true,
		"smtp_password": true, "smtp_from": true,
	}
	for k, v := range req {
		if !allowed[k] { continue }
		if err := models.SetPlatformConfig(k, v); err != nil { utils.InternalError(c, "保存配置失败: "+k); return }
	}
	utils.Success(c, gin.H{"message": "配置已更新"})
}

// ============ 隔离墙配置 ============

func GetWallConfig(c *gin.Context) {
	if !ensureAdmin(c) { return }
	utils.Success(c, models.GetWallConfig())
}

func UpdateWallConfig(c *gin.Context) {
	if !ensureAdmin(c) { return }
	var req models.WallConfig
	if err := c.ShouldBindJSON(&req); err != nil { utils.BadRequest(c, "参数错误"); return }
	if err := models.SetWallConfig(req); err != nil { utils.InternalError(c, "保存配置失败"); return }
	utils.Success(c, gin.H{"message": "隔离墙配置已更新"})
}

// ============ 远程站点互通 ============

func ListFederatedSites(c *gin.Context) {
	if !ensureAdmin(c) { return }
	var sites []models.FederatedSite
	models.DB.Order("id ASC").Find(&sites)
	utils.Success(c, sites)
}

func CreateFederatedSite(c *gin.Context) {
	if !ensureAdmin(c) { return }
	var req struct {
		Name        string `json:"name" binding:"required"`
		URL         string `json:"url" binding:"required"`
		APIURL      string `json:"api_url" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil { utils.BadRequest(c, "请填写完整信息"); return }
	policy := bluemonday.StrictPolicy()
	site := &models.FederatedSite{Name: policy.Sanitize(req.Name), URL: policy.Sanitize(req.URL), APIURL: policy.Sanitize(req.APIURL), Description: policy.Sanitize(req.Description), Status: "active"}
	if err := models.DB.Create(site).Error; err != nil { utils.InternalError(c, "添加失败"); return }
	utils.Success(c, site)
}

func UpdateFederatedSite(c *gin.Context) {
	if !ensureAdmin(c) { return }
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var site models.FederatedSite
	if err := models.DB.First(&site, id).Error; err != nil { utils.NotFound(c, "站点不存在"); return }
	var req struct{ Name, URL, APIURL, Description, Status *string }
	c.ShouldBindJSON(&req)
	policy := bluemonday.StrictPolicy()
	updates := map[string]interface{}{}
	if req.Name != nil { updates["name"] = policy.Sanitize(*req.Name) }
	if req.URL != nil { updates["url"] = policy.Sanitize(*req.URL) }
	if req.APIURL != nil { updates["api_url"] = policy.Sanitize(*req.APIURL) }
	if req.Description != nil { updates["description"] = policy.Sanitize(*req.Description) }
	if req.Status != nil { updates["status"] = policy.Sanitize(*req.Status) }
	if len(updates) > 0 { models.DB.Model(&site).Updates(updates) }
	utils.Success(c, gin.H{"message": "更新成功"})
}

func DeleteFederatedSite(c *gin.Context) {
	if !ensureAdmin(c) { return }
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	models.DB.Where("site_id = ?", id).Delete(&models.FederatedNovel{})
	models.DB.Delete(&models.FederatedSite{}, id)
	utils.Success(c, gin.H{"message": "已删除"})
}

func SyncFederatedSite(c *gin.Context) {
	if !ensureAdmin(c) { return }
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var site models.FederatedSite
	if err := models.DB.First(&site, id).Error; err != nil { utils.NotFound(c, "站点不存在"); return }
	resp, err := http.Get(site.APIURL + "/novels?page_size=50")
	if err != nil { utils.InternalError(c, "连接远程站点失败: "+err.Error()); return }
	defer resp.Body.Close()
	var result struct {
		Code int `json:"code"`
		Data struct {
			List []struct {
				ID uint `json:"id"`; Title, Category, Summary, CoverURL string
				Author struct{ Nickname string `json:"nickname"` } `json:"author"`
			} `json:"list"`
			Total int64 `json:"total"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil { utils.InternalError(c, "解析远程数据失败: "+err.Error()); return }
	synced := 0
	for _, item := range result.Data.List {
		var existing models.FederatedNovel
		if models.DB.Where("site_id = ? AND remote_id = ?", id, item.ID).First(&existing).Error == nil { continue }
		authorName := ""; if item.Author.Nickname != "" { authorName = item.Author.Nickname }
		models.DB.Create(&models.FederatedNovel{SiteID: uint(id), RemoteID: item.ID, Title: item.Title, Category: item.Category, Author: authorName, Summary: item.Summary, CoverURL: item.CoverURL, SourceURL: fmt.Sprintf("%s/novel/%d", site.URL, item.ID), CachedAt: time.Now()})
		synced++
	}
	now := time.Now(); count := int64(0)
	models.DB.Model(&models.FederatedNovel{}).Where("site_id = ?", id).Count(&count)
	models.DB.Model(&site).Updates(map[string]interface{}{"last_sync_at": now, "novel_count": count})
	utils.Success(c, gin.H{"message": "同步完成", "synced": synced, "total": count})
}

func ListPublicSites(c *gin.Context) {
	var sites []models.FederatedSite
	models.DB.Where("status = ?", "active").Order("id ASC").Find(&sites)
	utils.Success(c, sites)
}

func ListFederatedNovels(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1")); pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	siteID, _ := strconv.ParseUint(c.Query("site_id"), 10, 64)
	query := models.DB.Model(&models.FederatedNovel{})
	if siteID > 0 { query = query.Where("site_id = ?", siteID) }
	var total int64; query.Count(&total)
	var list []models.FederatedNovel
	query.Order("cached_at DESC").Offset((page-1)*pageSize).Limit(pageSize).Find(&list)
	utils.Success(c, gin.H{"list": list, "total": total})
}

// ============ 公共 API ============

func GetSiteInfo(c *gin.Context) {
	wallCfg := models.GetWallConfig()
	utils.Success(c, gin.H{"site_name": models.GetSiteName(), "wall_zones": wallCfg.Zones, "wall_enabled": wallCfg.Enabled, "email_verify": models.IsEmailVerifyEnabled(), "captcha_enabled": models.IsCaptchaEnabled()})
}

func GetPublicZoneDetail(c *gin.Context) {
	zoneName := c.Param("zone")
	cfg := models.GetWallConfig()
	utils.Success(c, cfg.GetZoneDetail(zoneName))
}