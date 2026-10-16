package handlers

import (
	"strconv"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ============ 特别关注（F14） ============

// ToggleSpecialFollow POST /api/follows/:id/special — 设置/取消特别关注
// body: { special: bool }
//   - special=true：未关注则先建立关注关系，再标记特别关注
//   - special=false：仅取消特别关注标记，保留普通关注
func ToggleSpecialFollow(c *gin.Context) {
	userID := currentUserID(c)
	targetID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || targetID == 0 {
		utils.BadRequest(c, "参数错误")
		return
	}
	if uint(targetID) == userID {
		utils.BadRequest(c, "不能对自己操作")
		return
	}

	// 目标用户必须存在
	var target models.User
	if err := models.DB.First(&target, uint(targetID)).Error; err != nil {
		utils.NotFound(c, "用户不存在")
		return
	}

	var req struct {
		Special bool `json:"special"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}

	if err := models.SetSpecialFollow(userID, uint(targetID), req.Special); err != nil {
		utils.InternalError(c, "操作失败")
		return
	}

	msg := "已取消特别关注"
	if req.Special {
		msg = "已设为特别关注"
	}
	utils.Success(c, gin.H{
		"message":    msg,
		"is_special": req.Special,
	})
}

// ListMySpecialFollows GET /api/follows/special?page=&size= — 我的特别关注列表
func ListMySpecialFollows(c *gin.Context) {
	userID := currentUserID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("size", "20"))

	users, total, err := models.ListSpecialFollowing(userID, page, pageSize)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	// 不回显邮箱等敏感字段
	for i := range users {
		users[i].Email = ""
	}
	utils.Success(c, gin.H{"list": users, "total": total})
}

// GetSpecialFeed GET /api/feed/special?page=&size= — 特别关注动态流（需登录）
// 聚合特别关注用户的小说更新/博客发布/论坛发帖，按时间倒序分页
func GetSpecialFeed(c *gin.Context) {
	userID := currentUserID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("size", "20"))

	items, total, err := models.GetSpecialFeed(userID, page, pageSize)
	if err != nil {
		utils.InternalError(c, "获取动态失败")
		return
	}

	// 黑名单 hide 模式（F13）：隐藏被当前用户屏蔽（hide）的作者所发布的动态。
	// 一次性取出隐藏作者集合，仅在本次返回 page 内过滤；total 仍为聚合总数。
	if userID > 0 && len(items) > 0 {
		if hidden, herr := HiddenAuthorIDs(userID); herr == nil && len(hidden) > 0 {
			hiddenSet := make(map[uint]struct{}, len(hidden))
			for _, id := range hidden {
				hiddenSet[id] = struct{}{}
			}
			kept := make([]models.FeedItem, 0, len(items))
			for _, it := range items {
				if _, blocked := hiddenSet[it.UserID]; blocked {
					continue
				}
				kept = append(kept, it)
			}
			items = kept
		}
	}

	utils.Success(c, gin.H{"list": items, "total": total})
}
