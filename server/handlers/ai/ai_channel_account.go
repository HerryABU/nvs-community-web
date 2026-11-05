package ai

// ==================== 渠道账户凭据（入库、加密、可复用） ====================
//
// 站长的原话：「这样的配置是要入库的，到时候直接点击拉取，其中因为 nap 权利很大，
// 需要对其加密入库，但不推荐使用 jwt 加密」。
//
// 所以这里做三件事：
//  1. 保存 = 把渠道地址 + nap_ + 用户 ID + sk_ 加密落库（只存密文 + 尾号）；
//  2. 复用 = 监控/拉取时不再要求手输，后端解密后当场用、用完即弃；
//  3. 收敛 = 对外接口只回「有没有配 / 尾号是什么 / 上次探测结果」，**永不回明文**。

import (
	"strconv"
	"strings"

	"nvs-server/handlers/admin"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	aihub "nvs-server/models/aihub"
)

// AdminSaveChannelAccount POST /api/admin/ai/channel-account
//
// 入参里 access_token / model_key：留空=不改；传 "-"=清空；传值=覆盖。
func AdminSaveChannelAccount(c *gin.Context) {
	if !admin.EnsureAdmin(c) {
		return
	}
	var req struct {
		BaseURL     string `json:"base_url" binding:"required"`
		Label       string `json:"label"`
		UserID      string `json:"user_id"`
		AccessToken string `json:"access_token"` // nap_
		ModelKey    string `json:"model_key"`    // sk_
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	acc, err := aihub.SaveAiChannelAccount(req.BaseURL, req.Label, req.UserID, req.AccessToken, req.ModelKey)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "已加密保存", redactAccount(acc))
}

// AdminGetChannelAccount GET /api/admin/ai/channel-account?base_url=
func AdminGetChannelAccount(c *gin.Context) {
	if !admin.EnsureAdmin(c) {
		return
	}
	base := strings.TrimSpace(c.Query("base_url"))
	acc, err := aihub.GetAiChannelAccount(base)
	if err != nil {
		// 没保存过不是错误：前端据此显示「未保存」表单
		utils.Success(c, gin.H{"exists": false})
		return
	}
	utils.Success(c, gin.H{"exists": true, "account": redactAccount(acc)})
}

// AdminListChannelAccounts GET /api/admin/ai/channel-accounts
func AdminListChannelAccounts(c *gin.Context) {
	if !admin.EnsureAdmin(c) {
		return
	}
	list := aihub.ListAiChannelAccounts()
	out := make([]gin.H, 0, len(list))
	for i := range list {
		out = append(out, redactAccount(&list[i]))
	}
	utils.Success(c, gin.H{"list": out})
}

// AdminDeleteChannelAccount DELETE /api/admin/ai/channel-account/:id
func AdminDeleteChannelAccount(c *gin.Context) {
	if !admin.EnsureAdmin(c) {
		return
	}
	id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	id := uint(id64)
	if id == 0 {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := aihub.DeleteAiChannelAccount(id); err != nil {
		utils.InternalError(c, "删除失败")
		return
	}
	utils.SuccessMessage(c, "已删除", nil)
}

// redactAccount 脱敏：密文一律不出现在响应里，只给「有没有 / 尾号 / 上次结果」
func redactAccount(a *aihub.AiChannelAccount) gin.H {
	if a == nil {
		return gin.H{}
	}
	return gin.H{
		"id":              a.ID,
		"base_url":        a.BaseURL,
		"label":           a.Label,
		"user_id":         a.UserID,
		"enabled":         a.Enabled,
		"has_token":       a.HasAccessToken(),
		"token_tail":      a.AccessTokenTail,
		"has_model_key":   a.HasModelKey(),
		"model_key_tail":  a.ModelKeyTail,
		"last_checked_at": a.LastCheckedAt,
		"last_ok":         a.LastOK,
		"last_models":     a.LastModels,
		"last_balance":    a.LastBalance,
		"last_used":       a.LastUsed,
		"last_currency":   a.LastCurrency,
		"last_error":      a.LastError,
		"updated_at":      a.UpdatedAt,
	}
}
