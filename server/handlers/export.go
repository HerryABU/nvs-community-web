package handlers

// export.go 子包桥接层。
//
// handlers 是一个大包，但业务域需要各自成目录（子包）。Go 里「目录 = 包」，
// 跨目录访问必须导出，而现有 155 个文件里有 100+ 处在调用这些横切函数。
// 为了**不改动既有声明与调用点**，这里用「导出别名」把需要的符号暴露给子包：
// 原函数名、原调用点一行都不用动，子包通过 handlers.CurrentUserID(...) 调用。
//
// 同理，反向依赖（子包里的函数被 handlers 侧调用）用回调变量解耦，避免
// handlers → 子包 → handlers 的循环 import。

import (
	"github.com/gin-gonic/gin"

	"nvs-server/models"
)

var (
	// CurrentUserID 当前登录用户 ID（0 = 未登录）
	CurrentUserID = currentUserID
	// CurrentUserRole 当前登录用户角色（空串 = 未登录）
	CurrentUserRole = currentUserRole
	// EnsureAdmin 管理员校验（失败时已写好 403 响应，调用方直接 return）
	EnsureAdmin = ensureAdmin
	// AtoiDefault 字符串转 int，失败取默认值
	AtoiDefault = atoiDefault
)

// MaybeTriggerAiBot 评论创建后触发 @ai 召唤。
//
// 实现位于子包 handlers/ai，由那边 init 时注册 —— 若让 handlers 直接 import 子包，
// 子包又要 import handlers 拿 CurrentUserID，就成环了。
// 未注册时为空实现，语义与「没有该功能」一致。
var MaybeTriggerAiBot = func(c *gin.Context, comment *models.Comment) {}
