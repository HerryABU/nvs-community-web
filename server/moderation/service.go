package moderation

import (
	"gorm.io/gorm"
	"nvs-server/models"
)

// Migrate 建表（main 里 AutoMigrateAll 之后调用，收 *gorm.DB）。
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&ModerationRecord{}, &ModerationConfig{}, &MaliciousReporter{}, &HiddenContent{})
}

// Init 初始化：确保配置行存在并启动定时批量审核调度器。
// 调度器失败不应阻断主流程。
func Init() {
	if _, err := GetConfig(); err != nil {
		// 配置读取失败仅记录，不影响启动
		_ = err
	}
	StartScheduler()
}

// ReviewOnWrite 发布/提交时送检（小说/博客）。
// 由 chapter / blog 发布 handler 在内容落库后调用。开关来自配置 NovelBlogOnWrite。
func ReviewOnWrite(scope string, targetID uint) {
	cfg, err := GetConfig()
	if err != nil || !cfg.Enabled || !cfg.NovelBlogOnWrite {
		return
	}
	content, ok := ResolveContent(scope, targetID)
	if !ok {
		return
	}
	res, e := ReviewText(scope, content, false, 0)
	if e != nil {
		return
	}
	_ = CreateRecord(&ModerationRecord{
		Scope: scope, TargetType: scope, TargetID: targetID, Trigger: TriggerWrite,
		ModelID: res.ModelID, ModelName: res.ModelName, Verdict: res.Verdict,
		Compliant: res.Verdict != VerdictFail, Reason: res.Reason, Categories: joinJSON(res.Categories),
	})
}

// IsHidden 判断某内容是否对指定用户隐藏（恶意举报者可见性控制）。
func IsHidden(userID uint, targetType string, targetID uint) bool {
	if userID == 0 {
		return false
	}
	var n int64
	models.DB.Model(&HiddenContent{}).
		Where("user_id = ? AND target_type = ? AND target_id = ?", userID, targetType, targetID).
		Count(&n)
	return n > 0
}
