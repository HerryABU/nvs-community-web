package handlers

import (
	"fmt"
	"net/http"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== 每日发文限额 / 用户配额体系（需求 §13.4） ====================

// enforceDailyPublish 预检当日发文限额；超限返回 429 并 false（调用方应 return）。
// kind: novel/chapter/blog/thread；addWords: 本次发布内容字数（rune 计），用于字数限额。
func enforceDailyPublish(c *gin.Context, kind string, addWords int) bool {
	userID := currentUserID(c)
	role := currentUserRole(c)
	allowed, usedCount, countLimit, usedWords, wordLimit := models.CheckDailyPublish(userID, role, kind, addWords)
	if allowed {
		return true
	}
	var msg string
	switch {
	case countLimit == 0:
		msg = "当前账号等级已被禁止发布内容，如有需求请联系站长调整额度"
	case countLimit > 0 && usedCount >= countLimit:
		msg = fmt.Sprintf("今日发布数量已达上限（%d/%d），次日 0 点重置；可联系站长或提升等级获取更多额度", usedCount, countLimit)
	default:
		msg = fmt.Sprintf("本次发布后今日总字数将超出上限（已用 %d 字 + 本次 %d 字，上限 %d 字），次日 0 点重置", usedWords, addWords, wordLimit)
	}
	utils.Error(c, http.StatusTooManyRequests, 42901, msg)
	return false
}

// GetMyPublishQuota GET /api/daily-publish/status — 当前用户今日发文额度（发布表单展示剩余量）
func GetMyPublishQuota(c *gin.Context) {
	userID := currentUserID(c)
	role := currentUserRole(c)
	enabled := models.GetDailyPublishConfig().Enabled
	countLimit, wordLimit := models.EffectivePublishLimit(userID, role)
	uc, uw := models.CountTodayPublish(userID)
	utils.Success(c, gin.H{
		"enabled":     enabled,
		"used_count":  int(uc),
		"count_limit": countLimit,
		"used_words":  int(uw),
		"word_limit":  wordLimit,
		"reset_at":    models.DailyResetTime(),
	})
}

// AdminGetDailyPublishConfig GET /api/admin/daily-publish/config — 读取每日发文限额配置
func AdminGetDailyPublishConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	utils.Success(c, models.GetDailyPublishConfig())
}

// AdminSetDailyPublishConfig PUT /api/admin/daily-publish/config — 保存配置（立即生效）
func AdminSetDailyPublishConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var cfg models.DailyPublishConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := models.SetDailyPublishConfig(cfg); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.Success(c, gin.H{"message": "每日发文限额已保存（立即生效）"})
}

// AdminListPublishOverrides GET /api/admin/daily-publish/overrides — 按人覆盖列表
func AdminListPublishOverrides(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	list, err := models.ListUserPublishQuotas()
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, list)
}

// AdminManagePublishOverride POST /api/admin/daily-publish/overrides — 设置/撤销按人覆盖
// body: { action: "set"|"remove", user_id, count, word_cap, note }
func AdminManagePublishOverride(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		Action  string `json:"action"`
		UserID  uint   `json:"user_id"`
		Count   int    `json:"count"`
		WordCap int    `json:"word_cap"`
		Note    string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if req.UserID == 0 {
		utils.BadRequest(c, "用户 ID 无效")
		return
	}
	if _, err := models.GetUserByID(req.UserID); err != nil {
		utils.NotFound(c, "用户不存在")
		return
	}
	switch req.Action {
	case "set":
		if err := models.SetUserPublishQuotaOverride(req.UserID, req.Count, req.WordCap, req.Note); err != nil {
			utils.InternalError(c, "保存失败")
			return
		}
		utils.Success(c, gin.H{"message": "已保存该用户的发文限额（立即生效）"})
	case "remove":
		if err := models.DeleteUserPublishQuotaOverride(req.UserID); err != nil {
			utils.InternalError(c, "操作失败")
			return
		}
		utils.Success(c, gin.H{"message": "已撤销该用户的限额覆盖（恢复跟随等级默认）"})
	default:
		utils.BadRequest(c, "未知的 action（支持 set / remove）")
	}
}
