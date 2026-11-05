package web

// Web 项目发布额度 + 超额审核（管理端 + 用户侧）
//
// 路由前缀刻意用 /web-quota（protected 组与 admin 组各一棵子树），避免与既有的
// /userhtmls、/userframes 通配段冲突导致 gin 路由注册 panic。
//
// 用户把某 web 项目（UserHTML / UserFrame）设为公开时改调本文件的
// SetHTMLPublic / SetFramePublic，由 quotam.CheckAndApplyWebPublish 决定
// 「额度内直接公开」还是「超额转 quota_review + 建审核单」，原有 UpdateHTML /
// UpdateFrame 的 is_public 逻辑不动（前端公开开关改调新接口即可）。

import (
	"nvs-server/handlers/admincommon"
	"nvs-server/handlers/hctx"
	"nvs-server/models"
	"nvs-server/utils/infra"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	quotam "nvs-server/models/quota"
	softdel "nvs-server/models/softdel"
)

// ==================== 用户侧：额度查询 + 公开开关 ====================

// MyWebQuota GET /api/web-quota/mine —— 查看本人额度/已用/剩余/待审单（数量 + 大小双维度）
func MyWebQuota(c *gin.Context) {
	userID := hctx.CurrentUserID(c)
	quota := quotam.GetWebPublishQuota(userID)
	used := quotam.CountPublicWeb(userID)

	var remaining int
	if quota < 0 {
		remaining = -1 // 不限
	} else {
		remaining = quota - int(used)
		if remaining < 0 {
			remaining = 0
		}
	}

	// 大小维度（MB；sizeQuota<=0 表示不限）
	sizeQuota := quotam.GetWebPublishQuotaSizeMB(userID)
	usedSize := quotam.CountPublicWebSize(userID)
	sizeRemaining := -1
	if sizeQuota > 0 {
		limitBytes := int64(sizeQuota) * 1024 * 1024
		sizeRemaining = int((limitBytes - usedSize) / 1024 / 1024)
		if sizeRemaining < 0 {
			sizeRemaining = 0
		}
	}

	var overrideVal *int
	var overrideSizeVal *int
	var overrideNote string
	isOverridden := false
	if ov := quotam.GetUserWebQuotaOverride(userID); ov != nil {
		isOverridden = true
		v := ov.Quota
		overrideVal = &v
		sv := ov.QuotaSizeMB
		overrideSizeVal = &sv
		overrideNote = ov.Note
	}

	requests, _ := quotam.ListMyWebQuotaRequests(userID)

	infra.Success(c, gin.H{
		"quota":             quota,
		"used":              used,
		"remaining":         remaining,
		"size_quota_mb":     sizeQuota,
		"size_used_bytes":   usedSize,
		"size_remaining_mb": sizeRemaining,
		"overridden":        isOverridden,
		"override_quota":    overrideVal,
		"override_size_mb":  overrideSizeVal,
		"override_note":     overrideNote,
		"global_default":    quotam.GetWebPublishQuotaConfig(),
		"global_size_mb":    quotam.GetWebPublishQuotaSizeConfig(),
		"requests":          requests,
	})
}

// SetHTMLPublic POST /api/web-quota/html-public —— 把某 HTML 扩展设为公开（带额度判断）
func SetHTMLPublic(c *gin.Context) {
	userID := hctx.CurrentUserID(c)
	var req struct {
		ID       uint `json:"id" binding:"required"`
		IsPublic bool `json:"is_public"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	var html softdel.UserHTML
	if err := models.DB.Where("id = ? AND user_id = ?", req.ID, userID).First(&html).Error; err != nil {
		infra.NotFound(c, "项目不存在或不属于你")
		return
	}

	if !req.IsPublic {
		// 取消公开：仅置 is_public=false（不改 review_status，避免误动内容安全扫描语义）
		if err := models.DB.Model(&html).Updates(map[string]interface{}{
			"is_public":  false,
			"updated_at": time.Now(),
		}).Error; err != nil {
			infra.InternalError(c, "操作失败")
			return
		}
		infra.Success(c, gin.H{"is_public": false, "allowed": true, "need_review": false, "message": "已取消公开"})
		return
	}

	allowed, _, reason, err := quotam.CheckAndApplyWebPublish(userID, quotam.WebQuotaTargetHTML, req.ID)
	if err != nil {
		infra.InternalError(c, "操作失败")
		return
	}
	if allowed {
		infra.Success(c, gin.H{"is_public": true, "allowed": true, "need_review": false, "message": "已公开发布到广场"})
	} else {
		infra.Success(c, gin.H{
			"is_public":   false,
			"allowed":     false,
			"need_review": true,
			"message":     reason + "，管理员通过后将在广场展示",
		})
	}
}

// SetFramePublic POST /api/web-quota/frame-public —— 把某 UI 模板设为公开（带额度判断）
func SetFramePublic(c *gin.Context) {
	userID := hctx.CurrentUserID(c)
	var req struct {
		ID       uint `json:"id" binding:"required"`
		IsPublic bool `json:"is_public"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	var frame softdel.UserFrame
	if err := models.DB.Where("id = ? AND user_id = ?", req.ID, userID).First(&frame).Error; err != nil {
		infra.NotFound(c, "模板不存在或不属于你")
		return
	}

	if !req.IsPublic {
		if err := models.DB.Model(&frame).Updates(map[string]interface{}{
			"is_public":  false,
			"updated_at": time.Now(),
		}).Error; err != nil {
			infra.InternalError(c, "操作失败")
			return
		}
		infra.Success(c, gin.H{"is_public": false, "allowed": true, "need_review": false, "message": "已取消公开"})
		return
	}

	allowed, _, reason, err := quotam.CheckAndApplyWebPublish(userID, quotam.WebQuotaTargetFrame, req.ID)
	if err != nil {
		infra.InternalError(c, "操作失败")
		return
	}
	if allowed {
		infra.Success(c, gin.H{"is_public": true, "allowed": true, "need_review": false, "message": "已公开发布到广场"})
	} else {
		infra.Success(c, gin.H{
			"is_public":   false,
			"allowed":     false,
			"need_review": true,
			"message":     reason + "，管理员通过后将在广场展示",
		})
	}
}

// ==================== 管理端：超额审核单 ====================

// AdminListWebQuotaRequests GET /api/admin/web-quota/requests?status=pending —— 超额待审列表（支持状态筛选）
func AdminListWebQuotaRequests(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	status := strings.TrimSpace(c.Query("status"))
	list, err := quotam.ListWebQuotaRequests(status)
	if err != nil {
		infra.InternalError(c, "查询失败")
		return
	}
	infra.Success(c, list)
}

// AdminApproveWebQuotaRequest POST /api/admin/web-quota/requests/:id/approve —— 批准超额公开
func AdminApproveWebQuotaRequest(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Note string `json:"note"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := quotam.ApproveQuotaRequest(uint(id), hctx.CurrentUserID(c), strings.TrimSpace(req.Note)); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.Success(c, gin.H{"message": "已批准，项目已公开发布到广场", "status": quotam.WebQuotaStatusApproved})
}

// AdminRejectWebQuotaRequest POST /api/admin/web-quota/requests/:id/reject —— 驳回超额公开
func AdminRejectWebQuotaRequest(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Note string `json:"note"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := quotam.RejectQuotaRequest(uint(id), hctx.CurrentUserID(c), strings.TrimSpace(req.Note)); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.Success(c, gin.H{"message": "已驳回，项目保持私有", "status": quotam.WebQuotaStatusRejected})
}

// ==================== 管理端：额度配置（全局默认 + 按人覆盖） ====================

// AdminGetWebQuotaConfig GET /api/admin/web-quota/config —— 全局默认额度 + 按人覆盖列表
func AdminGetWebQuotaConfig(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	overrides, err := quotam.ListUserWebQuotas()
	if err != nil {
		overrides = []quotam.UserWebQuota{}
	}
	infra.Success(c, gin.H{
		"max_per_user": quotam.GetWebPublishQuotaConfig(),
		"max_size_mb":  quotam.GetWebPublishQuotaSizeConfig(),
		"overrides":    overrides,
	})
}

// AdminSetWebQuotaConfig PUT /api/admin/web-quota/config —— 保存配置
// body.action:
//
//	"global"      —— 设置全局默认额度（max_per_user 数量；max_size_mb 大小，0=不限，可单独或同时提交）
//	"set_user"    —— 为指定用户单独设置额度（quota=-1 跟随全局 / 0 禁止 / >0 专属数量上限；
//	                  quota_size_mb=-1 跟随全局 / 0 不限 / >0 专属大小上限 MB）
//	"remove_user" —— 撤销某用户的额度覆盖（恢复跟随全局）
func AdminSetWebQuotaConfig(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	var req struct {
		Action      string `json:"action"`
		MaxPerUser  *int   `json:"max_per_user"`
		MaxSizeMB   *int   `json:"max_size_mb"`
		UserID      uint   `json:"user_id"`
		Quota       *int   `json:"quota"`
		QuotaSizeMB *int   `json:"quota_size_mb"`
		Note        string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}

	switch req.Action {
	case "global":
		if req.MaxPerUser == nil && req.MaxSizeMB == nil {
			infra.BadRequest(c, "请至少填写一项：max_per_user（数量）或 max_size_mb（大小，0=不限）")
			return
		}
		if req.MaxPerUser != nil {
			if *req.MaxPerUser < 0 || *req.MaxPerUser > 1000 {
				infra.BadRequest(c, "数量额度需在 0 ~ 1000 之间（0 = 禁止任何人公开发布）")
				return
			}
			if err := quotam.SetWebPublishQuotaConfig(*req.MaxPerUser); err != nil {
				infra.InternalError(c, "保存失败")
				return
			}
		}
		if req.MaxSizeMB != nil {
			if *req.MaxSizeMB < 0 || *req.MaxSizeMB > quotam.MaxWebQuotaSizeMB {
				infra.BadRequest(c, "大小额度需在 0 ~ 102400 MB 之间（0 = 不限）")
				return
			}
			if err := quotam.SetWebPublishQuotaSizeConfig(*req.MaxSizeMB); err != nil {
				infra.InternalError(c, "保存失败")
				return
			}
		}
		infra.Success(c, gin.H{"message": "全局默认额度已保存（立即生效）"})
	case "set_user":
		if req.UserID == 0 {
			infra.BadRequest(c, "用户 ID 无效")
			return
		}
		if req.Quota == nil && req.QuotaSizeMB == nil {
			infra.BadRequest(c, "请至少填写一项：quota（数量）或 quota_size_mb（大小）")
			return
		}
		if _, err := models.GetUserByID(req.UserID); err != nil {
			infra.NotFound(c, "用户不存在")
			return
		}
		// 未提交的维度沿用已保存值（-1 = 跟随全局），实现「只改其一」
		quota, quotaSizeMB := quotam.WebQuotaFollowGlobal, quotam.WebQuotaFollowGlobal
		if ov := quotam.GetUserWebQuotaOverride(req.UserID); ov != nil {
			quota, quotaSizeMB = ov.Quota, ov.QuotaSizeMB
		}
		if req.Quota != nil {
			quota = *req.Quota
		}
		if req.QuotaSizeMB != nil {
			quotaSizeMB = *req.QuotaSizeMB
		}
		if err := quotam.SetUserWebQuotaOverride(req.UserID, quota, quotaSizeMB, strings.TrimSpace(req.Note)); err != nil {
			infra.InternalError(c, "保存失败")
			return
		}
		infra.Success(c, gin.H{"message": "已保存按人额度（立即生效）"})
	case "remove_user":
		if req.UserID == 0 {
			infra.BadRequest(c, "用户 ID 无效")
			return
		}
		if err := quotam.DeleteUserWebQuotaOverride(req.UserID); err != nil {
			infra.InternalError(c, "操作失败")
			return
		}
		infra.Success(c, gin.H{"message": "已撤销该用户的额度覆盖（恢复跟随全局）"})
	default:
		infra.BadRequest(c, "未知的 action（支持 global / set_user / remove_user）")
	}
}
