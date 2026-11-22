package ochandler

// ==================== 角色形象图库 ====================
// 世界详情页的「形象图库」用到的 HTTP 层：
//   GET    /api/oc/characters/:cid/arts  角色卡下全部形象图
//   POST   /api/oc/characters/:cid/arts  新增
//   PUT    /api/oc/character-arts/:aid   修改
//   DELETE /api/oc/character-arts/:aid   删除
//   GET    /api/oc/projects/:pid/arts    世界下全部形象图（一次拉全，前端按角色分组）
//
// 规则都在 oc 包（art.go），这里只做鉴权、取参与错误文案。

import (
	"github.com/gin-gonic/gin"

	"nvs-server/handlers/hctx"
	"nvs-server/middleware"
	"nvs-server/oc"
	"nvs-server/utils/infra"
)

// projectToken 密码公开项目的访问码（query 或请求头，与项目接口口径一致）
func projectToken(c *gin.Context) string {
	return firstNonEmpty(c.Query("token"), c.GetHeader("X-Oc-Project-Token"))
}

// ListCharacterArts GET /api/oc/characters/:cid/arts
func ListCharacterArts(c *gin.Context) {
	cid := parseUint(c.Param("cid"))
	// 先过角色可见性：项目内角色对项目可见者开放，私有卡仍只有本人能看
	if _, err := oc.GetCharacterWithToken(cid, middleware.UserID(c), isAdmin(c), projectToken(c)); err != nil {
		infra.Forbidden(c, err.Error())
		return
	}
	list, err := oc.ListCharacterArts(cid)
	if err != nil {
		infra.InternalError(c, "查询失败")
		return
	}
	infra.Success(c, gin.H{"list": list})
}

// CreateCharacterArt POST /api/oc/characters/:cid/arts
func CreateCharacterArt(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	if uid == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	var in oc.ArtInput
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	a, err := oc.CreateCharacterArt(uid, parseUint(c.Param("cid")), isAdmin(c), in)
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已添加形象图", a)
}

// UpdateCharacterArt PUT /api/oc/character-arts/:aid
func UpdateCharacterArt(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	if uid == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	var in oc.ArtInput
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	a, err := oc.UpdateCharacterArt(uid, parseUint(c.Param("aid")), isAdmin(c), in)
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已保存", a)
}

// DeleteCharacterArt DELETE /api/oc/character-arts/:aid
func DeleteCharacterArt(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	if uid == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	if err := oc.DeleteCharacterArt(uid, parseUint(c.Param("aid")), isAdmin(c)); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已删除", nil)
}

// ListProjectArts GET /api/oc/projects/:pid/arts
func ListProjectArts(c *gin.Context) {
	pid := parseUint(c.Param("pid"))
	if _, err := oc.GetProject(pid, middleware.UserID(c), isAdmin(c), projectToken(c)); err != nil {
		infra.Forbidden(c, err.Error())
		return
	}
	list, err := oc.ListProjectArts(pid)
	if err != nil {
		infra.InternalError(c, "查询失败")
		return
	}
	infra.Success(c, gin.H{"list": list})
}
