package handlers

import (
	"strconv"
	"strings"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== 长篇 AI 记忆（§13.5：大纲 + 章节摘要 + 上下文组装） ====================

// ensureNovelOwner 校验作品归属（仅作者本人可管理 AI 记忆）
func ensureNovelOwner(c *gin.Context, novelID uint) bool {
	n, err := models.GetNovelByID(novelID)
	if err != nil {
		utils.NotFound(c, "作品不存在")
		return false
	}
	if n.AuthorID != currentUserID(c) {
		utils.Error(c, 403, 40301, "仅作者本人可管理该作品的 AI 记忆")
		return false
	}
	return true
}

// AiMemoryList GET /api/ai/memory?novel_id= — 我的作品记忆（大纲 + 摘要）
func AiMemoryList(c *gin.Context) {
	if !models.IsAiEnabled() {
		utils.BadRequest(c, "AI 功能未开启")
		return
	}
	novelID, _ := strconv.ParseUint(c.Query("novel_id"), 10, 64)
	if novelID == 0 {
		utils.BadRequest(c, "novel_id 无效")
		return
	}
	if !ensureNovelOwner(c, uint(novelID)) {
		return
	}
	list, err := models.ListNovelMemories(uint(novelID))
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	outline, summaries := models.CountNovelMemories(uint(novelID))
	utils.Success(c, gin.H{
		"list":       list,
		"outline":    outline,
		"characters": models.GetNovelCharacters(uint(novelID)),
		"summaries":  int(summaries),
		"budget":     models.AiContextBudget,
		"recent":     models.AiContextRecentChapters,
	})
}

// AiMemorySaveOutline PUT /api/ai/memory/outline — 保存/更新作品大纲
func AiMemorySaveOutline(c *gin.Context) {
	if !models.IsAiEnabled() {
		utils.BadRequest(c, "AI 功能未开启")
		return
	}
	var req struct {
		NovelID uint   `json:"novel_id"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if req.NovelID == 0 {
		utils.BadRequest(c, "novel_id 无效")
		return
	}
	if !ensureNovelOwner(c, req.NovelID) {
		return
	}
	if len([]rune(req.Content)) > 8000 {
		utils.BadRequest(c, "大纲过长（上限 8000 字）")
		return
	}
	if err := models.UpsertNovelOutline(req.NovelID, req.Content); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.Success(c, gin.H{"message": "大纲已保存"})
}

// AiMemorySaveCharacters PUT /api/ai/memory/characters — 保存/更新角色设定
// 角色设定是模板「角色信息」变量的来源，也是续写注入的稳定人设锚点。
func AiMemorySaveCharacters(c *gin.Context) {
	if !models.IsAiEnabled() {
		utils.BadRequest(c, "AI 功能未开启")
		return
	}
	var req struct {
		NovelID uint   `json:"novel_id"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if req.NovelID == 0 {
		utils.BadRequest(c, "novel_id 无效")
		return
	}
	if !ensureNovelOwner(c, req.NovelID) {
		return
	}
	if len([]rune(req.Content)) > 8000 {
		utils.BadRequest(c, "角色设定过长（上限 8000 字）")
		return
	}
	if err := models.UpsertNovelCharacters(req.NovelID, req.Content); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.Success(c, gin.H{"message": "角色设定已保存"})
}

// AiMemorySaveSummary PUT /api/ai/memory/summary — 保存/更新章节摘要
func AiMemorySaveSummary(c *gin.Context) {
	if !models.IsAiEnabled() {
		utils.BadRequest(c, "AI 功能未开启")
		return
	}
	var req struct {
		NovelID   uint   `json:"novel_id"`
		ChapterID uint   `json:"chapter_id"`
		Content   string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if req.NovelID == 0 || req.ChapterID == 0 {
		utils.BadRequest(c, "novel_id / chapter_id 无效")
		return
	}
	if !ensureNovelOwner(c, req.NovelID) {
		return
	}
	if len([]rune(req.Content)) > 4000 {
		utils.BadRequest(c, "摘要过长（上限 4000 字）")
		return
	}
	if err := models.UpsertChapterSummary(req.NovelID, req.ChapterID, req.Content); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.Success(c, gin.H{"message": "章节摘要已保存"})
}

// AiMemoryDeleteSummary DELETE /api/ai/memory/summary — 删除章节摘要
func AiMemoryDeleteSummary(c *gin.Context) {
	if !models.IsAiEnabled() {
		utils.BadRequest(c, "AI 功能未开启")
		return
	}
	var req struct {
		NovelID   uint `json:"novel_id"`
		ChapterID uint `json:"chapter_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if req.NovelID == 0 || req.ChapterID == 0 {
		utils.BadRequest(c, "novel_id / chapter_id 无效")
		return
	}
	if !ensureNovelOwner(c, req.NovelID) {
		return
	}
	if err := models.DeleteChapterMemory(req.NovelID, req.ChapterID); err != nil {
		utils.InternalError(c, "删除失败")
		return
	}
	utils.Success(c, gin.H{"message": "已删除"})
}

// AiMemoryContext POST /api/ai/memory/context — 预览组装的长篇上下文（不扣费）
func AiMemoryContext(c *gin.Context) {
	if !models.IsAiEnabled() {
		utils.BadRequest(c, "AI 功能未开启")
		return
	}
	var req struct {
		NovelID   uint   `json:"novel_id"`
		ChapterID uint   `json:"chapter_id"`
		Text      string `json:"text"`
		Budget    int    `json:"budget"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if req.NovelID == 0 {
		utils.BadRequest(c, "novel_id 无效")
		return
	}
	if !ensureNovelOwner(c, req.NovelID) {
		return
	}
	ctx, tokens := models.BuildNovelContext(req.NovelID, req.ChapterID, req.Text, req.Budget)
	utils.Success(c, gin.H{"context": ctx, "tokens": tokens})
}

// aiContextBlock 将长篇上下文包装为 system 前缀（AI 调用注入用）
func aiContextBlock(ctx string) string {
	ctx = strings.TrimSpace(ctx)
	if ctx == "" {
		return ""
	}
	return "以下是该作品的既有设定与前文脉络，续写时请严格保持人物、设定、语气一致：\n\n" + ctx + "\n\n——请基于以上信息继续创作。"
}
