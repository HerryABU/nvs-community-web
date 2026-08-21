package handlers

import (
	"nvs-server/middleware"

	"github.com/gin-gonic/gin"
)

// currentUserID 读取当前登录用户 ID（未登录返回 0）。
// 统一封装鉴权上下文读取，避免散落硬编码的 context key 字符串
// （历史 bug：曾出现 "userID" vs "user_id" 大小写不一致导致鉴权失效）。
func currentUserID(c *gin.Context) uint {
	return middleware.UserID(c)
}

// currentUserRole 读取当前用户角色（未登录返回空串）
func currentUserRole(c *gin.Context) string {
	return middleware.UserRole(c)
}

// currentUserEmail 读取当前用户邮箱（未登录返回空串）
func currentUserEmail(c *gin.Context) string {
	return middleware.UserEmail(c)
}

// setCurrentUserRole 在请求上下文中临时设置用户角色（如 reader 首次发布作品后同步提升为 author）
func setCurrentUserRole(c *gin.Context, role string) {
	c.Set(middleware.CtxUserRole, role)
}
