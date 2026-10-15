package handlers

import (
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// PoliceForumRow 警方论坛巡查行：在 Forum 基础上附带统计与门禁摘要，便于一屏纵览。
type PoliceForumRow struct {
	models.Forum
	ThreadTotal int64 `json:"thread_total"` // 帖子数（含软删除）
	PostTotal   int64 `json:"post_total"`   // 回复数（含软删除）
}

// GET /api/police/forums — 警方论坛巡查：列出全部论坛（含隐藏/待审/驳回/软删除）。
// 警方可借此纵览所有封锁、加密、隐藏论坛；点击进入时由 GetForum 的警方豁免直接放行，
// 无需房间号/密码。满足「对封锁加密的论坛也可以看到、隐藏的不需要房间号」的取证需求。
func PoliceListForums(c *gin.Context) {
	if !ensurePoliceOrAdmin(c) {
		return
	}

	// Unscoped：含软删除论坛；不做任何可见性过滤，全部返回
	var forums []models.Forum
	if err := models.DB.Unscoped().
		Order("sort_order ASC, id ASC").
		Find(&forums).Error; err != nil {
		utils.InternalError(c, "获取论坛列表失败")
		return
	}

	// 批量填充创建者用户名（避免逐条查库）
	creatorIDs := make([]uint, 0, len(forums))
	seen := map[uint]bool{}
	for _, f := range forums {
		if f.CreatorID != 0 && !seen[f.CreatorID] {
			seen[f.CreatorID] = true
			creatorIDs = append(creatorIDs, f.CreatorID)
		}
	}
	names := map[uint]string{}
	if len(creatorIDs) > 0 {
		var users []models.User
		models.DB.Select("id", "username").Where("id IN ?", creatorIDs).Find(&users)
		for _, u := range users {
			names[u.ID] = u.Username
		}
	}

	rows := make([]PoliceForumRow, 0, len(forums))
	for _, f := range forums {
		f.CreatorName = names[f.CreatorID]
		row := PoliceForumRow{Forum: f}
		models.DB.Unscoped().Model(&models.Thread{}).Where("forum_id = ?", f.ID).Count(&row.ThreadTotal)
		// posts 无 forum_id，需经 thread_id 关联统计
		var threadIDs []uint
		models.DB.Unscoped().Model(&models.Thread{}).Where("forum_id = ?", f.ID).Pluck("id", &threadIDs)
		if len(threadIDs) > 0 {
			models.DB.Unscoped().Model(&models.Post{}).Where("thread_id IN ?", threadIDs).Count(&row.PostTotal)
		}
		rows = append(rows, row)
	}

	utils.Success(c, gin.H{
		"forums": rows,
		"total":  len(rows),
	})
}
