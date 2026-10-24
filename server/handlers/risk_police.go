package handlers

// ==================== 充值风控（gap-audit #9）与警方工单（gap-audit #6）的接口层 ====================

import (
	"strconv"
	"strings"

	"nvs-server/models"
	"nvs-server/police"
	"nvs-server/risk"
	"nvs-server/security"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ─────────────── 充值风控 ───────────────

// AdminRiskConfig GET /api/admin/risk
func AdminRiskConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	utils.Success(c, gin.H{
		"config": risk.Load(), "stats": risk.Stats(),
		"note": "allow=正常入账；hold=入账但打标待复核；block=不入账待复核。程序不自动扣回已到账余额，钱的事必须有人判断。",
	})
}

// SaveRiskConfig PUT /api/admin/risk
func SaveRiskConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var in risk.Config
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := risk.Save(in); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.SuccessMessage(c, "已保存", risk.Load())
}

// AdminRiskEvents GET /api/admin/risk/events
func AdminRiskEvents(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	list, total, err := risk.List(page, size, c.Query("decision"), c.Query("status"))
	if err != nil {
		utils.InternalError(c, "读取失败")
		return
	}
	utils.Success(c, gin.H{"list": list, "total": total, "page": page, "size": size})
}

// HandleRiskEvent POST /api/admin/risk/events/:id/handle { status, note }
func HandleRiskEvent(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var in struct {
		Status string `json:"status"`
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := risk.Handle(uint(id), currentUserID(c), in.Status, in.Note); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "已处理", nil)
}

// ─────────────── 警方工单 ───────────────

func canHandleTickets(c *gin.Context) bool {
	role := currentUserRole(c)
	return role == models.RoleAdmin || role == models.RolePolice
}

// ListTickets GET /api/police/tickets
func ListTickets(c *gin.Context) {
	if !canHandleTickets(c) {
		utils.Forbidden(c, "仅警方/管理员可访问")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	var handlerID uint
	if c.Query("mine") == "1" {
		handlerID = currentUserID(c)
	}
	list, total, err := police.List(page, size, c.Query("status"), c.Query("priority"), c.Query("kind"), handlerID)
	if err != nil {
		utils.InternalError(c, "读取失败")
		return
	}
	utils.Success(c, gin.H{"list": list, "total": total, "stats": police.Stats()})
}

// GetTicket GET /api/police/tickets/:id
func GetTicket(c *gin.Context) {
	if !canHandleTickets(c) {
		utils.Forbidden(c, "仅警方/管理员可访问")
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	t, logs, err := police.Get(uint(id))
	if err != nil {
		utils.NotFound(c, "工单不存在")
		return
	}
	utils.Success(c, gin.H{"ticket": t, "logs": logs})
}

// CreateTicket POST /api/police/tickets
func CreateTicket(c *gin.Context) {
	if !canHandleTickets(c) {
		utils.Forbidden(c, "仅警方/管理员可建单")
		return
	}
	var in police.CreateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	t, err := police.Create(currentUserID(c), in)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "工单已创建", t)
}

// AssignTicket POST /api/police/tickets/:id/assign { handler_id }
func AssignTicket(c *gin.Context) {
	if !canHandleTickets(c) {
		utils.Forbidden(c, "仅警方/管理员可派单")
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var in struct {
		HandlerID uint `json:"handler_id"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.HandlerID == 0 {
		utils.BadRequest(c, "请指定处理人")
		return
	}
	if err := police.Assign(uint(id), in.HandlerID, currentUserID(c)); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "已派单", nil)
}

// SetTicketStatus POST /api/police/tickets/:id/status { status, note }
func SetTicketStatus(c *gin.Context) {
	if !canHandleTickets(c) {
		utils.Forbidden(c, "仅警方/管理员可处理")
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var in struct {
		Status string `json:"status"`
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := police.SetStatus(uint(id), currentUserID(c), in.Status, in.Note); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "已更新", nil)
}

// AddTicketNote POST /api/police/tickets/:id/notes { note }
func AddTicketNote(c *gin.Context) {
	if !canHandleTickets(c) {
		utils.Forbidden(c, "仅警方/管理员可处理")
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var in struct {
		Note string `json:"note"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := police.AddNote(uint(id), currentUserID(c), in.Note); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "已追加", nil)
}

// ─────────────── API Key 审计告警 ───────────────

// AdminKeyAlarms GET /api/admin/api-key-alarms
func AdminKeyAlarms(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	var acked *bool
	switch strings.TrimSpace(c.Query("acked")) {
	case "0":
		v := false
		acked = &v
	case "1":
		v := true
		acked = &v
	}
	list, total, err := security.ListKeyAlarms(page, size, c.Query("kind"), acked)
	if err != nil {
		utils.InternalError(c, "读取失败")
		return
	}
	utils.Success(c, gin.H{"list": list, "total": total})
}

// AckKeyAlarm POST /api/admin/api-key-alarms/:id/ack { note }
func AckKeyAlarm(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var in struct {
		Note string `json:"note"`
	}
	_ = c.ShouldBindJSON(&in)
	if err := security.AckKeyAlarm(uint(id), currentUserID(c), in.Note); err != nil {
		utils.InternalError(c, "处理失败")
		return
	}
	utils.SuccessMessage(c, "已标记处理", nil)
}
