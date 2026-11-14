package ochandler

// ==================== OC 立项制：项目 / 世界 / 成员 / 跨世界链接 ====================
// 「先立项、再创作」：项目是亚文化创作区的顶层容器（世界观/企划）。
// 成员在项目内创作角色卡与世界书；世界之间可互相链接；项目可选公开或密码公开。

import (
	"strconv"
	"strings"

	"nvs-server/handlers/hctx"
	"nvs-server/middleware"
	"nvs-server/oc"
	"nvs-server/utils/infra"

	"github.com/gin-gonic/gin"
)

// firstNonEmpty 返回第一个非空白值（用于 query/header 兼容取参）
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// ─── 项目 ───

// ListProjects GET /api/oc/projects?scope=usable|mine|public|joined&category=&keyword=
func ListProjects(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	ps, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	list, total, err := oc.ListProjects(middleware.UserID(c), c.Query("scope"), c.Query("category"), c.Query("keyword"), page, ps)
	if err != nil {
		infra.InternalError(c, "查询失败")
		return
	}
	infra.Success(c, gin.H{"list": list, "total": total})
}

// GetProject GET /api/oc/projects/:pid（密码项目带 ?token=）
func GetProject(c *gin.Context) {
	token := firstNonEmpty(c.Query("token"), c.GetHeader("X-Oc-Project-Token"))
	p, err := oc.GetProject(parseUint(c.Param("pid")), middleware.UserID(c), isAdmin(c), token)
	if err != nil {
		infra.Forbidden(c, err.Error())
		return
	}
	infra.Success(c, p)
}

// CreateProject POST /api/oc/projects
func CreateProject(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	if uid == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	var in oc.ProjectInput
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	p, err := oc.CreateProject(uid, in)
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "项目已创建", p)
}

// UpdateProject PUT /api/oc/projects/:pid
func UpdateProject(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	if uid == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	var in oc.ProjectInput
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	p, err := oc.UpdateProject(uid, parseUint(c.Param("pid")), in)
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已保存项目设置", p)
}

// DeleteProject DELETE /api/oc/projects/:pid
func DeleteProject(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	if uid == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	if err := oc.DeleteProject(uid, parseUint(c.Param("pid"))); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "项目已删除（世界已解绑保留）", nil)
}

// ProjectAccess POST /api/oc/projects/:pid/access  {password} → {token}
func ProjectAccess(c *gin.Context) {
	var in struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	token, err := oc.VerifyProjectPassword(parseUint(c.Param("pid")), strings.TrimSpace(in.Password))
	if err != nil {
		infra.Forbidden(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "验证通过", gin.H{"token": token})
}

// ─── 成员 ───

// ListProjectMembers GET /api/oc/projects/:pid/members
func ListProjectMembers(c *gin.Context) {
	list, err := oc.ListProjectMembers(parseUint(c.Param("pid")))
	if err != nil {
		infra.InternalError(c, "查询失败")
		return
	}
	infra.Success(c, gin.H{"list": list})
}

// AddProjectMember POST /api/oc/projects/:pid/members {username, role}
func AddProjectMember(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	if uid == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	var in struct {
		Username string `json:"username"`
		Role     string `json:"role"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	m, err := oc.AddProjectMember(parseUint(c.Param("pid")), uid, in.Username, in.Role)
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已添加成员", m)
}

// UpdateProjectMemberRole PUT /api/oc/projects/:pid/members/:uid {role}
func UpdateProjectMemberRole(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	if uid == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	var in struct {
		Role string `json:"role"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	target := parseUint(c.Param("uid"))
	if err := oc.UpdateProjectMemberRole(parseUint(c.Param("pid")), uid, target, in.Role); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已更新成员角色", nil)
}

// RemoveProjectMember DELETE /api/oc/projects/:pid/members/:uid
func RemoveProjectMember(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	if uid == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	if err := oc.RemoveProjectMember(parseUint(c.Param("pid")), uid, parseUint(c.Param("uid"))); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已移除成员", nil)
}

// ─── 项目 ↔ 世界 / 角色 归属 ───

// AttachWorld POST /api/oc/projects/:pid/worlds {book_id}
func AttachWorld(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	if uid == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	var in struct {
		BookID uint `json:"book_id"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	if err := oc.AttachWorldToProject(parseUint(c.Param("pid")), in.BookID, uid, isAdmin(c)); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已归入项目", nil)
}

// DetachWorld DELETE /api/oc/projects/:pid/worlds/:bid
func DetachWorld(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	if uid == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	if err := oc.DetachWorldFromProject(parseUint(c.Param("pid")), parseUint(c.Param("bid")), uid, isAdmin(c)); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已移出项目", nil)
}

// ProjectWorlds GET /api/oc/projects/:pid/worlds
func ProjectWorlds(c *gin.Context) {
	list, err := oc.ProjectWorlds(parseUint(c.Param("pid")))
	if err != nil {
		infra.InternalError(c, "查询失败")
		return
	}
	infra.Success(c, gin.H{"list": list})
}

// SetCharacterProject POST /api/oc/projects/:pid/characters {character_id}
func SetCharacterProject(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	if uid == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	var in struct {
		CharacterID uint `json:"character_id"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	if err := oc.SetCharacterProject(parseUint(c.Param("pid")), in.CharacterID, uid, isAdmin(c)); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已归入项目", nil)
}

// ─── 跨世界链接 ───

// ListWorldLinks GET /api/oc/world-links?book_id=
func ListWorldLinks(c *gin.Context) {
	list, err := oc.ListWorldLinks(parseUint(c.Query("book_id")))
	if err != nil {
		infra.InternalError(c, "查询失败")
		return
	}
	infra.Success(c, gin.H{"list": list})
}

// AddWorldLink POST /api/oc/world-links {from_book_id, to_book_id, label}
func AddWorldLink(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	if uid == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	var in struct {
		FromBookID uint   `json:"from_book_id"`
		ToBookID   uint   `json:"to_book_id"`
		Label      string `json:"label"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	l, err := oc.AddWorldLink(in.FromBookID, in.ToBookID, in.Label, uid, isAdmin(c))
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已建立跨世界链接", l)
}

// RemoveWorldLink DELETE /api/oc/world-links/:id
func RemoveWorldLink(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	if uid == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	if err := oc.RemoveWorldLink(parseUint(c.Param("id")), uid, isAdmin(c)); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已删除链接", nil)
}
