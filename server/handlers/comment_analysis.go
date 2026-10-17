package handlers

import (
	"strconv"
	"strings"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// isOwnerOfWork 判断当前登录用户是否为某作品/博客的作者（楼主）。
func isOwnerOfWork(c *gin.Context, targetType string, targetID uint) bool {
	uid := currentUserID(c)
	if uid == 0 || targetID == 0 {
		return false
	}
	if targetType == "blog" {
		if b, err := models.GetBlogByID(targetID); err == nil {
			return b.AuthorID == uid
		}
		return false
	}
	if n, err := models.GetNovelByID(targetID); err == nil {
		return n.AuthorID == uid
	}
	return false
}

// GetMentionSource GET /api/mention-source?target_type=blog&target_id=123
// 返回该作品/博客生效的 @ 提及来源（优先作品覆盖，否则站点默认），以及当前用户是否可修改。
func GetMentionSource(c *gin.Context) {
	targetType := strings.TrimSpace(c.Query("target_type"))
	targetID := uint(mustUint(c.Query("target_id")))
	effective := models.GetEffectiveMentionSource(targetType, targetID)
	canEdit := isAdminUser(c) || isOwnerOfWork(c, targetType, targetID)
	overrideSet := false
	if targetType != "" && targetID > 0 {
		overrideSet = models.GetSetting(models.MentionSourceWorkKey(targetType, targetID), "") != ""
	}
	utils.Success(c, gin.H{
		"effective":     effective,
		"site_default":  models.GetMentionSource(),
		"can_edit":      canEdit,
		"override_set":  overrideSet,
		"is_admin":      isAdminUser(c),
		"is_owner":      isOwnerOfWork(c, targetType, targetID),
	})
}

// SetMentionSource PUT /api/mention-source
// 站长（admin）可改站点默认（target_id=0）；站长或楼主可改本作品覆盖（target_id>0）。
func SetMentionSource(c *gin.Context) {
	var req struct {
		TargetType string `json:"target_type"`
		TargetID   uint   `json:"target_id"`
		Mode       string `json:"mode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	valid := map[string]bool{
		models.MentionSourceParticipants: true,
		models.MentionSourceGlobal:       true,
		models.MentionSourceManual:       true,
	}
	if !valid[req.Mode] {
		utils.BadRequest(c, "无效的提及来源模式")
		return
	}

	uid := currentUserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}

	// 站点默认：仅站长
	if req.TargetID == 0 {
		if !isAdminUser(c) {
			utils.Forbidden(c, "只有站长可设置站点默认提及来源")
			return
		}
		if err := models.SetSetting(models.MentionSourceKey, req.Mode); err != nil {
			utils.InternalError(c, "保存失败")
			return
		}
		utils.Success(c, gin.H{"ok": true, "effective": req.Mode})
		return
	}

	// 作品覆盖：站长或楼主
	if !isAdminUser(c) && !isOwnerOfWork(c, req.TargetType, req.TargetID) {
		utils.Forbidden(c, "只有站长或本作品作者可设置提及来源")
		return
	}
	if err := models.SetSetting(models.MentionSourceWorkKey(req.TargetType, req.TargetID), req.Mode); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.Success(c, gin.H{"ok": true, "effective": models.GetEffectiveMentionSource(req.TargetType, req.TargetID)})
}

// GetMentionCandidates GET /api/mention-candidates?scope=participants|global&target_type=&target_id=&q=
// @ 提及联想候选：participants=本作品参与者；global=全站用户搜索。
func GetMentionCandidates(c *gin.Context) {
	scope := strings.TrimSpace(c.Query("scope"))
	if scope == "" {
		scope = models.MentionSourceParticipants
	}
	targetType := strings.TrimSpace(c.Query("target_type"))
	targetID := uint(mustUint(c.Query("target_id")))
	q := strings.TrimSpace(c.Query("q"))
	limit := 20
	if l := c.Query("limit"); l != "" {
		if n := int(mustUint(l)); n > 0 {
			limit = n
		}
	}
	cands, err := models.GetMentionCandidates(targetType, targetID, scope, q, limit)
	if err != nil {
		utils.InternalError(c, "获取候选失败")
		return
	}
	utils.Success(c, cands)
}

// GetCommentNetwork GET /api/comment-network?target_type=blog|novel&target_id=&user_id=
// 评论网（社交图谱）分析：谁回复了谁的哪条评论，含时间估计与目标估计，可按 user_id 下钻到某评论者。
func GetCommentNetwork(c *gin.Context) {
	targetType := strings.TrimSpace(c.Query("target_type"))
	targetID := uint(mustUint(c.Query("target_id")))
	if targetType == "" || targetID == 0 {
		utils.BadRequest(c, "请指定作品类型与 id")
		return
	}
	filterUserID := uint(mustUint(c.Query("user_id")))
	net, err := models.GetCommentNetwork(targetType, targetID, filterUserID)
	if err != nil {
		utils.InternalError(c, "分析失败")
		return
	}
	utils.Success(c, net)
}

// mustUint 解析查询参数中的无符号整数（解析失败返回 0）
func mustUint(s string) uint64 {
	n, _ := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
	return n
}
