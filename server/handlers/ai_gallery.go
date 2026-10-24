package handlers

import (
	"encoding/json"
	"strconv"
	"strings"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== 作品广场（灵感广场） ====================

// galleryItem 公开作品视图（不含 cost/user_id 等隐私字段）
func galleryItem(t *models.AiMediaTask) gin.H {
	images := []string{}
	video := ""
	var payload map[string]interface{}
	if json.Unmarshal([]byte(t.Result), &payload) == nil {
		if arr, ok := payload["images"].([]interface{}); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok {
					images = append(images, s)
				}
			}
		}
		if s, ok := payload["video"].(string); ok {
			video = s
		}
	}
	return gin.H{
		"id": t.ID, "kind": t.Kind, "title": t.Title, "prompt": t.Prompt,
		"model_name": t.ModelName, "style": t.Style, "likes": t.Likes,
		"images": images, "video": video, "created_at": t.CreatedAt,
	}
}

// AiGalleryList GET /api/ai/gallery?kind=image|video&style=&page= — 公开作品流
func AiGalleryList(c *gin.Context) {
	// 开关：关闭「公开」后仅登录用户可见
	if !models.GetAiFeatureFlags().PublicGallery && currentUserID(c) == 0 {
		utils.BadRequest(c, "灵感广场仅登录可见")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	kind := strings.TrimSpace(c.Query("kind"))
	style := strings.TrimSpace(c.Query("style"))
	list, total, err := models.ListPublicMedia(kind, style, page, 24)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	items := make([]gin.H, 0, len(list))
	for i := range list {
		items = append(items, galleryItem(&list[i]))
	}
	utils.Success(c, gin.H{
		"list": items, "total": total, "page": page,
		"styles": models.GalleryStyles(kind),
	})
}

// AiGalleryPublish POST /api/ai/gallery/:id/publish — 发布/取消发布我的作品
func AiGalleryPublish(c *gin.Context) {
	userID := currentUserID(c)
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Public bool   `json:"public"`
		Title  string `json:"title"`
		Style  string `json:"style"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := models.PublishAiMedia(uint(id), userID, req.Public, req.Title, req.Style); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, gin.H{"message": map[bool]string{true: "已发布到灵感广场", false: "已取消发布"}[req.Public]})
}

// AiGalleryLike POST /api/ai/gallery/:id/like — 点赞
func AiGalleryLike(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := models.LikeAiMedia(uint(id)); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, gin.H{"message": "ok"})
}

// AiMyMediaPublic GET /api/ai/media/:id/public — 查我的作品发布状态（发布弹窗回填）
func AiMyMediaPublic(c *gin.Context) {
	userID := currentUserID(c)
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	t, err := models.GetAiMediaTask(uint(id), userID)
	if err != nil {
		utils.NotFound(c, "作品不存在")
		return
	}
	utils.Success(c, gin.H{"id": t.ID, "is_public": t.IsPublic, "title": t.Title, "style": t.Style})
}

// ==================== 思维导图 ====================

// AiMindmap POST /api/ai/mindmap — 需求 → 层级大纲（Markdown），前端渲染为导图
