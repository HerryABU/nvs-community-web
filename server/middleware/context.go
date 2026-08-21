package middleware

import "github.com/gin-gonic/gin"

// 鉴权上下文 key —— 唯一权威来源。
// 所有读写用户身份的地方都应引用这些常量或对应的 getter，避免散落硬编码字符串
// （历史上曾出现 "userID" vs "user_id" 大小写不一致导致鉴权失效的 bug）。
const (
	CtxUserID    = "userID"
	CtxUserEmail = "userEmail"
	CtxUserRole  = "userRole"
)

// UserID 从 gin context 读取当前登录用户 ID（未登录返回 0）
func UserID(c *gin.Context) uint {
	return c.GetUint(CtxUserID)
}

// UserRole 从 gin context 读取当前用户角色（未登录返回空串）
func UserRole(c *gin.Context) string {
	return c.GetString(CtxUserRole)
}

// UserEmail 从 gin context 读取当前用户邮箱（未登录返回空串）
func UserEmail(c *gin.Context) string {
	return c.GetString(CtxUserEmail)
}
