package scripthandler

import (
	"strconv"
	"strings"

	"nvs-server/collab"
	"nvs-server/middleware"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== 话本气泡体：作品角色管理 ====================
// 角色跨章节复用（头像 / 主题色 / 默认左右），气泡编辑器插入气泡时直接选角色。
//
// 权限（对齐 collab 能力矩阵）：
//   - 读：主笔 / 平台 / 生效中的协作者
//   - 写：仅主笔（作品级设置，平台角色只做合规处置，不获得创作能力）
func actorOf(c *gin.Context, novelID uint) collab.Actor {
	return collab.ActorOf(novelID, middleware.UserID(c), middleware.UserRole(c))
}

func canRead(c *gin.Context, novelID uint) bool {
	a := actorOf(c, novelID)
	return a.IsOwner || a.IsPlatform || a.ActiveMember()
}

func canWrite(c *gin.Context, novelID uint) bool {
	return actorOf(c, novelID).Can(collab.CapEditNovel)
}

// novelIDOf 取路由里的作品 ID，并校验作品存在
func novelIDOf(c *gin.Context) (uint, bool) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	novelID := uint(id)
	if novelID == 0 {
		utils.BadRequest(c, "作品 ID 无效")
		return 0, false
	}
	if _, err := models.GetNovelByID(novelID); err != nil {
		utils.NotFound(c, "作品不存在")
		return 0, false
	}
	return novelID, true
}

// ListCharacters GET /api/novels/:id/characters
func ListCharacters(c *gin.Context) {
	novelID, ok := novelIDOf(c)
	if !ok {
		return
	}
	if !canRead(c, novelID) {
		utils.Forbidden(c, "无权查看该作品的角色")
		return
	}
	list, err := models.ListNovelCharacters(novelID)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{"list": list})
}

// CreateCharacter POST /api/novels/:id/characters
func CreateCharacter(c *gin.Context) {
	novelID, ok := novelIDOf(c)
	if !ok {
		return
	}
	if !canWrite(c, novelID) {
		utils.Forbidden(c, "仅主笔可管理作品角色")
		return
	}
	var req struct {
		Name      string `json:"name"`
		Avatar    string `json:"avatar"`
		Color     string `json:"color"`
		Side      string `json:"side"`
		Intro     string `json:"intro"`
		SortOrder int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	ch := &models.NovelCharacter{
		NovelID:   novelID,
		Name:      strings.TrimSpace(req.Name),
		Avatar:    strings.TrimSpace(req.Avatar),
		Color:     strings.TrimSpace(req.Color),
		Side:      models.NormalizeCharacterSide(req.Side),
		Intro:     strings.TrimSpace(req.Intro),
		SortOrder: req.SortOrder,
	}
	if err := models.CreateNovelCharacter(ch); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, ch)
}

// UpdateCharacter PUT /api/novels/:id/characters/:cid
func UpdateCharacter(c *gin.Context) {
	novelID, ok := novelIDOf(c)
	if !ok {
		return
	}
	if !canWrite(c, novelID) {
		utils.Forbidden(c, "仅主笔可管理作品角色")
		return
	}
	cid, _ := strconv.ParseUint(c.Param("cid"), 10, 64)
	ch, err := models.GetNovelCharacter(uint(cid))
	if err != nil || ch.NovelID != novelID {
		utils.NotFound(c, "角色不存在")
		return
	}
	// 指针字段：nil = 不改，避免零值被静默覆盖
	var req struct {
		Name      *string `json:"name"`
		Avatar    *string `json:"avatar"`
		Color     *string `json:"color"`
		Side      *string `json:"side"`
		Intro     *string `json:"intro"`
		SortOrder *int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	fields := map[string]interface{}{}
	if req.Name != nil {
		fields["name"] = strings.TrimSpace(*req.Name)
	}
	if req.Avatar != nil {
		fields["avatar"] = strings.TrimSpace(*req.Avatar)
	}
	if req.Color != nil {
		fields["color"] = strings.TrimSpace(*req.Color)
	}
	if req.Side != nil {
		fields["side"] = models.NormalizeCharacterSide(*req.Side)
	}
	if req.Intro != nil {
		fields["intro"] = strings.TrimSpace(*req.Intro)
	}
	if req.SortOrder != nil {
		fields["sort_order"] = *req.SortOrder
	}
	if err := models.UpdateNovelCharacterFields(uint(cid), fields); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, gin.H{"message": "已保存"})
}

// DeleteCharacter DELETE /api/novels/:id/characters/:cid
func DeleteCharacter(c *gin.Context) {
	novelID, ok := novelIDOf(c)
	if !ok {
		return
	}
	if !canWrite(c, novelID) {
		utils.Forbidden(c, "仅主笔可管理作品角色")
		return
	}
	cid, _ := strconv.ParseUint(c.Param("cid"), 10, 64)
	ch, err := models.GetNovelCharacter(uint(cid))
	if err != nil || ch.NovelID != novelID {
		utils.NotFound(c, "角色不存在")
		return
	}
	if err := models.DeleteNovelCharacter(uint(cid)); err != nil {
		utils.InternalError(c, "删除失败")
		return
	}
	utils.Success(c, gin.H{"message": "已删除"})
}
