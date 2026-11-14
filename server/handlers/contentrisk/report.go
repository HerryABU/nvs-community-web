// Package contentrisk 内容风控上报：命中编码混淆/PII/异常外链时，
// 异步送 AI 审核 + 通知管理员。**不外搜**（不调用任何搜索/外部检索）。
package contentrisk

import (
	"fmt"

	"nvs-server/contentguard"
	"nvs-server/models"
	"nvs-server/moderation"

	"github.com/gin-gonic/gin"
)

// Report 上报风险内容（异步，不阻塞发布链路）。
// scene 用于 AI 审核 scope（comment/forum/blog/novel）。
func Report(c *gin.Context, d contentguard.Decision, userID uint, raw, scene string, targetID uint) {
	text := d.Decoded
	if text == "" {
		text = raw
	}
	// 异步 AI 审核（不走计费，复用监管模型 B）
	go func() {
		_, _ = moderation.ReviewText(scene, text, false, 0)
	}()

	// 异步通知管理员（站内信）
	go func() {
		var admins []models.User
		_ = models.DB.Where("role IN ?", []string{"admin", "police"}).Find(&admins).Error
		link := fmt.Sprintf("/admin/moderation?scene=%s&id=%d", scene, targetID)
		for _, a := range admins {
			_ = models.PushNotification(0, a.ID, "content_risk", "内容风控预警",
				fmt.Sprintf("用户 #%d 发布内容命中：%s", userID, d.Reason), link)
		}
	}()
}
