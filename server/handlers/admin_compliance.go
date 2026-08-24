package handlers

// 此文件包含 176 号令合规补全的管理后台 API：
//   - 算法推荐管理（第七条第九项）：GET/PUT /api/admin/algorithms
//   - 等保/关基识别记录（第七条第四/五项）：GET/PUT /api/admin/security-grade
//   - 备案信息（第七条第一项）已并入 /api/admin/config 与公开 /api/site-info

import (
	"strings"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// GetAlgoConfig 获取算法推荐配置（管理后台）
// GET /api/admin/algorithms
func GetAlgoConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	sortRule, rankPeriod, personalized, description := models.GetAlgoConfig()
	utils.Success(c, gin.H{
		"sort_rule":    sortRule,
		"rank_period":  rankPeriod,
		"personalized": personalized,
		"description":  description,
	})
}

// UpdateAlgoConfig 更新算法推荐配置（即时生效）
// PUT /api/admin/algorithms
func UpdateAlgoConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		SortRule     *string `json:"sort_rule"`    // updated_at/created_at/total_chapters
		RankPeriod   *string `json:"rank_period"`  // all/7d/30d/90d
		Personalized *bool   `json:"personalized"` // 个性化推荐开关
		Description  *string `json:"description"`  // 算法说明（面向用户）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if req.SortRule != nil {
		rule := strings.TrimSpace(*req.SortRule)
		if rule != "updated_at" && rule != "created_at" && rule != "total_chapters" {
			utils.BadRequest(c, "排序规则仅支持 updated_at / created_at / total_chapters")
			return
		}
		if err := models.SetPlatformConfig(models.AlgoSortRule, rule); err != nil {
			utils.InternalError(c, "保存失败")
			return
		}
	}
	if req.RankPeriod != nil {
		period := strings.TrimSpace(*req.RankPeriod)
		if period != "all" && period != "7d" && period != "30d" && period != "90d" {
			utils.BadRequest(c, "排行榜周期仅支持 all / 7d / 30d / 90d")
			return
		}
		if err := models.SetPlatformConfig(models.AlgoRankPeriod, period); err != nil {
			utils.InternalError(c, "保存失败")
			return
		}
	}
	if req.Personalized != nil {
		if err := models.SetPlatformConfig(models.AlgoPersonalized, map[bool]string{true: "true", false: "false"}[*req.Personalized]); err != nil {
			utils.InternalError(c, "保存失败")
			return
		}
	}
	if req.Description != nil {
		if err := models.SetPlatformConfig(models.AlgoDescription, strings.TrimSpace(*req.Description)); err != nil {
			utils.InternalError(c, "保存失败")
			return
		}
	}
	sortRule, rankPeriod, personalized, description := models.GetAlgoConfig()
	utils.Success(c, gin.H{
		"message":      "算法配置已更新",
		"sort_rule":    sortRule,
		"rank_period":  rankPeriod,
		"personalized": personalized,
		"description":  description,
	})
}

// GetSecurityGrade 获取等保/关基识别记录（管理后台）
// GET /api/admin/security-grade
func GetSecurityGrade(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	grade, note := models.GetSecurityGrade()
	utils.Success(c, gin.H{"grade": grade, "note": note})
}

// UpdateSecurityGrade 保存等保/关基识别记录
// PUT /api/admin/security-grade
func UpdateSecurityGrade(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		Grade *string `json:"grade"` // 1/2/3/未定级
		Note  *string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if req.Grade != nil {
		grade := strings.TrimSpace(*req.Grade)
		if grade != "1" && grade != "2" && grade != "3" && grade != "未定级" {
			utils.BadRequest(c, "等保定级仅支持 1 / 2 / 3 / 未定级")
			return
		}
		if err := models.SetPlatformConfig(models.SecurityGrade, grade); err != nil {
			utils.InternalError(c, "保存失败")
			return
		}
	}
	if req.Note != nil {
		if err := models.SetPlatformConfig(models.SecurityGradeNote, strings.TrimSpace(*req.Note)); err != nil {
			utils.InternalError(c, "保存失败")
			return
		}
	}
	grade, note := models.GetSecurityGrade()
	utils.Success(c, gin.H{"message": "等保记录已保存", "grade": grade, "note": note})
}
