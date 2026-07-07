package handlers

import (
	"strconv"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// POST /api/ratings — 提交或更新评分
func UpsertRating(c *gin.Context) {
	userID := c.GetUint("userID")

	var req struct {
		NovelID             uint `json:"novel_id" binding:"required"`
		TypeCompletion      int  `json:"type_completion"`
		NarrativeQuality    int  `json:"narrative_quality"`
		ThoughtDepth        int  `json:"thought_depth"`
		CommunityReputation int  `json:"community_reputation"`
		UpdateStability     int  `json:"update_stability"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}

	// 验证范围 1-5
	clamp := func(v int) int {
		if v < 1 { return 1 }
		if v > 5 { return 5 }
		return v
	}

	rating := &models.Rating{
		UserID:              userID,
		NovelID:             req.NovelID,
		TypeCompletion:      clamp(req.TypeCompletion),
		NarrativeQuality:    clamp(req.NarrativeQuality),
		ThoughtDepth:        clamp(req.ThoughtDepth),
		CommunityReputation: clamp(req.CommunityReputation),
		UpdateStability:     clamp(req.UpdateStability),
	}

	if err := models.UpsertRating(rating); err != nil {
		utils.InternalError(c, "评分失败")
		return
	}
	utils.Success(c, rating)
}

// GET /api/ratings — 获取某用户对某作品的评分
func GetUserRating(c *gin.Context) {
	userID := c.GetUint("userID")
	novelID, _ := strconv.ParseUint(c.Query("novel_id"), 10, 64)

	rating, err := models.GetRatingByUserNovel(userID, uint(novelID))
	if err != nil {
		utils.Success(c, nil)
		return
	}
	utils.Success(c, rating)
}

// GET /api/novels/:id/rating — 获取作品评分统计
func GetNovelRating(c *gin.Context) {
	novelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	stats, count, err := models.GetRatingStats(uint(novelID))
	if err != nil {
		utils.InternalError(c, "获取评分失败")
		return
	}

	overall := 0.0
	if count > 0 {
		overall = (stats["type_completion"] + stats["narrative_quality"] +
			stats["thought_depth"] + stats["community_reputation"] +
			stats["update_stability"]) / 5.0
	}

	utils.Success(c, gin.H{
		"dimensions": stats,
		"overall":    overall,
		"count":      count,
	})
}
