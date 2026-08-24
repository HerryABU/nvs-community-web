package handlers

// 176 号令合规补全的公开/用户侧 API：
//   GET  /api/algorithms/description  算法推荐说明（公开，算法透明度）
//   GET  /api/announcements           站点公告列表（公开，已发布）
//   GET  /api/users/me/preferences    用户偏好设置（JWT，个性化推荐开关）
//   PUT  /api/users/me/preferences    更新用户偏好设置

import (
	"nvs-server/middleware"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// GetAlgoDescription 公开算法推荐说明（面向普通用户的白话说明）
// GET /api/algorithms/description
func GetAlgoDescription(c *gin.Context) {
	_, _, personalized, description := models.GetAlgoConfig()
	utils.Success(c, gin.H{"description": description, "personalized_enabled": personalized})
}

// GetUserPreferences 获取当前用户偏好设置
// GET /api/users/me/preferences
func GetUserPreferences(c *gin.Context) {
	userID := middleware.UserID(c)
	if userID == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}
	user, err := models.GetUserByID(userID)
	if err != nil {
		utils.NotFound(c, "用户不存在")
		return
	}
	utils.Success(c, gin.H{"personalized_recommend": user.PersonalizedRecommend})
}

// UpdateUserPreferences 更新当前用户偏好设置
// PUT /api/users/me/preferences
func UpdateUserPreferences(c *gin.Context) {
	userID := middleware.UserID(c)
	if userID == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}
	var req struct {
		PersonalizedRecommend *bool `json:"personalized_recommend"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if req.PersonalizedRecommend != nil {
		if err := models.DB.Model(&models.User{}).Where("id = ?", userID).
			Update("personalized_recommend", *req.PersonalizedRecommend).Error; err != nil {
			utils.InternalError(c, "保存失败")
			return
		}
	}
	user, _ := models.GetUserByID(userID)
	utils.Success(c, gin.H{"message": "偏好已保存", "personalized_recommend": user.PersonalizedRecommend})
}
