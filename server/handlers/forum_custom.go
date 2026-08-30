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
		RoomCode    string `json:"room_code"` // 房间号（可选，自定义；留空自动生成）
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

	// 建版额度校验：全局默认（管理员配置） + 按人覆盖（管理员可单独削减到 0）
	quota := models.ForumQuotaFor(userID)
	if quota.Limit == 0 {
		utils.Forbidden(c, "管理员已限制你创建自建论坛，如有疑问请联系管理员")
		return
	}
	if quota.Limit > 0 && quota.Used >= int64(quota.Limit) {
		utils.Forbidden(c, fmt.Sprintf("每人最多创建 %d 个自建论坛（当前 %d 个），如需更多请联系管理员", quota.Limit, quota.Used))
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

	// 房间号：可自定义（数字+英文字母 4-16 位）或留空自动生成
	roomCode := strings.ToUpper(strings.TrimSpace(req.RoomCode))
	if roomCode != "" {
		valid := true
		if len(roomCode) < 4 || len(roomCode) > 16 {
			valid = false
		}
		for _, ch := range roomCode {
			if !((ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')) {
				valid = false
				break
			}
		}
		if !valid {
			utils.BadRequest(c, "房间号须为 4-16 位数字或英文字母（不含符号）")
			return
		}
		if models.RoomCodeExists(roomCode) {
			utils.BadRequest(c, "该房间号已被占用，请换一个或留空自动生成")
			return
		}
	} else {
		roomCode = models.GenerateUniqueRoomCode()
	}

	forum := &models.Forum{
		Name:         name,
		Description:  req.Description,
		Type:         "custom",
		CreatorID:    userID,
		IsPublic:     req.IsPublic,
		PasswordHash: passwordHash,
		RoomCode:     roomCode,
		ReviewStatus: "pending", // 自建论坛先创建直接可用，待管理员审查
	}
	if err := models.CreateForum(forum); err != nil {
		utils.InternalError(c, "创建论坛失败")
		return
	}

	utils.Success(c, forum)
}

// ListMyForums GET /api/forums/mine — 我创建的自建论坛（附建版额度状态）
func ListMyForums(c *gin.Context) {
	userID := currentUserID(c)
	var forums []models.Forum
	models.DB.Where("creator_id = ? AND type = ?", userID, "custom").
		Order("id DESC").Find(&forums)
	utils.Success(c, gin.H{
		"list":  forums,
		"quota": models.ForumQuotaFor(userID),
	})
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

// EnterForumByRoomCode POST /api/forums/enter-by-code — 凭房间号进入（不可见论坛的唯一入口凭证）。
// 请求：{room_code, password?}。两段式：
//  - 仅传 room_code：校验房间号有效性；若论坛有密码则返回 need_password=true（不视为错误），由前端弹密码输入框；
//  - room_code + password：校验房间号并校验密码，通过后返回 forum_id。
func EnterForumByRoomCode(c *gin.Context) {
	var req struct {
		RoomCode string `json:"room_code" binding:"required"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请输入房间号")
		return
	}

	forum, err := models.GetForumByRoomCode(req.RoomCode)
	if err != nil {
		utils.NotFound(c, "房间号不存在或论坛已删除")
		return
	}
	if forum.Type != "custom" {
		utils.NotFound(c, "房间号不存在")
		return
	}

	// 未提供密码：返回论坛信息与是否需密码（进入密码墙由前端二次提交）
	if req.Password == "" {
		utils.Success(c, gin.H{
			"forum_id":     forum.ID,
			"name":         forum.Name,
			"has_password": forum.HasPassword,
			"need_password": forum.HasPassword,
			"is_public":    forum.IsPublic,
		})
		return
	}

	// 提供密码：校验
	if forum.HasPassword && !utils.CheckPassword(req.Password, forum.PasswordHash) {
		utils.Forbidden(c, "密码错误")
		return
	}

	utils.Success(c, gin.H{
		"forum_id":     forum.ID,
		"name":         forum.Name,
		"has_password": forum.HasPassword,
		"need_password": false,
		"is_public":    forum.IsPublic,
	})
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

	// 管理员封禁标记：管理员执行的禁言，论坛创建者无权解除
	isAdmin := isAdminUser(c)

	// upsert：已存在则更新
	var existing models.ForumMute
	if err := models.DB.Where("forum_id = ? AND user_id = ?", uint(forumID), req.UserID).First(&existing).Error; err == nil {
		models.DB.Model(&existing).Updates(map[string]interface{}{
			"muted_until": until,
			"reason":      security.SanitizePlainText(req.Reason),
			"admin_ban":   existing.AdminBan || isAdmin, // 管理员升级/维持为管理员封禁；创建者重禁不降级
		})
	} else {
		models.DB.Create(&models.ForumMute{
			ForumID:    uint(forumID),
			UserID:     req.UserID,
			MutedUntil: until,
			Reason:     security.SanitizePlainText(req.Reason),
			AdminBan:   isAdmin,
		})
	}

	utils.Success(c, gin.H{"message": "已禁言"})
}

// ForumUnmuteUser POST /api/forums/:id/unmute — 解除禁言。
// 权限分层：管理员可解禁任意禁言；论坛创建者只能解禁自己/创建者执行的禁言，
// 无法解禁管理员封禁的（AdminBan=true）记录。
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

	// 创建者解禁时：管理员封禁的记录不可解
	if !isAdminUser(c) {
		var m models.ForumMute
		if err := models.DB.Where("forum_id = ? AND user_id = ?", uint(forumID), req.UserID).First(&m).Error; err == nil && m.AdminBan {
			utils.Forbidden(c, "该禁言由管理员执行，仅管理员可解除")
			return
		}
	}

	models.DB.Where("forum_id = ? AND user_id = ?", uint(forumID), req.UserID).Delete(&models.ForumMute{})
	utils.Success(c, gin.H{"message": "已解除禁言"})
}

// ListForumMutes GET /api/forums/:id/mutes — 论坛禁言列表（创建者/管理员）
func ListForumMutes(c *gin.Context) {
	forumID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	forum, err := models.GetForumByID(uint(forumID))
	if err != nil {
		utils.NotFound(c, "论坛不存在")
		return
	}
	if forum.Type != "custom" || !isForumModerator(c, forum) {
		utils.Forbidden(c, "无权查看")
		return
	}

	var mutes []models.ForumMute
	models.DB.Where("forum_id = ?", uint(forumID)).Order("created_at DESC").Find(&mutes)

	// 补充用户名与封禁执行者描述
	for i := range mutes {
		var u models.User
		if models.DB.Select("nickname", "username").First(&u, mutes[i].UserID).Error == nil {
			mutes[i].Username = u.Nickname
			if mutes[i].Username == "" {
				mutes[i].Username = u.Username
			}
		}
		if mutes[i].AdminBan {
			mutes[i].BannedBy = "管理员"
		} else {
			mutes[i].BannedBy = "论坛创建者"
		}
	}
	utils.Success(c, mutes)
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

	// 方案A：删除语义统一——回帖先归档到 softdel_posts 留证，再物理删除（posts 无 deleted_at）
	operator := ""
	if u, e := models.GetUserByID(userID); e == nil && u != nil {
		operator = u.Username
	}
	_, _ = models.ArchivePostsByThread(models.DB, uint(threadID), userID, operator,
		"删除帖子（级联删除回帖）", "moderator")
	models.DB.Where("thread_id = ?", uint(threadID)).Delete(&models.Post{})
	// 帖子本身：先归档留证，再软删除（threads 保留 deleted_at）
	if err := models.ArchiveSoftDelete("moderator", "thread", uint(threadID), userID, operator,
		"删除帖子", security.SanitizePlainText(thread.Title), thread); err == nil {
		models.DB.Delete(&models.Thread{}, uint(threadID))
	} else {
		// 留证失败则中止删除（数据完整性优先）
		utils.InternalError(c, "留证归档失败，删除已中止")
		return
	}
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

	// 方案A：删除语义统一——先归档到 softdel_posts 留证，再物理删除（posts 无 deleted_at）
	operator := ""
	if u, e := models.GetUserByID(userID); e == nil && u != nil {
		operator = u.Username
	}
	summary := post.Content
	if runes := []rune(summary); len(runes) > 200 {
		summary = string(runes[:200])
	}
	// 楼层模型：删的是楼层主回帖时，楼内回复一并留证 + 删除（否则回复变孤儿，页面上悬浮无主）
	var children []models.Post
	models.DB.Where("root_id = ?", postID).Find(&children)
	removed := 1
	for i := range children {
		cs := children[i].Content
		if runes := []rune(cs); len(runes) > 200 {
			cs = string(runes[:200])
		}
		_ = models.ArchiveSoftDelete("moderator", "post", children[i].ID, userID, operator,
			"删除回帖（级联删除楼内回复）", cs, children[i])
	}

	if err := models.ArchiveSoftDelete("moderator", "post", uint(postID), userID, operator,
		"删除回帖", summary, post); err != nil {
		utils.InternalError(c, "留证归档失败，删除已中止")
		return
	}
	models.DB.Delete(&models.Post{}, uint(postID))
	if len(children) > 0 {
		childIDs := make([]uint, 0, len(children))
		for _, ch := range children {
			childIDs = append(childIDs, ch.ID)
		}
		models.DB.Where("id IN ?", childIDs).Delete(&models.Post{})
		removed += len(children)
	}
	models.DB.Model(&models.Thread{}).Where("id = ?", post.ThreadID).
		UpdateColumn("post_count", gorm.Expr("CASE WHEN post_count >= ? THEN post_count - ? ELSE 0 END", removed, removed))
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
