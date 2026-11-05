package models

import "gorm.io/gorm"

// 跨域解耦钩子：下列实现后续将随各自业务域下沉到子包。根包只持有函数变量、
// 不回引域子包，避免「根包被所有子包 import」造成的「根↔域」导入循环
// （与 db.go 中 BlogCommentNotifier / BlogFeedProvider 同一机制）。
//
// 默认值指向当前根包实现，保证未加载任何域子包时（例如独立测试二进制）
// 行为与改造前完全一致；域子包可在 init() 中覆盖为自己的实现。
var (
	// AutoMigrateAll 使用的迁移辅助（实现位于 posts_migrate / comment / forum / userframe）
	DropPostDeletedAtHook         func(db *gorm.DB) error = DropPostDeletedAt
	DropLegacyIndexesHook         func(db *gorm.DB) error = DropLegacyIndexes
	BackfillCommentFloorsHook     func(db *gorm.DB)       = BackfillCommentFloors
	BackfillPostFloorsHook        func(db *gorm.DB)       = BackfillPostFloors
	BackfillUserHTMLExtractedHook func(db *gorm.DB)       = BackfillUserHTMLExtracted

	// 评论 / 论坛 → 站内通知；posts 迁移 → 软删除留证
	PushNotificationHook  func(actorID, targetUserID uint, notifType, title, content, link string) error                                       = PushNotification
	ArchiveSoftDeleteHook func(source, ctype string, originalID, deletedBy uint, operator, reason, summary string, snapshot interface{}) error = ArchiveSoftDelete
)
