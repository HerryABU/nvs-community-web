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

// HiddenAuthorIDs 返回 userID 以 hide 模式（屏蔽内容）拉黑的全部作者 ID。
// 供评论列表 / 动态流 / 论坛帖子在一次请求内「一次性取出」隐藏作者集合，再在内存中过滤，
// 避免逐条查询；block 模式的写侧拦截仍由 CreateComment 等业务函数负责。
func HiddenAuthorIDs(userID uint) ([]uint, error) {
	var ids []uint
	if userID == 0 {
		return ids, nil
	}
	err := models.DB.Model(&models.Blacklist{}).
		Where("user_id = ? AND mode = ?", userID, models.BlacklistModeHide).
		Pluck("blocked_user_id", &ids).Error
	return ids, err
}

// FilterHiddenItems 从 items 中剔除「当前用户以 hide 模式拉黑的作者」所产生的内容。
// getAuthor 取出条目作者 ID；viewerID=0（未登录）或无隐藏记录时原样返回（零开销）。
// 供评论/回帖/帖子/动态等各类列表复用，语义统一：hide=不看其内容。
func FilterHiddenItems[T any](viewerID uint, items []T, getAuthor func(T) uint) []T {
	if viewerID == 0 || len(items) == 0 {
		return items
	}
	hidden, err := HiddenAuthorIDs(viewerID)
	if err != nil || len(hidden) == 0 {
		return items
	}
	hiddenSet := make(map[uint]struct{}, len(hidden))
	for _, id := range hidden {
		hiddenSet[id] = struct{}{}
	}
	kept := make([]T, 0, len(items))
	for _, it := range items {
		if _, blocked := hiddenSet[getAuthor(it)]; blocked {
			continue
		}
		kept = append(kept, it)
	}
	return kept
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
