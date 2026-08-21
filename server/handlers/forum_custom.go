package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"nvs-server/models"
	"nvs-server/security"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ==================== 用户自建论坛 ====================

// CreateUserForum POST /api/forums — 用户创建自建论坛（先创建直接可用，后续管理员审查）
func CreateUserForum(c *gin.Context) {
	userID := currentUserID(c)

	var req struct {
		Name        string `json:"name" binding:"required,max=128"`
		Description string `json:"description"`
		IsPublic    bool   `json:"is_public"` // 是否在大厅自建区公开显示
		Password    string `json:"password"`  // 进入密码（可选）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请填写论坛名称")
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		utils.BadRequest(c, "请填写论坛名称")
		return
	}

	// 进入密码（可选）加密存储
	passwordHash := ""
	if req.Password != "" {
		h, err := utils.HashPassword(req.Password)
		if err != nil {
			utils.InternalError(c, "密码处理失败")
			return
		}
		passwordHash = h
	}

	forum := &models.Forum{
		Name:         name,
		Description:  req.Description,
		Type:         "custom",
		CreatorID:    userID,
		IsPublic:     req.IsPublic,
		PasswordHash: passwordHash,
		ReviewStatus: "pending", // 自建论坛先创建直接可用，待管理员审查
	}
	if err := models.CreateForum(forum); err != nil {
		utils.InternalError(c, "创建论坛失败")
		return
	}

	utils.Success(c, forum)
}

// ListMyForums GET /api/forums/mine — 我创建的自建论坛
func ListMyForums(c *gin.Context) {
	userID := currentUserID(c)
	var forums []models.Forum
	models.DB.Where("creator_id = ? AND type = ?", userID, "custom").
		Order("id DESC").Find(&forums)
	utils.Success(c, forums)
}

// VerifyForumPassword POST /api/forums/:id/verify — 验证论坛进入密码
func VerifyForumPassword(c *gin.Context) {
	forumID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请输入进入密码")
		return
	}

	forum, err := models.GetForumByID(uint(forumID))
	if err != nil {
		utils.NotFound(c, "论坛不存在")
		return
	}

	if !forum.HasPassword {
		utils.Success(c, gin.H{"ok": true})
		return
	}
	if !utils.CheckPassword(req.Password, forum.PasswordHash) {
		utils.Forbidden(c, "密码错误")
		return
	}
	utils.Success(c, gin.H{"ok": true})
}

// ==================== 论坛创建者审核与禁言 ====================

// isForumModerator 是否论坛管理者（管理员 或 论坛创建者）
func isForumModerator(c *gin.Context, forum *models.Forum) bool {
	if ensureAdmin(c) {
		return true
	}
	return forum.CreatorID != 0 && forum.CreatorID == currentUserID(c)
}

// ForumMuteUser POST /api/forums/:id/mute — 禁言（论坛创建者或管理员）
func ForumMuteUser(c *gin.Context) {
	forumID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	forum, err := models.GetForumByID(uint(forumID))
	if err != nil {
		utils.NotFound(c, "论坛不存在")
		return
	}
	if forum.Type != "custom" || !isForumModerator(c, forum) {
		utils.Forbidden(c, "无权操作")
		return
	}

	var req struct {
		UserID         uint `json:"user_id" binding:"required"`
		DurationHours  int  `json:"duration_hours"` // 禁言时长（小时），0=永久
		Reason         string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}

	var until *time.Time
	if req.DurationHours > 0 {
		t := time.Now().Add(time.Duration(req.DurationHours) * time.Hour)
		until = &t
	}

	// upsert：已存在则更新
	var existing models.ForumMute
	if err := models.DB.Where("forum_id = ? AND user_id = ?", uint(forumID), req.UserID).First(&existing).Error; err == nil {
		models.DB.Model(&existing).Updates(map[string]interface{}{
			"muted_until": until,
			"reason":      security.SanitizePlainText(req.Reason),
		})
	} else {
		models.DB.Create(&models.ForumMute{
			ForumID:    uint(forumID),
			UserID:     req.UserID,
			MutedUntil: until,
			Reason:     security.SanitizePlainText(req.Reason),
		})
	}

	utils.Success(c, gin.H{"message": "已禁言"})
}

// ForumUnmuteUser POST /api/forums/:id/unmute — 解除禁言（论坛创建者或管理员）
func ForumUnmuteUser(c *gin.Context) {
	forumID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	forum, err := models.GetForumByID(uint(forumID))
	if err != nil {
		utils.NotFound(c, "论坛不存在")
		return
	}
	if forum.Type != "custom" || !isForumModerator(c, forum) {
		utils.Forbidden(c, "无权操作")
		return
	}

	var req struct {
		UserID uint `json:"user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}

	models.DB.Where("forum_id = ? AND user_id = ?", uint(forumID), req.UserID).Delete(&models.ForumMute{})
	utils.Success(c, gin.H{"message": "已解除禁言"})
}

// DeleteThreadByModerator DELETE /api/threads/:id — 删除帖子（作者/论坛创建者/管理员）
func DeleteThreadByModerator(c *gin.Context) {
	userID := currentUserID(c)
	threadID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	thread, err := models.GetThreadByID(uint(threadID))
	if err != nil {
		utils.NotFound(c, "帖子不存在")
		return
	}

	// 权限：作者本人 / 论坛创建者 / 管理员
	if thread.UserID != userID {
		forum, ferr := models.GetForumByID(thread.ForumID)
		if ferr != nil || (forum.Type != "custom" || !isForumModerator(c, forum)) {
			utils.Forbidden(c, "无权删除此帖子")
			return
		}
	}

	models.DB.Where("thread_id = ?", uint(threadID)).Delete(&models.Post{})
	models.DB.Delete(&models.Thread{}, uint(threadID))
	// 更新论坛帖子计数
	if thread.ForumID > 0 {
		models.DB.Model(&models.Forum{}).Where("id = ?", thread.ForumID).
			UpdateColumn("thread_count", gorm.Expr("CASE WHEN thread_count > 0 THEN thread_count - 1 ELSE 0 END"))
	}
	// 合规审计：内容删除处置（含作者/所属论坛）
	auditEmergency(c, models.AuditContentDelete, "thread", uint(threadID),
		fmt.Sprintf("管理员删除帖子：%s（作者ID：%d，ForumID：%d）",
			security.SanitizePlainText(thread.Title), thread.UserID, thread.ForumID))
	utils.Success(c, gin.H{"message": "帖子已删除"})
}

// DeletePostByModerator DELETE /api/posts/:id — 删除回帖（作者/论坛创建者/管理员）
func DeletePostByModerator(c *gin.Context) {
	userID := currentUserID(c)
	postID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	var post models.Post
	if err := models.DB.First(&post, postID).Error; err != nil {
		utils.NotFound(c, "回帖不存在")
		return
	}

	if post.UserID != userID {
		thread, terr := models.GetThreadByID(post.ThreadID)
		if terr != nil {
			utils.Forbidden(c, "无权删除")
			return
		}
		forum, ferr := models.GetForumByID(thread.ForumID)
		if ferr != nil || (forum.Type != "custom" || !isForumModerator(c, forum)) {
			utils.Forbidden(c, "无权删除此回帖")
			return
		}
	}

	models.DB.Delete(&models.Post{}, uint(postID))
	models.DB.Model(&models.Thread{}).Where("id = ?", post.ThreadID).
		UpdateColumn("post_count", gorm.Expr("CASE WHEN post_count > 0 THEN post_count - 1 ELSE 0 END"))
	// 合规审计：内容删除处置（含作者/所属帖子）
	auditEmergency(c, models.AuditContentDelete, "post", uint(postID),
		fmt.Sprintf("管理员删除回帖（作者ID：%d，ThreadID：%d）", post.UserID, post.ThreadID))
	utils.Success(c, gin.H{"message": "回帖已删除"})
}

// checkUserMuted 全局禁言检查（评论/发帖/回帖前调用），返回 true 表示已拦截
func checkUserMuted(c *gin.Context) bool {
	userID := currentUserID(c)
	user, err := models.GetUserByID(userID)
	if err != nil || user == nil {
		return false
	}
	if user.IsMuted() {
		reason := user.MuteReason
		if reason == "" {
			reason = "您已被禁言，请稍后再试"
		} else {
			reason = "您已被禁言：" + reason
		}
		utils.Forbidden(c, reason)
		return true
	}
	return false
}
