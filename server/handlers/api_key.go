package handlers

// API Key 体系 handler：
//   - 用户侧：创建/查看/修改/重生成/删除自己的 API Key（配额在管理员角色上限内自设）
//   - 管理员侧：全局各角色配额配置、爬虫规则、限流日志与统计、全站 Key 一览
// 联邦 Key 的管理（/api/admin/sites/:id/keys/*）见 admin_site.go。

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"nvs-server/models"
	"nvs-server/security"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	"github.com/microcosm-cc/bluemonday"
)

// apiKeyRoleForUser 用户角色 → Key 角色（vip_author 归入 author）
func apiKeyRoleForUser(role string) string {
	switch role {
	case "admin":
		return "admin"
	case "author", "vip_author":
		return "author"
	default:
		return "reader"
	}
}

// parseOptionalTime 解析可选时间（支持 RFC3339 / "2006-01-02 15:04:05" / "2006-01-02"；空 = nil）
func parseOptionalTime(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	layouts := []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"}
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l, s, time.Local); err == nil {
			if l == "2006-01-02" {
				// 仅日期 → 当天 23:59:59 过期
				t = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			}
			return &t, nil
		}
	}
	return nil, fmt.Errorf("时间格式无效")
}

// validateAllowedIPs 校验 IP/CIDR 白名单（逗号分隔；空 = 不限制）
func validateAllowedIPs(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "/") {
			if _, _, err := net.ParseCIDR(part); err != nil {
				return fmt.Errorf("网段格式无效: %s", part)
			}
		} else if net.ParseIP(part) == nil {
			return fmt.Errorf("IP 格式无效: %s", part)
		}
	}
	return nil
}

// ==================== 用户侧：我的 API Key ====================

// ListMyAPIKeys GET /api/apikeys —— 我的 Key 列表 + 当前角色配额上限
func ListMyAPIKeys(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}
	var list []models.APIKey
	models.DB.Where("user_id = ?", uid).Order("id DESC").Find(&list)
	role := apiKeyRoleForUser(currentUserRole(c))
	cfg := models.LoadAPIRateLimitConfig()
	quota := cfg.RoleQuotaFor(role)
	utils.Success(c, gin.H{
		"list":       list,
		"role":       role,
		"role_quota": gin.H{"req": quota.Req, "window": quota.Window},
	})
}

// CreateAPIKey POST /api/apikeys —— 创建 Key（完整 Key 仅此一次返回）
func CreateAPIKey(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}
	var req struct {
		Name        string `json:"name"`
		QuotaPerMin int    `json:"quota_per_min"` // 0 = 使用角色上限
		AllowedIPs  string `json:"allowed_ips"`
		ExpiresAt   string `json:"expires_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := validateAllowedIPs(req.AllowedIPs); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	expiresAt, err := parseOptionalTime(req.ExpiresAt)
	if err != nil {
		utils.BadRequest(c, "过期时间格式无效（支持 YYYY-MM-DD / YYYY-MM-DD HH:MM:SS / RFC3339）")
		return
	}
	if expiresAt != nil && expiresAt.Before(time.Now()) {
		utils.BadRequest(c, "过期时间不能早于当前时间")
		return
	}

	role := apiKeyRoleForUser(currentUserRole(c))
	cfg := models.LoadAPIRateLimitConfig()
	roleQuota := cfg.RoleQuotaFor(role)
	quota := req.QuotaPerMin
	if quota <= 0 {
		quota = roleQuota.Req
	}
	if quota > roleQuota.Req {
		utils.BadRequest(c, fmt.Sprintf("请求配额不能超过当前角色上限 %d 次/%d 秒", roleQuota.Req, roleQuota.Window))
		return
	}

	full, prefix, hash, err := models.GenerateAPIKey(role)
	if err != nil {
		utils.InternalError(c, "生成 Key 失败")
		return
	}
	policy := bluemonday.StrictPolicy()
	key := &models.APIKey{
		UserID: uid, Name: policy.Sanitize(strings.TrimSpace(req.Name)),
		Role: role, KeyHash: hash, KeyPrefix: prefix,
		QuotaPerMin: quota, AllowedIPs: strings.TrimSpace(req.AllowedIPs),
		ExpiresAt: expiresAt, IsActive: true,
	}
	if err := models.DB.Create(key).Error; err != nil {
		utils.InternalError(c, "保存 Key 失败")
		return
	}
	utils.Success(c, gin.H{
		"id": key.ID, "key": full, "key_prefix": prefix, "role": role,
		"quota_per_min": quota, "name": key.Name,
	})
}

// UpdateAPIKey PATCH /api/apikeys/:id —— 修改（名称/配额/IP 白名单/过期/启停）
func UpdateAPIKey(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var key models.APIKey
	if err := models.DB.Where("id = ? AND user_id = ?", id, uid).First(&key).Error; err != nil {
		utils.NotFound(c, "Key 不存在")
		return
	}
	var req struct {
		Name        *string `json:"name"`
		QuotaPerMin *int    `json:"quota_per_min"`
		AllowedIPs  *string `json:"allowed_ips"`
		ExpiresAt   *string `json:"expires_at"` // 传空串 = 清除过期
		IsActive    *bool   `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = bluemonday.StrictPolicy().Sanitize(strings.TrimSpace(*req.Name))
	}
	if req.QuotaPerMin != nil {
		roleQuota := models.LoadAPIRateLimitConfig().RoleQuotaFor(key.Role)
		q := *req.QuotaPerMin
		if q <= 0 || q > roleQuota.Req {
			utils.BadRequest(c, fmt.Sprintf("请求配额需在 1 ~ %d 之间", roleQuota.Req))
			return
		}
		updates["quota_per_min"] = q
	}
	if req.AllowedIPs != nil {
		if err := validateAllowedIPs(*req.AllowedIPs); err != nil {
			utils.BadRequest(c, err.Error())
			return
		}
		updates["allowed_ips"] = strings.TrimSpace(*req.AllowedIPs)
	}
	if req.ExpiresAt != nil {
		t, err := parseOptionalTime(*req.ExpiresAt)
		if err != nil {
			utils.BadRequest(c, "过期时间格式无效")
			return
		}
		updates["expires_at"] = t
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if len(updates) > 0 {
		models.DB.Model(&key).Updates(updates)
	}
	utils.Success(c, gin.H{"message": "已更新", "key_prefix": key.KeyPrefix})
}

// RegenerateAPIKey POST /api/apikeys/:id/regenerate —— 重新生成（旧 Key 立即失效）
func RegenerateAPIKey(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var key models.APIKey
	if err := models.DB.Where("id = ? AND user_id = ?", id, uid).First(&key).Error; err != nil {
		utils.NotFound(c, "Key 不存在")
		return
	}
	full, prefix, hash, err := models.GenerateAPIKey(key.Role)
	if err != nil {
		utils.InternalError(c, "生成 Key 失败")
		return
	}
	models.DB.Model(&key).Updates(map[string]interface{}{
		"key_hash": hash, "key_prefix": prefix, "last_used_at": nil,
	})
	utils.Success(c, gin.H{"message": "已重新生成，旧 Key 立即失效", "key": full, "key_prefix": prefix})
}

// DeleteAPIKey DELETE /api/apikeys/:id —— 删除（吊销）
func DeleteAPIKey(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	res := models.DB.Where("id = ? AND user_id = ?", id, uid).Delete(&models.APIKey{})
	if res.RowsAffected == 0 {
		utils.NotFound(c, "Key 不存在")
		return
	}
	utils.Success(c, gin.H{"message": "已删除（吊销）"})
}

// ==================== 管理员侧：全局限流配置 ====================

// GetRateLimitConfig GET /api/admin/ratelimit/config
func GetRateLimitConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	cfg := models.LoadAPIRateLimitConfig()
	utils.Success(c, gin.H{
		"config":                  cfg,
		"builtin_bot_keywords":    security.BuiltinBotKeywords(),
		"builtin_block_keywords":  security.BuiltinBlockKeywords(),
	})
}

// UpdateRateLimitConfig PUT /api/admin/ratelimit/config —— 保存后立即热生效
func UpdateRateLimitConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var cfg models.APIRateLimitConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := models.SaveAPIRateLimitConfig(cfg); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	security.ReloadBotRules()
	utils.Success(c, gin.H{"message": "限流配置已保存并立即生效"})
}

// ListRateLimitLogs GET /api/admin/ratelimit/logs?type=&page=&page_size=
func ListRateLimitLogs(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	idType := strings.TrimSpace(c.Query("type"))
	list, total, err := models.ListAccessLogs(page, pageSize, idType)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{"list": list, "total": total})
}

// GetRateLimitStats GET /api/admin/ratelimit/stats
func GetRateLimitStats(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	utils.Success(c, security.RateLimitStats())
}

// ClearRateLimits POST /api/admin/ratelimit/clear —— 清空内存限流计数（误判解封）
func ClearRateLimits(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	security.ClearRateLimits()
	utils.Success(c, gin.H{"message": "限流计数已清空（如属误判，对应 IP/Key 已解封）"})
}

// AdminListAPIKeys GET /api/admin/ratelimit/keys —— 全站用户 Key 一览（附用户名）
func AdminListAPIKeys(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var keys []models.APIKey
	models.DB.Order("id DESC").Limit(500).Find(&keys)

	// 收集用户名
	userIDs := make([]uint, 0, len(keys))
	for _, k := range keys {
		userIDs = append(userIDs, k.UserID)
	}
	names := map[uint]string{}
	if len(userIDs) > 0 {
		var users []models.User
		models.DB.Where("id IN ?", userIDs).Find(&users)
		for _, u := range users {
			names[u.ID] = u.Username
		}
	}
	type keyRow struct {
		models.APIKey
		Username string `json:"username"`
	}
	rows := make([]keyRow, 0, len(keys))
	for _, k := range keys {
		rows = append(rows, keyRow{APIKey: k, Username: names[k.UserID]})
	}
	utils.Success(c, rows)
}
