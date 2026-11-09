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

// lockedForumPayload 未解锁时返回的最小化负载：只暴露 id/type/is_public/locked，
// 不泄露论坛名称、描述、房间号、密码标记等元数据。
func lockedForumPayload(f *models.Forum) gin.H {
	return gin.H{
		"id":        f.ID,
		"type":      f.Type,
		"is_public": f.IsPublic,
		"locked":    true,
	}
}

// lockedPayload 统一的 locked 响应负载（论坛 + 空列表）。
func lockedPayload(f *models.Forum, extra gin.H) gin.H {
	out := gin.H{
		"forum":      lockedForumPayload(f),
		"locked":     true,
		"threads":    []models.Thread{},
		"total":      0,
		"page":       1,
		"sub_forums": []models.Forum{},
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// checkForumAccess 校验当前用户对论坛的访问权限，供所有论坛/帖子读取与写入接口复用，
// 防止「改 URL 数字即可进入」的路径穿透（IDOR）。规则：
//
//  1. 论坛创建者本人 / 警方 / 管理员：直接放行；
//  2. 敏感区（type=sensitive）：必须 24 小时内在该分区有「进入确认」落库记录
//     （仅前端 localStorage 确认不算，服务端要能查到 zone_access_logs）；
//  3. 非公开（is_public=false）自建论坛：必须提供正确房间号；
//  4. 有密码的论坛：必须提供正确密码。
//
// 通过返回 (true, nil)；未通过返回 (false, locked 负载)。
func checkForumAccess(c *gin.Context, f *models.Forum, userID uint) (bool, gin.H) {
	if f == nil || f.ID == 0 {
		return false, nil
	}
	// 创建者本人（未登录 userID=0 不豁免）
	if userID != 0 && f.CreatorID == userID {
		return true, nil
	}
	// 警方取证 / 管理员豁免
	if adminsite.IsPoliceOrAdminUser(c) {
		return true, nil
	}

	// 敏感区服务端门禁：确认动作必须已落库且未过期
	if f.Type == "sensitive" && f.Zone != "" {
		if !platform.HasRecentZoneEnter(userID, f.Zone, 24*time.Hour) {
			return false, lockedPayload(f, nil)
		}
	}

	// 房间号 / 密码：query 与自定义请求头二选一（密码不走 URL 是审计要求，
	// 兼容旧调用保留 query 兜底）。
	room := c.Query("room")
	pass := c.Query("password")
	if room == "" {
		room = c.GetHeader("X-Forum-Room")
	}
	if pass == "" {
		pass = c.GetHeader("X-Forum-Password")
	}

	if !f.IsPublic && !strings.EqualFold(strings.TrimSpace(room), f.RoomCode) {
		return false, lockedPayload(f, nil)
	}
	if f.HasPassword && !secure.CheckPassword(pass, f.PasswordHash) {
		return false, lockedPayload(f, nil)
	}
	return true, nil
}

// checkForumAccessByID 按论坛 ID 取论坛后做门禁；论坛不存在返回 (false, nil)。
func checkForumAccessByID(c *gin.Context, forumID, userID uint) (bool, *models.Forum, gin.H) {
	f, err := models.GetForumByID(forumID)
	if err != nil {
		return false, nil, nil
	}
	ok, payload := checkForumAccess(c, f, userID)
	return ok, f, payload
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
