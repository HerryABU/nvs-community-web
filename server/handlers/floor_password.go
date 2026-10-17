// floor_password.go — 楼中楼「楼加密」处理器（入口限制，非内容加密）：
//
//	VerifyFloorPassword POST /api/comments/:id/verify-password  验证楼的访问密码
//	SetFloorPassword    POST /api/comments/:id/password         设置/修改/解除楼的访问密码
//	GetFloorDetail      GET  /api/comments/:id/detail           读取单楼全部内容（凭密码解锁后）
//
// 安全模型与「加密论坛」同构：
//   - 密码只走请求体 / X-Floor-Password 头，不进 URL（防日志/历史泄露）；
//   - 解锁豁免：楼作者本人、警方（police）、管理员（admin）；
//   - 其他用户即使持有有效 token/cookie/JWT，无密码也拿不到楼内容
//     （列表脱敏 + 楼内回复剔除 + 楼详情 403，三条读取路径全部收口）。
package handlers

import (
	"strconv"
	"strings"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// floorPasswordMinLen 楼的访问密码最小长度（防 1 位弱密码）
const floorPasswordMinLen = 4

// resolveFloorPasswordViewer 统一解析查看者身份与请求携带的解锁密码（单楼端点）。
func resolveFloorPasswordViewer(c *gin.Context) (viewerID uint, isStaff bool, password string) {
	viewerID = currentUserID(c)
	isStaff = isPoliceOrAdminUser(c)
	password = c.GetHeader("X-Floor-Password")
	if password == "" {
		password = strings.TrimSpace(c.Query("password"))
	}
	return
}

// parseFloorPasswords 解析 X-Floor-Password 头（评论列表的多楼解锁场景）。
// 格式：`id:password,id:password`。单楼端点（verify/set/detail）直接传纯密码，
// 由调用方包成 map[uint]string{id: pwd}。密码不含逗号/冒号（SetFloorPassword 已校验）。
func parseFloorPasswords(raw string) map[uint]string {
	out := map[uint]string{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		idx := strings.Index(part, ":")
		if idx <= 0 {
			continue
		}
		id, err := strconv.ParseUint(strings.TrimSpace(part[:idx]), 10, 64)
		if err != nil || id == 0 {
			continue
		}
		if pwd := strings.TrimSpace(part[idx+1:]); pwd != "" {
			out[uint(id)] = pwd
		}
	}
	return out
}

// VerifyFloorPassword POST /api/comments/:id/verify-password
// 请求体：{password}。用于前端 🔒 占位解锁：验证通过后前端把密码存入
// sessionStorage，后续列表/详情请求带 X-Floor-Password 头。
func VerifyFloorPassword(c *gin.Context) {
	floorID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请输入访问密码")
		return
	}

	floor, err := models.GetCommentByID(uint(floorID))
	if err != nil {
		utils.NotFound(c, "楼层不存在")
		return
	}
	if !floor.HasPassword {
		utils.Success(c, gin.H{"ok": true, "has_password": false})
		return
	}

	viewerID, isStaff, _ := resolveFloorPasswordViewer(c)
	if !models.FloorAccessUnlocked(floor, viewerID, isStaff, map[uint]string{floor.ID: req.Password}) {
		utils.Forbidden(c, "密码错误")
		return
	}
	utils.Success(c, gin.H{"ok": true, "has_password": true})
}

// SetFloorPassword POST /api/comments/:id/password
// 请求体：{password}。空密码 = 解除加密；非空 = 设置/修改访问密码。
// 权限：楼作者本人 或 管理员/警方（协助处置）。仅顶层评论（楼）可设。
func SetFloorPassword(c *gin.Context) {
	userID := currentUserID(c)
	floorID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	password := strings.TrimSpace(req.Password)

	floor, err := models.GetCommentByID(uint(floorID))
	if err != nil {
		utils.NotFound(c, "楼层不存在")
		return
	}
	// 只有「楼」（顶层评论）能设访问密码；楼内回复不单独加密
	if floor.ParentID != nil && *floor.ParentID != 0 {
		utils.BadRequest(c, "只有楼层主贴可以设置访问密码")
		return
	}
	// 作者本人或警方/管理员
	if floor.UserID != userID && !isPoliceOrAdminUser(c) {
		utils.Forbidden(c, "无权修改此楼的访问密码")
		return
	}

	if password == "" {
		// 解除加密
		if err := models.SetFloorPassword(floor.ID, "", false); err != nil {
			utils.InternalError(c, "解除加密失败")
			return
		}
		utils.Success(c, gin.H{"ok": true, "has_password": false, "message": "已解除该楼访问密码"})
		return
	}
	if len([]rune(password)) < floorPasswordMinLen {
		utils.BadRequest(c, "访问密码至少 "+strconv.Itoa(floorPasswordMinLen)+" 位")
		return
	}
	// 逗号/冒号是多楼密码头（id:pwd,id:pwd）的分隔符，禁止出现在密码里
	if strings.ContainsAny(password, ",:") {
		utils.BadRequest(c, "访问密码不能包含逗号或冒号")
		return
	}

	hash, err := utils.HashPassword(password)
	if err != nil {
		utils.InternalError(c, "密码处理失败")
		return
	}
	if err := models.SetFloorPassword(floor.ID, hash, true); err != nil {
		utils.InternalError(c, "设置访问密码失败")
		return
	}
	utils.Success(c, gin.H{"ok": true, "has_password": true, "message": "已设置该楼访问密码"})
}

// GetFloorDetail GET /api/comments/:id/detail
// 单楼整读（楼主贴 + 全部楼内回复）。加密楼未解锁时返回 403（前端引导输密码），
// 杜绝「拿楼层 id 拼 URL 直接进楼」绕过列表脱敏。
func GetFloorDetail(c *gin.Context) {
	floorID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	floor, err := models.GetCommentByID(uint(floorID))
	if err != nil {
		utils.NotFound(c, "楼层不存在")
		return
	}
	viewerID, isStaff, password := resolveFloorPasswordViewer(c)
	if !models.FloorAccessUnlocked(floor, viewerID, isStaff, map[uint]string{floor.ID: password}) {
		utils.Forbidden(c, "该楼已加密，请输入访问密码")
		return
	}

	full, replies, err := models.GetFloorWithReplies(floor.ID)
	if err != nil {
		utils.InternalError(c, "读取楼层失败")
		return
	}
	utils.Success(c, gin.H{
		"floor":   full,
		"replies": replies,
	})
}
