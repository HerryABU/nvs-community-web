// auth_minor.go — 未成年检测与分级限制（F21.1）。
// 合规要点（《未成年人网络保护条例》）：
//   - 出生日期支持「自报」：不强制实名即可填报，服务端据此推算周岁并置 minor_protected；
//   - 实名认证（含国家 API 高级模式）通过的出生日期具有更高可信度，自报不可覆盖已实名数据；
//   - minor_mode=strict 时未成年人禁止发布作品（小说/博客/章节），评论等社区互动不受限；
//   - minor_mode=soft 仅前端提醒；off 关闭。
package handlers

import (
	"strings"
	"time"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// birthDateLayout 出生日期标准格式（前端 el-date-picker 默认 value-format）
const birthDateLayout = "2006-01-02"

// GET /api/auth/minor-status — 查询未成年模式与我的状态（需登录）
func GetMinorStatus(c *gin.Context) {
	userID := currentUserID(c)
	if userID == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	user, err := models.GetUserByID(userID)
	if err != nil || user == nil {
		utils.NotFound(c, "用户不存在")
		return
	}

	mode := string(models.GetMinorMode())
	resp := gin.H{
		"minor_mode":      mode,
		"birth_date_set":  user.BirthDate != nil,
		"minor_protected": user.MinorProtected,
		// 平台开启了保护且尚未填报出生日期时，前端应弹窗询问
		"need_ask": mode != string(models.MinorModeOff) && user.BirthDate == nil,
	}
	if user.BirthDate != nil {
		resp["birth_date"] = user.BirthDate.Format(birthDateLayout)
		resp["age"] = utils.AgeAt(*user.BirthDate, time.Now())
	}
	utils.Success(c, resp)
}

// POST /api/auth/birthdate — 自报出生日期（需登录）
// body: { birth_date: "2008-05-20" }
func SetBirthDate(c *gin.Context) {
	userID := currentUserID(c)
	if userID == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	var req struct {
		BirthDate string `json:"birth_date" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请选择出生日期")
		return
	}
	birth, err := time.Parse(birthDateLayout, strings.TrimSpace(req.BirthDate))
	if err != nil {
		utils.BadRequest(c, "出生日期格式应为 YYYY-MM-DD")
		return
	}
	now := time.Now()
	if birth.After(now) {
		utils.BadRequest(c, "出生日期不能晚于今天")
		return
	}
	if birth.Year() < 1900 {
		utils.BadRequest(c, "出生日期过早，请检查")
		return
	}

	user, err := models.GetUserByID(userID)
	if err != nil || user == nil {
		utils.NotFound(c, "用户不存在")
		return
	}
	// 已实名（含国家 API 核验）的出生日期来自证件，不允许自报覆盖
	if user.RealNameVerified {
		utils.Forbidden(c, "你已完成实名认证，出生日期以证件为准，不可自报修改")
		return
	}

	age := utils.AgeAt(birth, now)
	minor := age < 18
	if err := models.DB.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"birth_date":      &birth,
		"minor_protected": minor,
	}).Error; err != nil {
		utils.InternalError(c, "保存失败")
		return
	}

	recordUserAudit(c, userID, models.AuditChangeProfile, "填报出生日期（未成年检测）", "success")
	utils.SuccessMessage(c, "已保存", gin.H{
		"age":             age,
		"minor_protected": minor,
	})
}

// checkMinorPublishRestricted 严格保护模式下拦截未成年人发布作品，
// 已代发错误响应时返回 true。管理员/警方豁免（治理需要）。
func checkMinorPublishRestricted(c *gin.Context) bool {
	if models.GetMinorMode() != models.MinorModeStrict {
		return false
	}
	userID := currentUserID(c)
	if userID == 0 {
		return false // 未登录由后续鉴权处理
	}
	user, err := models.GetUserByID(userID)
	if err != nil || user == nil {
		return false
	}
	if user.Role == "admin" || user.Role == "police" {
		return false
	}
	if !user.MinorProtected {
		return false
	}
	utils.Forbidden(c, "本站已开启严格保护模式，未成年人不可发布作品，评论互动不受影响")
	return true
}
