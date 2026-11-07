package ochandler

// ==================== OC 组（阵容）接口层 ====================
// OC 组是用户个人创作资产，与群组解耦：权限以「拥有者 / 公开过审 / 管理员」为主，
// 不再走群组权限（CanManageGroup 在 groupID==0 时返回 false，不适合 OC 组）。
// 群组可反向召唤 OC 组（见 invoke.go）。

import (
	"strconv"

	"nvs-server/middleware"
	"nvs-server/oc"
	"nvs-server/utils/infra"

	"github.com/gin-gonic/gin"
)

// ListOcGroups GET /api/oc/groups?scope=mine|public|usable&keyword=&page=&page_size=
func ListOcGroups(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	ps, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	list, total, err := oc.ListOcGroups(middleware.UserID(c), c.Query("scope"), c.Query("keyword"), page, ps)
	if err != nil {
		infra.InternalError(c, "查询失败")
		return
	}
	infra.Success(c, gin.H{"list": list, "total": total})
}

// GetOcGroup GET /api/oc/groups/:gid —— 详情（含成员角色卡，含 furry 字段）
func GetOcGroupHandler(c *gin.Context) {
	id := parseUint(c.Param("gid"))
	g, err := oc.GetOcGroup(id, middleware.UserID(c), isAdmin(c))
	if err != nil {
		infra.Forbidden(c, err.Error())
		return
	}
	members, merr := oc.ListOcGroupMembers(id)
	if merr != nil {
		infra.InternalError(c, "读取成员失败")
		return
	}
	infra.Success(c, gin.H{"group": g, "members": members})
}

// CreateOcGroup POST /api/oc/groups
func CreateOcGroupHandler(c *gin.Context) {
	uid := middleware.UserID(c)
	if uid == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	var in oc.OcGroupInput
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	g, err := oc.CreateOcGroup(uid, in)
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已创建 OC 组", g)
}

// UpdateOcGroup PUT /api/oc/groups/:gid
func UpdateOcGroupHandler(c *gin.Context) {
	uid := middleware.UserID(c)
	id := parseUint(c.Param("gid"))
	var in oc.OcGroupInput
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	g, err := oc.UpdateOcGroup(uid, id, in)
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已保存", g)
}

// DeleteOcGroup DELETE /api/oc/groups/:gid
func DeleteOcGroupHandler(c *gin.Context) {
	if err := oc.DeleteOcGroup(middleware.UserID(c), parseUint(c.Param("gid"))); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已删除 OC 组（保留记录以备追溯）", nil)
}

// ListOcGroupMembers GET /api/oc/groups/:gid/members
func ListOcGroupMembersHandler(c *gin.Context) {
	id := parseUint(c.Param("gid"))
	if _, err := oc.GetOcGroup(id, middleware.UserID(c), isAdmin(c)); err != nil {
		infra.Forbidden(c, err.Error())
		return
	}
	members, err := oc.ListOcGroupMembers(id)
	if err != nil {
		infra.InternalError(c, "读取成员失败")
		return
	}
	infra.Success(c, gin.H{"members": members})
}

// AddOcGroupMember POST /api/oc/groups/:gid/members { character_id }
func AddOcGroupMemberHandler(c *gin.Context) {
	uid := middleware.UserID(c)
	id := parseUint(c.Param("gid"))
	var in struct {
		CharacterID uint `json:"character_id"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.CharacterID == 0 {
		infra.BadRequest(c, "请选择要加入的角色卡")
		return
	}
	if err := oc.AddOcGroupMember(id, uid, in.CharacterID); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已加入 OC 组", nil)
}

// RemoveOcGroupMember DELETE /api/oc/groups/:gid/members/:cid
func RemoveOcGroupMemberHandler(c *gin.Context) {
	uid := middleware.UserID(c)
	id := parseUint(c.Param("gid"))
	cid := parseUint(c.Param("cid"))
	if err := oc.RemoveOcGroupMember(id, uid, cid); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已移出 OC 组", nil)
}

// ReorderOcGroupMembers PUT /api/oc/groups/:gid/members/order { character_ids:[...] }
func ReorderOcGroupMembersHandler(c *gin.Context) {
	uid := middleware.UserID(c)
	id := parseUint(c.Param("gid"))
	var in struct {
		CharacterIDs []uint `json:"character_ids"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	if err := oc.ReorderOcGroupMembers(id, uid, in.CharacterIDs); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已调整排序", nil)
}

// ReviewOcGroup POST /api/admin/oc-groups/:gid/review { approve, note } —— 平台审核公开 OC 组
func ReviewOcGroupHandler(c *gin.Context) {
	if !isAdmin(c) {
		infra.Forbidden(c, "仅管理员可审核")
		return
	}
	id := parseUint(c.Param("gid"))
	var in struct {
		Approve bool   `json:"approve"`
		Note    string `json:"note"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	if err := oc.ReviewOcGroup(id, in.Approve, in.Note); err != nil {
		infra.InternalError(c, "审核失败")
		return
	}
	msg := "已驳回"
	if in.Approve {
		msg = "已通过"
	}
	infra.SuccessMessage(c, msg, nil)
}
