package novel

import (
	"strconv"

	"nvs-server/handlers/hctx"
	"nvs-server/models"
	"nvs-server/models/social"
	"nvs-server/utils/infra"

	"github.com/gin-gonic/gin"
)

// GET /api/novels/:id/derivatives
// 列出引用该原著（站内同人）的衍生作品。可见性：仅原著作者的粉丝或作者本人可见（#76）。
func ListDerivatives(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		infra.BadRequest(c, "无效的作品 ID")
		return
	}
	origin, err := models.GetNovelByID(uint(id))
	if err != nil {
		infra.NotFound(c, "作品不存在")
		return
	}

	userID := hctx.CurrentUserID(c)
	visible := origin.AuthorID == userID
	if !visible && userID != 0 {
		visible = social.IsFollowing(userID, origin.AuthorID)
	}
	if !visible {
		infra.Success(c, gin.H{"visible": false, "items": []models.Novel{}})
		return
	}

	var list []models.Novel
	if err := models.DB.Where("derivative_origin_id = ? AND derivative_type = ?", id, "site").
		Preload("Author").Order("created_at DESC").Find(&list).Error; err != nil {
		infra.InternalError(c, "查询衍生作品失败")
		return
	}
	for i := range list {
		list[i].CategoryNames = models.GetNovelCategoryNames(list[i].ID)
	}

	infra.Success(c, gin.H{
		"visible": true,
		"items":   list,
		"origin": gin.H{
			"id":     origin.ID,
			"title":  origin.Title,
			"author": originAuthorName(origin),
		},
	})
}

func originAuthorName(n *models.Novel) string {
	if n.Author != nil {
		return n.Author.Nickname
	}
	return ""
}
