package models

import "gorm.io/gorm"

// 跨域解耦钩子：根包只持有函数变量、不回引域子包，避免「根包被所有子包 import」
// 造成的「根↔域」导入循环（与 db.go 中 BlogCommentNotifier 同一机制）。
//
// 仍留在根包的实现保留默认值，保证独立测试二进制行为不变；已下沉到
// models/softdel 的实现由该子包 init() 覆盖（见 softdel/hooks.go），
// 未加载该子包时为 nil，AutoMigrateAll 会跳过对应辅助步骤。
// 因此 main.go 与 zz_domain_import_test.go 均需 blank import 该子包。
var (
	// AutoMigrateAll 使用的迁移辅助
	DropPostDeletedAtHook         func(db *gorm.DB) error // softdel/posts_migrate.go
	DropLegacyIndexesHook         func(db *gorm.DB) error // softdel/posts_migrate.go
	BackfillCommentFloorsHook     func(db *gorm.DB)       = BackfillCommentFloors
	BackfillPostFloorsHook        func(db *gorm.DB)       = BackfillPostFloors
	BackfillUserHTMLExtractedHook func(db *gorm.DB)       // softdel/userframe.go

	// 评论 / 论坛 → 站内通知；posts 迁移 → 软删除留证
	PushNotificationHook  func(actorID, targetUserID uint, notifType, title, content, link string) error                                       = PushNotification
	ArchiveSoftDeleteHook func(source, ctype string, originalID, deletedBy uint, operator, reason, summary string, snapshot interface{}) error // softdel/softdel.go
)
