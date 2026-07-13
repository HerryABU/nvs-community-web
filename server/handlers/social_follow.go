package handlers

import (
	"strconv"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ============ 关注作者 ============

// POST /api/follow/:id — 关注作者
func FollowAuthor(c *gin.Context) {
	userID := c.GetUint("userID")
	authorID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	if uint(authorID) == userID {
		utils.BadRequest(c, "不能关注自己")
		return
	}

	if models.IsFollowing(userID, uint(authorID)) {
		utils.BadRequest(c, "已关注该作者")
		return
	}

	if err := models.CreateFollow(userID, uint(authorID)); err != nil {
		utils.InternalError(c, "关注失败")
		return
	}
	utils.Success(c, gin.H{"message": "关注成功"})
}

// DELETE /api/follow/:id — 取消关注
func UnfollowAuthor(c *gin.Context) {
	userID := c.GetUint("userID")
	authorID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	if err := models.DeleteFollow(userID, uint(authorID)); err != nil {
		utils.InternalError(c, "取消失败")
		return
	}
	utils.Success(c, gin.H{"message": "已取消关注"})
}

// GET /api/following — 我关注的人列表
func ListFollowing(c *gin.Context) {
	userID := c.GetUint("userID")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))

	users, total, err := models.GetFollowingList(userID, page, 20)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	for i := range users {
		users[i].Email = ""
	}
	utils.Success(c, gin.H{"list": users, "total": total})
}

// GET /api/followers — 关注我的人列表
func ListFollowers(c *gin.Context) {
	userID := c.GetUint("userID")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))

	users, total, err := models.GetFollowerList(userID, page, 20)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	for i := range users {
		users[i].Email = ""
	}
	utils.Success(c, gin.H{"list": users, "total": total})
}

// GET /api/follow/check/:id — 检查是否已关注某人
func CheckFollow(c *gin.Context) {
	userID := c.GetUint("userID")
	authorID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	isFollow := models.IsFollowing(userID, uint(authorID))
	utils.Success(c, gin.H{"is_following": isFollow})
}

// GET /api/follow/stats — 关注统计
func GetFollowStats(c *gin.Context) {
	userID := c.GetUint("userID")
	following := models.GetFollowingCount(userID)
	followers := models.GetFollowersCount(userID)
	utils.Success(c, gin.H{"following": following, "followers": followers})
}
