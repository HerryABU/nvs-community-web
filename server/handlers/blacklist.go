package handlers

import (
	"strconv"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ============ 用户黑名单（F13） ============

// AddBlacklist POST /api/blacklist — 把某用户加入黑名单（已拉黑则更新模式）
// body: { user_key 或 user_id, mode?, reason? }
//   - user_key：用户名或数字 id（复用 ResolveUserKey 的解析规则）
//   - mode：block=禁止在其评论区评论（默认）/ hide=屏蔽其内容不看
//   - reason：可选理由（≤256 字符，超长自动截断）
func AddBlacklist(c *gin.Context) {
	userID := currentUserID(c)

	var req struct {
		UserKey string `json:"user_key"`
		UserID  uint   `json:"user_id"`
		Mode    string `json:"mode"`
		Reason  string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}

	// 目标用户：user_key 优先，兼容前端直接传数字 user_id
	key := req.UserKey
	if key == "" {
		if req.UserID == 0 {
			utils.BadRequest(c, "请指定要拉黑的用户")
			return
		}
		key = strconv.FormatUint(uint64(req.UserID), 10)
	}
	target, err := ResolveUserKey(key)
	if err != nil || target == nil {
		utils.NotFound(c, "用户不存在")
		return
	}

	if target.ID == userID {
		utils.BadRequest(c, "不能拉黑自己")
		return
	}
	// 站长/警方/管理员不可被拉黑：保障平台治理与取证通道畅通
	if models.IsAdminOrPolice(target.Role) {
		utils.Forbidden(c, "无法拉黑站长/警方/管理员")
		return
	}

	mode := req.Mode
	if mode != models.BlacklistModeHide {
		mode = models.BlacklistModeBlock
	}

	entry := &models.Blacklist{
		UserID:        userID,
		BlockedUserID: target.ID,
		Mode:          mode,
		Reason:        utils.ClampString(req.Reason, utils.MaxReason),
	}
	if err := models.AddBlacklist(entry); err != nil {
		utils.InternalError(c, "操作失败")
		return
	}

	utils.Success(c, gin.H{
		"message": "已加入黑名单",
		"id":      entry.ID,
		"mode":    entry.Mode,
	})
}

// RemoveBlacklist DELETE /api/blacklist/:id — 解除拉黑（仅本人记录可删）
func RemoveBlacklist(c *gin.Context) {
	userID := currentUserID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		utils.BadRequest(c, "参数错误")
		return
	}

	entry, err := models.GetBlacklistByID(uint(id))
	if err != nil {
		utils.NotFound(c, "黑名单记录不存在")
		return
	}
	if entry.UserID != userID {
		utils.Forbidden(c, "无权操作此记录")
		return
	}
	if err := models.RemoveBlacklist(userID, uint(id)); err != nil {
		utils.InternalError(c, "解除失败")
		return
	}

	utils.Success(c, gin.H{"message": "已解除拉黑"})
}

// ListMyBlacklist GET /api/blacklist?page=&page_size= — 我的黑名单列表
func ListMyBlacklist(c *gin.Context) {
	userID := currentUserID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	pg, err := models.ListBlacklist(userID, page, pageSize)
	if err != nil {
		utils.InternalError(c, "获取黑名单失败")
		return
	}

	utils.Success(c, gin.H{
		"list":  pg.List,
		"total": pg.Total,
		"page":  page,
	})
}
