package handlers

// ==================== 注册门槛 / 设备指纹 / 邀请码（gap-audit #14）====================
// 前台：注册前置校验（前端也能提前提示，不用等提交失败）
// 后台：配置、邀请码管理、设备绑定列表（同一设备多少账号一眼看清）

import (
	"strconv"
	"strings"

	"nvs-server/gate"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// RegisterGateHint GET /api/auth/register-hint —— 注册前提示（是否需要邀请码/限制）
func RegisterGateHint(c *gin.Context) {
	cfg := gate.Load()
	utils.Success(c, gin.H{
		"enabled":           cfg.Enabled,
		"require_invite":    cfg.RequireInvite,
		"post_min_hours":    cfg.PostMinAgeHours,
		"comment_min_hours": cfg.CommentMinAgeHours,
		"disposable_block":  cfg.BlockDisposableEmail,
	})
}

// RegisterGateCheck POST /api/auth/register-check —— 提交前预检（不消费邀请码）
func RegisterGateCheck(c *gin.Context) {
	deviceID, _ := gate.DeviceIDFrom(c)
	var in struct {
		Email  string `json:"email"`
		Invite string `json:"invite"`
	}
	_ = c.ShouldBindJSON(&in)
	if err := gate.CanRegister(gate.RegisterInput{
		IP: c.ClientIP(), DeviceID: deviceID, Email: in.Email, Invite: in.Invite,
	}); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "可以注册", gin.H{"device_id": deviceID})
}

// AdminGateConfig GET /api/admin/gate
func AdminGateConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	utils.Success(c, gin.H{
		"config": gate.Load(),
		"note":   "门槛默认全关；开启后：邀请码挡批量注册、设备指纹挡同机小号、账号年龄挡注册即刷。设备指纹可被伪造，属提高成本而非绝对防御。",
	})
}

// SaveGateConfig PUT /api/admin/gate
func SaveGateConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var in gate.Config
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := gate.Save(in); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.SuccessMessage(c, "已保存", gate.Load())
}

// AdminInvites GET /api/admin/gate/invites
func AdminInvites(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	list, total, err := gate.ListInvites(page, size)
	if err != nil {
		utils.InternalError(c, "读取失败")
		return
	}
	utils.Success(c, gin.H{"list": list, "total": total, "page": page, "size": size})
}

// CreateInvites POST /api/admin/gate/invites
func CreateInvites(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var in struct {
		Count    int    `json:"count"`
		MaxUses  int    `json:"max_uses"`
		Note     string `json:"note"`
		TTLHours int    `json:"ttl_hours"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if in.Count <= 0 {
		in.Count = 1
	}
	if in.Count > 100 {
		in.Count = 100
	}
	if in.MaxUses <= 0 {
		in.MaxUses = 1
	}
	out := make([]string, 0, in.Count)
	for i := 0; i < in.Count; i++ {
		inv, err := gate.CreateInvite(currentUserID(c), in.MaxUses, in.Note, in.TTLHours)
		if err != nil {
			utils.InternalError(c, "生成失败")
			return
		}
		out = append(out, inv.Code)
	}
	utils.SuccessMessage(c, "已生成", gin.H{"codes": out})
}

// RevokeInvite POST /api/admin/gate/invites/:id/revoke
func RevokeInvite(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := gate.RevokeInvite(uint(id)); err != nil {
		utils.InternalError(c, "作废失败")
		return
	}
	utils.SuccessMessage(c, "已作废（保留已用次数）", nil)
}

// AdminDeviceBindings GET /api/admin/gate/devices —— 按设备聚合，看「谁在开小号」
func AdminDeviceBindings(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	keyword := strings.TrimSpace(c.Query("keyword"))
	limit := 50
	type row struct {
		DeviceID string `json:"device_id"`
		Accounts int64  `json:"accounts"`
		LastIP   string `json:"last_ip"`
		LastAt   string `json:"last_at"`
	}
	var rows []row
	q := models.DB.Model(&gate.DeviceBinding{}).
		Select("device_id, COUNT(1) as accounts, MAX(ip) as last_ip, MAX(created_at) as last_at").
		Group("device_id").Having("COUNT(1) >= 1").
		Order("accounts DESC, MAX(created_at) DESC").Limit(limit)
	if keyword != "" {
		q = q.Where("device_id LIKE ? OR ip LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if err := q.Scan(&rows).Error; err != nil {
		utils.InternalError(c, "读取失败")
		return
	}
	cfg := gate.Load()
	utils.Success(c, gin.H{
		"list": rows, "max_accounts_per_device": cfg.MaxAccountsPerDevice,
		"note": "accounts≥3 的条目是「同设备多账号」的重点关注对象；但如果站点在校园网/公司网后面，会因服务端兜底指纹（ip+ua）撞车而误判，判断时以 X-Device-Id 上报的为准。",
	})
}
