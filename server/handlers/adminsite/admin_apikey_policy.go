package adminsite

// 管理员对 API Key 的「按把 / 按人」管控：
//   - 按把：封禁 / 解封某一个人名下的某一把 Key（api_keys.is_banned）
//   - 按人：封禁该用户全部 API Key（含以后新建的）+ 为该用户设置专属配额（限流）
//
// 与既有能力的关系：
//   - 角色全局配额（admin/author/reader/federated）见 UpdateRateLimitConfig；
//   - 本文件的「按人配额」是叠加在角色上限之上的收紧层，取 min(角色上限, 按人配额)；

import (
	"nvs-server/handlers/admincommon"
	"nvs-server/utils/infra"
	"strconv"
	"strings"
	"time"

	"nvs-server/handlers/hctx"
	"nvs-server/models"

	"github.com/gin-gonic/gin"
	apikey "nvs-server/models/apikey"
)

// ==================== 按把封禁 ====================

// AdminBanAPIKey POST /api/admin/ratelimit/keys/:id/ban —— 封禁指定 Key
func AdminBanAPIKey(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var key apikey.APIKey
	if err := models.DB.First(&key, id).Error; err != nil {
		infra.NotFound(c, "Key 不存在")
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "管理员封禁"
	}
	now := time.Now()
	models.DB.Model(&key).Updates(map[string]interface{}{
		"is_banned":  true,
		"ban_reason": reason,
		"banned_at":  now,
		"banned_by":  hctx.CurrentUserID(c),
	})
	infra.Success(c, gin.H{"message": "已封禁该 API Key", "key_prefix": key.KeyPrefix})
}

// AdminUnbanAPIKey POST /api/admin/ratelimit/keys/:id/unban —— 解封指定 Key
func AdminUnbanAPIKey(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var key apikey.APIKey
	if err := models.DB.First(&key, id).Error; err != nil {
		infra.NotFound(c, "Key 不存在")
		return
	}
	models.DB.Model(&key).Updates(map[string]interface{}{
		"is_banned":  false,
		"ban_reason": "",
		"banned_at":  nil,
		"banned_by":  0,
	})
	infra.Success(c, gin.H{"message": "已解封该 API Key", "key_prefix": key.KeyPrefix})
}

// ==================== 按人封禁 ====================

// AdminBanUserAPIKeys POST /api/admin/ratelimit/users/:id/keys/ban —— 封禁某用户的全部 API Key
// duration_hours：封禁时长（小时），0 或留空 = 永久封禁
func AdminBanUserAPIKeys(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	uid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if uid == 0 {
		infra.BadRequest(c, "用户 ID 无效")
		return
	}
	if _, err := models.GetUserByID(uint(uid)); err != nil {
		infra.NotFound(c, "用户不存在")
		return
	}
	var req struct {
		Reason        string `json:"reason"`
		DurationHours int    `json:"duration_hours"`
	}
	_ = c.ShouldBindJSON(&req)

	p := apikey.GetUserAPIPolicy(uint(uid))
	if p == nil {
		p = &apikey.UserAPIPolicy{UserID: uint(uid)}
	}
	p.KeysBanned = true
	p.BanReason = strings.TrimSpace(req.Reason)
	if p.BanReason == "" {
		p.BanReason = "管理员封禁"
	}
	if req.DurationHours > 0 {
		t := time.Now().Add(time.Duration(req.DurationHours) * time.Hour)
		p.BanUntil = &t
	} else {
		p.BanUntil = nil // 永久封禁
	}
	if err := apikey.SaveUserAPIPolicy(p); err != nil {
		infra.InternalError(c, "封禁失败")
		return
	}
	infra.Success(c, gin.H{"message": "已封禁该用户的全部 API Key", "ban_until": p.BanUntil})
}

// AdminUnbanUserAPIKeys POST /api/admin/ratelimit/users/:id/keys/unban —— 解除该用户的整体封禁
// 注意：仅清除「用户级封禁」，单把 Key 的独立封禁需单独解封。
func AdminUnbanUserAPIKeys(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	uid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	p := apikey.GetUserAPIPolicy(uint(uid))
	if p == nil {
		infra.Success(c, gin.H{"message": "该用户无封禁记录"})
		return
	}
	p.KeysBanned = false
	p.BanReason = ""
	p.BanUntil = nil
	if err := apikey.SaveUserAPIPolicy(p); err != nil {
		infra.InternalError(c, "解封失败")
		return
	}
	infra.Success(c, gin.H{"message": "已解除该用户的 API 封禁"})
}

// ==================== 按人限流配额 ====================

// AdminGetUserAPIQuota GET /api/admin/ratelimit/users/:id/quota —— 查询某用户的 API 策略
func AdminGetUserAPIQuota(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	uid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	p := apikey.GetUserAPIPolicy(uint(uid))
	if p == nil {
		// 无记录 = 跟随角色全局，回一个零值策略便于前端表单回填
		p = &apikey.UserAPIPolicy{UserID: uint(uid)}
	}
	infra.Success(c, p)
}

// AdminSetUserAPIQuota PUT /api/admin/ratelimit/users/:id/quota —— 设置某用户的专属配额 / Key 数上限
// 传 0 表示「跟随全局/不限制」；quota_per_min 只能比角色上限更严格（服务端按 min 生效）。
func AdminSetUserAPIQuota(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	uid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if uid == 0 {
		infra.BadRequest(c, "用户 ID 无效")
		return
	}
	user, err := models.GetUserByID(uint(uid))
	if err != nil || user == nil {
		infra.NotFound(c, "用户不存在")
		return
	}
	var req struct {
		QuotaPerMin int    `json:"quota_per_min"`
		WindowSecs  int    `json:"window_secs"`
		MaxKeys     int    `json:"max_keys"`
		Note        string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	if req.QuotaPerMin < 0 || req.QuotaPerMin > 1000000 {
		infra.BadRequest(c, "配额需在 0 ~ 1000000 之间（0 = 跟随角色全局）")
		return
	}
	if req.WindowSecs < 0 || req.WindowSecs > 86400 {
		infra.BadRequest(c, "时间窗口需在 0 ~ 86400 秒之间（0 = 跟随角色全局）")
		return
	}
	if req.MaxKeys < 0 || req.MaxKeys > 1000 {
		infra.BadRequest(c, "Key 数上限需在 0 ~ 1000 之间（0 = 不限制）")
		return
	}

	p := apikey.GetUserAPIPolicy(uint(uid))
	if p == nil {
		p = &apikey.UserAPIPolicy{UserID: uint(uid)}
	}
	p.QuotaPerMin = req.QuotaPerMin
	p.WindowSecs = req.WindowSecs
	p.MaxKeys = req.MaxKeys
	p.Note = strings.TrimSpace(req.Note)
	if err := apikey.SaveUserAPIPolicy(p); err != nil {
		infra.InternalError(c, "保存失败")
		return
	}
	infra.Success(c, gin.H{"message": "已保存（立即生效）", "policy": p})
}

// AdminListUserAPIPolicies GET /api/admin/ratelimit/users —— 所有被单独设置/封禁过的用户
func AdminListUserAPIPolicies(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	list, err := apikey.ListUserAPIPolicies()
	if err != nil {
		infra.InternalError(c, "查询失败")
		return
	}
	infra.Success(c, list)
}

// AdminDeleteUserAPIPolicy DELETE /api/admin/ratelimit/users/:id/quota —— 撤销按人设置（恢复跟随全局）
func AdminDeleteUserAPIPolicy(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	uid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	models.DB.Where("user_id = ?", uid).Delete(&apikey.UserAPIPolicy{})
	infra.Success(c, gin.H{"message": "已撤销该用户的专属设置（恢复跟随全局配额）"})
}
