package handlers

import (
	"strconv"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== AI 提示词库（§5.4） ====================

// AiPromptList GET /api/ai/prompts?kind=&q= — 我的提示词 + 平台预设
func AiPromptList(c *gin.Context) {
	userID := currentUserID(c)
	list, err := models.ListAiPrompts(userID, c.Query("kind"), c.Query("q"))
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	// 去掉 content 之外的无用字段无必要；这里原样返回，前端按需展示
	utils.Success(c, gin.H{
		"list":       list,
		"categories": models.ListAiPromptCategories(userID),
	})
}

type aiPromptReq struct {
	Kind     string `json:"kind"`
	Category string `json:"category"`
	Title    string `json:"title"`
	Content  string `json:"content"`
}

// AiPromptCreate POST /api/ai/prompts — 新建自定义提示词
func AiPromptCreate(c *gin.Context) {
	userID := currentUserID(c)
	var req aiPromptReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	p, err := models.CreateAiPrompt(userID, req.Kind, req.Category, req.Title, req.Content)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, p)
}

// AiPromptUpdate PUT /api/ai/prompts/:id — 修改自定义提示词
func AiPromptUpdate(c *gin.Context) {
	userID := currentUserID(c)
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req aiPromptReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := models.UpdateAiPrompt(uint(id), userID, req.Kind, req.Category, req.Title, req.Content); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, gin.H{"message": "已保存"})
}

// AiPromptDelete DELETE /api/ai/prompts/:id — 删除自定义提示词
func AiPromptDelete(c *gin.Context) {
	userID := currentUserID(c)
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := models.DeleteAiPrompt(uint(id), userID); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, gin.H{"message": "已删除"})
}

// AiPromptUse POST /api/ai/prompts/:id/use — 记录使用（用于常用排序）
func AiPromptUse(c *gin.Context) {
	userID := currentUserID(c)
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	p, err := models.GetAiPrompt(uint(id), userID)
	if err != nil {
		utils.NotFound(c, "提示词不存在")
		return
	}
	models.TouchAiPrompt(p.ID)
	utils.Success(c, gin.H{"message": "ok"})
}
