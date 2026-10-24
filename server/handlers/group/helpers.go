package grouphandler

import (
	"encoding/json"
	"strings"

	"nvs-server/middleware"
	"nvs-server/models"
	"nvs-server/security"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== 群组接口层内部辅助 ====================
// 群组 handler 放在独立子包（handlers/group）以保持功能内聚，
// 因此不能直接用 handlers 包内的未导出辅助函数，这里按需最小实现（语义与主包一致）。

// requirePlatformAdmin 管理员守卫（与 handlers.ensureAdmin 等价：查库判定 role=admin）
func requirePlatformAdmin(c *gin.Context) bool {
	uid := middleware.UserID(c)
	user, _ := models.GetUserByID(uid)
	if user == nil || user.Role != "admin" {
		utils.Forbidden(c, "需要管理员权限")
		c.Abort()
		return false
	}
	return true
}

// markBadWords 违禁词 mark 动作命中 → 写待审标记（与 handlers.flagBadWords 同语义，去重）
func markBadWords(hits []security.BadWordHit, contentType string, contentID, userID uint, raw, title string) {
	var matched []string
	for _, h := range hits {
		if h.Action == "mark" {
			matched = append(matched, h.Matched)
		}
	}
	if len(matched) == 0 {
		return
	}
	if exists, _ := models.HasPendingFlag(contentType, contentID); exists {
		return
	}
	b, _ := json.Marshal(matched)
	_ = models.CreateBadWordFlag(&models.BadWordFlag{
		ContentType: contentType, ContentID: contentID, UserID: userID,
		ContentTitle: title, Snippet: snippetOf(raw), MatchedWords: string(b), Status: "pending",
	})
}

// snippetOf 内容快照（前 200 字），供管理员快速判断
func snippetOf(s string) string {
	r := []rune(s)
	if len(r) > 200 {
		return string(r[:200])
	}
	return s
}

// displayNameOf 用户展示名：昵称优先，其次用户名（nil 安全）
func displayNameOf(u *models.User) string {
	if u == nil {
		return ""
	}
	if strings.TrimSpace(u.Nickname) != "" {
		return u.Nickname
	}
	return u.Username
}
