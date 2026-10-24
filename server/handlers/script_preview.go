package handlers

import (
	"nvs-server/models"
	"nvs-server/scriptmode"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== 话本气泡体（script mode）预览 ====================

// ScriptPreview POST /api/script-preview { content, novel_id? }
// 编辑器侧实时预览：把「角色：台词」源文渲染成气泡 HTML（全部转义，无 XSS 面）。
// novel_id 存在时校验当前用户是否为该作品的创作成员，避免当成开放渲染服务滥用。
func ScriptPreview(c *gin.Context) {
	var req struct {
		Content string `json:"content"`
		NovelID uint   `json:"novel_id"`
		Mode    string `json:"mode"` // script（纯话本，默认） / mixed（散文与话本混排）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if len([]rune(req.Content)) > 20000 {
		utils.BadRequest(c, "预览内容过长（上限 20000 字）")
		return
	}
	if req.NovelID > 0 {
		novel, err := models.GetNovelByID(req.NovelID)
		if err != nil {
			utils.NotFound(c, "作品不存在")
			return
		}
		uid := currentUserID(c)
		if uid == 0 {
			utils.Unauthorized(c, "请先登录")
			return
		}
		if novel.AuthorID != uid && currentUserRole(c) != models.RoleAdmin && currentUserRole(c) != models.RolePolice {
			utils.Forbidden(c, "仅作品创作成员可预览")
			return
		}
	}
	lines, speakers := scriptmode.CountDialogue(req.Content)
	html := scriptmode.RenderHTML(req.Content)
	if req.Mode == "mixed" {
		html = scriptmode.RenderMixedHTML(req.Content)
	}
	utils.Success(c, gin.H{
		"html":          html,
		"speakers":      scriptmode.Speakers(req.Content),
		"lines":         lines,
		"speaker_count": speakers,
		"emote_keys":    scriptmode.EmoteKeys(), // 编辑器表情提示
	})
}
