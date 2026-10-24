package handlers

import (
	"strconv"
	"strings"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// AiMarks GET /api/ai/marks?items=blog:12,novel_chapter:34
// 公开批量查询 AI 生成标记（用于详情页徽标）——只回模型名，不含任何用户内容。
func AiMarks(c *gin.Context) {
	raw := strings.TrimSpace(c.Query("items"))
	if raw == "" {
		utils.Success(c, gin.H{"marks": map[string]string{}})
		return
	}
	parts := strings.Split(raw, ",")
	if len(parts) > 50 {
		parts = parts[:50] // 上限保护
	}
	items := make([]models.AiMarkItem, 0, len(parts))
	for _, p := range parts {
		kv := strings.SplitN(strings.TrimSpace(p), ":", 2)
		if len(kv) != 2 {
			continue
		}
		id, err := strconv.ParseUint(kv[1], 10, 64)
		if err != nil {
			continue
		}
		items = append(items, models.AiMarkItem{Type: kv[0], ID: uint(id)})
	}
	utils.Success(c, gin.H{"marks": models.GetAiContentMarks(items)})
}
