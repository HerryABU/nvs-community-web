package forum

import (
	"strings"
	"time"

	"nvs-server/handlers/adminsite"
	"nvs-server/models"
	"nvs-server/models/platform"
	"nvs-server/utils/secure"

	"github.com/gin-gonic/gin"
)

// 论坛锁定类型：隔离墙（敏感分区）与密码（房间号/密码）分开处理，
// 前端据此展示不同门禁（敏感区确认 vs 密码/房间号输入）。
const (
	LockTypeSensitive = "sensitive" // 隔离墙：需进入确认（zone_access_logs）
	LockTypePassword  = "password"  // 密码/房间号门禁
)

// lockedForumPayload 未解锁时返回的最小化论坛负载：只暴露 id/type/is_public/locked，
// 不泄露论坛名称、描述、房间号、密码标记等元数据。
func lockedForumPayload(f *models.Forum) gin.H {
	return gin.H{
		"id":        f.ID,
		"type":      f.Type,
		"is_public": f.IsPublic,
		"locked":    true,
	}
}

// lockedPayload 统一的 locked 响应负载（论坛 + 空列表 + 锁定类型）。
// 敏感区会附带 zone（分区名，前端确认弹窗需要；分区名本身不属机密）。
func lockedPayload(f *models.Forum, lockType string, extra gin.H) gin.H {
	out := gin.H{
		"forum":      lockedForumPayload(f),
		"locked":     true,
		"lock_type":  lockType,
		"threads":    []models.Thread{},
		"total":      0,
		"page":       1,
		"sub_forums": []models.Forum{},
	}
	if lockType == LockTypeSensitive && f.Zone != "" {
		out["zone"] = f.Zone
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// lockTypeOf 判定论坛的锁定类型：
//   - 敏感分区（type=sensitive 且绑定 zone）→ 隔离墙；
//   - 其余（非公开/有密码）→ 密码门禁。
func lockTypeOf(f *models.Forum) string {
	if f != nil && f.Type == "sensitive" && f.Zone != "" {
		return LockTypeSensitive
	}
	return LockTypePassword
}

// checkForumAccess 校验当前用户对论坛的访问权限，供所有论坛/帖子读取与写入接口复用，
// 防止「改 URL 数字即可进入」的路径穿透（IDOR）。两类门禁分开：
//
//  1. 论坛创建者本人 / 警方 / 管理员：直接放行；
//  2. 隔离墙（type=sensitive 且绑定 zone）：仅校验 24 小时内是否有该分区
//     的进入确认落库记录（zone_access_logs）——仅前端 localStorage 确认不算；
//  3. 密码门禁（其余情况）：非公开需正确房间号，有密码需正确密码。
//
// 通过返回 (true, "", nil)；未通过返回 (false, 锁定类型, locked 负载)。
func checkForumAccess(c *gin.Context, f *models.Forum, userID uint) (bool, string, gin.H) {
	if f == nil || f.ID == 0 {
		return false, "", nil
	}
	// 创建者本人（未登录 userID=0 不豁免）
	if userID != 0 && f.CreatorID == userID {
		return true, "", nil
	}
	// 警方取证 / 管理员豁免
	if adminsite.IsPoliceOrAdminUser(c) {
		return true, "", nil
	}

	// ① 隔离墙（敏感分区）：只校验进入确认，不要求房间号/密码
	if lockTypeOf(f) == LockTypeSensitive {
		if platform.HasRecentZoneEnter(userID, f.Zone, 24*time.Hour) {
			return true, "", nil
		}
		return false, LockTypeSensitive, lockedPayload(f, LockTypeSensitive, nil)
	}

	// ② 密码门禁：房间号 / 密码（query 与自定义请求头二选一）
	room := c.Query("room")
	pass := c.Query("password")
	if room == "" {
		room = c.GetHeader("X-Forum-Room")
	}
	if pass == "" {
		pass = c.GetHeader("X-Forum-Password")
	}
	if !f.IsPublic && !strings.EqualFold(strings.TrimSpace(room), f.RoomCode) {
		return false, LockTypePassword, lockedPayload(f, LockTypePassword, nil)
	}
	if f.HasPassword && !secure.CheckPassword(pass, f.PasswordHash) {
		return false, LockTypePassword, lockedPayload(f, LockTypePassword, nil)
	}
	return true, "", nil
}

// checkForumAccessByID 按论坛 ID 取论坛后做门禁；论坛不存在返回 (false, "", nil, nil)。
func checkForumAccessByID(c *gin.Context, forumID, userID uint) (bool, string, *models.Forum, gin.H) {
	f, err := models.GetForumByID(forumID)
	if err != nil {
		return false, "", nil, nil
	}
	ok, lockType, payload := checkForumAccess(c, f, userID)
	return ok, lockType, f, payload
}

// redactThreadForum 去除帖子详情中外泄的论坛凭证字段（房间号 / 密码哈希标记）。
func redactThreadForum(t *models.Thread) {
	if t == nil || t.Forum == nil {
		return
	}
	t.Forum.RoomCode = ""
	t.Forum.PasswordHash = ""
	t.Forum.HasPassword = false
}
